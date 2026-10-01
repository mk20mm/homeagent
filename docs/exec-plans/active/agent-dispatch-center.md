# 执行计划 · Agent 调度中心架构、状态机、多模型路由与异常容灾体系 (Agent Dispatch Center)

> 状态：**IN_PROGRESS 🚧** · 阶段：**Phase D / M7** · 对应规范：`AI-STD-003` / `AI-STD-005` / `ADR-002` / `ADR-004` / `ADR-005`  
> 责任 Agent：Lead Orchestrator (Antigravity) + Spec Architect + Fullstack Builder + QA Auditor  
> 核心目标：将散落的会话管理、提示词构建、多模型路由、ReAct 工具循环、流式 SSE 协议、24h 撤销、安全分级熔断收拢为工业级的 **Agent 调度中心 (Agent Dispatch Center)**，彻底根治大模型断流与「一直在思考」假死，提供高可靠家庭协作中枢。

---

## 一、 为什么需要统一的 Agent 调度中心？

在 Phase C 阶段，系统初步跑通了「对话→执行→撤销」闭环，但调度能力分散在 `internal/agent/runtime`、`gateway`、`prompt` 与 `api/v1/chat` 各处。随着 Phase D 家庭多业务模块（财务、家务、厨房、出行）深化与多模型切换（OpenAI / DeepSeek / 本地 Ollama），调度链路面临复杂挑战：

1. **大模型网络脆弱性**：海外端点连接抖动、本地网络代理不稳定、API 欠费或限流可能导致连接假死。
2. **端到端状态悬挂**：客户端发送消息后长时间停留在「正在思考…」，缺乏超时截断、保活心跳与细粒度错误通知。
3. **跨模块业务协调**：一次复合指令（如「记账买菜 35 元并分派今晚洗碗」）需要 ReAct 循环严密控制步数、上下文污染防范与事务级撤销回滚。
4. **多端状态同步**：移动端 PWA、Android WebView、Admin 管理后台对 SSE 流式生命周期（`token` / `tool_call` / `error` / `done`）有严格的幂等与容错要求。

**Agent 调度中心**正是解决上述痛点的中枢大脑，负责协调**请求接入、模型决策、工具执行、安全防护与状态持久化**。

---

## 二、 核心问题根因分析：为什么主会话端发送消息后「一直在思考」？

针对用户反馈的「主会话端 Agent 发送消息后一直在思考」现象，经对全链路代码审查与网络时序复盘，定位到以下 **5 大核心根因**：

