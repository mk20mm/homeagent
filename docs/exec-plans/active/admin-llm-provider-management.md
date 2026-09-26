# AI 供应商与模型配置闭环重构 执行计划

> 对齐 **AI-STD-006（仓库 Harness）**。
> **目标**：彻底解决管理后台 AI 供应商与模型配置的表单 Bug 与能力缺失，实现从「预设选择 → 密钥配置 → ⚡连通性测试 → 模型 CRUD → 设为默认 → Web 对话即时生效」的完整全生命周期闭环。
> **前置**：V1.0-A 提交 0~4 已全量达成。

## 任务卡片分配与推进清单

| 任务卡片 | 交付层级 | 任务内容 | 状态 | 验收标准 |
| :--- | :--- | :--- | :--- | :--- |
| **T-AP01** | **契约先行** | `api/openapi.yaml` 补充测试连接、模型新增、模型删除接口 → `make generate` 同步 Go/TS 生成文件 | ✅ 已完成 | `openapi.yaml` 校验通过，Go 桩与 TS schema 自动生成无误 |
| **T-AP02** | **后端领域** | 领域服务实现 `TestConnection`（真机 Ping + 延迟测量）、`CreateModel`、`DeleteModel` 及 Handler 路由 | ✅ 已完成 | Go 单元测试通过，Parent 角色越权拦截，返回耗时与精准错误码 |
| **T-AP03** | **管理端 UI** | 重构 `admin/src/pages/AdminPage.tsx`：卡片式层级布局、模板预设一键填入、表单防自动填充、测试连接与模型 CRUD | ✅ 已完成 | 浏览器不再自动填错凭据，支持 Atria/DeepSeek/OpenAI 等预设，实时显示健康状态与延迟 |
| **T-AP04** | **闭环回归** | Admin 配置 Atria Dawn Preview 连通性测试 → 设为默认 → Web 端联调 → 全量测试回归 | ✅ 已完成 | `go test` + `typecheck` + `lint` + `vitest` + `build` + `docs:check` 全绿 |

## 决策日志

1. **测试连接策略**：后端使用通用 `CreateChatCompletion` 发送 1 token 轻量探测包，直接精准捕获 HTTP 401（Key 错）、402（欠费）、404（地址错）及网络延迟（ms），避免前端跨域探测与密钥外泄。
2. **表单安全与防自动填充**：在密码与 URL 表单上添加 `autoComplete="new-password"` 与防探测属性，阻断浏览器密码管理器错填账号（已彻底解决「爸爸/dev-baba」被错误填充进 API 配置的问题）。
3. **供应商与模型层级化**：打破原有的平铺表格，按 Provider 分组内嵌 Models，强化从属关系与快捷一键预设（Atria ASI / DeepSeek / OpenAI / 硅基流动 / Ollama）。
