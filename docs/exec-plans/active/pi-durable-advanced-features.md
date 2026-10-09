# 执行计划 · Pi Agent 高级持久化特性全量落地 (Pi Durable Advanced Features)

> 状态：**COMPLETED ✅** · 阶段：**Phase D / M9** · 对应规范：`AI-STD-003` / `ADR-004` / `ADR-007`  
> 责任 Agent：Lead Orchestrator (Antigravity) + Spec Architect + Fullstack Builder + QA Auditor  
> 目标：按顺序逐一落地参考自 Pi Agent（@earendil-works/pi-durable）的 4 大高级架构特性：  
> 1. 人类实时干预与驾驶 (Steering)  
> 2. 多端协同实时观测 (Live Watchers)  
> 3. 会话时光机与路径分叉 (Time-Travel & Forks)  
> 4. 树状子智能体级联生命周期 (Subagents Tree Lifecycle)  

---

## 一、 整体架构蓝图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            HomeAgent 调度中枢                               │
│                                                                             │
│  ┌───────────────────────┐                 ┌─────────────────────────────┐  │
│  │   1. Steering (驾驶)   │                 │   2. Live Watchers (广播)   │  │
│  │ POST /runs/{id}/steer │                 │ GET /runs/{id}/events (SSE) │  │
│  │ POST /runs/{id}/cancel│                 │ 任何客户端并发接入/断线重温   │  │
│  │ 运行中打断注入新指令  │                 │ 内存 Pub/Sub 广播事件       │  │
│  └──────────┬────────────┘                 └──────────────▲──────────────┘  │
│             │                                             │                 │
│             ▼                                             │ broadcast       │
│  ┌────────────────────────────────────────────────────────┴──────────────┐  │
│  │                     Runtime 核心状态机 (executeLoop)                  │  │
│  │          检查点 checkpoint() ──> 写入 RunEvent ──> 广播事件           │  │
│  └──────────┬────────────────────────────────────────────────────────────┘  │
│             │                                                               │
│             ├─────────────────────────────┬─────────────────────────────────┤
│             ▼                             ▼                                 │
│  ┌───────────────────────┐   ┌───────────────────────────────────────────┐  │
│  │  3. Time-Travel/Forks │   │       4. Subagent Trees (子智能体树)       │  │
│  │ POST /conv/{id}/fork  │   │ parent_run_id / root_run_id 树状引用       │  │
│  │ 历史消息指定节点分叉  │   │ 父任务取消级联取消子任务                   │  │
│  │ 开启平行对话世界      │   │ 子任务事件冒泡至父 Run                     │  │
│  └───────────────────────┘   └───────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 二、 任务执行与验收清单 (全量通过 ✅)

### 特性 1 · 人类实时干预与驾驶 (Steering) ✅
- **T-PI-01** [契约] ✅ `api/openapi.yaml` 增加 `POST /runs/{runId}/steer` 和 `POST /runs/{runId}/cancel`，运行 `codegen.ps1` 同步类型。
- **T-PI-02** [内存管控] ✅ 在 `internal/agent/runtime` 实现 `Hub`（并发安全的活跃 Run 注册器），维护每个 Run 的 `cancelFunc` 与 `steerChan`。
- **T-PI-03** [循环注入] ✅ 在 `runtime.executeLoop` 中每一轮迭代工具执行后检查 `steerChan`：若有人类干预指令，记录 `EventHumanSteered` 检查点并作为 user 消息注入上下文；若收到取消信号则原子中止，记录 `EventRunCancelled`。
- **T-PI-04** [接口与领域] ✅ `internal/api/v1/runs.go` 实现 Steer 与 Cancel 端点，对接领域层 `run.Service` 并触发运行时 Hub。
- **T-PI-05** [单元测试] ✅ 编写 `advanced_features_test.go` 与 `runs_test.go`，覆盖“执行中注入干预指令”与“执行中紧急取消”。

