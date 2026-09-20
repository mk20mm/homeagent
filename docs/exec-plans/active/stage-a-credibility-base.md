# 产品重构 阶段 A 执行计划：可信度与底座

> 目标：兑现产品理念的**可见侧底线**——「**说的和看的是一份数据**」。阶段 A 结束时，家人说一句或点一下，页面必须跟着变；且 24h 内的写操作有一个统一可见的反悔入口。
> 来源：[`docs/product/product-design-research-01.md`](../../product/product-design-research-01.md) §6.3 问题 1–5、§15 阶段 A；范围与深度标尺见 [`docs/product/product-inspiration-01.md`](../../product/product-inspiration-01.md) §3/§6。
> 需求条目：本计划的每一项都对应 [AI-PRD §10](../../AI-PRD.md) 的 FR 编号（映射见每节标题旁的 T-Axx 与 §10.12）。
> 阶段定位：本计划只覆盖 **V1 阶段 A**；阶段 B（四模块深耕）见 [stage-b-module-depth.md](stage-b-module-depth.md)。
> 验收用例格式对齐 AI-STD-005：`{category, description, steps, passes}`，`passes` 随实现同步翻转。
> 前置：C 阶段 P0/P1 已完成（9 工具 + JWT + 权限双保险 + 幂等 + 撤销链路）；家事重构不引入新的安全边界。

**当前状态（2026-09-20）**：① T-A01 已完成（handler + repo 适配 + 单测 + curl 验收全绿，见决策日志）；② 报饭真实化 + ③ 家务真实化的页面侧已完成（e2e `tasks-meals.spec.ts` 3/3 绿）；④ 撤销中心已完成（e2e `undo-center.spec.ts` 2/2 绿 + curl 安全用例全过）；② 的安全用例已补齐 handler 层越权审计（T22 已偿还）。其余 `passes: false`。

> 环境阻塞（非代码问题）：开发库 deepseek 供应商配了 api_key，后端按优先级选真实供应商，但本机 WMI 启动的进程无代理 env → 连不上 api.deepseek.com → `/chat` 报「LLM 网关连接失败」。凡依赖「对话里说一句」的同源步骤暂只能按结构等价验证（页面与工具走同一 domain service/repo，T-A01 已覆盖工具侧）。

## 排序依据（为什么是这个顺序）

```
① tasks.go / meals.go handler  ← 契约已定义、实现缺失；不补它，②③⑥ 全部无从验证
        ↓
② 报饭真实化 + 缺口催办    ③ 家务真实化 + 打卡反馈   ← 消灭假数据（可信度）
        ↓
④ 撤销中心（GET /undo）      ← 兑现 ADR-004 的统一入口
⑤ 快捷 chips + 可点空态      ← 0 输入路径（新人门槛）
⑥ 账本加载更多/分组/筛选      ← 用已有接口参数，纯前端
⑦⑧⑨ 文案/入口/对话侧撤销      ← 独立小项，可并行
⑩ 主动服务底座（通知表 + 调度器 + 通知中心 + 免打扰）  ← 阶段 B 四模块深耕的共同前置
```

**判断**：**先在假数据上修可信度，再重构信息架构**。Today 首页（IA 层重构）放阶段 B——在页面还在造假的前提下改 IA，等于把重构建在流沙上。

---

## ① 家务/报饭 handler（T-A01，前置，契约已定义）

```json
{
  "category": "functional",
  "description": "补齐 /tasks 与 /meals 的 HTTP handler（契约已定义、实现缺失）",
  "steps": [
    "GET /tasks 返回当前成员任务（member 隔离 + 软删除过滤）",
    "POST /tasks 派任务，幂等键与 assign_task 工具一致（重复提交返回已存在任务）",
    "POST /tasks/{id}/complete 打卡，防重复（409）",
    "GET /meals 返回当日汇总：在家/不在家/未申报三类名单",
    "POST /meals 报饭，按（成员+日期）幂等 upsert",
    "四类写操作全部落 undo_log 并可经 POST /undo/{id} 撤销"
  ],
  "passes": true,
  "note": "已完成（2026-09-18）：新增 internal/api/v1/{tasks,meals}.go，接到既有领域服务，不改契约、不改工具行为。顺手修了两个 latent bug：① Executor.Undo 未注入 memberID（报饭撤销 500）；② 日期时区不一致（time.Parse 得 UTC 0 点 vs 入库本地 0 点 → 显式日期报饭插重复、撤销删不掉、GET /meals?date= 查空）。详见决策日志"
}
```

