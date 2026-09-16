# C 阶段执行计划：运行时联通

> 目标：跑通「对话→执行→撤销」核心闭环（AI-PRD §8.10 一期 MVP 边界）。
> 验收用例格式对齐 AI-STD-005：`{category, description, steps, passes}`，`passes` 随实现同步翻转。
> 前置：A（后端生成链路）/ B（前端骨架）已完成并验收。

**当前状态（2026-09-16）：P0 关键路径 9/9 全绿，真机端到端验收通过（记账→撤销闭环 + 权限双保险 + 幂等 + 审计留痕）。下一步 P1。**

## 优先级评估（依赖 × 价值）

```
P0 关键路径（缺一，闭环不成立）✅
  网关 fake provider → 会话管理 → 提示词构建 → ReAct 循环 → chat SSE
                                         ↓
                              repository 层 + record_expense + undo API
P1 一期完整工具集（9 个工具）+ 认证权限 + 幂等 + 用量埋点  ← 进行中
P2 评测套件 + 前端联通 + 观测 handler
```

**排序依据**：P0 任意一项缺失，P1/P2 全部无法验证（工具无调度器执行、前端无真实 SSE 可连）。
P1 内部按工具风险排序：高风险（记账）先行，因为它的撤销/幂等/审计约束最强，先做能尽早暴露架构问题。

---

## P0 · 关键路径（对话→执行→撤销闭环）✅ 9/9

```json
{
  "category": "functional",
  "description": "LLM 网关 Provider 接口 + fake provider，不依赖真实 API Key 即可跑通流式链路",
  "steps": [
    "定义 Provider 接口（StreamChat + Function Calling 抽象）",
    "实现 fake provider：按预设脚本返回 token 流与 tool_call",
    "go-openai 适配器作为 OpenAI 兼容供应商的真实实现",
    "注入 fake provider 调用 StreamChat，收到 token 事件序列"
  ],
  "passes": true,
  "note": "internal/agent/gateway：Provider/ScriptedProvider/RecordExpenseScript。go-openai 适配器留 P1（真实供应商接入）。"
}
```

```json
{
  "category": "functional",
  "description": "会话管理：新建/取历史/持久化消息，按 member_id 严格隔离",
  "steps": [
    "POST /conversations 创建会话，返回 conversation_id",
    "同一成员多次消息复用同一会话，历史消息正确落 message 表",
    "A 成员的查询不返回 B 成员的会话（隔离断言）"
  ],
  "passes": true,
  "note": "session.Service + repo/conversation.go。会话由 /chat 首消息隐式创建（Ensure）， conversations 列表 handler 在 P2。"
}
```

```json
{
  "category": "safety",
  "description": "系统提示词构建器：人设 + 家庭信息 + 权限内工具 + 时间，无权限工具不进提示词",
  "steps": [
    "按成员角色注入人设（老人简洁 / 小孩活泼）",
    "注入当前时间与农历",
    "成员缺少 expense.write 时，record_expense 不出现在工具定义中",
    "对比两份不同权限成员的提示词，工具清单差异符合权限矩阵"
  ],
  "passes": true,
  "note": "prompt.Build 按角色切语气；工具定义经 ChatRequest.Tools 注入（function calling），源头过滤在 GET /tools + Executor 双层生效。农历留 P1。"
}
```

```json
{
  "category": "functional",
  "description": "ReAct 工具循环：LLM 输出含 tool_call → 执行 → 结果回传 → 继续生成，直到无调用",
  "steps": [
    "fake provider 输出 tool_call(record_expense)",
    "调度器捕获 tool_call，交给 Executor 执行",
    "执行结果作为 tool_result 回传 LLM，LLM 继续生成",
    "LLM 输出无 tool_call 时循环正常终止，返回最终回复"
  ],
  "passes": true,
  "note": "runtime.Run；失败错误回传 LLM 重试，先持久化后 done。"
}
```

```json
{
  "category": "reliability",
  "description": "ReAct 循环上限：危险分级控制步数（低 15 / 中 8 / 高 3），防死循环与成本失控",
  "steps": [
    "构造连续输出 tool_call 的 fake provider",
    "低风险工具在第 15 步被强制终止",
    "高风险工具在第 3 步被强制终止",
    "终止时返回明确的超限提示，不静默挂起"
  ],
  "passes": true,
  "note": "TestRunHighRiskMaxTurns + TestRunCumulativeHighRiskTerminates + InfiniteToolScript 覆盖。"
}
```

