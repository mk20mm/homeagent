# 家事 Agent · 架构设计（ARCHITECTURE）

> 2026-10-01 最新设计：[财务、家务、厨房第一步架构](homeagent-v1-architecture.md)；验收与 Gemini 交接由该文件链接。保留单 Agent 和确定性执行器，以真实设备结果与家庭菜谱协同为核心；个人可部署、不要求企业 API 资质。本文历史清单不代表当前全部实现；新的本地撤销/设备补偿边界见 ADR-007。新设计未实现。

> 对齐 **AI-STD-003（智能体架构标准）** · 阶段：**Phase 03（Architecture）** · 版本：v0.1 · 日期：2026-09-15
>
> 原则：**组件必须为解决具体工程问题而存在**。禁止为画完整架构图而引入 RAG / 向量库 / MCP / Multi-Agent / Memory / Planner（详见 [AI-PRD.md §7](AI-PRD.md)）。
> **先画确定性边界，再画 Agent**：what model may decide / what code must enforce（见 [AI-PRD.md §3](AI-PRD.md)）。

---

## 1. 架构总览（七层）

系统分七层，自上而下：客户端层 → 接入层 → 应用层 → AI 层 → 业务层 → 基础设施层 → 外部服务。

> 📊 **分层图**：[ui/architecture-system.md](ui/architecture-system.md)（mermaid 源码 + 渲染图 `architecture-system.png`）

| 层                      | 组成                                                                  | 存在理由                                                        |
| ----------------------- | --------------------------------------------------------------------- | --------------------------------------------------------------- |
| L1 客户端层             | Web/PWA（Vite+React+TS）；Android App（Expo·三期）                    | 对话主入口，PWA 打通手机与桌面                                  |
| L2 接入层               | Nginx/Caddy 反向代理 · TLS · 静态资源                                 | 单 VPS 统一入口、HTTPS 卸载                                     |
| L3 应用层（Go·Gin）     | API 路由 `/api/v1` · 认证中间件(JWT) · 权限中间件(矩阵)               | 契约先行（OpenAPI → oapi-codegen）                              |
| L4 AI 层 — Agent 调度器 | 会话管理 / 提示词构建 / LLM 网关 / ReAct 循环 / 工具执行器 / 用量埋点 | 项目核心，详见 §3 与 [agent-调度器设计.md](agent-调度器设计.md) |
| L5 业务层               | 家务 / 日程 / 财务 / 购物 / 用餐 / 系统管理 / 幸福度包                | 领域模块，工具化暴露给 Agent                                    |
| L6 基础设施层           | SQLite→Postgres · Cron · 审计日志 · undo_log                          | 持久化、定时任务、可撤销与留痕                                  |
| L7 外部服务             | LLM 供应商（OpenAI/Anthropic/DeepSeek/智谱/Ollama）· iCal 订阅输出    | 模型与日历生态外置                                              |

---

## 2. 组件清单（Component Inventory）

> 每个组件回答「解决什么工程问题」，不为了图上好看而存在。

### 2.1 AI Gateway（LLM 网关）

- **工程问题**：多供应商（海外/国内/本地）差异大，且需统一计费、限流、路由、审计。
- **职责**：统一 `Provider` 接口，适配 OpenAI/Anthropic/DeepSeek/智谱/Ollama；封装流式 + Function Calling；凭证加密存储；**用量埋点**（token/成本）。
- **决策**：见 [ADR-002](ADR/ADR-002-自建Agent调度器而非框架.md)。

### 2.2 Agent Runtime（调度器）

- **工程问题**：开放、多步的家庭事务请求，路径不可预写（一句话可能含记账+报饭+提醒），Workflow 模式覆盖不了。
- **职责**：会话管理、提示词构建、ReAct 工具循环、危险分级控制、结果卡片。
- **详见**：[agent-调度器设计.md](agent-调度器设计.md)（六个核心组件、工具注册表、危险分级、撤销机制、Go 实现要点）。
- **运行时约束**（AI-STD-003 Runtime SHOULD）：max turns（15/8/3）、超时、取消、成本上限（用量埋点）、checkpoint（会话状态持久化）、重试、并发（goroutine + channel）、trace。

### 2.3 Tool Layer（工具层）

- **工程问题**：Agent 与确定性系统的契约——副作用必须可校验、可撤销、可审计。
- **每个工具 MUST 定义**：Name/Purpose、When to use / When not、Parameter schema + Validation、Permission / Risk level、Idempotency / Timeout / Retry、Error contract / Output schema、Undo（写操作必填）、Observability。
- **参数在可信代码校验，禁止把 LLM 当唯一安全闸门。**
- 一期工具清单：财务 `record_expense` `query_budget`；家务 `assign_task` `complete_task` `list_my_tasks`；烹饪与用餐 `save_recipe` `query_recipe` `suggest_dinner`；系统 `switch_model` `list_models`。完整字段见 [agent-调度器设计.md 工具注册表](agent-调度器设计.md)。

### 2.4 Business Services / DB

- 家务/财务/用餐等业务 service，被工具调用，**不直接暴露给 LLM**（PRD 阶段只定义 Tool Intent，不暴露整个 Swagger）。