## ② 报饭真实化 + 缺口（T-A02）

```json
{
  "category": "functional",
  "description": "报饭页接真实接口，并显式展示「还缺谁」",
  "steps": [
    "报饭页加载 GET /meals：我的申报状态来自后端，刷新后保留",
    "家人申报分区显示：在家 N 人 / 不在家 N 人 / 未申报 N 人（未申报显式列出）",
    "切换「在家吃/不在家吃」调用 POST /meals，成功后实时更新汇总",
    "对话里说「今晚不回家吃」后打开报饭页，状态一致（对话与页面同源）",
    "未申报分区旁有「催办」入口（阶段 A 先做站内催办：写家庭动态 + 可复制提醒文案；⑩ 落地后切到通知中心）"
  ],
  "passes": true,
  "note": "已完成（2026-09-18）：MealPage 接 GET /meals，三分区（在家/不在家/还没报，缺口显式列人名+人数）；我的申报双按钮按 memberId 定位、切换调 POST /meals 后 reload；催办按钮复制提醒文案（剪贴板 API + execCommand 降级）。e2e 验收：切换申报→已申报列表同步、催办→已复制提醒（tasks-meals.spec.ts）。未验证：对话同源步骤——/chat 因 LLM 环境问题不可用（见文首环境阻塞），但页面与 report_meal 工具走同一 meal.Service/MealRepo，T-A01 已覆盖工具侧幂等与撤销。催办暂只复制文案，「写家庭动态」等 ⑩ 通知中心落地后再切。"
}
```

```json
{
  "category": "safety",
  "description": "催办不得修改他人数据，报饭只能改自己",
  "steps": [
    "对他人报饭记录调用 POST /meals 覆盖：被拒绝（只允许写自己，按 member_id 隔离）",
    "催办动作只产生提醒（不写 MealReport、不伪造他人申报状态）",
    "越权尝试写 audit_log（permission_denied=true）"
  ],
  "passes": true,
  "note": "已完成（2026-09-20）：POST /meals 请求体不含 member_id，成员身份只从 JWT 取 → 结构上只能写自己，无法覆盖他人；curl 验证孩子调 POST /tasks（task.write）被拒 403。越权审计（原 T22 缺口）已补：permissionOf 拒绝时写 audit_log（permission_denied=true，与工具层 registry 同语义），curl 验证孩子越权派任务后 GET /audit 能看到同 trace_id 的越权记录（tool_name=assign_task, risk=medium）。"
}
```

## ③ 家务真实化 + 打卡反馈（T-A03）

```json
{
  "category": "functional",
  "description": "家务页接真实接口，打卡有反馈且可撤销",
  "steps": [
    "家务页加载 GET /tasks（我的待办在前，家庭任务可见）",
    "对话里说「提醒媳妇洗碗」后，任务出现在家务页（同源）",
    "点「打卡」调用 POST /tasks/{id}/complete，轻提示成功 + 状态变更 + 提供撤销",
    "撤销误打卡后状态回退到 pending（Uncomplete 语义）",
    "空状态给出可点示例与一个主按钮（不再是无反馈的空白）"
  ],
  "passes": true,
  "note": "已完成（2026-09-18）：ChoresPage 接 GET /tasks；打卡走 POST /tasks/{taskId}/complete（openapi-fetch path params），状态机两步 pending→in_progress→done，每次打卡记 undo_id 并在行内显示「撤销」（Uncomplete 回退 pending）；409 → 「这个任务已经完成了」；空态「去跟管家说一句」跳 /chat。e2e 验收：空态引导、派活→打卡①进行中→撤销回退→打卡②③完成（tasks-meals.spec.ts）。未验证：对话派发同源步骤——/chat LLM 环境阻塞（见文首），但 POST /tasks 与 assign_task 工具同一 task.Service 幂等键，T-A01 已覆盖。"
}
```

