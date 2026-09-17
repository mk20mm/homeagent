# C 阶段执行计划：运行时联通

> 目标：跑通「对话→执行→撤销」核心闭环（AI-PRD §8.10 一期 MVP 边界）。
> 验收用例格式对齐 AI-STD-005：`{category, description, steps, passes}`，`passes` 随实现同步翻转。
> 前置：A（后端生成链路）/ B（前端骨架）已完成并验收。

**当前状态（2026-09-17）：P0 关键路径 9/9 ✅；P1 工具集 + JWT + 观测 handler ✅ 11/11；P2 前端双端联调 ✅ + 供应商/模型配置链路 ✅（api_key 加密入库 + 脱敏回显 + 数据库为单一真相源），余 evals 评测套件、会话历史与账单/任务 handler。**

## 优先级评估（依赖 × 价值）

```
P0 关键路径（缺一，闭环不成立）✅
  网关 fake provider → 会话管理 → 提示词构建 → ReAct 循环 → chat SSE
                                         ↓
                              repository 层 + record_expense + undo API
P1 一期完整工具集（9 个工具）+ 认证权限 + 幂等 + 用量埋点  ✅ 11/11
P2 评测套件 + 前端联通 + 观测 handler（观测 ✅、前端联通 ✅，余 evals）
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
  "note": "internal/agent/gateway：Provider/ScriptedProvider/RecordExpenseScript；go-openai 适配器（P1 完成，支持 OpenAI/DeepSeek 等兼容端点）。"
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
  "note": "session.Service + repo/conversation.go。会话由 /chat 首消息隐式创建（Ensure），conversations 列表 handler 在 P2。"
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
  "note": "prompt.Build 按角色切语气；工具定义经 ChatRequest.Tools 注入（function calling），源头过滤在 GET /tools + Executor 双层生效。农历留 P2。"
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
  "note": "真机 curl 验收（JWT 后）：token 流 → tool_call 卡片（¥120 食材）→ 收尾文本 → done。断开取消由 c.Request.Context() 传递，单测覆盖。"
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
  "note": "真机验收：撤销成功 undone:true；重复撤销 409；越权撤销（孩子撤爸爸的）404；undo_log active→used；撤销审计 undone:true + trace_id 关联 + risk 继承。"
}
```

---

## P1 · 一期工具集 + 认证权限 ✅ 10/11

```json
{
  "category": "safety",
  "description": "JWT 认证中间件：未认证请求被拒绝，claims 注入 member_id 与角色",
  "steps": [
    "无 Token 请求 /api/v1/* 返回 401（除 /health 与 /auth/token）",
    "携带有效 Token 的请求解析出 member_id 并注入 ctx",
    "过期/伪造 Token 返回 401 且 trace_id 可查"
  ],
  "passes": true,
  "note": "真机验收：POST /auth/token（name+auth_token）换 JWT；无 token 401；伪造/截断签名 401；错误令牌 401（统一错误信息防枚举）。单测 5 项：往返/过期/伪造/篡改/空 secret fail-closed。"
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
  "note": "真机验收：孩子 GET /tools 只见 5 个工具（无记账/派发）；孩子 /chat 触发 record_expense 被执行层拒绝，错误信息「无权限调用此工具」干净无噪音。"
}
```

```json
{
  "category": "functional",
  "description": "query_budget 工具：查本月预算与已用，低风险只读",
  "steps": ["用户问『这个月预算还有多少』触发 query_budget", "返回预算与已用金额，不做任何写操作"],
  "passes": true,
  "note": "复用 expense.Service.QueryBudget；centsToYuan 边界转换单测覆盖（含负数/单分）。month 参数留 schema，后端固定查本月。"
}
```

