# 骨架搭建执行计划（家事 Agent）

> 对齐 **AI-STD-006（仓库 Harness）**。

> **状态：✅ 已归档（2026-09-18）** —— 阶段 A（后端生成链路）与阶段 B（前端工作区）交付完成；阶段 C（运行时联通）由 [c-runtime.md](c-runtime.md) 细化并落地，本文件保留作为决策历史。

> 已锁定决策：UUID 主键 / 金额 Int 分 / Task 命名 / Category 表+预算字段；
> pnpm(npm i -g pnpm) / OpenAPI V1 完整端点 / Fake provider 跑通 / 先打通生成链路。
> 开始日期 2025-08 下旬（空仓库）；A/B 阶段已验收，进行 C 阶段。

## 目标

把"对话→执行"闭环的最小骨架立起来：契约驱动双端类型、数据库可建可迁、前端两端可跑通、
LLM 网关与 Agent runtime 可联通。不追功能完整，追**生成链路与反馈回路可靠**。

## 阶段 A：后端生成链路（契约 → 代码生成 → DB）✅

### A1. OpenAPI 契约 `api/openapi.yaml` ✅

端点（对齐 V1 工具清单 + 前端 5 页 + 管理端 3 页）：

- system: `GET /health` `GET /tools` `GET /models`
- chat: `POST /chat`（SSE）、`GET/POST /conversations`、`GET /conversations/{id}/messages`
- expense: `GET/POST /expenses`、`GET /expenses/summary`、`GET /budget`
- task: `GET/POST /tasks`、`POST /tasks/{id}/complete`
- meal: `GET/POST /meals`
- undo: `POST /undo/{id}`
- admin: `GET /audit` `GET /usage`

schemas: Health/Error/ToolSpec/RiskLevel/Model/ChatRequest/Conversation/Message/
Expense(+ExpenseCreate, amount_cents:int)/Task(+TaskCreate,TaskStatus 枚举)/
MealReport/AuditLog。错误码枚举对齐 apperr.Code。

### A2. oapi-codegen 生成 ✅

- `api/oapi-codegen.yaml`（generate: gin-server, strict-server, client, types）
- `internal/openapi/gen.go` 的 `//go:generate` 指令
- `.gitignore` 加例外：`!internal/openapi/*.gen.go`
- `go.mod` 加 `github.com/oapi-codegen/oapi-codegen/v2 v2.8.0` + `github.com/oapi-codegen/runtime`
- Makefile 加 `gen-api` 目标；`make generate` 同时跑 ent + openapi

### A3. ent 14 表 schema ✅

拆 `schema.go` → 多文件 + 两个 Mixin：

- `mixin.go`: `TimeMixin`(created_at/updated_at/deleted_at)、`AppendMixin`(仅 created_at)
- `family.go` `member.go` `conversation.go`(+message) `expense.go`(+category)
  `task.go`(+task_template) `meal.go` `llm.go`(provider/model/usage) `undo.go` `audit.go`
- 全表 `field.UUID("id", uuid.UUID{}).Default(uuid.New)`
- expense.amount → `field.Int64`（分）；Chore→Task 改名（仅 schema.go 引用，安全）
- edges：family→member、member→conversation→model、conversation→message、
  member→expense→category、member→task、task_template→task、member→meal_report、
  provider→model→usage、member→undo_log/audit_log
- 索引：expense unique(idempotency_key) + index(member_id,occurred_at)；
  task index(assignee_id,status)；meal unique(member_id,date)；
  message index(conversation_id,created_at)；undo index(member_id,status,expires_at)；
  audit index(trace_id)、index(member_id,created_at)；usage index(model_id,created_at)
- `internal/store/ent/generate.go` 的 `//go:generate`（此前完全没有，make generate 空跑）
- `.gitignore` 加 `!internal/store/ent/generate.go`

### A4. SQLite 接入（纯 Go 驱动）✅

- `modernc.org/sqlite` v1.59（本机无 gcc，mattn/go-sqlite3 编译不过）
- `internal/store/store.go`：Open + schema 迁移（`--migrate` 建表）
- MustSeed：1 家庭 + 3 成员（爸爸/奶奶/孩子，带权限模板）+ 6 分类（食材/日用/外卖/出行/餐饮/其他，带 monthly_budget）
- main.go `--migrate` flag 解析

### A5. 验收 ✅

`make generate` → `go build ./...` → `make test` 全绿 →
`go run ./cmd/homeagent --migrate` 建库成功 → `curl /api/v1/health` 200。

## 阶段 B：前端工作区 + 类型生成链路 ✅

### B1. 根工作区 ✅

