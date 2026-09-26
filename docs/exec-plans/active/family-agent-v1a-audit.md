# 家庭 AI 代理人 V1.0-A · 代码与行为基线复核（A-00）

> 对齐 **AI-STD-006（仓库 Harness）**。
> 对齐：`docs/家庭AI代理人_V1.0现有产品价值重构_执行PRD_V1.0.md` §5 A-00。
> 状态：✅ 已完成（2026-09-23）。本文所有结论由**仓库代码直读 + 契约直读**得出，可复核。
> 执行代理在进入 A-01~A-05 实现前必须先读本文。

## A-00-01 用户功能清单逐项核实

> 判定：✅ 核实（代码与 PRD 描述一致）／🟡 部分核实（能力存在但有缺口或语义偏差）／❌ 不符

### 对话与入口

| PRD 描述 | 判定 | 仓库事实 |
| --- | --- | --- |
| ChatPage + useSSE + POST /chat：SSE 对话 | ✅ | `web/src/pages/ChatPage/ChatPage.tsx` + `hooks/useSSE.ts`；SSE 事件 `token/tool_call/done/error`（`runtime.go` emit） |
| ReAct runtime 与 13 个工具 | ✅ | `internal/agent/runtime`；main.go 注册 13 工具（9 可见 + update_expense 隐藏 + undo_last + create_event + list_events） |
| quick chips | ✅ | `ChatPage/quickChips.ts`；按 `GET /tools` 权限过滤 + 时段加权；draft 只填入不发送 |
| 会话管理 | ✅ | 侧边栏新建/切换/删除（软删除）；`/conversations` 契约 |
| 模型选择 | ✅ | 顶部下拉读 `GET /models`，库默认模型为单一真相源 |
| 通知角标 | ✅ | 🔔 + 未读计数，30 秒轮询，`formatUnread` 封顶 99+ |

### 记账（参考实现，不得重写）

| PRD 描述 | 判定 | 仓库事实 |
| --- | --- | --- |
| 对话与页面记账 | ✅ | `record_expense` 工具 + `POST /expenses` handler |
| 结构化结果卡片 | ✅ | `ExpenseCard.tsx`：金额/分类行内修正 + `在账本中查看` 深链 |
| PATCH /expenses/{id} | ✅ | 契约 + `repo.Update`（member 隔离 + 幂等键同步 + 旧值快照） |
| ?focus= 定位 | ✅ | MoneyPage 高亮定位 |
| 流水筛选与加载 | ✅ | 游标分页 + category/start_date/end_date 过滤 |
| 24 小时撤销 | ✅ | `Executor` 硬编码 `ExpiresIn: 86400`；撤销中心 + 卡片 + `undo_last` 对话侧 |
| **账单可见性** | ✅ **按成员隔离** | `repo.ListExpenses` 用 `expense.HasMemberWith(member.IDEQ(...))`——**账单是私有的，不是家庭共享** |

### 家务

| PRD 描述 | 判定 | 仓库事实 |
| --- | --- | --- |
| assign_task / POST /tasks | ✅ | 工具 + handler 同一服务 |
| 任务列表 | ✅ | `GET /tasks` = 我的待办（仅 assignee=我 且未完成） |
| pending → in_progress → done | ✅ | `repo.Complete` 两次迁移：pending→in_progress→done |
| 误打卡回退 | ✅ | `Uncomplete`（pending + 清 completed_at），撤销中心可达 |
| 到期通知 | ✅ | 调度器 `TaskDueSource` 扫 `due_at` |
| **频率/轮值** | ❌ 不存在 | 任务无 interval/rotation 字段；PRD 已列为延后，正确 |

### 用餐

| PRD 描述 | 判定 | 仓库事实 |
| --- | --- | --- |
| 二值申报 | ✅ | 在家/不在家，人+日期幂等（unique 键兜底） |
| 未申报名单 | ✅ | `DailySummary`：全体活跃成员 − 已申报 |
| 缺口自动通知 | ✅ | 调度器 `MealGapSource`（16:00 后） |
| 「催办」仅复制文案 | ✅ 核实 | `MealPage.nudge` = `copyText()` 复制到剪贴板，**无任何发送机制** |
| **用餐汇总可见性** | 🟡 **全家庭可见** | `DailySummary` 不按 member 过滤；`GET /meals` 权限键是 `meal.write`，已登录成员均见全家报饭 |
| 默认值推断 | ❌ 不存在 | PRD 已列为延后，正确 |