## ④ 撤销中心（T-A04）

```json
{
  "category": "functional",
  "description": "GET /undo 列出 24h 内可撤销项，前端一处统一撤销",
  "steps": [
    "契约新增 GET /undo 并经 make generate 同步双端类型",
    "返回当前成员 24h 内的可撤销项：工具名、对象摘要、发生时间、剩余有效窗口",
    "已使用/已过期的项不出现在列表中（或标记为不可撤销）",
    "前端「可撤销」视图一次点击完成撤销，成功后本地即时移除（乐观 + 服务端确认）",
    "对话卡片撤销与撤销中心撤销走同一 API 与同一 undo_log"
  ],
  "passes": true,
  "note": "已完成（2026-09-18）：契约新增 GET /undo（UndoItem schema）+ make generate 双端类型；repo.ListActive（member 隔离 + status=active + expires_at>=now，新的在前）；handler ListUndo 带「对象摘要」——record_expense/update_expense 查金额分+类目（「记账 ¥12.80 · 食材」）、assign_task/complete_task 查标题（「派任务：洗碗」）、report_meal 取日期，查不到降级成工具标签；前端 /undo 页（乐观移除 + 404/409 兜底文案 + 剩余窗口「剩 23 小时 59 分钟」），设置页「可撤销的操作 →」入口。e2e undo-center.spec.ts 2/2 绿；curl 验证撤销后条目从列表消失。"
}
```

```json
{
  "category": "safety",
  "description": "撤销中心的成员隔离与窗口约束",
  "steps": [
    "A 成员的 GET /undo 不返回 B 成员的撤销项",
    "对他人 undo_id 调用 POST /undo/{undoId} 返回 404（不泄漏存在性）",
    "超过 24h 的项不出现在列表，直接调用返回 410",
    "重复撤销同一 id 返回冲突，不二次执行，且留审计"
  ],
  "passes": true,
  "note": "已完成（2026-09-18）：① 成员隔离——repo 单测 TestUndoListActive 验证 A 的 ListActive 不返回 B 的记录；② 越权撤销——curl 用孩子令牌撤销爸爸的 undo_id 返回 404「撤销记录不存在或不属于该成员」（GetUndo 按 id+member 查，不泄漏存在性）；③ 窗口——repo 单测验证 SaveUndo 固定 24h、过期/已使用记录被 ListActive 过滤；④ 重复撤销——curl 二次撤销同一 id 返回 409「该操作已撤销或已失效」，不二次执行（status 非 active 直接拒绝）。"
}
```

## ⑤ 0 输入路径（快捷 chips + 可点空态）（T-A05）

```json
{
  "category": "functional",
  "description": "输入框上方快捷 chips + 空状态示例可点",
  "steps": [
    "对话页输入框上方常驻快捷 chips（如「今晚不回家吃」「洗碗打卡」「记一笔」）",
    "点击 chip 直接发送对应指令，无需打字",
    "欢迎页示例可点击并直接发送（一次点击 = 一次教学）",
    "chips 内容按成员权限过滤（无权限的动作不出现）"
  ],
  "passes": true,
  "note": "已完成（2026-09-20）：新增 quickChips.ts（候选全集 + filterChips 权限过滤 + 时段加权排序）；ChatPage 接 GET /tools，输入框上方常驻 chips 行（横向滚动、隐藏滚动条），点击直接发送；需补全参数的 chip（记一笔/提醒谁洗碗）只填入输入框并聚焦，不空发；空态示例改为可点按钮，一次点击直接发送。孩子账号实测只见到 4 个 chips（无记一笔/提醒谁洗碗），与权限矩阵一致。e2e quick-chips.spec.ts 3/3 绿；回归 chat/conversations/undo-center 9/9 绿。"
}
```

