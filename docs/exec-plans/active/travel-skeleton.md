# 执行计划 · M6 家庭出行模块骨架与 Tab 栏 5 入口（Phase D）

> 状态：**IN_PROGRESS ⏳** · 负责人：Lead Orchestrator + Subagents · 阶段：**Phase D**  
> 核心目标：落地底部 Tab 栏 5 入口对齐 + 爱车台账（车辆信息/里程/保养）+ 出行记录（加油/充电/停车/保养/高速）+ 自动联动家庭财务支出（交通/出行分类）。

---

## 一、 任务拆解与优先级 (WBS)

| 阶段 | 任务编号 | 任务名称 | 责任 Agent | 交付产物 | 状态 |
|---|---|---|---|---|---|
| **P0-1** | T-TRV-01 | TabBar 5 入口重构与设置收纳侧边栏 | Frontend Dev | `TabBar.tsx`, `ConversationSidebar.tsx` | **已完成 ✅** |
| **P0-2** | T-TRV-02 | OpenAPI 契约更新与全量生成 | Spec Architect | `api/openapi.yaml`, `api.gen.go`, `schema.d.ts` | **已完成 ✅** |
| **P0-3** | T-TRV-03 | Ent 实体与数据库迁移 (`Vehicle`, `TripRecord`) | Backend Dev | `internal/store/ent/schema/vehicle.go`, `trip.go` | **已完成 ✅** |
| **P0-4** | T-TRV-04 | 仓储层实现 (`VehicleRepo`, `TripRepo`) | Backend Dev | `internal/store/repo/vehicle.go`, `repo/trip.go` | **已完成 ✅** |
| **P0-5** | T-TRV-05 | 领域服务与 Agent 工具 (`record_trip` + Undo + 财务记账联动) | Backend Dev | `internal/domain/travel/`, `internal/agent/tool/` | **已完成 ✅** |
| **P0-6** | T-TRV-06 | HTTP API 路由与处理器 (`/vehicles`, `/trips`) | Backend Dev | `internal/api/v1/travel.go`, `router.go` | **已完成 ✅** |
| **P0-7** | T-TRV-07 | 前端移动端 `TravelPage` 页面与卡片组件 | Frontend Dev | `web/src/pages/TravelPage/`, `App.tsx` | **已完成 ✅** |
| **P0-8** | T-TRV-08 | 质量门禁自检 (Go Test + TS Check + Build) | QA Auditor | `scripts/verify-all.ps1` | **已完成 ✅** |
| **P0-9** | T-TRV-09 | 主项目双端同步与上线汇报 | Lead Orchestrator | `D:\main\company_projects\sun\007\ai_agent` 同步 | **进行中 ⏳** |

---

## 二、 关键设计决策记录 (ADR / Decision Log)

1. **Tab 栏 5 入口契约**：严格对齐 `AI-PRD.md §8.11`，为 `[💬对话] [✅家务] [💰财务] [🍳厨房] [🚗出行]`，将 `⚙️设置` 收入左侧豆包式抽屉底部。
2. **出行费用自动联动记账**：当用户或 Agent 记录加油、充电、停车、保养或高速费时，不仅在出行台账沉淀里程与详情，还会同时自动生成一条分类为「交通」的支出记录（金额以 `int64` 分存储），实现一次录入双向入账。
3. **撤销不变量支持**：`record_trip` 同样必须接入 `undo_log`，撤销时同时级联回滚生成的联动财务支出。