### 日程

| PRD 描述 | 判定 | 仓库事实 |
| --- | --- | --- |
| create_event / POST /events | ✅ | 工具 + handler，幂等 + undo_id |
| 重复规则查询展开 | ✅ | `domain/calendar` 查询时展开，不预生成实例 |
| 单实例跳过 | ✅ | `DELETE /events/{id}/instances/{occurrence}` → EventOverride(action=skip) |
| private 可见性 | ✅ | 服务层 `canSee`：private 只见创建人；仓储 `ListInRange`/`ListByOwner` 分离 |
| 家庭成员端可视入口 | ❌ **不存在** | 5 个 Tab（对话/家务/记账/报饭/我的）无日程；无日历页、无列表页、无详情页 |
| **事件详情接口** | ❌ **不存在** | 契约无 `GET /events/{id}`。详情只能靠 `GET /events` 窗口列表定位 |
| **事件修改接口** | ❌ **不存在** | 契约无 PATCH。只能删除（整系列 / 单实例跳过），**不能改时间/标题** |
| **修改单次/整系列** | ❌ 不支持 | Override 仅支持 skip；modify 分支字段（start_at/title）已在 schema 预留但**服务层未实现** |

### 底座

| PRD 描述 | 判定 | 仓库事实 |
| --- | --- | --- |
| 登录与 JWT | ✅ | `POST /auth/token` → HS256，7 天；`middleware.JWTAuth` |
| 执行层权限校验及审计 | ✅ | `Executor.Execute` 拒绝并记 `permission_denied=true`；handler 侧 `permissionOf` 亦记审计 |
| 撤销中心 | ✅ | `GET /undo` + `POST /undo/{id}`；24h；成员隔离；摘要按工具类型查对象 |
| 事件级幂等调度 | ✅ | cron 每分钟；唯一键 `(type,ref_id,member_id,scheduled_at)` |
| 静默通知 ADR-006 | ✅ | 零声音零震动；不做按类型静音、不做免打扰时段（有意取舍） |
| OpenAPI 类型契约 | ✅ | `api/openapi.yaml` → oapi-codegen + openapi-typescript 双端生成 |
| evals 与 e2e | ✅ | evals 29 条；e2e 14 个 spec。**PRD 正确指出：须重新运行复核，不视为体验改善的证据** |

## A-00-03 PRD 点名的三个关键疑点（明确结论）

**疑点 1：事件详情/更新/取消接口是否已有？**

- **取消**：✅ 有两种——`DELETE /events/{id}`（删整系列，可撤销）与 `DELETE /events/{id}/instances/{occurrence}`（跳过单次）。
- **详情**：❌ **没有** `GET /events/{id}`。`EventInstance` schema 已定义完整字段，但只能通过列表窗口查询拿到。
- **更新**：❌ **没有** PATCH。**A-02-04 的「修改」需求无法用现有接口满足**；`EventOverride` 的 modify 分支字段（`start_at`/`end_at`/`title`/`description`）已在 ent schema 与 `toInstance` 合并逻辑里预留，但服务层没有 `ModifyInstance` 方法、没有写接口。

**结论**：A-02 本期范围 = 准确查看 + 已有取消操作。修改单次/整系列记录为**独立缺口**，不为本期表面「可编辑」破坏重复规则（符合 PRD §9 重复日程风险与 A-02 范围决策）。

**疑点 2：重复系列是只支持删除单次，还是支持修改单次/修改全系列？**

- 只支持「跳过单次」（skip）。**不支持修改单次，不支持修改全系列**。
- 删整系列走 `CalendarDelete` 软删除（可撤销恢复）。
- 「跳过单次」与「删整系列」语义分离，不会误删系列——A-02-05 的作用范围告知需求可由现有能力满足。

**疑点 3：撤销中心实际支持哪些写操作与 24h 限制？**

- 24h 窗口：`Executor` 写 undo_log 时 `ExpiresIn: 86400`（硬编码，秒）。
- 支持撤销的写工具（实现 `WriteTool`）：`record_expense`、`update_expense`、`assign_task`、`complete_task`、`report_meal`、`create_event`、`undo_last`（自身撤销最近一条）。
- 撤销 = `Executor.Undo` 按 tool_name 分发到工具 `Undo()`，统一走成员隔离 + 审计。
- **`POST /undo/{id}` 不校验目标对象是否仍存在**：软删除的账单/任务仍可再撤销（幂等安全），但 A-04 的「已撤销不重复计数」需在聚合层处理。