---

## 3. 数据流（Data Flow）

以「今天买菜花了 120」为例（完整时序图见 [ui/sequence-dialog.md](ui/sequence-dialog.md)）：

```
用户消息 → 应用层(认证/权限中间件)
  → ①会话管理(取历史/模型/成员身份)
  → ②提示词构建(人设+家庭信息+权限内财务工具+农历)
  → ③LLM 网关(Provider 路由, SSE 流式, 用量埋点)
  → ④ReAct 循环: 含 tool_call?
       是 → ⑤工具执行器(权限校验→参数校验→执行→写 undo_log→写审计)
            → 结果回传 LLM → 回到 ④
       否 → ⑥结果卡片(可撤销) → 流式回复用户
用户点撤销 → undo API → 工具 Undo() → 软删除/回滚 + 标记 undo_log + 记审计
```

**关键设计**：执行结果以结构化卡片回显，写操作同步落 undo_log 与审计，再回传 LLM 继续生成——**先持久化，再回复**，保证「说出口即已落地」。

---

## 4. 信任边界（Trust Boundary）

> 对齐 AI-STD-003 / AI-STD-007。Prompt Injection 当作**持续存在的攻击面**设计，不宣称已解决。

| 边界                                  | 内容                                         | 控制措施                                                         |
| ------------------------------------- | -------------------------------------------- | ---------------------------------------------------------------- |
| **可信代码（trusted code）**          | 业务 service、工具执行器、权限矩阵、校验逻辑 | 静态类型、单测、代码评审                                         |
| **模型边界（model boundary）**        | LLM 的输出（含 tool_call 参数、回复措辞）    | 不可信：参数必经 JSON Schema 校验 + 权限校验；回复不含未承诺信息 |
| **不可信输入（untrusted input）**     | 用户消息（可能含注入）、工具结果回灌         | 用户消息不得当系统指令；工具结果标注 No Authority，不触发提权    |
| **敏感数据（sensitive data）**        | 金额、家庭成员信息、API Key                  | Key 加密存储；成员数据按 member_id 隔离；敏感字段不进上下文      |
| **写 / 批准边界（write / approval）** | A3 级（记账/改预算/删任务/改权限）           | Preview + Confirm + 必须可撤销 + 审计                            |
| **外部系统（external system）**       | LLM 供应商、iCal 输出                        | 网关统一出口；无第三方下单/支付通道（见 PRD §7）                 |

**护栏分层（高风险策略 MUST NOT 只存在于 Prompt）**：
`Schema/Rule（代码硬校验）` → `Policy Engine（权限矩阵 + 危险分级）` → `Model Classifier（LLM 判别，仅辅助）` → `Human Approval（A3 确认）`。

---

## 5. 横切一等组件（Cross-Cutting）

> 在 AI-STD-003 中与业务组件同级，不是事后补的。

| 组件                   | 位置                     | 说明                                            |
| ---------------------- | ------------------------ | ----------------------------------------------- |
| **Permission（权限）** | 提示词构建 + 工具执行器  | 双保险，见 [ADR-005](ADR/ADR-005-权限双保险.md) |
| **Audit（审计）**      | 工具执行器               | 涉钱/权限操作强制留痕，撤销也留痕               |
| **Guardrail（护栏）**  | 循环上限 + 累计风险终止  | 防无界循环 / 成本失控                           |
| **Eval**               | 见 §10 评测埋点          | 评测驱动开发，非事后补                          |
| **Observability**      | LLM 网关用量埋点 + trace | 见 §10                                          |

---

## 6. 状态设计（State）

| 状态                                  | 存储                                                              | 策略                                               |
| ------------------------------------- | ----------------------------------------------------------------- | -------------------------------------------------- |
| 会话状态（消息历史/当前模型/成员）    | SQLite，会话表                                                    | 按 member_id 严格隔离；超长截断/摘要（Compaction） |
| Agent 运行时状态（循环步数/累计风险） | 内存（会话级）                                                    | 断线可恢复：SSE 重连续上（V2）                     |
| undo_log                              | SQLite，`id/user_id/tool_name/params/undo_data/expires_at/status` | 有效期 24h，cron 清理；撤销成功后标记已用          |
| 审计日志                              | SQLite，审计表                                                    | 只增不改，按权限可见                               |
| 凭证                                  | 加密存储                                                          | 绝不落明文                                         |

> AI-STD-003：生产 State SHOULD 外置。一期单机 SQLite 即满足；迁 Postgres 时状态随库外置（见 [ADR-003](ADR/ADR-003-SQLite起步可迁移Postgres.md)）。
> **Memory 策略**：一期不做长期记忆（V3 才做，届时定义写入/读取条件、TTL、权限、去污染、用户可见性、删除）。Memory ≠ 把所有历史带上。

---

## 7. 复杂度阶梯定位（为什么是 Single Agent）

> 对齐 AI-STD-001 复杂度阶梯与 AI-STD-003 Multi-Agent 准入。

