# 执行计划 · Agent 调度中心架构、状态机、多模型路由与异常容灾体系 (Agent Dispatch Center)

> 状态：**IN_PROGRESS 🚧** · 阶段：**Phase D / M7** · 对应规范：`AI-STD-003` / `AI-STD-005` / `ADR-002` / `ADR-004` / `ADR-005`  
> 责任 Agent：Lead Orchestrator (Antigravity) + Spec Architect + Fullstack Builder + QA Auditor  
> 完整设计真相源：[docs/agent-dispatch-center.md](../../agent-dispatch-center.md)

---

## 一、 核心目标与背景

在 Phase C 阶段，系统初步跑通了「对话→执行→撤销」闭环，但调度能力分散在 `internal/agent/runtime`、`gateway`、`prompt` 与 `api/v1/chat` 各处。随着 Phase D 家庭多业务模块（财务、家务、厨房、出行）深化与多模型切换（OpenAI / DeepSeek / 本地 Ollama），调度链路面临复杂挑战：

1. **大模型网络脆弱性**：海外端点连接抖动、本地网络代理不稳定、API 欠费或限流可能导致连接假死。
2. **端到端状态悬挂**：客户端发送消息后长时间停留在「正在思考…」，缺乏超时截断、保活心跳与细粒度错误通知。
3. **跨模块业务协调**：一次复合指令（如「记账买菜 35 元并分派今晚洗碗」）需要 ReAct 循环严密控制步数、上下文污染防范与事务级撤销回滚。
4. **多端状态同步**：移动端 PWA、Android WebView、Admin 管理后台对 SSE 流式生命周期（`token` / `tool_call` / `error` / `done`）有严格的幂等与容错要求。

**Agent 调度中心**正是解决上述痛点的中枢大脑，负责协调**请求接入、模型决策、工具执行、安全防护与状态持久化**。

---

## 二、 核心问题根因与已落地加固（彻底解决「一直在思考」假死）

针对用户反馈的「主会话端 Agent 发送消息后一直在思考」现象，经对全链路代码审查与网络时序复盘，定位到以下 **5 大核心根因**，并已在当前批次完成代码加固：

### 1. 根因 ①：Runtime 循环完全忽略了 `StreamEvent.Err`（已修复 ✅）
- **现象**：`OpenAIProvider` 在 `stream.Recv()` 出错（如 API 密钥无效、额度耗尽、连接断开）时向通道发送 `StreamEvent{Err: err}`。
- **缺陷**：旧版 `internal/agent/runtime/runtime.go` 循环中对 `e.Err != nil` 完全无判断直接跳过！通道关闭后，调度器误以为模型正常回答完毕，生成了一条 `content: ""` 的空消息落库，并向前端推送 `{"type":"done"}`。
- **加固**：在循环头部侦测 `if e.Err != nil { streamErr = e.Err; break }`，立即触发 `fail("大模型响应异常: "+streamErr.Error(), streamErr)`，向前端派发显式 `{"type":"error", "error":"..."}`。

### 2. 根因 ②：OpenAIProvider 客户端缺省无限等待（Timeout = 0）（已修复 ✅）
- **现象**：`go-openai` 默认的 `openai.DefaultConfig(apiKey)` 未配置 `HTTPClient` 超时（Go 标准库默认为 0，即永不超时）。
- **缺陷**：当网络代理（如本机 Clash/airtcp）、海外网关或企业防火墙发生丢包挂起时，底层 TCP 连接永远不释放，goroutine 阻塞死锁在 `stream.Recv()`。
- **加固**：为 `OpenAIProvider` 显式注入带 60s 整体超时的 `&http.Client{Timeout: 60 * time.Second}`，强制在网络静默超时后中断流并释放连接池。