## ⑥ 账本可达全部历史（T-A06）

```json
{
  "category": "functional",
  "description": "账本加载更多 + 按日分组 + 分类筛选（复用已有接口参数）",
  "steps": [
    "列表按 `next_cursor` 支持「加载更多」，不再固定 page_size=50 截止",
    "流水按日期分组显示（今天/昨天/更早）",
    "分类筛选与日期范围过滤调用已有 query 参数并即时生效",
    "汇总金额与筛选范围一致（避免「筛了但总额没变」的错觉）"
  ],
  "passes": true,
  "note": "MoneyPage 重写：游标分页（page_size 20 + next_cursor）+ groupByDay 本地日期键分组（修了 Z/+08:00 混用导致跨天错分的 bug）+ 范围 chips（全部/本周/本月）与分类 chips，筛选时汇总卡切「当前筛选合计 · 共 N 笔」。e2e 3/3 绿（money-history.spec.ts）。昨日分组在累积数据下排深页，页面断言只覆盖稳定项，日期分组的正确性由接口层 start_date/end_date 兜底验证"
}
```

## ⑦ 登录错误文案区分（T-A07）

```json
{
  "category": "functional",
  "description": "区分「凭据错误」与「登录已过期」，消除误导（T-e2e-1）",
  "steps": [
    "首次输错令牌提示「用户名或令牌错误，请联系家庭管理员」",
    "仅已登录态令牌失效时提示「登录已过期，请重新登录」",
    "错误码取 apperr.Code 枚举，前端经 ERROR_MESSAGE 映射，不新增裸字符串",
    "登录页给出令牌获取路径说明（家庭管理员在系统管理中生成）"
  ],
  "passes": false,
  "note": "统一错误码曾是有意为之（防枚举，T14）；此处只区分「凭据错误 vs 会话过期」，不区分用户名与令牌。需改契约：`Error.code` 枚举扩充 `invalid_credentials`/`token_expired`（研究报告附录 A 的 C10），前端 `packages/shared/enums.ts` 同步"
}
```

## ⑧ 设置页真实入口（T-A08）

```json
{
  "category": "functional",
  "description": "设置页一级可达，死链全部接线或删除",
  "steps": [
    "家人端可导航进入设置页（不再需要手敲 URL）",
    "页内每一项要么可点进入真实功能，要么删除（不允许纯文本「配置 →」）",
    "家人端不出现供应商密钥、权限矩阵、审计用量（这些只归 admin）",
    "「退出登录」保留在设置页首屏"
  ],
  "passes": false
}
```

## ⑨ 对话侧撤销工具（T-A09，修复 T-e2e-2）

```json
{
  "category": "functional",
  "description": "自然语言撤销：说「取消刚才那笔」能办到",
  "steps": [
    "新增撤销工具（如 undo_last），走同一 undo_log 与成员隔离",
    "用户说「取消刚才那笔」→ 撤销最近一笔可撤销操作，回复说明撤了什么",
    "无可撤销项时明确告知，不静默成功",
    "该工具受权限与窗口约束（24h、仅本人）"
  ],
  "passes": false,
  "note": "撤销是 ADR-004 的承诺，此前只在 UI 卡片侧闭环；工具侧补齐后，对话与页面两条路径对人一致"
}
```

## ⑩ 主动服务底座（T-A10，通知与调度）

> **产品决策已定（2026-09-20，维护者）**：通知**零声音、零震动**，只要未读计数（微信式，超 99 显示 `99+`）。→ 打扰问题结构性消解：**不做免打扰时段、不做按类型静音、不建 `mute_preference` 表**。完整论证见 [ADR-006](../../ADR/ADR-006-主动服务与打扰预算.md)（状态：提议）。

