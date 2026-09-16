# 后端代码规范（Go）

> 对齐 AI-STD-006（智能体式编码）、AI-STD-003（架构）。目标：让 Agent 能自主导航、让人能放心审计。

---

## 1. 目录结构（单体单仓）

```
ai_agent/
├── cmd/homeagent/main.go        # 唯一入口，只做装配
├── internal/
│   ├── api/                     # L3 应用层：HTTP handler、中间件
│   │   ├── v1/                  # /api/v1 路由（OpenAPI 生成桩）
│   │   └── middleware/          # auth(jwt) / permission(矩阵) / recover / cors
│   ├── agent/                   # L4 AI 层（核心）
│   │   ├── runtime.go           # ReAct 循环（步数上限、超时、取消）
│   │   ├── session.go           # 会话管理
│   │   ├── prompt.go            # 提示词构建（权限内工具注入）
│   │   ├── gateway/             # LLM 网关：provider 适配 + 用量埋点
│   │   ├── tool/                # 工具层：注册表 + 执行器
│   │   │   ├── registry.go
│   │   │   └── executor.go      # 权限校验→参数校验→执行→undo_log→审计
│   │   └── card.go              # 结果卡片
│   ├── domain/                  # L5 业务层：家务/财务/日程/购物/用餐/系统
│   │   ├── chore/  expense/  meal/  ...
│   │   └── service.go           # 每个 service 一个接口 + impl
│   ├── store/                   # L6 数据层：ent schema + repository
│   ├── infra/                   # 配置、日志、加密、cron
│   └── openapi/                 # oapi-codegen 生成的 server 桩（勿手改）
├── docs/                        # 见 docs/README.md
├── evals/                       # 评测套件（AI-STD-005）
├── api/openapi.yaml             # 契约先行，唯一真相源
├── go.mod
└── Makefile
```

**规则**：

- `internal/` 外不写业务逻辑；`cmd/` 只装配依赖、启动服务。
- **禁止跨层反向依赖**：`domain` 不得 import `api`；`agent` 不得 import `store` 之外的 L6 细节（通过 repository 接口）。
- OpenAPI 是接口唯一真相源，改接口先改 `api/openapi.yaml`，再 `make generate`。

---

## 2. 分层与依赖方向

```
api(handler) → agent → domain(service) → store(repository 接口)
                  ↓
              gateway → 外部 LLM
                  ↓
              tool/executor → undo_log + 审计
```

- 每层只依赖下一层的**接口**，不依赖实现（依赖注入在 `cmd/main.go` 装配）。
- 业务 service **不直接暴露给 LLM**（ARCHITECTURE §2.4）；只有 tool 层能被 Agent 调用。

---

## 3. 工具层规范（项目核心，最严格）

每个工具实现 `tool.Tool` 接口：

```go
type Tool interface {
    Spec() Spec                  // name / description / inputSchema / risk / permission
    Execute(ctx, Input) (Result, error)
}

type WriteTool interface {       // 写操作必须实现，编译期约束（ADR-004）
    Tool
    Undo(ctx, undoData) error
}
```

**MUST**：

- **写操作必须实现 `Undo`**，否则编译不过（`WriteTool` 约束）。
- 参数在**可信代码**用 JSON Schema 校验，禁止把 LLM 输出直接透传给 service。
- 执行链路固定顺序：`权限校验 → 参数校验 → 执行 → 写 undo_log → 写审计`，由 executor 统一包办，工具实现里**不得**自己写日志替代审计。
- 工具命名：snake_case，动词开头（`record_expense` / `assign_task`）。
- 幂等键：写工具必须声明幂等维度（如 `record_expense` = user+amount+hint+day）。

**MUST NOT**：

- 工具实现里直接操作 `*ent.Client`，必须走 domain service。
- 高风险（财务/权限）工具省略 `Preview` 能力（A3 级，见 PRD §4）。

---

## 4. 错误处理

```go
// internal/apperr/apperr.go
type Error struct {
    Code apperr.Code   // ErrInvalidInput / ErrPermission / ErrNotFound / ErrInternal
    Msg  string        // 面向用户（中文）
    Cause error        // 原始错误（仅日志，不外泄）
}
```

- **错误三段式**：Code（程序判断）/ Msg（给用户）/ Cause（给日志）。
- handler 统一把 `apperr.Error` 翻译为 HTTP 状态 + 错误体；未知错误一律 500 + `系统内部错误`，**禁止把堆栈/SQL 泄漏给前端**。
- service 层返回 `apperr`，不返回裸 `error`；`fmt.Errorf` 只用于包裹 cause。
- **不要忽略 error**，`_ =` 需注释说明为何忽略。

---

## 5. 并发与上下文

- 所有 service 方法首个参数必须是 `ctx context.Context`。
- Agent 循环用 `ctx` 承载超时与取消（max turns 15/8/3 + 全局超时）。
- SSE 流式输出：一个会话一个 goroutine，通过 channel 推事件；**客户端断开必须触发 ctx 取消**，防止泄漏。
- 共享状态（会话运行时）用 mutex 保护，禁止 goroutine 间共享可变 map 不加锁。

---

## 6. 数据层（ent）

- Schema 定义在 `internal/store/ent/schema`，字段加 `Optional()`/`Default()` 显式声明。
- **软删除统一**：需要撤销的实体（账单、任务）带 `deleted_at`，撤销 = 软删除回滚（配合 undo_log）。
- 事务：跨实体写操作（如记账+分类联动）必须 `WithTx` 包裹。
- 迁移：`make migrate`，禁止手改线上库；schema 变更先提 ADR。

---

## 7. 命名与风格

- 包名小写单词（`expense` 非 `ExpenseService`）；导出符号 PascalCase；私有小驼峰。
- 文件名小写下划线（`record_expense.go`），与工具名一致。
- 接口名优先用业务名而非技术名（`ExpenseRecorder` 而非 `IRecord`）。
- 常量用 const，魔法数字必须有具名常量（如危险分级步数 `MaxTurnsHigh = 3`）。
- `gofmt` + `goimports` 强制；`golangci-lint` 跑 `errcheck/govet/staticcheck/ineffassign`。

---

## 8. 测试与评测

| 层        | 测试                              | 命令              |
| --------- | --------------------------------- | ----------------- |
| 工具/业务 | 单元测试（表驱动）                | `make test`       |
| API       | httptest 走 OpenAPI 桩            | `make test-api`   |
| Agent     | 录制 LLM 响应回放（不真打供应商） | `make test-agent` |
| Eval      | 20-50 真实任务金标准              | `make eval`       |

- 工具测试**必须覆盖撤销链路**（执行→撤销→状态回滚）。
- 权限测试：每个工具至少一条越权拒绝用例（安全否决项不得被平均分抵消，AI-STD-005）。
- 禁止测试里真调外部 LLM（用 fake provider）；CI 跑 eval smoke。

---

## 9. 日志、配置、审计

- 日志：`slog` 结构化，含 trace_id；敏感字段（金额明细、API Key）**默认脱敏**。
- 配置：`internal/config`，读环境变量 + 可选 yaml；**密钥只从环境/加密存储读，不进 git**。
- 审计：涉钱/权限操作经 executor 落审计表，只增不改；撤销操作也留痕。

---

## 10. PR 与 Definition of Done

- 一个改动一个 PR；跨模块改动先提 ADR。
- DoD：单测绿 + lint 过 + 涉及工具的 eval smoke 过 + 涉及 schema 的迁移脚本附上。
- **Agent 不得自己批准自己的高风险改动**（AI-STD-006）；R3+ 变更需人审。