```json
{
  "category": "functional",
  "description": "chat SSE handler：POST /chat 流式返回 token / tool_call / done 三类事件",
  "steps": [
    "POST /chat 携带 content，建立 SSE 连接",
    "逐个收到 event: token 事件，可拼成完整回复",
    "工具执行时收到 event: tool_call（含卡片数据）",
    "收到 event: done 表示流结束",
    "客户端断开连接时服务端取消生成（context cancel）"
  ],
  "passes": true,
  "note": "真机 curl 验收：token 流 → tool_call 卡片（¥120 食材）→ 收尾文本 → done。断开取消由 c.Request.Context() 传递，单测覆盖。"
}
```

```json
{
  "category": "functional",
  "description": "repository 层：expense 的 ent 查询封装，领域层不直接碰 ent.Client",
  "steps": [
    "定义 ExpenseRepo 接口（Create/Delete/Summary/ByID）",
    "用 ent.Client 实现，金额字段为 int64 分",
    "expense.Service 依赖接口而非具体实现（依赖单向断言）"
  ],
  "passes": true,
  "note": "repo.Store 聚合；编译期 var _ domexp.ExpenseRepo = (*Store)(nil)。"
}
```

```json
{
  "category": "functional",
  "description": "record_expense 端到端：『今天买菜花了 120』→ 入库 + undo_log + 审计 + 结果卡片",
  "steps": [
    "用户消息触发 tool_call(record_expense{amount:120, hint:'买菜'})",
    "账单入库，金额存 12000 分（int64），非 float",
    "undo_log 同步写入，expires_at = 24h 后",
    "audit_log 记录 trace_id / 工具名 / 参数摘要",
    "前端收到卡片「已记账 ¥120 · 食材」并带撤销按钮"
  ],
  "passes": true,
  "note": "真机验收：卡片 {amount:120, category:食材, hint:买菜}，账单 12000 分，undo_log 24h，审计含 trace_id。前端卡片渲染在 P2。"
}
```

```json
{
  "category": "reliability",
  "description": "撤销链路：卡片撤销按钮 → POST /undo/{id} → 软删除 + 标记 undo_log + 留痕",
  "steps": [
    "执行工具后 undo_log 处于 pending 状态",
    "POST /undo/{id} 触发工具 Undo()，账单软删除",
    "undo_log 标记为已用，audit_log 记录撤销事件",
    "重复撤销同一 id 返回冲突错误，不二次执行",
    "超过 24h 窗口的撤销被拒绝并提示已过期"
  ],
  "passes": true,
  "note": "真机验收：撤销成功 undone:true；重复撤销 409；越权撤销（孩子撤爸爸的）404；undo_log active→used；撤销审计 undone:true + trace_id 关联。"
}
```

---

## P1 · 一期工具集 + 认证权限（进行中）

```json
{
  "category": "safety",
  "description": "JWT 认证中间件：未认证请求被拒绝，claims 注入 member_id 与角色",
  "steps": [
    "无 Token 请求 /api/v1/* 返回 401（除 /health）",
    "携带有效 Token 的请求解析出 member_id 并注入 ctx",
    "过期/伪造 Token 返回 401 且 trace_id 可查"
  ],
  "passes": false,
  "note": "P0 临时用 DevAuth（X-Member-ID header / ?member_id），JWT 下一步。"
}
```

```json
{
  "category": "safety",
  "description": "权限双保险：运行时再校验，越权工具调用被拒绝并记录",
  "steps": [
    "无 expense.write 的成员调用 record_expense，执行器拒绝",
    "拒绝事件写 audit_log（permission_denied = true）",
    "GET /tools 对该成员不返回 record_expense（源头过滤已生效）"
  ],
  "passes": true,
  "note": "真机验收：孩子 GET /tools 返回空；无认证 403；越权执行被 Executor 拒绝并留 permission_denied 审计（单测 TestRunPermissionDenied 覆盖执行层）。"
}
```

```json
{
  "category": "functional",
  "description": "query_budget 工具：查本月预算与已用，低风险只读",
  "steps": ["用户问『这个月预算还有多少』触发 query_budget", "返回预算与已用金额，不做任何写操作"],
  "passes": false
}
```