```json
{
  "category": "functional",
  "description": "assign_task 工具：派发家务任务，中风险可撤销",
  "steps": ["『让小明洗碗』创建任务并指定执行人", "任务出现在被指派人清单", "撤销后任务软删除"],
  "passes": true,
  "note": "单测覆盖：幂等键（assigner+assignee+title+day）冲突回查、按名找执行人不存在报错、软删除后不出现在待办。"
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
  "passes": true,
  "note": "单测覆盖状态机 pending→in_progress→done、非执行人打卡被拒、重复打卡 409、Uncomplete 回退 pending 并清完成时间。"
}
```

```json
{
  "category": "functional",
  "description": "list_my_tasks 工具：查我的待办，低风险只读",
  "steps": ["『我有什么任务』返回当前成员待办列表", "不返回其他成员的任务"],
  "passes": true,
  "note": "单测覆盖 member 隔离：执行人只见自己名下的任务。"
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
  "passes": true,
  "note": "单测覆盖 Upsert 幂等更新、Remove 撤销、DailySummary 在家/不在家名单。撤销=删除当日记录（schema 无软删除）。"
}
```

```json
{
  "category": "functional",
  "description": "suggest_dinner 工具：晚餐建议，A0 只读无副作用",
  "steps": ["『今晚吃什么』返回建议", "不写任何表，不需要撤销"],
  "passes": true,
  "note": "菜单池确定性规则（不调 LLM 生成菜单）；Permission=\"\" 表示无需权限——Registry 源头过滤与 Executor 执行校验对空权限统一放行。"
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
  "passes": true,
  "note": "单测覆盖：启用清单、按 id/名字模糊查找、禁用模型不可见、会话切换 + 默认模型回退。runtime 注入 conversation_id 到 ctx 供工具取用。撤销=切回旧模型。"
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
  "passes": true,
  "note": "写入链路 + GET /usage handler 均已接通（按日聚合在应用层完成，避开 SQLite 日期函数方言）。cost_cents 未计：T10 改 int64 分后由单价计算。"
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
  "passes": true,
  "note": "2026-09-17 联调完成：JWT 登录守卫、模型切换接 GET /models、tool_call 卡片带 undo_id、撤销走 POST /undo/{id}（乐观回滚 + 服务端确认）。自动重连用例随真实供应商联调时覆盖。"
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
  "passes": true,
  "note": "2026-09-17：DebugPage 经 JWT 认证接 GET /tools + /chat SSE；Dashboard 的 GET /usage、GET /audit handler 已补齐并真机返回数据。"
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
  "passes": true,
  "note": "2026-09-17：GET /audit handler 接通（v1 视图类型 + repo 适配），游标分页与过滤真机验证通过；顺手补 audit_log.latency_ms 漏写。"
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
  "note": "DI 全链路装配完成；9/9 工具全部注册并真机可见。--migrate 打印 name+auth_token 供登录换 JWT。"
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
        JWT ✅ + 权限双保险 ✅ ──→ 9 工具 ✅          ✅
                                        ↓
                            用量埋点 ✅(写入) / GET /usage ⏳
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
| 调试端点 GET /debug/state 临时引入并删除           | 真机验收需直查三表；JWT 上线后已删除（T11 偿还），断 repo→api/v1 反向依赖                       |
| 撤销审计继承工具 risk 且 audit.Log 兜底空值        | audit_log.risk 是必填 enum 无默认值，撤销审计漏设导致写库静默失败（错误被 `_ =` 吞）；审计留痕是安全否决项，错误必须显式处理 |
| JWT 签发用 name + auth_token（预共享密钥）         | 单家庭自用场景：无 OAuth 依赖，登录零配置；失败信息统一「用户名或令牌错误」防枚举；subtle 常量时间比对防时序攻击 |
| auth_token 开发期明文（dev-<名字>）                | P1 聚焦闭环；加密存储留技术债 T13（EncryptionKey 已备），生产前必须改                          |
| 无权限工具 Permission=\"\" 统一放行                | suggest_dinner/list_models/switch_model 全员可用；源头过滤与执行校验对空 Permission 一致放行，避免「全员可用却在清单里看不见」的错配 |
| go-openai 适配器兼容 OpenAI 协议端点                | DeepSeek/OpenAI/Ollama 均兼容；base_url 可配；tool_call 分片在网关内聚合，runtime 不处理分片    |
| TaskRepo/MealRepo 方法名避开 Create/Delete/Summary | Store 是聚合根，不能同时实现两个同名不同签名的接口方法；仓储接口按领域语义命名（Assign/Remove/DailySummary） |
| undo_id 从 undo_log 透传到 SSE tool_call 事件        | 前端撤销按钮需要回指 id：UndoStore.Save 返回 id → Executor 回填 Result.UndoID → runtime Event 携带 undo_id；之前卡片只有数据没有 id，撤销按钮无法接线 |
| 前端 api client baseUrl 用绝对 origin                | openapi-fetch 内部 `new Request(url)` 在无 document base 的环境（测试/SSR）对相对 URL 抛 Failed to parse URL；浏览器里靠 document 兜底一直没暴露，测试一接真接口就炸 |
| api client 的 fetch 延迟解析                         | createClient 在模块导入期捕获 globalThis.fetch，而 MSW 在 beforeAll 才 patch，导致 api.* 全部绕过拦截（表现为接口静默失败被 catch 吞）；`fetch: (...a) => fetch(...a)` 换成调用期解析 |
| 流式消息 id 由 store 持有（streamId）               | useSSE 的 onEvent 闭包在 connect 时已固定，React 批处理会让回调拿到过期 id；beginStream 在 store 内分配 id，回调调无参 action，闭包不持有 id |
| getAuth 快照按 localStorage 原始字符串缓存           | useSyncExternalStore 用 Object.is 比较快照，getAuth 每次 JSON.parse 返回新对象会引发「Maximum update depth exceeded」无限重渲染；按 raw 字符串比对既稳定引用又能感知外部清空/跨标签改动 |
| 模型配置以数据库为单一真相源，环境变量降为 fallback  | 验收问题：admin 页能看到模型清单但改不了（写端点缺失），且后端只看环境变量、不读库——清单是装饰。现在 main.go 启动按「库默认模型 + 解密 api_key → 环境变量 → 脚本供应商」选供应商，admin 改完重启即生效 |
| api_key 用 AES-256-GCM 加密落库，接口只回脱敏值     | 供应商密钥是家庭最高敏字段：Encrypt 时随机 nonce（同明文每次不同密文，防比对）；handler 层永不接触明文（只有 store→main 装配网关时解密一次）；脱敏视图首尾各 4 位 |
| 记账幂等命中显式提示 duplicated，且不写 undo_log   | 验收问题：脚本供应商固定编造「买菜120」，反复记账命中同一幂等键，service 静默返回「已记账」但库无新记录——「说成功却查不到」。改为 service 返回 duplicated 标记，工具 Summary 变「今天已记过这笔，未重复记账」、card 带 duplicated，且 UndoData 置空（撤销会误删早先那笔；Executor 对空 UndoData 天然不写 undo_log，不产生撤销按钮） |
| 会话管理 API 补齐（list/create/messages/delete）   | 验收问题：会话无历史/新建/清除。session.Repository 补 ListConversations + DeleteConversation（软删除，消息随会话不可见）；标题为空时由首条用户消息截断 20 字回填（豆包式）；handler 一律带 memberID 隔离校验；前端侧边栏抽屉（新建/切换/删除） |

## 验收标准

P0 全部 `passes: true` → 闭环可演示（一句话记账 + 撤销）。✅ **2026-09-16 达成**
P1 全部 `passes: true` → 一期 MVP 功能完整（9 工具 + 权限 + 用量）。✅ **11/11 达成（2026-09-17）**
P2 全部 `passes: true` → 可交付 Alpha：评测有回归、前端双端联调完成。**前端双端联调 ✅（2026-09-17）；会话历史/新建/删除 ✅（2026-09-17，含豆包式侧边栏 + 幂等命中显式提示）；余 evals 评测套件、账单/任务 handler。**
