---
name: homeagent-feature-loop
description: >-
  Autonomous end-to-end feature delivery workflow for the HomeAgent repository.
  Use when taking a high-level user idea or requirement from concept to verified,
  committed production code with minimal user intervention.
---

# HomeAgent 自主研发主闭环 (Autonomous Feature Loop)

本工作流定义了在 HomeAgent 仓库中，从用户一句高层想法到高质量、全自检、有视觉凭证的完整落地闭环。
用户扮演「首席架构师与产品总监」（定意图、做难点裁决、定取舍），Agent 负责全流程自主落地。

## 8 步自闭环阶段

```
[1. 意图解析与决策收敛] ──> [2. 领域模型与 PRD 契约同步] ──> [3. 契约先行 Codegen]
                                                                        │
[6. 视觉走查与全门禁] <── [5. 前端组件与交互] <── [4. 后端实体与工具落地]
         │
[7. 踩坑沉淀与自愈] ──> [8. 成果汇报与决策点呈报]
```

---

### 第 1 步：意图解析与决策收敛
- **原则**：不拿无谓的琐碎问题打扰用户；遇到分歧、架构取舍、关键体验路径时，使用 `ask_question` 工具提供预设选项让用户一键选择。
- 分析需求涉及的家庭生活上下文（财务/家务/厨房/出行/健康等）。

### 第 2 步：领域模型与 PRD 契约同步
- 在写业务代码前，必须先更新知识库：
  1. `docs/DOMAIN/家庭领域模型.md`（限界上下文、实体关系图、统一语言术语、联动规则、Eval 样本）。
  2. `docs/AI-PRD.md`（痛点、功能迭代版本、MVP 边界）。
  3. `docs/ARCHITECTURE.md`（工具清单、安全风险等级 A0-A4、幂等策略）。
  4. `AGENTS.md`（当前阶段状态）。

### 第 3 步：契约先行 Codegen
- **不可违反**：绝对不手写生成代码。
- 编辑 `api/openapi.yaml` 增加/调整接口契约与 Schema。
- 运行代码生成：
  ```powershell
  .\scripts\codegen.ps1
  ```
- 确认生成无误：`internal/openapi/api.gen.go`、`internal/store/ent/`、`web/src/api/schema.d.ts` 同步更新。

### 第 4 步：后端实体、领域与工具落地
- 严格遵循依赖单向原则：`api/v1 -> domain -> store`。
- **实体层**：修改 `internal/store/ent/schema/*.go`，注意：
  - 主键一律 UUID：`field.UUID("id", uuid.UUID{}).Default(uuid.New)`。
  - 金额一律 `int64` 分，禁止浮点数。
  - 必填 enum 字段必须显式处理或设默认值，防止 Save() 静默失败。
- **领域服务**：`internal/domain/<module>/service.go`。
- **工具层**：`internal/agent/tool/`，写操作工具强制实现 `Undoable` 接口与 24h 撤销。
- **HTTP 路由**：`internal/api/v1/`，使用 `memberIDFrom`、`abortWith`、`permissionOf`。

### 第 5 步：前端组件与交互设计
- **API 调用**：一律走 `src/api/client.ts`（openapi-fetch + `unwrap`），禁止手写原生 fetch。
- **金额处理**：展示使用 `formatYuanGrouped` / `formatYuan`，输入转换使用 `yuanToCents`。
- **视觉风格**：严格遵循 **Grok Bot 现代交互 × Apple HIG 极简美学**：
  - 苹果蓝主题高亮、浅灰磨砂背景、卡片式布局。
  - 底部悬浮毛玻璃胶囊输入框、状态胶囊（`✦ 正在思考…` / `⚡ 正在执行操作…`）。
  - 纯 CSS Modules，变量引用设计令牌。

### 第 6 步：全链路自动化门禁自检
- 运行仓库全量自动化门禁：
  ```powershell
  .\scripts\verify-all.ps1
  ```
- 门禁必须全绿通过：
  - [x] Go Build 0 编译错误
  - [x] Go Unit Tests 全部 Passed
  - [x] TS Typecheck 全工作区 0 错误
  - [x] Frontend Unit Tests 全部 Passed
  - [x] Playwright E2E 走查测试通过

### 第 7 步：视觉走查与自愈记录
- 若测试中发现环境异常或逻辑断点，启动自愈：
  1. 使用 `view_file` 检查 `web/e2e-shots/` 下最新截图。
  2. 遇到环境踩坑（如 Vite 重载、代理 502、编码问题），立刻查阅 `homeagent-env-troubleshooting` 并将新解法沉淀进文档。
- 保证无残留僵尸进程（运行 `.\scripts\harness-env.ps1`）。

### 第 8 步：成果汇报与决策点呈报
- 提交规范的语义化 Git Commit：
  `feat(finance): add income recording and monthly balance summary card`
- 向用户展示：
  1. 完成的核心变更清单。
  2. 走查凭证与关键截图信息。
  3. 需要用户决策的高层取舍点（如有）。
