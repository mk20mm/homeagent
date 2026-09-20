# E2E 问题清单（HomeAgent）

> 对齐 **AI-STD-006（仓库 Harness）**。

> 由 Playwright 浏览器级端到端测试循环产生。每轮跑完自动归档失败项与发现的问题。
> 契约：`web/e2e/*.spec.ts`（12 用例，覆盖登录守卫 / 对话记账+撤销 / 会话管理 / 记账页联动）。

## 轮次记录

| 轮次 | 时间       | 结果     | 备注                                                      |
| ---- | ---------- | -------- | --------------------------------------------------------- |
| R1   | 2026-09-17 | 0/12     | web dev server 未运行（ERR_CONNECTION_REFUSED），环境问题 |
| R2   | 2026-09-17 | 5/12     | 服务恢复；测试脚本自身 bug（expect 未 import 等）         |
| R3   | 2026-09-17 | 10/12    | 修测试 bug 后；暴露 unwrap 204 误判（真实 bug）           |
| R4   | 2026-09-17 | 12/12 ✅ | 修 unwrap 后全量通过（21.1s）                             |

## 已修复（真实问题）

### P0-1 `unwrap` 把 204 成功响应当错误抛出

- **现象**：删除会话后侧边栏列表不移除，但后端实际已删除（库查无记录）。影响**所有 204 删除类操作**（目前是删除会话，未来删除账单/任务同样受影响）。
- **根因**：`web/src/api/client.ts` `unwrap` 只判 `res.data !== undefined`；204 No Content 时 openapi-fetch 返回 `{data: undefined, error: undefined}`，走到 throw 分支抛"系统内部错误"，调用方 `deleteConversation` 中断、本地状态不更新。
- **修复**：改为**有 error 才抛**，204 时 `data` 为 undefined 直接返回（删除类调用不消费返回值）。
- **复现**：登录 → 打开会话侧边栏 → 删除任一会话 → 列表项不消失（刷新页面后才消失）。
- **验证**：`e2e/conversations.spec.ts >> 删除会话后从列表移除`。

### P0-2 记账页数据是写死的 MOCK，不联动真实数据

- **现象**：对话记账成功后，记账页（`/money`）"本月支出"与流水列表永远不变。
- **根因**：`MoneyPage` 保留骨架阶段 MOCK 常量，从未调接口；后端 `GET /expenses`、`GET /expenses/summary` 也未实现（P2 剩余项）。
- **修复**：后端新增 `ListExpenses`（member 隔离 + 软删除过滤 + occurred_at 倒序 + 游标分页 + 分类/日期过滤）与 `ExpenseSummary`（复用 `expense.Service.QueryBudget`）；`MoneyPage` 改为进页面加载真实接口（react-router 切换自动重挂载→自动刷新）。
- **验证**：`e2e/money.spec.ts >> 对话记账后记账页出现该笔流水`。

### P1-1 记账幂等命中静默返回"已记账"，用户误以为成功

- **现象**：同一天同金额同内容的账单重复记，工具仍回"已记账"，但库无新记录（"说成功却查不到"）。验收时叠加脚本供应商固定编造"买菜120"放大了该问题。
- **修复**：`expense.Service.RecordExpense` 返回 `duplicated` 标记；工具 Summary 改"今天已记过这笔，未重复记账"，card 带 `duplicated`；且 `UndoData` 置空（撤销会误删早先那笔，Executor 对空 UndoData 天然不写 undo_log）。

## 已修

### T-e2e-1 错误令牌登录提示"登录已过期"，文案误导（2026-09-20 修，T-A07）

- **现象**：登录页输入错误令牌，提示"登录已过期，请重新登录"——令牌是**无效**而非**过期**，用户困惑。
- **根因**：后端对 401 场景统一返回同一 code（`unauthorized`），前端 `ERROR_MESSAGE` 映射成"登录已过期"。
- **修复**：契约 `Error.code` 扩充 `invalid_credentials`（登录凭据错误，保持防枚举——成员不存在与令牌错误同码同文案）；JWT 中间件的令牌失效保持 `unauthorized`（=会话过期）。`make generate` 双端同步。登录页首次输错提示"用户名或令牌错误，请联系家庭管理员"；已登录态被 401 踢出时 client 写 sessionStorage 标记、登录页读后提示"登录已过期，请重新登录"；页脚加令牌获取路径说明。
- **验证**：e2e `auth.spec.ts` 5/5 绿（含本回归用例 + 会话过期用例），回归 9/9 绿。

## 待修（已暴露，未处理）

### T-e2e-2 自然语言"撤销/取消"时 LLM 无法操作

- **现象**：用户在对话里说"取消刚才那笔"，LLM 回复"当前工具没有提供删除/撤销支出的功能"——撤销能力只挂在前端卡片按钮上，LLM 无对应工具。
- **建议**：补一个 `undo_last_expense` 工具（或让 LLM 能查询并调用 undo），走同一 undo_log 体系。
- **关联**：ADR-004「写操作全量可撤销」目前只在 UI 卡片侧闭环，对话侧未闭环。

## 测试基础设施问题（本轮修复，非产品 bug）

- `e2e/helpers.ts` 使用 `expect` 未 import → `ReferenceError`，4 个用例连带失败。
- `waitForStreamDone` 等待条件错误（输入框始终 enabled），改为等"发送按钮 disabled→enabled"。
- 侧边栏用例 `toHaveCount(1)` 断言写错（应为"至少 1 个"），改为 `not.toHaveCount(0)`。
- 未用唯一标记导致 retry 重跑时创建重复同名会话，删除用例计数错乱；改用 `Date.now()` 唯一 tag。
- `vite.config.ts` 的 vitest 未排除 `e2e/**`，`*.spec.ts` 被单测误抓（Playwright `test.describe` 在 vitest 下报错）；加 `exclude`。