### 3. 根因 ③：缺少 SSE 保持活动心跳 (Keep-Alive Ping)
- **现象**：在首字生成（TTFT）时间长或工具执行耗时较长时，中间反向代理（如 Nginx、CDN 或移动网络 NAT 网关）会在 15–30 秒无数据传输时直接切断连接。
- **缺陷**：由于未向流中周期性写入注释行（`: ping\n\n`）或心跳帧，连接已被中间件掐断但客户端无感知，处于死等状态。
- **加固**：在 `internal/api/v1/chat.go` 与 `useSSE.ts` 建立 15 秒双向心跳感知协议。

### 4. 根因 ④：前端与后端对「空回复」缺乏安全兜底（已修复 ✅）
- **现象**：当大模型因系统 Prompt 触发安全审查或空输出时，返回 0 个 token 且无工具调用。
- **缺陷**：旧逻辑直接退出，前端 `ChatPage` 收到 `done` 时消息内容为空，由于 `isEmpty && !isStreamingThis` 条件判断导致气泡折叠消失，用户感知为「发了消息毫无反应」。
- **加固**：在调度器 ReAct 循环终止前，若 `text.Len() == 0 && len(toolCalls) == 0`，自动注入兜底友好提示「未能生成有效回复，请重试或更换模型」，确保用户交互有始有终。

### 5. 根因 ⑤：动态网关配置失联与未填 API Key 兜底
- **现象**：用户在设置页切换模型后，若选中的模型所属供应商未在数据库中配置真实 API Key，且未设置环境变量。
- **加固**：在模型列表与调度器入口处标记「未配置密钥」，未就绪模型在前端置灰或在请求前提示。

---

## 三、 调度中心六大核心子系统全景

详见主设计：[docs/agent-dispatch-center.md](../../agent-dispatch-center.md)
1. **会话与上下文治理 (Session & Context Governance)**：成员隔离（member_id 多租户隔离）与历史窗口拉取；
2. **动态模型网关与凭据中心 (Dynamic Gateway & Credential Center)**：数据库凭据优先（AES-256-GCM 解密）、环境变量兜底、脚本引擎回退与 60s 看门狗超时；
3. **动态系统提示词与权限双保险滤网 (Prompt Builder & Dual Security Filter)**：角色人设、实时公历/农历时间、权限源头过滤（`SpecsWithPermission`，ADR-005）；
4. **ReAct 核心执行引擎与危险分级熔断器 (ReAct Engine & Risk-Tier Guard)**：步数硬上限截断（低 15 / 中 8 / 高 3 步）、累计金额熔断（连续 ≥2 次高风险写操作暂停确认）、TTFT 与用量埋点；
5. **统一工具执行总线与全量可撤销链路 (Unified Tool Bus & 24h Undo Pipeline)**：13 个业务工具生命周期管理、JSON Schema 参数校验、24h `undo_log` 逆向回滚（ADR-004）、敏感审计留痕；
6. **流式事件协议与结构化卡片渲染 (Streaming SSE & Action Cards)**：统一协议事件（`token` / `tool_call` / `done` / `error` / `ping`）与卡片乐观回滚。

---

## 四、 任务分解表与推进状态 (WBS)