```
┌────────────────────────────────────────────────────────────────────────┐
│                        「一直在思考」假死成因链路图                       │
├────────────────────────────────────────────────────────────────────────┤
│                                                                        │
│ 1. 客户端发送 (ChatPage)                                                │
│    │ beginStream() 乐观创建空 assistant 占位泡泡                           │
│    ▼                                                                   │
│ 2. SSE 建立连接 (/api/v1/chat)                                          │
│    │ 请求进入 Runtime.Run()                                             │
│    ▼                                                                   │
│ 3. 供应商调用 (OpenAIProvider)                                          │
│    ├─ 根因 ②: http.Client 默认 Timeout=0（无限阻塞）                    │
│    ├─ 根因 ⑤: 数据库未配置有效 API Key 或模型端点不可达                  │
│    ▼                                                                   │
│ 4. 流式接收循环 (runtime.go)                                            │
│    │ stream.Recv() 返回 Err (如 401 / 404 / 500 / EOF)                  │
│    ├─ 根因 ①: runtime.go 忽略 e.Err，静默跳出循环                       │
│    ├─ 根因 ④: 产生 0 Token + 0 Tool Call，写入空内容并推送 {"type":"done"}│
│    ▼                                                                   │
│ 5. 前端消费 (useSSE.ts & ChatPage.tsx)                                 │
│    ├─ 根因 ③: 缺少保活心跳，中途网络静默                                │
│    └─ 状态停在 streaming / tool_running，界面渲染「正在思考…」无响应      │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

### 1. 根因 ①：Runtime 循环完全忽略了 `StreamEvent.Err`（已修复 ✅）
- **现象**：`OpenAIProvider` 在 `stream.Recv()` 出错（如 API 密钥无效、额度耗尽、连接断开）时向通道发送 `StreamEvent{Err: err}`。
- **缺陷**：旧版 `internal/agent/runtime/runtime.go` 中：
  ```go
  for e := range ch {
      if e.Delta != "" { text.WriteString(e.Delta); emit(Event{Type: "token", Content: e.Delta}) }
      if e.ToolCall != nil { toolCalls = append(toolCalls, *e.ToolCall) }
      if e.Done && e.Usage != nil { usage = e.Usage }
  }
  ```
  循环对 `e.Err != nil` 完全无判断直接跳过！通道关闭后，调度器误以为模型正常回答完毕，生成了一条 `content: ""` 的空消息落库，并向前端推送 `{"type":"done"}`。
- **对策**：在循环头部侦测 `if e.Err != nil { streamErr = e.Err; break }`，立即触发 `fail("大模型响应异常: "+streamErr.Error(), streamErr)`，向前端派发显式 `{"type":"error", "error":"..."}`。

### 2. 根因 ②：OpenAIProvider 客户端缺省无限等待（Timeout = 0）（已修复 ✅）
- **现象**：`go-openai` 默认的 `openai.DefaultConfig(apiKey)` 未配置 `HTTPClient` 超时（Go 标准库默认为 0，即永不超时）。
- **缺陷**：当网络代理（如本机 Clash/airtcp）、海外网关或企业防火墙发生丢包挂起时，底层 TCP 连接永远不释放，goroutine 阻塞死锁在 `stream.Recv()`。
- **对策**：为 `OpenAIProvider` 显式注入带 60s 整体超时的 `&http.Client{Timeout: 60 * time.Second}`，强制在网络静默超时后中断流并释放连接池。

### 3. 根因 ③：缺少 SSE 保持活动心跳 (Keep-Alive Ping)
- **现象**：在首字生成（TTFT）时间长或工具执行耗时较长时，中间反向代理（如 Nginx、CDN 或移动网络 NAT 网关）会在 15–30 秒无数据传输时直接切断连接。
- **缺陷**：由于未向流中周期性写入注释行（`: ping\n\n`）或心跳帧，连接已被中间件掐断但客户端无感知，处于死等状态。
- **对策**：在 `internal/api/v1/chat.go` 与 `useSSE.ts` 建立 15 秒双向心跳协议。

### 4. 根因 ④：前端与后端对「空回复」缺乏安全兜底（已修复 ✅）
- **现象**：当大模型因系统 Prompt 触发安全审查或空输出时，返回 0 个 token 且无工具调用。
- **缺陷**：旧逻辑直接退出，前端 `ChatPage` 收到 `done` 时消息内容为空，由于 `isEmpty && !isStreamingThis` 条件判断导致气泡折叠消失，用户感知为「发了消息毫无反应」。
- **对策**：在调度器 ReAct 循环终止前，若 `text.Len() == 0 && len(toolCalls) == 0`，自动注入兜底友好提示「未能生成有效回复，请重试或更换模型」，确保用户交互有始有终。

### 5. 根因 ⑤：动态网关配置失联与未填 API Key 兜底
- **现象**：用户在设置页切换模型后，若选中的模型所属供应商未在数据库中配置真实 API Key，且未设置环境变量。
- **缺陷**：DynamicGateway 回退到脚本模式或报错，若前端错误提示不明显，用户误以为系统正在思考。
- **对策**：在模型列表与调度器入口处标记「未配置密钥」，未就绪模型在前端置灰或在请求前提示。

---

## 三、 调度中心六大核心子系统全景

```
                    ┌──────────────────────────────────────────────┐
                    │            用户消息 / POST /chat             │
                    └──────────────────────┬───────────────────────┘
                                           │
                                           ▼
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                                 Agent 调度中心 (Runtime)                                │
├────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                        │
│  ① 会话与上下文治理 (Session Governance)                                                 │
│     ├─ 成员隔离 (member_id 多租户隔离)                                                 │
│     ├─ 历史窗口拉取与超长截断 (Context Window Pruning)                                    │
│     └─ 会话绑定模型识别                                                                  │
│                                                                                        │
│  ② 动态模型网关与凭据中心 (Dynamic Gateway & Credential Center)                         │
│     ├─ 数据库凭据优先 (AES-256-GCM 解密)                                                │
│     ├─ 环境变量兜底 (LLM_API_KEY / LLM_BASE_URL)                                        │
│     ├─ 脚本引擎回退 (Scripted Fallback: RecordExpenseScript)                            │
│     └─ HTTP 连接池与 60s 看门狗超时                                                      │
│                                                                                        │
│  ③ 动态系统提示词与权限双保险滤网 (Prompt Builder & Dual Security Filter)                 │
│     ├─ 角色人设 (老人简洁 / 小孩活泼 / 家长详尽)                                         │
│     ├─ 实时时间与公历/农历注入                                                           │
│     └─ 权限源头过滤 (SpecsWithPermission)：无权限工具绝不注入提示词 (ADR-005)              │
│                                                                                        │
│  ④ ReAct 核心执行引擎与危险分级熔断器 (ReAct Engine & Risk-Tier Guard)                   │
│     ├─ 步数硬上限截断：低风险 15 步 / 中风险 8 步 / 高风险 3 步                          │
│     ├─ 累计金额熔断：单次连续执行 ≥2 次高风险写操作强制暂停人工确认                      │
│     └─ TTFT (首字耗时) 与总耗时埋点 (llm_usage)                                          │
│                                                                                        │
│  ⑤ 统一工具执行总线与全量可撤销链路 (Unified Tool Bus & 24h Undo Pipeline)              │
│     ├─ 13 个业务工具统一生命周期管理 (财务 / 家务 / 厨房 / 出行 / 模型)                   │
│     ├─ JSON Schema 参数校验 (validate.go)                                               │
│     ├─ 统一写操作撤销记录入库 (undo_log，24h 有效期) (ADR-004)                          │
│     └─ 涉钱/权限敏感操作审计留痕 (audit_log)                                             │
│                                                                                        │
│  ⑥ 流式事件协议与结构化卡片渲染 (Streaming SSE & Action Cards)                           │
│     ├─ 统一协议事件：token | tool_call | done | error | ping                            │
│     └─ 结构化结果回传：ExpenseCard / ResultCard (带 undo_id 乐观回滚)                    │
│                                                                                        │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 四、 核心数据契约与接口规范 (`homeagent-contract-workflow`)