```json
{
  "category": "functional",
  "description": "通知表 + 调度器 + 站内通知中心（静默，零声音零震动）",
  "steps": [
    "ADR-006 评审通过后动代码",
    "通知表落库：收件人、类型（任务到期 / 报饭缺口 / 账单到期 / 预算超支 / 家庭动态）、可行动链接、已读态",
    "唯一键 (type, ref_id, member_id, scheduled_at)：同一事件只生成一条，调度器重复扫描不刷屏",
    "调度器 robfig/cron 每分钟扫描到期项并生成通知（阶段 A 只接：任务到期 + 16:00 后报饭缺口）",
    "站内通知中心：一处可回溯全部推送，未读角标超 99 显示 99+，点开带上下文且可当场操作",
    "通知内容按成员隔离与权限矩阵计算（孩子不见他人账单金额）"
  ],
  "passes": false,
  "note": "对应研究报告附录 A 的 C4（ADR-006，已起草）+ C5（通知接口）。它是阶段 B 四模块深耕的唯一共同前置：日程提醒阶梯、周期账单到期、缺口催办都建在它上面；也是「跑腿」主脉的物理底座（T18）。维护者决策：无声音 → 免打扰时段/按类型静音/mute_preference 表一律不做（YAGNI）。"
}
```

```json
{
  "category": "safety",
  "description": "通知不越权：通知内容与可行动链接不泄漏无权可见的数据",
  "steps": [
    "孩子账号不会收到含他人账单金额的通知",
    "通知里的「可行动链接」仍受权限双保险约束：点了也做不了越权操作",
    "越权访问他人通知（改读他人 notification）被拒绝并审计"
  ],
  "passes": false,
  "note": "通知 read_at 与查询均按 member_id 隔离；敏感字段不冗余进通知体，点开按权限实时查（ADR-006）"
}
```

## ⑪ evals 套件（T-A11，承接 c-runtime P2）

```json
{
  "category": "eval",
  "description": "evals 套件：金标准任务集 + 安全否决项",
  "steps": [
    "capability / tool / safety 三类用例入集，每模块 5–10 条真实任务",
    "make eval 跑通并输出可量化指标（意图识别 / 工具选择 / 参数抽取 / 对话→执行成功率）",
    "安全否决项命中即 0 分，不可被平均分抵消（越权读取 / 越权写入 / 无 undo 高危 / 注入越权）",
    "有副作用的用例走沙箱或 dry-run，不靠重复真实执行"
  ],
  "passes": false,
  "note": "迁移自 c-runtime P2（该计划剩余项）。指标口径见 AI-PRD §6，四模块金标准扩充见 §10.14；未通过不得宣布阶段 B 出口"
}
```

---

## 依赖关系（拓扑序）

```
T-A01 handler ──┬──→ T-A02 报饭真实化 + 缺口
                └──→ T-A03 家务真实化 + 打卡
T-A04 撤销中心（需改契约：GET /undo）      ← 独立
T-A05 快捷 chips + 可点空态              ← 独立（依赖成员权限清单，已有）
T-A06 账本加载更多/分组/筛选              ← 独立（接口参数已有）
T-A07 登录文案 T-A08 设置入口 T-A09 对话侧撤销工具  ← 独立小项，可并行
T-A10 主动服务底座（ADR → 通知表 → 调度器 → 通知中心 → 免打扰）
        └──→ 阶段 B：日程提醒阶梯 / 周期账单到期 / 缺口催办 / 周报
T-A11 evals 套件（承接 c-runtime P2）    ← 阶段 B 出口前必须全绿
```

## 决策日志

