# 执行计划 · 持久化运行时检查点与崩溃恢复机制 (Durable Runtime Checkpoint)

> 状态：**COMPLETED ✅** · 阶段：**Phase D / M8** · 对应规范：`AI-STD-003` / `ADR-004` / `ADR-007`  
> 责任 Agent：Gemini 3.8 Flash Max  
> 设计灵感：Pi Durable（@earendil-works/pi-durable）核心理念在 Go 原生架构的自主实现  
> 前置依赖：调度中心 B0 阶段已完成（AgentRun / RunStep / RunEvent 持久化建模 ✅）

---

## 一、 核心目标

将 `internal/agent/runtime` 的 ReAct 循环从**进程内短暂循环**升级为**持久检查点状态机**：

1. 每轮 LLM 响应和工具执行前后写检查点到 `RunEvent`
2. 工具按 `Replayable` 语义分级——安全工具崩溃后自动重放，危险工具报告中断
3. 进程重启后扫描未完成的 `AgentRun(status=running)` 并从断点恢复
4. 历史消息超阈值时自动摘要压缩，防止上下文窗口爆炸

---

## 二、 架构全景

```
进程启动
    │
    ▼
┌──────────────────────────────────────────────────────────────┐
│ main.go: startRecovery()                                      │
│   └── 扫描 AgentRun WHERE status = 'running'                  │
│       └── 对每个未完成的 Run:                                   │
│           ├── 加载最后一条 RunEvent 检查点                       │
│           ├── 读取 RunStep 列表与完成状态                        │
│           ├── 恢复 LLM 上下文（已完成步骤的结果 + 未完成步骤意图）│
│           └── 继续 runtime.RunFromCheckpoint()                  │
└──────────────────────────────────────────────────────────────┘

正常请求流
    │
    ▼
┌──────────────────────────────────────────────────────────────┐
│ runtime.Run()                                                │
│                                                              │
│  ① 创建 AgentRun(status=running)                             │
│  ② for turn := 1; !stopped; turn++ {                         │
│       ✎ checkpoint("llm_request_sent", turn, msgs_hash)      │
│       调 LLM → 拿到 response                                 │
│       ✎ checkpoint("llm_response_received", turn, text+calls)│
│       if len(toolCalls) > 0 {                                │
│         for _, tc := range toolCalls {                        │
│           ✎ checkpoint("tool_intent", tc.name, tc.args)       │
│           执行工具                                             │
│           ✎ checkpoint("tool_result", tc.name, result/error)  │
│         }                                                     │
│       }                                                       │
│     }                                                         │
│  ③ UpdateRunStatus → completed                                │
│  ④ 持久化消息 + emit done                                     │
└──────────────────────────────────────────────────────────────┘
```

---

## 三、 任务分解表 (WBS)

### Phase 1 · 工具重放语义标记（0.5 天）

| 编号 | 任务名称 | 交付产物 | 验收条件 | 状态 |
|------|---------|---------|---------|------|
| T-DRC-01 | `tool.Spec` 新增 `Replayable bool` 字段 | `internal/agent/tool/tool.go` | 编译通过；字段存在且默认 false | ✅ |
| T-DRC-02 | 标记所有现有工具的重放安全性 | 各工具文件（`record_expense.go` 等） | 查询类工具（`query_budget`、`suggest_dinner`、`list_models`、`list_my_tasks` 等）标为 `Replayable: true`；写操作保持 `false` | ✅ |
| T-DRC-03 | `tool.Executor` 新增 `IsReplayable(name string) bool` 方法 | `internal/agent/tool/executor.go` | 单测验证：传入 safe 工具名返回 true，unsafe 返回 false | ✅ |