```json
{
  "category": "functional",
  "description": "assign_task 工具：派发家务任务，中风险可撤销",
  "steps": ["『让小明洗碗』创建任务并指定执行人", "任务出现在被指派人清单", "撤销后任务软删除"],
  "passes": false
}
```

```json
{
  "category": "functional",
  "description": "complete_task 工具：家务打卡，幂等防重复",
  "steps": [
    "被指派人完成打卡，状态置 done",
    "同一任务重复打卡返回冲突，不重复计入",
    "撤销误打卡后状态回退"
  ],
  "passes": false
}
```

```json
{
  "category": "functional",
  "description": "list_my_tasks 工具：查我的待办，低风险只读",
  "steps": ["『我有什么任务』返回当前成员待办列表", "不返回其他成员的任务"],
  "passes": false
}
```

```json
{
  "category": "functional",
  "description": "report_meal 工具：报饭（今晚是否在家吃），按人+日期幂等",
  "steps": [
    "『今晚在家吃』更新当日报饭记录",
    "同一天重复上报为更新而非新增",
    "汇总接口返回今晚在家人数"
  ],
  "passes": false
}
```

```json
{
  "category": "functional",
  "description": "suggest_dinner 工具：晚餐建议，A0 只读无副作用",
  "steps": ["『今晚吃什么』返回建议", "不写任何表，不需要撤销"],
  "passes": false
}
```

```json
{
  "category": "functional",
  "description": "switch_model / list_models 工具：会话内切模型，工具集不变",
  "steps": [
    "list_models 返回已启用模型清单",
    "switch_model 切换会话当前模型",
    "切换后 ReAct 循环工具集与之前一致"
  ],
  "passes": false
}
```

```json
{
  "category": "reliability",
  "description": "记账幂等：幂等键 (member+amount+hint+day)，重复请求返回已存在账单",
  "steps": [
    "同一成员同日同金额同提示提交两次",
    "第二次返回已存在账单，不新建记录",
    "数据库不出现两条相同账单"
  ],
  "passes": true,
  "note": "真机验收：两次「买菜 120」返回同一 expense_id，库中仅一条账单；唯一约束冲突回查 + CodeConflict 由领域层判定。"
}
```

```json
{
  "category": "observability",
  "description": "LLM 网关用量埋点：token / 成本 / 延迟写入 llm_usage 表",
  "steps": [
    "一次流式调用完成后 llm_usage 新增一条记录",
    "记录含 model_id / prompt_tokens / completion_tokens / cost_cents / latency_ms",
    "GET /usage 汇总近 7 日总 token 与总成本"
  ],
  "passes": false,
  "note": "写入链路已通（store 实现 gateway.UsageRecorder）；cost 字段是 float 元，技术债 T10 改 int64 分；GET /usage handler 未做。"
}
```

```json
{
  "category": "safety",
  "description": "累计风险终止：一次对话执行多个高风险工具时提前终止并提示人工确认",
  "steps": [
    "构造连续 2 次记账的对话",
    "第 2 次高风险执行后调度器终止循环",
    "向用户返回『涉及金额操作较多，建议人工确认』"
  ],
  "passes": true,
  "note": "TestRunCumulativeHighRiskTerminates 覆盖（累计 2 次高风险终止）。"
}
```

---

## P2 · 评测、前端联通、观测

```json
{
  "category": "eval",
  "description": "evals 金标准任务集：20–50 个真实家庭任务（capability / tool / safety 三类）",
  "steps": [
    "记账/报饭/打卡 top 场景用例入集（capability）",
    "工具选择与参数抽取用例（tool）",
    "越权/注入/无界循环用例（safety）",
    "make eval 跑通且输出可量化指标"
  ],
  "passes": false
}
```

```json
{
  "category": "eval",
  "description": "安全否决项评测：命中即 0 分，不可被平均分抵消",
  "steps": [
    "越权查他人账单 → Critical",
    "无权限工具被执行 → Critical",
    "A3 高危操作无 undo → Critical",
    "Prompt Injection 触发越权 → Critical"
  ],
  "passes": false
}
```

```json
{
  "category": "functional",
  "description": "web ChatPage 接真实 SSE：消息流、工具卡片、撤销按钮全部联调后端",
  "steps": [
    "发送消息，token 逐字流式渲染",
    "工具执行显示「正在记账…」状态",
    "结果卡片渲染且撤销按钮可点",
    "断线后 useSSE 自动重连"
  ],
  "passes": false
}
```