## A-00-02 基线走查（四条流程）

走查脚本要求可回归。现有 e2e spec 覆盖情况：

| 流程 | 现有 e2e 覆盖 | 基线结论 |
| --- | --- | --- |
| 对话记账 | `money.spec.ts`、`chat.spec.ts`、`undo-chat.spec.ts` | ✅ 闭环真实：记账→入库→撤销→账本联动 |
| 家务打卡 | `tasks-meals.spec.ts`（3/3） | ✅ 闭环真实：派发→打卡→撤销回退 |
| 用餐申报 | `tasks-meals.spec.ts` | ✅ 闭环真实：申报→汇总→未申报缺口→催办复制 |
| 对话创建日程 | ❌ **无 e2e** | 🟡 后端 curl 验收过（建/展开/跳过/撤销/权限），但**无 Playwright 覆盖、无前端入口** |

**基线命令（本期每次交付前重跑）**：

```powershell
$env:Path = "$env:SCOOP\apps\go\current\bin;$env:PATH"; $env:GOPROXY = 'https://goproxy.cn,direct'
go test ./...                    # Go 全量
pnpm -r run typecheck            # 双端类型
pnpm run lint                    # ESLint + Stylelint
pnpm -r run test                 # Vitest + MSW
# e2e 需先启服务 + dev server（见 AGENTS.md 本机环境注意）
```

## A-00-04 改动清单（A-01~A-05 对应）

### 契约层（`api/openapi.yaml` → `make generate`）

| 需求 | 改动 | 必要性 |
| --- | --- | --- |
| A-02-01 日程列表入口 | 可复用 `GET /events`（已存在），**无需改契约** | 复用 |
| A-02-02 结果卡片→详情 | **新增 `GET /events/{eventId}`**（返回该事件的单个实例视图，按可见性过滤） | **必需**——否则深链无目标接口 |
| A-02-04 修改 | 本期**不做**（记录缺口） | 延后 |
| A-01 Today 聚合 | 走「新增 `GET /today`」路线（**已定**：服务端聚合，一次性返回四块，权限服务端一次性裁剪） | **必需**——4 次前端请求 → 1 次，权限集中 |

### 领域/服务层

| 需求 | 改动 |
| --- | --- |
| A-02-02 事件详情 | `domain/calendar` 加 `GetInstance(ctx, eventID, memberID)`：取事件 + 可见性判定 + 返回首个/指定实例 |
| A-01 Today | 新增 `domain/today`（聚合服务，依赖各模块读接口，不建表）或 handler 内组装 |
| A-03 卡片 | task/meal/event 工具的 Card 字段补齐人类可读字段（已有 task_id/at_home/date/event_id，够用） |

### 前端

| 需求 | 改动 |
| --- | --- |
| A-01 | 新增 `pages/TodayPage`；`App.tsx` 路由 `/` 改为 Today（对话移到 `/chat`，TabBar 第一项改「对话」指向 /chat）——**已定** |
| A-02 | 新增 `pages/EventsPage`（列表，复用 `GET /events`）+ `EventDetailPage`（复用新 `GET /events/{id}`） |
| A-03 | `ResultCard` 分支化：按 `card.type` 渲染 task/meal/event 可读结构 + 「去 X 查看」深链；**删除 `JSON.stringify(m.card)`** |
| A-04 | 轻量失效约定：写操作成功后 invalidate 对应查询（react-router 重挂载或显式 refetch） |

### 测试

| 需求 | 改动 |
| --- | --- |
| A-00 基线 | 本文 + 重跑基线命令 |
| A-01~A-05 | 每项需求至少一个单测 + 一个 e2e（E2E-01~07 对应） |
| 权限 | 越权用例：跨成员 private 事件深链、Today 聚合权限裁剪 |

## 权限矩阵初核（A-05 前置）

