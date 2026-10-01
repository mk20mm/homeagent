# Gemini 实施交接：HomeAgent 第一步

> 日期：2026-10-01。本文是实施委托规格；当前会话仅交付设计，不改业务代码、不安装 HA、不触发真实设备、不发送其他聊天消息。

## 1. 目标与权威文档

实施财务、家务、厨房三域的真实家庭执行助手。先读：

1. 根 AGENTS.md 与 `docs/README.md`。
2. [本轮架构](homeagent-v1-architecture.md)：产品行为、模型/代码边界、个人设备接入、事务/外部副作用、API/工具和数据模型。
   先读配套 [调度中心主设计](agent-dispatch-center.md)：中心是核心，三域是其能力；直接调用/复合计划/修改控制分流，两类循环与完成条件按此实现。
3. [验收规格](homeagent-v1-acceptance.md)：逐项登记证据，未执行不可标通过。
4. [ADR-007](ADR/ADR-007-本地撤销与设备补偿.md)：本地撤销和设备补偿；原 ADR-004 不能强套在机器人清扫上。
5. 现有前后端规范、设计令牌和契约生成技能，按实际修改范围阅读。

本轮架构优先于旧“报饭→推荐菜单”核心规划，报饭不扩大、旧数据保留；不把健康、学业、出行扩建带进第一阶段。不为了架构完整提前做通用 Agent 平台、插件商店、向量库或多 Agent。

## 2. 开始前确认事实

- 检查 Git 工作区，保留用户和其他开发者改动，不 reset/clean；不要把设计文档状态改为实现完成。
- 对照 main.go 的真实工具注册和 API 路由检查已有能力，不按旧文档里✅假定完成。现有厨房页默认菜谱是本地常量，仓库无 Recipe schema，需真正持久化。
- 设备信息尚未确定时仍可实施软件和 Fake HA 协议测试；实机适配需要实际型号/固件/HA版本/支持实体/动作，未提供就列待条件。
- 家庭管理员可以配置自己的 HA URL/token；品牌个人账号配对由本人完成，不替他申请企业开放平台资质，不为了接入破解固件或移除既有配对。

## 3. 实施拆分

### B0：先把执行底座做可靠

- 修 runtime 风险字符串比较、同批 stopped 不停止、StreamEvent.Err 忽略、固定 adult/空姓名、必要错误吞掉。
- 将 local_write 与 device_command 分开；本地事务 UoW 覆盖业务/undo/audit/步骤成果/关键事件；设备派发不在 SQL 事务内等待。
- request_id + operation_key，旧内容日维度键迁移为兼容数据字段，新动作不错误合并。
- AgentRun/RunStep/RunEvent、run_id 返回、持久化重放与版本冲突；pending/unknown 的恢复规则。
- 按架构普通历史记账是明确可逆内部写，取消原 PRD 与代码的统一财务高风险误分类；支付仍不开放。

通过 A01–A12 再推进领域动作。

### B1：财务

- 复用 expenses/incomes/categories，金额分和家庭时区；修正、撤销、合并流水/家庭授权汇总、预算。
- 固定账单 rule/occurrence、月/年月底边界、重复扫描幂等；到期只生成待办，已支付明确确认后才记实际支出。
- 对话与页面快捷入口共用应用命令，输出同一实体/undo_id；按 F01–F12 验证。

### B2：家务与设备

- 人工 Task 复用并扩展；新增 DeviceBinding/Job、Provider 和 HA Adapter；启用/禁用与设备动作权限。
- 仅 Home Assistant 与 Fake 两个适配器；REST 发 service、WS订阅状态，重连协调与有限轮询。
- 命令 queued/accepted/running/unknown/needs_confirmation 等状态；HTTP成功不标清扫完成；不假定 docked 就完成。
- 明确记录设备完成证据能力；暂停/返航为补偿，物理动作无 fake Undo；只允许已验证动作和范围。
- C01–C14 软件门禁；C15–C18 按实际硬件分别报告。不要用自己写 HA states 证明真机成功。