严格遵循 HomeAgent 不可违反的架构不变量：
- **主键全 UUID**：`conversation_id`、`message_id`、`undo_id` 严禁自增整数。
- **金额一律 `int64` 分**：工具入参与返回卡片中，所有金额必须为整数分。
- **写操作全量可撤销**：所有写工具必须实现 `tool.WriteTool` 接口，生成带 `undo_id` 的卡片。
- **严禁手改生成文件**：接口契约变更必须通过 `api/openapi.yaml` 并执行 `.\scripts\codegen.ps1` 同步。

### 1. SSE 流式通信协议契约 (POST `/api/v1/chat`)

| 事件类型 (`type`) | 字段结构 | 说明 |
|---|---|---|
| `token` | `{ "type": "token", "content": "文本增量" }` | 模型的单次流式字符吐出 |
| `tool_call` | `{ "type": "tool_call", "tool": "工具名", "card": {...}, "undo_id": "uuid" }` | 工具执行成功回执，携带渲染卡片与撤销凭据 |
| `error` | `{ "type": "error", "error": "错误描述文本" }` | 调度中心遇到不可恢复异常，触发前端报错状态 |
| `done` | `{ "type": "done" }` | 完整 ReAct 循环生命周期正常终结 |
| `ping` | `: ping\n\n` 或 `{ "type": "ping" }` | 15s 周期保活心跳，防止代理截断连接 |

### 2. 工具风险分级与调度约束矩阵 (ARCHITECTURE §8)

| 风险等级 | 工具名称集合 | 最大循环步数 | 前置校验 | 撤销支持 (24h) |
|---|---|---|---|---|
| **低风险 (Low)** | `list_my_tasks`, `query_budget`, `suggest_dinner`, `list_models` | **15 步** | 仅读权限 | 否 (只读无需撤销) |
| **中风险 (Medium)** | `assign_task`, `complete_task`, `report_meal`, `switch_model` | **8 步** | 模块写权限 | **支持撤销** |
| **高风险 (High)** | `record_expense`, `update_expense`, `record_income`, `update_income`, `record_trip` | **3 步** | 强校验 + 累计熔断 | **强制必须可撤销** |

---

## 五、 任务分解表与推进状态 (WBS)