| 数据 | 当前隔离 | Today 展示口径 |
| --- | --- | --- |
| 账单 | **按 member 隔离**（`HasMemberWith`） | 只展示本人的；**不存在「家庭账本」聚合，不要假设可共享** |
| 任务 | assignee=我 才可见（`ListMyTasks`） | 展示我的待办；他人任务**不可见** |
| 报饭 | **全家庭可见**（`DailySummary` 无 member 过滤） | 用餐概览可展示全家；但「我是否已申报」是个人状态 |
| 日程 | private 仅创建人；family 全家 | 按 `canSee` 逐实例过滤 |
| 通知 | 按 member 隔离 | 只展示本人的 |

**A-05-02 结论**：PRD 的担心方向反了——账单**已经是私有的**，不是「如果当前为共享模式」。真正需要注意的是**报饭汇总全家庭可见**这一点是既有产品语义（做饭人要看全家谁没报），本期保持，不擅自改。

**A-05 阻塞规则确认**：若 Today 某数据源无法在服务端确认归属，**先屏蔽该条目**并报告，不用前端隐藏代替授权。

## 与 PRD 的差异说明（PRD 假设 vs 仓库实际）

| PRD 表述 | 仓库实际 | 影响 |
| --- | --- | --- |
| 「日程：create_event / POST /events、重复规则查询展开、单实例跳过、private 可见性」 | ✅ 全部属实 | 无 |
| 「当前缺少家庭成员端的可视入口」 | ✅ 属实（且连列表/详情接口都没有） | A-02 需补详情接口 |
| 「修改接口与重复系列编辑语义未经核实」 | 已核实：**无修改接口，仅 skip + 删系列** | A-02 范围收窄为查看 + 已有取消 |
| 「个人账单如果当前为共享模式…」 | 账单**按 member 隔离**，非共享 | A-05-02 无需改造，记录即可 |
| 「家务任务涉及别的成员时…不悄悄增加对方已同意」 | ✅ `assign_task` 只是创建任务，无接受/拒绝流 | 无 |
| 「催办仅复制文案」 | ✅ 属实（`copyText`） | A-03-03 文案须写「已复制，尚未发送」 |

## 已定决策（2026-09-23）

1. **Today 数据来源 = 新增 `GET /today`**：服务端聚合「我的待办 / 今日日程 / 用餐状态 / 通知动态」四块，权限在服务端一次性裁剪。新增 `domain/today` 聚合服务（不建表，依赖各模块读接口）。
2. **默认入口 = Today 占 `/`，对话移 `/chat`**：TabBar 第一项「对话」指向 `/chat`；Today 作为新首屏。符合 PRD「Today 是今天的行动摘要」+「对话仍是可随时抵达的核心入口」。

## 环境阻塞披露（PRD G4：不虚报通过）

**LLM 网关不可用：DeepSeek 账户余额不足。**

- 现象：`POST /chat` 返回 200 SSE，但流内 `{"type":"error","error":"LLM 网关连接失败"}` + 「服务暂时不可用，请稍后重试」。
- 定性证据（临时探针，已删）：库内 deepseek 供应商 key 解密正常、网络可达 `api.deepseek.com`，直测返回 **HTTP 402 `{"error":{"message":"Insufficient Balance"}}`**。
- 结论：**纯账户余额问题，非代码缺陷**。我本期改动文件（router 加 1 条路由、calendar handler/domain、前端日程页/路由/TabBar）**不涉及** runtime/gateway/chat 链路；`go test ./...` 含 runtime 与 gateway 单测（mock 供应商）全绿。
- 影响：所有**依赖对话建账/建事件**的 e2e（chat / conversations / money 对话记账分支 / new-features 对话卡片）无法通过。**不依赖 LLM 的 e2e 16/16 全过**（auth、notifications、settings、undo-center、new-features 的 FAB 分支、events、verify-render）。
- 处置：不改动 chat 链路。需要**充值 DeepSeek 或切换供应商**后重跑 chat 路径回归。本期 A-01~A-05 的后端与页面可用 API 造数据验证，只有「对话内卡片渲染」这一环卡在此。

## 完成定义

- [x] A-00-01 功能清单逐项核实（上文 6 张表）
- [x] A-00-02 四条流程基线走查 + 基线命令
- [x] A-00-03 事件详情/更新/取消接口、重复系列语义、撤销中心支持范围
- [x] A-00-04 前端/服务端/契约/测试改动清单
- [x] 权限矩阵初核与阻塞规则确认

**结论**：可进入提交 1（A-02 日程可查）。A-02 需先补 `GET /events/{eventId}` 契约 + 领域方法，再前端列表/详情页；修改能力记录为缺口不做。