### 特性 2 · 多端协同实时观测 (Live Watchers) ✅
- **T-PI-06** [契约] ✅ `api/openapi.yaml` 增加 `GET /runs/{runId}/events` (text/event-stream)，同步双端契约。
- **T-PI-07** [广播总线] ✅ 在 `internal/agent/runtime/hub.go` 实现非阻塞广播 `Subscribe(runID)` 与 `Broadcast(runID, event)`，检查点 `checkpoint()` 自动推送到订阅者。
- **T-PI-08** [流式端点] ✅ 在 `api/v1/runs.go` 实现 `WatchRunEvents`：连接建立时先回放所有历史事件（防止客户端重连或慢入漏看），接着实时流式推送新产生的事件直到任务终结，并带 15s 心跳保活。
- **T-PI-09** [单元测试] ✅ 编写 `advanced_features_test.go` 与 `runs_test.go`，验证多客户端并发订阅同个任务，事件无丢包、无阻塞。

### 特性 3 · 会话时光机与路径分叉 (Time-Travel & Forks) ✅
- **T-PI-10** [契约] ✅ `api/openapi.yaml` 增加 `POST /conversations/{conversationId}/fork`，请求体包含 `fork_at_message_id`，同步双端契约。
- **T-PI-11** [领域服务] ✅ 在 `internal/agent/session` 实现 `Fork` 方法：截取原会话截至指定消息的历史记录，创建新分支会话并克隆历史。
- **T-PI-12** [仓储层] ✅ 在 `internal/store/repo/conversation.go` 实现 `ForkConversation`，支持事务性历史截取克隆与 `forked_from_id` / `forked_at_message_id` 标记。
- **T-PI-13** [API 端点] ✅ 在 `internal/api/v1/conversations.go` 暴露 `ForkConversation` 端点，在 `router.go` 注册。
- **T-PI-14** [单元测试] ✅ 编写 `conversation_test.go` 与 `conversations_test.go`，验证分支会话独立演进、消息截取精准且互不污染。

### 特性 4 · 树状子智能体级联生命周期 (Subagents Tree Lifecycle) ✅
- **T-PI-15** [模型扩展] ✅ `schema/run.go` 的 `AgentRun` 实体增加 `parent_run_id` 和 `root_run_id` 字段，执行 `codegen.ps1`。
- **T-PI-16** [领域层] ✅ `internal/domain/run` 增加子任务派生方法 `CreateChildRun`，自动继承 `root_run_id`，并记录 `child_run_spawned` 事件。
- **T-PI-17** [级联取消] ✅ 在 `service.CancelRun` 中自动查找所有直接/间接活跃子任务，执行级联取消；完成状态保持不变。
- **T-PI-18** [仓储支持] ✅ 在 `internal/store/repo/run.go` 实现 `CreateChildRun` 与 `FindChildRuns`。
- **T-PI-19** [单元测试] ✅ 编写 `service_test.go` 与 `conversation_test.go`，验证父任务派生子孙任务、树状引用正确及父任务取消时子任务级联中断。
- **T-PI-20** [全量门禁] ✅ 运行全仓编译、测试与类型检查，全部 100% 通过：
  - `go test ./...`: 全部包通过（API、Domain、Runtime、Store、Repo、Auth 等）
  - `go build ./cmd/homeagent`: 二进制顺利生成
  - `pnpm -r run typecheck`: 前端三端（packages/shared, web, admin）零报错

---

## 三、 收益与使用方式速览

1. **人类实时干预 (Steering)**：
   - 接口：`POST /api/v1/runs/{runId}/steer {"instruction": "先停一下，改去洗衣服"}`
   - 效果：即使大模型当前还在循环执行多步任务，该指令会在当前步骤完成后立即被注入作为人类补充要求，纠正下一步行动路线。
2. **多端协同观测 (Live Watchers)**：
   - 接口：`GET /api/v1/runs/{runId}/events` (SSE)
   - 效果：手机端、iPad端、Web管理端可同时监听同一任务执行，刚连入的客户端会自动获得历史所有检查点回放，随后无缝接驳实时推送。
3. **会话时光机与路径分叉 (Time-Travel & Forks)**：
   - 接口：`POST /api/v1/conversations/{conversationId}/fork {"fork_at_message_id": "...", "title": "新分支"}`
   - 效果：对先前对话某步回答不满意时，可直接从该消息处分叉出平行对话分支，原对话完整保留不受任何影响。
4. **树状子任务生命周期 (Subagents Tree)**：
   - 接口：`CreateChildRun` 建立子/孙任务，关联 `parent_run_id` 与 `root_run_id`。
   - 效果：父级任务一旦被用户紧急取消，下属所有进行中的子智能体与硬件设备任务级联全部停止，不会出现遗留后台孤儿任务继续执行的情况。