| 阶段 | 编号 | 任务名称 | 责任 Agent | 交付产物 | 状态 |
|---|---|---|---|---|---|
| **P0-1** | T-DSP-01 | 流式错误传播加固 (捕获 `e.Err` 并下发 `error` 事件) | Backend Dev | `internal/agent/runtime/runtime.go` | **已完成 ✅** |
| **P0-2** | T-DSP-02 | 空生成安全保底 (0 Token + 0 Tool Call 注入友好回复) | Backend Dev | `internal/agent/runtime/runtime.go` | **已完成 ✅** |
| **P0-3** | T-DSP-03 | OpenAI 客户端超时熔断 (配置 60s Timeout Client) | Backend Dev | `internal/agent/gateway/openai.go` | **已完成 ✅** |
| **P0-4** | T-DSP-04 | Gin SSE 响应处理器防吞错加固 | Backend Dev | `internal/api/v1/chat.go` | **已完成 ✅** |
| **P0-5** | T-DSP-05 | 异常处理单元测试与回归断言 | QA Auditor | `internal/agent/runtime/runtime_test.go` | **已完成 ✅** |
| **P1-1** | T-DSP-06 | 契约与主设计规范归档 (`agent-dispatch-center.md`) | Spec Architect | `docs/agent-dispatch-center.md` | **已完成 ✅** |
| **P1-2** | T-DSP-07 | 执行计划与索引全面同步 | Spec Architect | `docs/exec-plans/active/agent-dispatch-center.md`, `README.md` | **已完成 ✅** |
| **P1-3** | T-DSP-08 | 全仓编译与双端类型门禁自检 (`scripts/verify-all.ps1`) | QA Auditor | `go build`, `tsc --noEmit` | **已完成 ✅** |
| **P1-4** | T-DSP-09 | 主项目与镜像仓库双端推送同步 | Lead Orchestrator | `D:\main\company_projects\sun\007\ai_agent` 同步 | **已完成 ✅** |
| **P2-1** | T-DSP-10 | Run/RunStep/RunEvent 持久化建模与生命周期仓储 | Backend Dev | `internal/store/ent/schema/run.go`, `internal/store/repo/run.go` | **已完成 ✅** |
| **P2-2** | T-DSP-11 | 编排契约与双端类型生成 (`/runs*`, `request_id`, 报饭响应修复) | Spec Architect | `api/openapi.yaml`, `scripts/codegen.ps1` | **已完成 ✅** |
| **P2-3** | T-DSP-12 | 调度中心 REST API 与单向分层领域服务 (`run.Service`) | Fullstack Builder | `internal/api/v1/runs.go`, `internal/domain/run/` | **已完成 ✅** |
| **P2-4** | T-DSP-13 | 复合任务规划工具 (`submit_task_plan`) 拓扑有向无环校验与撤销闭环 | Fullstack Builder | `internal/domain/run/plan_tool.go`, `plan_tool_test.go` | **已完成 ✅** |
| **P2-5** | T-DSP-14 | SSE 保活心跳注释帧与事务追踪透传 | Backend Dev | `internal/api/v1/chat.go`, `internal/agent/runtime/` | **已完成 ✅** |

---

## 五、 决策日志 (ADR / Decision Log)

1. **坚持自研 Go 原生调度器 (ADR-002)**：不接入 Python LangChain / CrewAI，内存占用仅 20MB，单进程协程流式与 SQLite 极简部署。
2. **错误显式下发禁止静默吞错**：大模型调用异常时，必须以 `{ "type": "error", "error": "..." }` 直达前端，打破悬停黑盒，恢复输入交互。
3. **空生成兜底保护**：若模型未产生 Token 与工具调用，注入默认友好回复，消除幽灵气泡。
4. **两类循环职责划分**：决策循环调 LLM（低频）；代码执行循环推物理事件（高频、零 Token 消耗、无网络延迟）。
5. **分层单向与防循环导入**：`api/v1` 仅依赖 `domain/run.Service`，`repo.Store` 向上实现仓储接口，规避 `api/v1` 与 `repo` 的循环依赖。
6. **复合计划严格有向无环 (DAG)**：`submit_task_plan` 在系统边界强制依赖单向无环，上限 10 步，支持撤销为 `cancelled`。

---

## 六、 验收与质量门禁记录

- `go test ./...`: 全项目所有测试包（repo/agent/domain/api/auth/crypto）全部通过，包含新增的 `TestRunRepo_Lifecycle`, `TestSubmitTaskPlanTool_ExecuteAndDAG`, `TestRunsAPI_Endpoints`。
- `pnpm -r run typecheck`: 全工作区（shared / web / admin）0 类型报错。
- `pnpm -r run test`: 前端与共享包单元测试全部通过。
- `go build ./cmd/homeagent`: 二进制编译成功。