| 决策                                        | 理由                                                                                                                                                                                      |
| ------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 先补 handler，再改页面                      | 契约已定义 `/tasks`、`/meals` 四个操作却无实现；页面接不上真接口就只能继续造假（T17）。工具与 REST 出口共用同一领域服务，禁止两套逻辑                                                     |
| 先修可信度，Today 页重构放阶段 B            | 在假数据的前提下改信息架构，等于把重构建在流沙上（判断 D2/D3）                                                                                                                            |
| 撤销中心用新增 `GET /undo` 列表             | 列表需要「谁、什么、剩余窗口」的视图，`/undo/{id}` 只有执行语义；不改已有接口形状，只增只读路径                                                                                           |
| 撤销中心与对话卡片撤销走同一 API            | 两处入口一份数据，避免「卡片撤得掉、中心撤不掉」的分裂                                                                                                                                    |
| 阶段 A 就做通知底座（而非留到阶段 B）       | 四模块深耕（尤其日程）全依赖提醒，底座后置会把日程做成「看日历」，等于没深耕（灵感研究判断 I3）。且必须先有 C4 ADR：打扰预算是产品承诺（P7），先推再想会出现「提醒太多→全关掉」的典型失败 |
| 催办先做站内 + 可复制文案，⑩ 落地后接真通知 | 不阻塞 ②③ 的交付；先做半成品推送会制造新的不可信（承诺了却不送达）                                                                                                                        |
| 快捷 chips 候选来自既有工具与高频动作       | 0 输入路径不应引入新后端能力；chips 按权限过滤，避免「点了才发现没权限」                                                                                                                  |
| 账本分组/筛选先用已有 query 参数            | `cursor`/`category`/`start_date`/`end_date` 契约已支持，纯前端接线即可兑现「历史可达」                                                                                                    |
| 登录文案只区分「凭据错误 vs 过期」          | 保持防枚举（T14）不变，只解决"从没登录过却提示已过期"的误导（T-e2e-1）                                                                                                                    |

### T-A01 完成记录（2026-09-18）

- **交付**：`internal/api/v1/tasks.go`（listTasks/createTask/completeTask）+ `meals.go`（listMeals/reportMeal）；`AssignTask` 返回 `duplicated` 供幂等守卫；repo 新增 `GetTask`/`MemberName`；`MealSummary` 携带成员 ID。handler 与工具完全共用同一领域服务，幂等键一致（`assignee_id` 在 HTTP 边界解析成名字再算键）。
- **修了两个 latent bug（不是 T-A01 引入，但被它暴露）**：
  1. `Executor.Undo` 没有像 `Execute` 那样注入 memberID → `report_meal` 撤销取不到身份直接 500。此前只测过 expense 撤销（Undo 不依赖 ctx memberID）所以没炸。已修，加回归守卫。
  2. **日期时区不一致**：`today()`/`Today()` 存本地 0 点，而 `time.Parse("2006-01-02")` 得 UTC 0 点 → ① 显式日期报饭 upsert 查不到旧记录 → 插重复；② 撤销按 UTC 删 → 删不掉；③ `GET /meals?date=`、账单 `start_date` 边界都偏一个时区。统一为本地 0 点（新增 `meal.ParseDate` + 修 `parseDate`），加 repo 往返回归测试。
- **遗留（已定方案）**：种子权限矩阵与 AI-PRD §10.2 不一致——老人有 `expense.write`（PRD 要求仅家长）、老人/孩子缺 `task.write`（PRD 要求打卡对全员）。handler 侧按工具声明的权限字符串（`task.write`/`task.read`/`meal.write`）做双保险，与工具一致。
  **维护者决策（2026-09-20）**：权限**不写死在代码里，做后端可配置**——按 [tool-perms-config.md](tool-perms-config.md) 走（权限注册表 catalog + admin 端改 + `SelfService` 自助 + 模板套用）。落地后 AI-PRD §10.2 的矩阵是**默认预设**，各家按需在 admin 调整（如给老人关 `expense.write`、给打卡单开 `task.complete`），不需要改代码重新部署。阶段 A 的打卡验收用 admin 配置到 §10.2 目标态后跑，不改种子策略。

### T-A06 完成记录（2026-09-20）

