# 执行计划 · 家庭财务模块深化（P0 + 年月时间维度）

> 状态：**COMPLETED ✅** · 负责人：Lead Orchestrator + Subagents · 阶段：**Phase D**
> 核心目标：落地收入记录 + 年月时间选择器 + 财务总览卡片（结余/收/支）+ 收支合并流水，实现家庭财务完整闭环。

---

## 一、 任务拆解与优先级 (WBS)

| 阶段 | 任务编号 | 任务名称 | 责任 Agent | 交付产物 | 状态 |
|---|---|---|---|---|---|
| **P0-1** | T-FIN-01 | OpenAPI 契约更新与 Codegen | Spec Architect | `api/openapi.yaml`, `api.gen.go`, `schema.d.ts` | **已完成 ✅** |
| **P0-2** | T-FIN-02 | Ent 实体与数据库迁移 (`Income`) | Backend Dev | `internal/store/ent/schema/income.go` | **已完成 ✅** |
| **P0-3** | T-FIN-03 | 仓储层实现 (`IncomeRepo` + 年月过滤) | Backend Dev | `internal/store/repo/income.go`, `repo/expense.go` | **已完成 ✅** |
| **P0-4** | T-FIN-04 | 领域服务与工具 (`record_income` + Undo) | Backend Dev | `internal/domain/expense/`, `internal/agent/tool/` | **已完成 ✅** |
| **P0-5** | T-FIN-05 | HTTP API 路由与处理器 (`/incomes`, `/finance/summary`) | Backend Dev | `internal/api/v1/incomes.go`, `finance.go`, `router.go` | **已完成 ✅** |
| **P0-6** | T-FIN-06 | 前端 TabBar 与年月切换器组件 | Frontend Dev | `TabBar.tsx`, `MonthPicker` (胶囊+弹窗) | **已完成 ✅** |
| **P0-7** | T-FIN-07 | 前端总览卡片升级（结余大字 + 收支双柱） | Frontend Dev | `MoneyPage.tsx`, `MoneyPage.module.css` | **已完成 ✅** |
| **P0-8** | T-FIN-08 | 前端双模记账浮层（收支切换 + 日期补记） | Frontend Dev | `MoneyPage.tsx` | **已完成 ✅** |
| **P0-9** | T-FIN-09 | 前端收支合并流水明细与筛选 | Frontend Dev | `MoneyPage.tsx` | **已完成 ✅** |
| **P0-10**| T-FIN-10 | 全量质量门禁自检与 E2E 走查 | QA Auditor | `scripts/verify-all.ps1`, Playwright 截图 (11/11 tests) | **已完成 ✅** |
| **P0-11**| T-FIN-11 | 主项目同步与成果汇报 | Lead Orchestrator | `D:\main\company_projects\sun\007\ai_agent` 同步 | **已完成 ✅** |

---

## 二、 关键设计决策记录 (ADR / Decision Log)

1. **统一财务收支总览接口**：设计 `GET /finance/summary?year=2026&month=9`，一次性返回 `total_income_cents`, `total_expense_cents`, `balance_cents`，避免前端并发两次请求产生时序撕裂。
2. **时间维度支持全年与单月**：
   - 当 `period=month` 时，传入 `year=YYYY&month=M`。
   - 当 `period=year` 时，传入 `year=YYYY`，统计整年累计收支。
3. **收入记录 24h 撤销与幂等约束**：
   - 幂等键：`sha256(member_id + amount_cents + source + day)`。
   - 撤销机制：写 `undo_log`，撤销软删除对应 `income` 记录。
