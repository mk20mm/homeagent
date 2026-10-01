# Agent 驾驭工程设计入口

> 2026-10-01 用户修订：第一步聚焦财务、家务、厨房，当前仅设计，具体实施交给 Gemini。旧“报饭优先、六域同时扩建”路线已撤换；不删除现有报饭功能或数据。

## 权威交付

- [完整架构设计](homeagent-v1-architecture.md)：真实设备接入、执行与结果观察、家庭菜谱/步骤/多菜排程、账本/预算/固定账单、核心 Runtime、契约/数据/权限与恢复。
- [AI 调度中心主设计](agent-dispatch-center.md)：请求理解、能力装配、计划校验、派发、反馈、完成/调整，三域由中心串联。
- [完整验收规格](homeagent-v1-acceptance.md)：80个用例，包含18项调度协调；软件、HA协议、实机和真实模型分开出证据。
- [Gemini 实施交接](gemini-v1-handoff.md)：批次、契约生成、禁止替代物和交付清单。
- [设备恢复语义 ADR-007](ADR/ADR-007-本地撤销与设备补偿.md)：本地撤销与设备取消/补偿分离。

## 源码审阅基线

截至本轮静态阅读，现有 Runtime/Executor 为可演进底座，但以下缺口需由 Gemini 在 B0 修复：

| 源码 | 已有 | 缺口 |
| --- | --- | --- |
| `internal/agent/runtime/runtime.go` | ReAct/SSE、风险累计 | 字符串风险比较、停止后同批继续调用、忽略 StreamEvent.Err；末尾保存历史，缺真实成员上下文与 Run 恢复 |
| `internal/agent/tool/registry.go` | 权限、输入校验、undo/audit | 业务/撤销分开提交，错误被忽略；风险与写操作类型混用 |
| `cmd/homeagent/main.go` | 13个注册工具（2个隐藏） | 公开11个再按权限减少；不能将旧架构规划表视为已实现 |
| `internal/domain/expense/service.go` | 金额分、规则分类、日维度去重 | 同日同内容的两次真实动作可能被合并，新协议用 request/operation_key |
| `internal/store/ent/schema/task.go` | 人工任务和完成状态 | 尚无设备任务/命令、能力与观察证据 |
| `web/src/pages/KitchenPage/KitchenPage.tsx` | 大字做饭指导、报饭 API | 菜谱/搭配是本地常量，尚无真实 Recipe/CookingSession 与资源排程 |

本轮没有业务代码变更，也没有执行产品测试、接入设备或真实模型评测。