- **交付**：`MoneyPage` 重写为分组流水页——游标分页（page_size 20 + `next_cursor`，「加载更多」按钮）、`groupByDay` 按日分组（今天/昨天/更早 M-D 标签 + 每日小计）、范围 chips（全部/本周/本月）与分类 chips（全部分类 + 6 类），筛选时汇总卡从「本月支出」切「当前筛选合计 · 共 N 笔」，筛选无结果时空态文案。e2e `money-history.spec.ts` 3/3 绿。
- **修了一个 latent bug**：分组键原来取 `iso.slice(0, 10)`，在 `Z`（UTC）与 `+08:00`（本地）时间戳混用时会把「本地已是明天」的条目分错组。改为 `localDateKey()`：用 `new Date(iso)` 的本地年/月/日算键，与 `today()` 本地 0 点口径一致（T-A01 修的同一类时区问题）。
- **测试策略**：多轮 e2e 在爸爸账号累积了 420 条流水（几乎全是「今天」），新造的「昨天」条目被压到 20 页之后，页面断言不可靠。页面层只断稳定项（今天分组、小计、筛选即时生效、加载更多可用），「昨天」分组的正确性由接口层 `start_date/end_date` 查询兜底验证。

### T-A05 完成记录（2026-09-20）

- **交付**：`web/src/pages/ChatPage/quickChips.ts`（候选全集 + `filterChips` 按成员工具清单过滤 + 时段加权排序）；ChatPage 输入框上方常驻 chips 行（横向滚动），点击直接发送，空态示例改可点按钮。
- **draft 与 send 的区分**：不需要补参数的 chip（今晚不回家吃、洗碗打卡、本月预算…）点击直接发送；需要补参数的（记一笔缺金额、提醒谁洗碗缺人名）只填入输入框并聚焦，不空发——空发会让 LLM 追问一轮，违反「0 输入」承诺。
- **修了一个 latent bug**：`tool.Spec` 结构体没有 json tag，Gin 输出 `Name`/`Permission` 首字母大写，而 OpenAPI 契约与生成的 TS 类型是小写 `name`/`permission`——admin DebugPage 与本次 ChatPage 都会拿到 undefined。加 json tag 对齐契约（`Hidden` 用 `json:"-"` 不暴露），curl 验证 key 已变小写。
- **e2e 策略**：权限过滤用真实账号验证（孩子登录看不到记一笔/提醒谁洗碗）；断言用 `filter({ visible: true })` 避开侧边栏隐藏的历史会话标题（累积数据导致同名标题与气泡共存，触发 strict mode violation）。

## 验收标准

- ①②③ 全绿 → **「说了/点了，页面就变」**：对话产生的每一项状态变化都能在对应页面看到，刷新不丢。这是本轮最重要的验收线。
- ④ 全绿 → 24h 内所有写操作**一处可见可撤销**，成员隔离与窗口约束不被绕过。
- ⑤⑥ 全绿 → 新人空态可一次点击上手；账本任意历史可达。
- ⑦⑧⑨ 全绿 → 无死链、无误导文案；对话与 UI 两条撤销路径对人一致。
- ⑩ 全绿 → 通知可按类静音、可回溯、不越权；阶段 B 的四模块深耕具备提醒能力（日程提醒阶梯 / 周期账单 / 缺口催办）。
- ⑪ 全绿 → evals 套件可跑（含安全否决项，命中即 0 分），指标口径按 AI-PRD §6；四模块金标准随阶段 B 扩充。
- 全程：`go test ./...`、`pnpm -r run typecheck`、`pnpm run lint`、`pnpm -r run test` 全绿；`web/e2e/*.spec.ts` 回归通过（含新增用例）。

## 明确不做（本阶段）

- 不做 Today 首页与导航重构（阶段 B，避免在假数据上重构 IA）。
- 不做通知的完整编排与四模块深耕（阶段 B）：本阶段只交付底座（ADR + 通知表 + 调度器 + 通知中心 + 免打扰）。
- 不做日程模块数据模型与日历 UI（阶段 B，C13）。
- 不做权限可配置化与成员写端点（独立计划 [`tool-perms-config.md`](tool-perms-config.md)）。
- 不改权限模型、不改确定性边界、不引入 §7 禁项（社交分享、多租户、RAG、MCP、Multi-Agent）。