- 本项目处于阶梯 **Single Agent（开放多步）**：路径不可预写，需要模型在循环中选下一步，但**一个 Agent + 一个工具集**已完全覆盖。
- **不引入 Multi-Agent 的理由**：工具数量少（一期 10 个）、无上下文隔离需求、无并行净收益、职责不互斥；Multi-Agent MUST 由 Eval 或明确独立并行职责证明必要，本项目没有该证据。
- **不引入 Workflow 框架**：家庭事务请求是开放的（一句话可含多意图），纯可预写路径覆盖不了。

---

## 8. 工具清单（Tool Inventory · 一期）

| 模块 | 工具                     | Risk | Autonomy | 幂等                          | 撤销                 |
| ---- | ------------------------ | ---- | -------- | ----------------------------- | -------------------- |
| 财务 | `record_expense`         | 高   | A3       | 是（user+amount+hint+day）    | 必须                 |
| 财务 | `record_income`          | 高   | A3       | 是（user+amount+source+day）  | 必须                 |
| 财务 | `create_recurring_bill`  | 高   | A3       | 是（user+title+cycle）        | 可撤销（删除账单）   |
| 财务 | `query_finance_summary`  | 低   | A1       | 查询，天然幂等                | 不需要               |
| 财务 | `query_expense_report`   | 低   | A1       | 查询，天然幂等                | 不需要               |
| 财务 | `query_budget`           | 低   | A1       | 查询，天然幂等                | 不需要               |
| 家务 | `assign_task`            | 中   | A2       | 是                            | 可撤销               |
| 家务 | `complete_task`          | 中   | A2       | 是（防重复打卡）              | 可撤销（撤销误打卡） |
| 家务 | `list_my_tasks`          | 低   | A1       | 查询                          | 不需要               |
| 烹饪 | `save_recipe`            | 中   | A2       | 是（user+title）              | 可撤销               |
| 烹饪 | `query_recipe`           | 低   | A1       | 查询                          | 不需要               |
| 烹饪 | `suggest_dinner`         | 低   | A0       | —                             | 不需要               |
| 出行 | `record_vehicle_expense` | 高   | A3       | 是（user+type+amount+day）    | 必须（联动 record_expense） |
| 系统 | `switch_model`           | 中   | A2       | 是                            | 可撤销（切回）       |
| 系统 | `list_models`            | 低   | A1       | 查询                          | 不需要               |

> 工具热插拔：注册表支持动态注册，新模块工具直接挂载，不改调度器核心（见 PRD §8.1）。

---

## 9. 架构决策索引（ADR）

| ADR                                                | 决策                                      | 状态 |
| -------------------------------------------------- | ----------------------------------------- | ---- |
| [ADR-001](ADR/ADR-001-技术栈选型Go.md)             | 后端技术栈选 Go                           | 接受 |
| [ADR-002](ADR/ADR-002-自建Agent调度器而非框架.md)  | 自建轻量 Agent 调度器，不引入重型框架     | 接受 |
| [ADR-003](ADR/ADR-003-SQLite起步可迁移Postgres.md) | 数据层 SQLite 起步，ent 保平滑迁 Postgres | 接受 |
| [ADR-004](ADR/ADR-004-写操作全量可撤销.md)         | 所有写操作强制可撤销（编译期约束）        | 接受 |
| [ADR-005](ADR/ADR-005-权限双保险.md)               | 权限双保险：提示词过滤 + 执行层校验       | 接受 |

> ADR 写作规范与完整清单见 [ADR/README.md](ADR/README.md)。

---

## 10. 观测与评测埋点（Observability / Eval Points）

> 对齐 AI-STD-008（可观测）与 AI-STD-005（评测）。一期轻量起步，预留字段。

**埋点位置**：

| 埋点         | 位置                    | 字段                                                                  |
| ------------ | ----------------------- | --------------------------------------------------------------------- |
| LLM 调用     | LLM 网关                | trace_id / model / provider / tokens / cost / latency / 错误          |
| 工具调用     | 工具执行器              | trace_id / tool_name / 参数摘要 / risk_level / 结果 / 耗时 / 是否撤销 |
| 权限事件     | 工具执行器 + 提示词构建 | 命中/拒绝、越权尝试                                                   |
| 业务 outcome | 业务 service            | 记账成功/失败、打卡成功、报饭汇总                                     |

**最小 Trace 字段**（AI-STD-008）：request/trace id、用户/会话、model/prompt/context 版本、工具名与参数摘要、状态、tokens、cost、latency、error、业务 outcome。敏感内容默认摘要，明文按权限查看。

**Eval 套件**：一期用 20–50 个真实家庭任务金标准（详见 [AI-PRD.md §6](AI-PRD.md)），CI 跑 eval smoke，发布前跑全量回归 + safety。

---

## 11. 下一步（Phase 03 → 04/05）

- [ ] OpenAPI 契约骨架 + oapi-codegen 生成 server 桩 / TS client
- [ ] 工具 Spec 细化（每个工具补齐 Idempotency / Timeout / Retry / Error contract）
- [ ] Context-Policy / Context-Budget 细化（AI-STD-004 交付物）
- [ ] 威胁模型与行为规格（AI-STD-007 / 009，Phase 11/12 前置输入）