```json
{
  "category": "functional",
  "description": "admin Debug 工具调试台接真实工具清单与 /chat",
  "steps": [
    "GET /tools 返回当前成员工具清单",
    "选工具填参发送，收到真实 tool_call 回执",
    "用量统计页显示 llm_usage 汇总"
  ],
  "passes": false
}
```

```json
{
  "category": "functional",
  "description": "会话历史接口：GET /conversations 与 /conversations/{id}/messages",
  "steps": ["会话列表按最后消息时间倒序", "消息按时间正序返回，含角色与卡片数据", "跨成员不可见"],
  "passes": false
}
```

```json
{
  "category": "functional",
  "description": "账单与任务 handler：GET /expenses、/expenses/summary、GET-POST /tasks",
  "steps": [
    "账单列表分页与汇总金额正确（分→元展示）",
    "任务列表按状态过滤",
    "POST /tasks/{id}/complete 打卡生效"
  ],
  "passes": false
}
```

```json
{
  "category": "observability",
  "description": "审计日志 handler：GET /audit 支持游标分页与越权过滤",
  "steps": [
    "游标分页正确加载更多",
    "permission_denied=true 只返回越权尝试记录",
    "按 tool_name 过滤生效"
  ],
  "passes": false,
  "note": "repo.ListAudit 查询已实现（游标 + 过滤），handler 未接。"
}
```

```json
{
  "category": "functional",
  "description": "main.go 撤 noop 桩：注入真实 expense.Service、工具注册表、Agent 调度器",
  "steps": [
    "启动时注册 9 个工具到 registry",
    "装配 gateway → runtime → handler 依赖链",
    "启动后 GET /tools 返回非空清单",
    "POST /chat 可完成一次真实对话"
  ],
  "passes": true,
  "note": "DI 全链路装配完成；--migrate 打印成员 id 供开发期认证。当前注册 1/9 工具，其余随 P1 补齐。"
}
```

---

## 依赖关系（拓扑序）

```
fake provider ──→ ReAct 循环 ──→ chat SSE handler ──→ main.go 装配  ✅
                     ↑
会话管理 ────────────┤
提示词构建 ──────────┤
                     ↓
repository 层 ──→ record_expense ──→ undo API       ✅
                     ↓
              权限双保险 ✅ + JWT ──→ 其余 8 工具     ← P1
                                        ↓
                                  用量埋点 / 累计风险终止 ✅
                                        ↓
                                  evals + 前端联调    ← P2
```

## 决策日志

| 决策                                               | 理由                                                                                           |
| -------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| fake provider 优先于真实供应商接入                 | 解耦：链路验证不依赖外部 Key 可用性与网络（A/B 阶段同类决策延续）                              |
| record_expense 作为首个真实工具                    | 风险最高（A3），撤销/幂等/审计约束最强，尽早暴露架构缺陷                                       |
| 幂等键用 (member+amount+hint+day) 而非金额精度匹配 | AI-PRD §3 明确：金额计算与归类由代码控制，不由 LLM 算钱                                        |
| evals 放 P2 而非 P0                                | AI-PRD §6 要求先定义评测，但套件落地需真实行为可回放；P0 用例本身即评测输入，P2 固化为回归套件 |
| JWT 放 P1 而非 P0                                  | P0 用 fake provider 本地联调，认证先行会拖慢关键路径；但权限双保险的执行层校验在 P0 就要内建   |
| 工具 memberID 从 ctx 取，由 Executor 统一注入      | 分层：Tool.Execute 签名不引 memberID 参数，领域工具从 ctx 取；注入点单一可审计（曾因遗漏注入导致真机「成员不存在」） |
| 调试端点 GET /debug/state 临时引入                 | 真机验收需直查 undo_log/audit/expense 三表；记技术债 T11，P1 删。代价是 repo→api/v1 临时反向依赖 |
| 撤销审计继承工具 risk 且 audit.Log 兜底空值        | audit_log.risk 是必填 enum 无默认值，撤销审计漏设导致写库静默失败（错误被 `_ =` 吞）；审计留痕是安全否决项，错误必须显式处理 |

## 验收标准

P0 全部 `passes: true` → 闭环可演示（一句话记账 + 撤销）。✅ **2026-09-16 达成**
P1 全部 `passes: true` → 一期 MVP 功能完整（9 工具 + 权限 + 用量）。
P2 全部 `passes: true` → 可交付 Alpha：评测有回归、前端双端联调完成。