**代码定位：**
- [`tool.go`](file:///C:/Users/ZhuanZ/orca/workspaces/ai_agent/gramma/internal/agent/tool/tool.go) L30-40：`Spec` 结构体，在 `Idempotency` 后新增 `Replayable bool`
- 各工具注册处在 `internal/domain/expense/`、`internal/domain/travel/`、`internal/domain/run/` 等目录

---

### Phase 2 · ReAct 循环检查点注入（2 天）

| 编号 | 任务名称 | 交付产物 | 验收条件 | 状态 |
|------|---------|---------|---------|------|
| T-DRC-04 | 定义检查点事件类型常量 | `internal/agent/runtime/checkpoint.go`（新文件） | 定义 `EventLLMRequestSent`、`EventLLMResponseReceived`、`EventToolIntent`、`EventToolResult`、`EventTurnCompleted` 字符串常量 | ✅ |
| T-DRC-05 | 实现 `checkpoint()` 函数 | `internal/agent/runtime/checkpoint.go` | 函数签名 `func checkpoint(ctx, runSvc, runID, stepID, eventType, payload) error`；内部调用 `runSvc.RecordRunEvent()` | ✅ |
| T-DRC-06 | `runtime.Run()` 入口创建 `AgentRun` | `internal/agent/runtime/runtime.go` L75 附近 | 在 ReAct 循环开始前调用 `runSvc.CreateRun()`；`request_id` 幂等防重——同一 `RequestID` 若已有 `running` 的 Run 则复用而非创建 | ✅ |
| T-DRC-07 | ReAct 循环中注入 4 个检查点 | `internal/agent/runtime/runtime.go` 循环体内 | 在 LLM 调用前、LLM 响应后、工具执行前、工具执行后各插入一条 `checkpoint()` 调用 | ✅ |
| T-DRC-08 | 循环正常结束时更新 Run 状态为 `completed` | `runtime.go` 循环结束后 | 调用 `runSvc.UpdateRunStatus(runID, version, "completed")`；若循环因 error/熔断终止则写 `failed` | ✅ |
| T-DRC-09 | `Runtime` 结构体新增 `RunService` 依赖 | `runtime.go` L43-58 | `Options` 增加 `RunService run.Service`，`Runtime` 增加 `runSvc` 字段，`main.go` 装配时注入 | ✅ |
| T-DRC-10 | 单元测试：验证检查点序列完整性 | `internal/agent/runtime/runtime_test.go` | mock RunService，验证一次正常 2-turn 对话后 RunEvent 序列为：`llm_request_sent → llm_response_received → tool_intent → tool_result → llm_request_sent → llm_response_received → turn_completed` | ✅ |

**代码定位：**
- [`runtime.go`](file:///C:/Users/ZhuanZ/orca/workspaces/ai_agent/gramma/internal/agent/runtime/runtime.go) L74-267：整个 `Run()` 方法
- [`service.go`](file:///C:/Users/ZhuanZ/orca/workspaces/ai_agent/gramma/internal/domain/run/service.go)：`run.Service` 接口
- [`run.go schema`](file:///C:/Users/ZhuanZ/orca/workspaces/ai_agent/gramma/internal/store/ent/schema/run.go) L137-161：`RunEvent` 已有 `event_type` + `payload` 字段可直接复用

---

### Phase 3 · 进程重启崩溃恢复（1.5 天）

| 编号 | 任务名称 | 交付产物 | 验收条件 | 状态 |
|------|---------|---------|---------|------|
| T-DRC-11 | 实现 `RecoverInterruptedRuns()` 函数 | `internal/agent/runtime/recovery.go`（新文件） | 扫描 `AgentRun WHERE status IN ('running', 'awaiting_input')` 且 `updated_at < now()-60s`（防止抢走正在执行的 Run） | ✅ |
| T-DRC-12 | 从最后检查点重建 LLM 上下文 | `recovery.go` | 根据 RunEvent 序列重建 `[]gateway.Message`：已完成的 tool_result 转为 tool message，未完成的 tool_intent 根据 Replayable 决定重放还是报告 | ✅ |
| T-DRC-13 | 实现 `RunFromCheckpoint()` | `runtime.go`（新方法）或复用 `Run()` | 接受重建的上下文 + 未完成的 RunID，跳过已完成步骤，从断点继续 ReAct 循环 | ✅ |
| T-DRC-14 | `main.go` 启动时调用恢复逻辑 | `cmd/homeagent/main.go` | 在 HTTP 服务启动**之后**、以 goroutine 异步执行恢复；恢复的 Run 通过现有 SSE 通道推送进度（如果客户端已重连） | ✅ |
| T-DRC-15 | 恢复时 unsafe 工具中断处理 | `recovery.go` | 若最后检查点是 `tool_intent` 且工具 `Replayable=false`，将该步骤标为 `unknown`，向 LLM 发送 `"上一次执行在工具 {name} 执行中被中断，无法确认执行结果，请决定下一步"` | ✅ |
| T-DRC-16 | 单元测试：模拟崩溃恢复场景 | `internal/agent/runtime/recovery_test.go` | 场景 A：第 2 个工具执行中崩溃（safe 工具）→ 恢复后自动重放；场景 B：unsafe 工具崩溃 → 恢复后报告中断给 LLM | ✅ |

**代码定位：**
- [`main.go`](file:///C:/Users/ZhuanZ/orca/workspaces/ai_agent/gramma/cmd/homeagent/main.go)：进程入口，装配处
- [`repo/run.go`](file:///C:/Users/ZhuanZ/orca/workspaces/ai_agent/gramma/internal/store/repo/run.go)：`ListRuns(status="running")` 已有实现可直接用

---

### Phase 4 · 上下文 Compaction 摘要压缩（2 天）

| 编号 | 任务名称 | 交付产物 | 验收条件 | 状态 |
|------|---------|---------|---------|------|
| T-DRC-17 | 定义 Compaction 策略与阈值 | `internal/agent/session/compaction.go`（新文件） | 常量 `CompactThreshold = 50`（消息数）、`RetainRecent = 10`（保留最近 N 轮原文） | ✅ |
| T-DRC-18 | 实现摘要生成与替换 | `compaction.go` | 将 threshold 之前的消息拼接为单条 prompt → 调 LLM 生成 ≤500 字摘要 → 替换为一条 `{role: system, content: "[会话摘要] ..."}` + 保留最近 RetainRecent 条原文 | ✅ |
| T-DRC-19 | 在 `runtime.Run()` 入口处触发压缩检查 | `runtime.go` L93 之后 | `if len(history) > CompactThreshold { history = compact(ctx, history) }`；压缩结果持久化回 conversation | ✅ |

**代码定位：**
- [`session.go`](file:///C:/Users/ZhuanZ/orca/workspaces/ai_agent/gramma/internal/agent/session/session.go) L52：`LoadMessages` 返回全量历史，Compaction 在其上层做

---

## 四、 不变量与约束

1. **检查点不阻塞 SSE 推送**：`checkpoint()` 失败时 `slog.Error` 记录但不中断循环——降级为"无检查点"而非崩溃
2. **幂等 request_id 复用**：同一 `request_id` 若已有 `completed` 的 Run，直接返回结果不重复执行
3. **恢复窗口 60 秒**：仅恢复 `updated_at` 超过 60 秒的 running Run，防止正常执行中的 Run 被抢
4. **Compaction 不删原文**：压缩只影响传给 LLM 的上下文窗口，原始消息在 message 表中永久保留（审计需要）
5. **现有接口零破坏**：`tool.Tool` 和 `tool.WriteTool` 接口不变，`Replayable` 是 `Spec` 的新可选字段

---

## 五、 执行指令（面向 Gemini 3.8 Flash Max）

### 环境准备
```powershell
# 每次新 shell 必须执行
$env:Path = "$env:SCOOP\apps\go\current\bin;$env:PATH"
$env:GOPROXY = 'https://goproxy.cn,direct'

# 清理残留进程
Get-Process homeagent -ErrorAction SilentlyContinue | Stop-Process -Force
```

### 执行顺序
1. **按 Phase 1 → 2 → 3 → 4 严格顺序**，每个 Phase 完成后必须通过质量门禁
2. **每个 Phase 完成后运行**：
   ```powershell
   $env:Path = "$env:SCOOP\apps\go\current\bin;$env:PATH"; $env:GOPROXY = 'https://goproxy.cn,direct'
   go build ./cmd/homeagent
   go test ./...
   pnpm -r run typecheck
   ```
3. **所有 Phase 完成后运行全量门禁**：
   ```powershell
   powershell -ExecutionPolicy Bypass -File .\scripts\verify-all.ps1 -SkipE2E
   ```

### 必读文件
- `AGENTS.md`：项目不变量（契约先行、金额 int64 分、写操作可撤销、权限双保险）
- `internal/agent/runtime/runtime.go`：当前 ReAct 循环完整实现
- `internal/agent/tool/tool.go`：Spec / Tool / WriteTool 接口定义
- `internal/domain/run/service.go` + `internal/store/repo/run.go`：RunService 仓储层
- `internal/store/ent/schema/run.go`：AgentRun / RunStep / RunEvent 数据模型
- `internal/agent/session/session.go`：会话与消息持久化

### 禁止事项
- ❌ 不得修改 `*.gen.go` 生成文件
- ❌ 不得修改 `api/openapi.yaml`（本任务无 API 契约变更）
- ❌ 不得引入新的外部依赖（纯 Go 标准库 + 已有 ent/go-openai）
- ❌ 不得删改已有单元测试
- ❌ PowerShell 禁用 `Set-Content` 写中文文件（用 write 工具）

---

## 六、 验收矩阵

| 编号 | 验收项 | 验证方式 |
|------|-------|---------|
| V-01 | `tool.Spec.Replayable` 字段存在且查询类工具为 true | `go vet ./internal/agent/tool/...` + 代码审查 |
| V-02 | 正常对话在 `RunEvent` 中产生完整检查点序列 | `runtime_test.go` 单测 |
| V-03 | safe 工具崩溃后自动重放 | `recovery_test.go` 场景 A |
| V-04 | unsafe 工具崩溃后报告中断而非重放 | `recovery_test.go` 场景 B |
| V-05 | 进程重启后 running 状态的 Run 被自动恢复 | `recovery_test.go` 集成验证 |
| V-06 | 历史消息 > 50 条时自动摘要压缩 | `compaction_test.go` 或手动验证 |
| V-07 | 全量质量门禁通过 | `go test ./...` 全 PASS + `pnpm -r run typecheck` 0 错误 |
