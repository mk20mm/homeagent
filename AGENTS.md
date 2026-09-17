# AGENTS.md · 家事 Agent（HomeAgent）

> 本文是**地图，不是说明书**。智能体运行时读不到的东西（聊天记录、人脑里的知识）等于不存在。
> 深层真相都在 `docs/`，按需渐进式阅读。对齐 AI-STD-006（仓库 Harness）。

## 项目一句话

单家庭自用的 AI 协作中枢：家人说一句话，Agent 调用后端工具把家务/用餐/账单/日程办到位。
Go 后端（Gin + ent + SQLite）+ React 前端（web 移动端 PWA / admin 管理端）+ OpenAPI 契约驱动双端类型。

**当前阶段**：A/B 骨架已完成；C 阶段 **P0 关键路径 ✅ 9/9**、**P1 工具集+JWT+观测 ✅ 11/11**（9 工具 + JWT 认证 + 权限双保险 + 幂等 + go-openai 适配器 + GET /models·/audit·/usage）、**P2 前端双端联调 ✅**（JWT 登录守卫 + ChatPage 真实 SSE + undo_id 撤销闭环 + admin Debug/仪表盘/审计），进入 P2 剩余项（evals 评测套件、会话历史接口、账单/任务 handler）。
进度与决策日志见 `docs/exec-plans/active/`，技术债见 `docs/tech-debt.md`。

## 不可违反的不变量（改任何代码前先读）

1. **契约先行**：改接口先改 `api/openapi.yaml` → `make generate` → Go 桩与 TS schema 同步生成。禁止手改生成文件（`*.gen.go`、`**/schema.d.ts`）。
2. **主键全 UUID；金额一律 `int64` 分**。展示用 `packages/shared/money.ts` 的 `formatYuan*`，输入用 `yuanToCents`。禁止 float 存金额。
3. **写操作全量可撤销**（ADR-004）：工具必须实现撤销接口，`undo_log` 记录，24h 窗口。
4. **权限双保险**（ADR-005）：工具注册表按成员角色源头过滤 + 运行时再校验。`GET /tools` 只返回该成员能调的。
5. **禁止手写 fetch**：API 调用一律走 `src/api/client.ts`（openapi-fetch + `unwrap`）。SSE 流式走 `useSSE` hook。
6. **边界处解析数据形状**：外部输入在进入领域层前必须被解析/校验（`unwrap`、ent schema 约束），领域层不防御。
7. **分层依赖单向**：`api/v1 → domain → store`；`infra/config` 横切；`store` 禁止反向依赖 `domain`。横切关注点走显式 Provider 接口。
8. **错误三段式**：`{code, message, trace_id}`，code 取 `apperr.Code` 枚举，前端用 `ERROR_MESSAGE` 映射中文。

## 目录地图

```
api/openapi.yaml            # 契约：接口唯一真相源
internal/
  api/v1/                   # HTTP handler（当前 noop 桩，C 阶段撤）
  domain/expense/           # 领域服务
  store/ent/schema/*.go     # ent 14 表 schema（9 个领域文件 + 2 mixin）
  openapi/api.gen.go        # 生成（勿改）
  agent/tool/               # 工具注册表 + 接口约束
cmd/homeagent/main.go       # 入口，--migrate flag
web/   admin/               # 前端两端口（5173 / 3001）
packages/shared/            # 前后端共享枚举 + 金额工具
docs/                       # 知识库（见 docs/README.md 地图）
docs/exec-plans/            # 执行计划（一等公民，带状态与决策日志）
docs/tech-debt.md           # 技术债登记
```

文档阅读顺序见 `docs/README.md`（新人 30 分钟 / 架构评审 / 产品评审 / 安全评审）。
「为什么这么选」在 `docs/ADR/`。领域术语在 `docs/DOMAIN/家庭领域模型.md`。

## 常用命令

```bash
# Go（PowerShell 下 make 不可用，直接用 go）
make generate          # ent + oapi-codegen + 前端 schema 全量生成
make migrate           # 建库 + 种子数据（data/homeagent.db）
make test              # go test ./...（无 -race：本机无 gcc）
go run ./cmd/homeagent # 启服务，:8080，/api/v1/health

# 本地联调认证（JWT）：--migrate 打印 name+auth_token，再换令牌
curl -X POST http://localhost:8080/api/v1/auth/token \
  -H 'Content-Type: application/json' \
  -d '{"name":"爸爸","auth_token":"dev-baba"}'   # → token，后续 Authorization: Bearer <token>
# 真实 LLM：LLM_API_KEY + LLM_MODEL + LLM_BASE_URL 环境变量；留空则用脚本供应商

# 前端（pnpm 工作区）
pnpm -r run typecheck  # 全部类型检查
pnpm run lint          # ESLint + Stylelint
pnpm -r run test       # Vitest + MSW
pnpm -r run build      # 双端构建
pnpm run format        # Prettier
```