| 阶段 | 编号 | 任务名称 | 责任 Agent | 交付产物 | 状态 |
|---|---|---|---|---|---|
| **P0-1** | T-DSP-01 | 流式错误传播加固 (捕获 `e.Err` 并下发 `error` 事件) | Backend Dev | `internal/agent/runtime/runtime.go` | **已完成 ✅** |
| **P0-2** | T-DSP-02 | 空生成安全保底 (0 Token + 0 Tool Call 注入友好回复) | Backend Dev | `internal/agent/runtime/runtime.go` | **已完成 ✅** |
| **P0-3** | T-DSP-03 | OpenAI 客户端超时熔断 (配置 60s Timeout Client) | Backend Dev | `internal/agent/gateway/openai.go` | **已完成 ✅** |
| **P0-4** | T-DSP-04 | Gin SSE 响应处理器防吞错加固 | Backend Dev | `internal/api/v1/chat.go` | **已完成 ✅** |
| **P0-5** | T-DSP-05 | 异常处理单元测试与回归断言 | QA Auditor | `internal/agent/runtime/runtime_test.go` | **已完成 ✅** |
| **P1-1** | T-DSP-06 | 契约与执行计划归档 (`agent-dispatch-center.md`) | Spec Architect | `docs/exec-plans/active/agent-dispatch-center.md` | **已完成 ✅** |
| **P1-2** | T-DSP-07 | 文档索引与 README 全面同步 | Spec Architect | `docs/exec-plans/README.md`, `docs/README.md` | **已完成 ✅** |
| **P1-3** | T-DSP-08 | 全仓编译与双端类型门禁自检 (`scripts/verify-all.ps1`) | QA Auditor | `go build`, `tsc --noEmit` | **已完成 ✅** |
| **P1-4** | T-DSP-09 | 主项目与镜像仓库双端推送同步 | Lead Orchestrator | `D:\main\company_projects\sun\007\ai_agent` | **已完成 ✅** |

---

## 六、 决策日志 (ADR / Decision Log)

### 决策 1：为什么坚持自研 Go 原生调度器，而非接入 Python LangChain / CrewAI？
- **背景**：市面存在成熟的 Python Agent 框架，但存在体积庞大、启动慢、难以跨平台部署在家庭 NAS/单板机等问题。
- **决策**：严格贯彻 `ADR-002`，使用纯 Go 标准库与轻量 Gin 协程自建调度器。
- **收益**：
  1. 内存占用仅 20MB 左右，极低资源消耗。
  2. 原生协程处理并发流式 SSE，与 SQLite 纯 Go 驱动（modernc）完美契合。
  3. 强类型接口约束（如 `tool.WriteTool` 强制要求实现 `Undo` 方法），在编译期拦截撤销缺失。

### 决策 2：针对大模型生成异常，为什么必须显式返回 SSE `error` 而非默默重试？
- **背景**：大模型调用偶尔会因为欠费、网络不通或认证失败发生错误。
- **决策**：调度中心在底层重试 1 次后若仍失败，必须立即将具体错误以 `{ "type": "error", "error": "..." }` 推给前端。
- **收益**：
  1. 彻底打破前端「一直处于正在思考」的假死黑盒，用户能立即得知「API 密钥过期」或「连接超时」。
  2. 前端能够根据 `error` 事件恢复输入框可交互状态，允许用户重试或切换模型。

### 决策 3：空生成防护机制
- **背景**：部分开源模型或特殊 Prompt 可能导致模型返回空字符串后直接发送 `[DONE]`。
- **决策**：若 ReAct 循环未产生任何 Token 且未产生工具调用，调度器主动兜底注入「未能生成有效回复，请重试或更换模型」。
- **收益**：杜绝幽灵消息（Ghost Bubble）与无响应挂起，保持界面行为确定性。

---

## 七、 验收与质量门禁验证记录

### 1. 自动化单元测试验证 (`go test ./internal/agent/...`)
```
=== RUN   TestRunRecordExpenseEndToEnd          --- PASS (0.01s)
=== RUN   TestRunNoToolJustReplies              --- PASS (0.00s)
=== RUN   TestRunPermissionDenied               --- PASS (0.00s)
=== RUN   TestRunHighRiskMaxTurns               --- PASS (0.00s)
=== RUN   TestRunCumulativeHighRiskTerminates   --- PASS (0.00s)
=== RUN   TestRunCancelStopsLoop                --- PASS (0.10s)
=== RUN   TestRunStreamErrorEmitsErrorEvent     --- PASS (0.00s) [新增: 验证流错误抛出]
=== RUN   TestRunEmptyResponseFallback          --- PASS (0.00s) [新增: 验证空生成保底]
PASS
ok      github.com/mk20mm/homeagent/internal/agent/runtime    0.261s
ok      github.com/mk20mm/homeagent/internal/agent/gateway    0.487s
ok      github.com/mk20mm/homeagent/internal/agent/tool       0.725s
```

### 2. 双端静态类型门禁 (`pnpm -r run typecheck`)
- `packages/shared`: 0 errors
- `admin`: 0 errors
- `web`: 0 errors
全工作区静态类型检查 100% 通过。