### B3：厨房

- 实际 Recipe/修订/会话快照，统一配料与步骤结构；搜索和显式份量缩放规则。
- KitchenProfile、CookingPlan/Session、简单确定性资源排程；人、灶、锅具和步骤依赖真实约束。
- 持久化步骤、timer expires_at、延长重排、多人协作、准备清单；大字指导可继续复用。
- K01–K15；计时到零不是熟了；后台声音不能保证，锁屏提醒不作为首阶段承诺。

### B4：联动与验收

- 完成调度中心有限复合计划、明确目标完成条件、输入输出引用、领域事件依赖放行、资源冲突和任务修订；新增中心用例D01–D18。不把submit_task_plan保存成功当作整件事执行成功。

- 用户明确采购金额才记账；清单估算不变支出。
- 做饭完成后的收尾按用户明确请求拆人/设备任务，已有结构化依赖足够，不引入全局流程编排框架。
- X01–X05、真实模型Eval、移动走查；全部结果矩阵与待条件项交付。

## 4. 契约冻结建议

所有新/改 API 先编辑 `api/openapi.yaml`，新实体先编辑 ent schema；运行 `scripts/codegen.ps1`，禁止手改 `*.gen.go/schema.d.ts`。路径建议已列于架构；如必须调整，保留行为语义并记录理由，不能略过结果状态/幂等/权限。

统一 request 写参数包含 `request_id:UUID`；修改含 `expected_version:int`；金额为 `amount_cents:int64`；时间 RFC3339、家庭本地日期另用 `YYYY-MM-DD`。外部设备 API 只收内置枚举，不收 URL/token/domain/service 自由参数。

统一成果结构示例（设计形状，先在 OpenAPI 形成正式 schema）：

```json
{
  "run_id": "UUID",
  "step_id": "UUID",
  "status": "committed",
  "summary": "已记食材支出 ¥68.50",
  "data": {"amount_cents": 6850, "version": 1},
  "entity_refs": [{"kind": "expense", "id": "UUID", "version": 1}],
  "effect": "local_write",
  "recovery": {"mode": "undo", "undo_id": "UUID"},
  "trace_id": "TRACE"
}
```

设备结果示例：`status=waiting_external`、`data.command_status=accepted`、`effect=device_command`、recovery 仅给已支持的 cancel/compensate 及动作；禁止返回 undo_id 表示能撤销清扫。Run API 的终态与聊天流 done 分开定义。

错误仍为 `{code,message,trace_id}`；新错误先改后端枚举/OpenAPI/前端中文映射。API写入202是异步受理，不是设备成功。HTTP与工具内部返回结构可不同，但映射后状态与实体必须一致。

## 5. 不得用替代物通过验收

- 假菜谱本地状态不能代替真实家庭菜谱；Fake设备不能代替实机。
- 不实现报饭催办、积分/排行榜、家庭广播流来充当核心价值。
- 不上企业 API、银行抓取、支付、外卖下单；不新增未获授权的设备动作。
- 不在模型里靠“请务必安全”代替代码权限/资源/金额校验。
- 不用无限重试或查不到结果便重发来掩盖外部副作用不确定性。
- 不给每个新字段造独立微服务/数据表/后台页面；JSON结构可校验则采用简洁存储。

## 6. 最终交付

交付可评审的差异、代码生成和迁移记录、测试矩阵（A/F/C/K/X/D）、移动截图、真实模型结果、设备兼容卡、配置步骤和限制。说明哪些为软件通过、HA协议通过、实机通过或待条件，不把总状态都写成✅。

更新 exec-plan 与技术债已偿还状态；改变行为时同步 ADR/PRD/DOMAIN，不覆盖设计历史。任何删开发库、丢未提交代码、迁移已有设备配对等破坏性动作必须由用户明确授权。