## 本机环境注意（踩过的坑）

- **无 gcc** → SQLite 只能用 `modernc.org/sqlite`（纯 Go）。驱动名是 `sqlite`，ent dialect 用 `dialect.SQLite`（"sqlite3"）。正确姿势见 `internal/store/store.go`。
- **PowerShell 5.1**：无 `&&`，链式用 `;` 或 `if ($?)`。**写中文文件必须用 write 工具**，`Set-Content` 会把 UTF-8 中文写成 GBK 乱码。
- **Go 环境每次新 shell 要设**：`$env:Path = "$env:SCOOP\apps\go\current\bin;$env:PATH"; $env:GOPROXY = 'https://goproxy.cn,direct'`。
- **oapi-codegen 的 output 路径相对 CWD**（不是配置文件目录）→ `make gen-api` 会 `cd internal/openapi`。
- **TypeScript 锁 5.9.3**：7.x 与 openapi-typescript 7.13 的 `ts.factory` API 不兼容。
- **ent v0.14.6**：索引不支持 `.Comment()`；边 FK 列要用 `index.Fields("x").Edges("edge")` 组合。**enum 字段必填无默认值时，不设值会让 Save() 静默失败**（审计曾因此漏记撤销事件——`_ = err` 吞错是帮凶，审计/日志类调用必须显式处理错误）。
- **curl 验收用 body 文件**：PowerShell 传 JSON 给 `curl.exe -d` 转义易错（单引号内 `\"` 行为不稳）；`Set-Content -Encoding UTF8` 带 BOM 会导致 JSON 解析 400。正确姿势：`[IO.File]::WriteAllText($f, $json, (New-Object Text.UTF8Encoding $false))` 后 `-d "@$f"`。
- **启服务前先清残留进程**：`Get-Process go,homeagent | Stop-Process -Force`，否则旧二进制占 8080，新请求路由到旧服务（表现为 /chat 404、/tools 返回旧空清单）。推荐 `go build -o homeagent.exe` 后启二进制，避免 `go run` 编译期占端口。
- 本机活动代理是 airtcp（127.0.0.1:5780）；配置里若残留 7897（Clash Verge）是死端口。
- **前端 dev server 用 `Start-Process node -ArgumentList 'node_modules\vite\bin\vite.js'`**（Start-Process pnpm.cmd 会被工具的 ChildProcess.kill 回收）；web 5173 / admin 3001，`/api` 代理到 8080。
- **openapi-fetch 两个坑**：① baseUrl 必须是绝对 origin（`window.location.origin + '/api/v1'`），相对 URL 在无 document base 的环境（测试/SSR）会抛 `Failed to parse URL`；② client 在**模块导入期**捕获 `globalThis.fetch`，而 MSW 在 `beforeAll` 才 patch，捕获到的引用绕过拦截——用 `fetch: (...a) => fetch(...a)` 延迟到调用期解析。两者在浏览器里靠 document 兜底一直没暴露，接真接口写测试才炸。
- **临时探针测试**（连开发库查证数据用）：① 文件名**必须**以 `_test.go` 结尾，否则 `go test` 把它当普通源文件，与目录里的 `package store` 冲突报 `found packages store and store_test`；② 库相对路径按**包目录**算——从 `internal/store/repo` 到项目内库是 `../../../data/homeagent.db`（repo→store→internal→根），少写一层会指到项目根之外，曾在 `sun\007\data\` 误建过副本库（清密钥清错库，表现为接口仍显示已配置）。稳妥起见用绝对路径。

## 工作方式（Harness）

- 人定方向、定约束、验结果；执行交给智能体。
- 卡住时不要"更努力"，问：**缺什么能力/文档/约束，才能让智能体可靠地做出来？** 答案写进本文件或 docs/。
- 每次改动同步更新：计划状态（exec-plans）、新增不变量（本文 + CONVENTIONS）、技术债（tech-debt）。
- 发现坏模式立即偿还，技术债是高息贷款。
