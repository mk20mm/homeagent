# 技术债登记（Tech Debt）

> 技术债是高息贷款：发现坏模式立即偿还，而不是让它传播数天或数周。
> 对齐 AI-STD-006。偿还时在此标记 ✅ 并保留记录（决策历史比代码更难重建）。

| #   | 债务                                                                       | 影响                                             | 偿还时机                      | 状态 |
| --- | -------------------------------------------------------------------------- | ------------------------------------------------ | ----------------------------- | ---- |
| T1  | ~~`internal/api/v1` 全是 noop 桩~~ → 已撤桩，chat/tools/undo/debug 真实实现 | ~~前端两端只能跑骨架~~ → 双端可联调真实 SSE        | C 阶段                        | ✅   |
| T2  | CORS 白名单硬编码 5173/3001                                                | 换端口/加域名需改代码                            | C 阶段迁入 `infra/config`     | ⏳   |
| T3  | admin 产物 1MB（AntD 全量打包，无 code-split）                             | 首屏加载慢                                       | admin 功能稳定后按路由懒加载  | ⏳   |
| T4  | admin 权限矩阵页只读占位（无成员写端点）                                   | 无法在管理端改权限                               | C 阶段成员服务上线后          | ⏳   |
| T5  | JWT 认证未实现（当前 DevAuth：X-Member-ID header 明文）                    | 任意调用者可伪造成员身份触发工具                 | P1 紧随工具集之后             | ⏳   |
| T6  | ~~无 repository 抽象~~ → repo.Store 聚合 + 领域层依赖 ExpenseRepo 接口     | ~~领域层与存储耦合~~ → 依赖单向已断言             | C 阶段随 expense.Service 落地 | ✅   |
| T7  | MSW handlers 只有 `/health` 骨架                                           | 关键交互（撤销/危险确认/模型切换）尚无 mock 用例 | C 阶段 handler 真实化后补     | ⏳   |
| T8  | 无 CI（lint/test/typecheck 只能本地手动跑）                                | 坏模式可能在会话间漂移                           | 二期（先保证本地命令稳定）    | ⏳   |
| T9  | `docs/功能边界与迭代规划.md` 是跳转桩，内容已并入 AI-PRD §8                | 索引冗余                                         | 随下次文档整理清理            | ⏳   |
| T10 | `llm_usage.cost` 是 float（元），违反「金额一律 int64 分」                  | 成本统计精度损失，与账单领域不一致               | P1 用量埋点完善时改 int64 分  | ⏳   |
| T11 | 调试端点 `GET /debug/state` 及 repo→api/v1 临时反向依赖                     | 分层违规（store 依赖 api）；暴露内部表结构        | P1 删除（JWT 上线后无需要）   | ⏳   |
| T12 | `data/homeagent.db` 未 gitignore + 调试残留进程占 8080                     | 库文件误提交；端口冲突导致验收假失败              | 立即                          | ⏳   |

## 偿还记录

- **2026-09-16 · T1**：撤掉全部 noop 桩。`chat.go`（SSE 流式）、`undo.go`（24h 窗口 + member 隔离）、`router.go`（DevAuth 组）、`debug.go`（临时验收端点）落地；`main.go` 完成 repo→registry→executor→session→provider→runtime→handler 全链路 DI。
- **2026-09-16 · T6**：`internal/store/repo` 落地（Store 聚合 expense/conversation/undolog/audit/usage），领域层只依赖 `domexp.ExpenseRepo` 接口，编译期 `var _` 断言。顺带断开 `api/v1 → store/repo` 依赖边（v1 定义本地 `UndoStore`/`UndoRecord`，repo 适配）。