- `npm i -g pnpm`
- `package.json`（root，private:true）+ `pnpm-workspace.yaml`（web/admin/packages/*）
- `.gitignore` 补 `node_modules/` `dist/` `*.local`

### B2. `packages/shared` ✅

类型/枚举（RiskLevel/TaskStatus/错误码，与后端 openapi 生成结果对齐）、
`formatMoney(cents→元)`、常量。

### B3. API client 生成（openapi-typescript + openapi-fetch）✅

- `web/package.json` devDeps: openapi-typescript@7.13、openapi-fetch@0.17
- `web/src/api/schema.d.ts` 由 `openapi-typescript ../api/openapi.yaml -o` 生成
- `web/src/api/client.ts`：createClient + 错误码归一化（unwrap）
- **禁止手写 fetch**（CONVENTIONS-frontend §1）

### B4. `web/` 移动端骨架 ✅

Vite 8 + React 19 + TS 5.9.3 + PWA（vite-plugin-pwa）

- pages: Chat / Chores / Money / Meal / Settings（对齐 page-01~05 设计图）
- components: ResultCard（含撤销按钮）、TabBar
- stores: zustand（会话/登录态/成员），对话状态机 useReducer
  （idle/streaming/tool_running/error）
- hooks: useSSE（断线重连）
- styles/tokens.ts：design-system 令牌（#0A84FF/#7C3AED/圆角间距）+ elder 适老模式
- CORS：后端中间件放行 5173（web）与 3001（admin）

### B5. `admin/` 管理端骨架 ✅

Vite + React 19 + AntD 6.6.4，端口 3001

- layouts 侧栏；pages: Dashboard（用量/最近调用）/ Debug（工具调试台，走 /chat SSE 实测回执）/
  Admin（供应商模型清单 + 成员权限矩阵只读占位）/ Audit（游标分页 + 越权过滤）
- 对齐 web-01~03 设计图

### B6. 工程化 ✅

ESLint 10（flat config）+ Prettier + Stylelint；Vitest 5 + MSW 2（撤销交互用例已就位，handlers 骨架含 /health）；
Makefile 加：`gen-web`（双端 openapi-typescript 重新生成）、`lint-web`、`typecheck-web`、`test-web`。

### B 验收 ✅

`pnpm -r run typecheck/lint/test/build` 与 `pnpm run stylelint/format:check` 全 exit 0；
web 产物 331KB（PWA 6 文件预缓存）、admin 产物 1MB；后端启动 `/health` → ok。

## 阶段 C：运行时联通（生成链路通了之后）⏳

任务清单与验收用例见 **[c-runtime.md](c-runtime.md)**（按 P0/P1/P2 排定优先级，`passes` 字段随实现翻转）。

- [ ] LLM 网关 `internal/agent/gateway/`：Provider 接口（Chat/Stream/Embedding）+ go-openai 适配 + fake provider（不真连供应商跑通链路）+ 用量埋点写 `llm_usage` 表
- [ ] Agent runtime `internal/agent/`：session/prompt 管理、ReAct 循环（危险分级控制 MAX_TURNS）、
      结果回流 card
- [ ] api/v1 真实 handler：chat SSE（客户端断开触发取消）+ JWT + undo 窗口校验
- [ ] main.go 撤 noop 桩，注入真实 expense.Service
- [ ] repository 层：expense/task/meal 的 ent 查询封装（领域层依赖接口，不直接碰 ent client）
- [ ] 一期 9 工具全实现（record_expense 骨架已有，其余 8 个新建）
- [ ] 评测：`evals/` 跑「买菜 120」「报饭」等真实任务回放（不真调供应商）

## 决策日志

| 日期       | 决策                                                                              | 被否方案与理由                                                                                      |
| ---------- | --------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| 表设计阶段 | 主键全 UUID；金额 Int 分；Chore→Task 改名；Category 独立表 + monthly_budget_cents | 自增 ID（未来分库分表痛）；float 存金额（精度漂移，财务系统大忌）；内联分类字符串（无法做预算聚合） |
| A2         | oapi-codegen **v2.8.0**                                                           | v1 已废弃（停止维护，输出路径语义不同）                                                             |
| A4         | `modernc.org/sqlite` 纯 Go 驱动                                                   | mattn/go-sqlite3 需 CGO，本机无 gcc 编译不过                                                        |
| B1         | pnpm 12.4.2 工作区                                                                | npm/yarn 无硬链接，node_modules 重复占空间                                                          |
| B3         | openapi-typescript 7.13 + openapi-fetch 0.17                                      | 手写类型 + axios（契约漂移无保护，违反不变量 5）                                                    |
| B4/B5      | TS 锁 5.9.3；React 19 + react-router 8                                            | TS 7.x 与 openapi-typescript 7.13 的 `ts.factory` API 不兼容（实测）                                |
| B5         | admin 用 AntD 6.6.4；web 用自写组件                                               | 移动端 AntD 太重且样式难覆盖设计图；管理端表单多，AntD 值                                           |
| B6         | ESLint flat config（eslint.config.mjs）                                           | eslintrc 格式 ESLint 10 起不支持                                                                    |
| 全程       | Fake provider 跑通 SSE，不真连 LLM                                                | 真接 Key 会把骨架验证与供应商可用性耦合，C 阶段再接                                                 |

## 已识别风险（提前处理）

1. 本机无 gcc → SQLite 只能 modernc 纯 Go 驱动
2. `.gitignore` 的 `internal/store/ent/*.go` 会误忽略 generate.go → 加 `!` 例外
3. ent generate 指令不存在 → make generate 目前空跑，必须补
4. CORS 白名单只有 5173/3001 → 前端两端口都要在
5. oapi-codegen v1 已废弃 → 用 v2.8.0（github.com/oapi-codegen/oapi-codegen/v2）

## 不做（本轮边界）

真实 LLM 接入（C 阶段）、repository 完整实现、业务工具全集、Playwright E2E（二期）
