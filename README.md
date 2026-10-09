# 家事 Agent（HomeAgent）

仓库：https://github.com/mk20mm/homeagent

> 一个以 AI 对话为主入口的家庭协作中枢：家人说一句话，Agent 自动把家务、用餐、日程、账单、出行与生活检索安排到位，支持多步骤跨域编排、主动提醒与定期复盘。

---

## 本地极速启动

```bash
git clone https://github.com/mk20mm/homeagent.git
cd homeagent

# 1. 契约生成与数据库迁移
make generate   # ent ORM + OpenAPI Go 桩 + 前端 TS Schema 自动生成
make migrate    # 建库 + 种子数据（SQLite 纯 Go 驱动，无需 gcc）

# 2. 启动 Go 后端服务（端口 :8080）
go run ./cmd/homeagent

# 3. 启动前端（pnpm 工作区）
pnpm install
pnpm -r run dev # web 移动端 PWA :5173 / admin 管理端 :3001
```

---

## 这是什么

面向单家庭自用的智能协作中枢。不让家人多学一个“管理系统”，而是把所有高频家庭事务都收拢进一个统一对话交互中枢——**动嘴一句话，系统全跑妥**。

---

## 核心特性

- **动嘴即办（100% 免费语音转文字）**：
  - **双引擎零成本**：集成浏览器原生 `Web Speech API`，毫秒级实时流式出字，无需购买任何云端语音 Key；备选纯 C++ `whisper.cpp` 离线私有化服务，家庭私密语音 100% 不离家。
  - **iOS 18 拟态磨砂抽屉**：对齐 Google Stitch 设计，内置 20 柱 Web Audio FFT 动态频谱动画、60s 防呆计时与一键填入。
- **智能搜索与权威信源（Smart Search）**：
  - **全网真实抓取**：免商业 Key 实时爬取 DuckDuckGo 真实公网网页与维基百科 OpenSearch，彻底消除 404 死链，目标网址 100% 浏览器秒开。
  - **安全隔离防护**：外部不可信抓取数据强制通过 `[EXTERNAL_UNTRUSTED_SEARCH_DATA]` 注入清洗，防御 Prompt Injection。
  - **Stitch 视觉对齐卡片**：绿色官方核验徽标、数字信源胶囊与安全外链跳转。
- **Durable 智能体运行时与无限思考画布（Infinite Canvas）**：
  - **Pi-Agent 级执行保障**：执行图持久化检查点（Checkpointing）、长上下文自动压缩（Compaction）与系统宕机断点恢复（Crash-Recovery）。
  - **动态思考拓扑画布**：三栏式交互、多轮双向 Ack 确认、步骤状态节点可视化与执行轨迹回放。
- **跨域多步骤调度中心（Dispatch Center）**：
  - 支持多领域动作编排（如“做完红烧肉后提醒洗碗并记账”），严格有向无环图校验与幂等放行。
- **对话即执行与可撤销机制**：
  - 领域写操作全量记录 `undo_log`，支持 24 小时内本地极速一键撤销与实时回滚。
- **出行与爱车台账（Travel）**：
  - 加油、充电、停车、维保、路桥通行一键记录，费用实时自动汇入家庭收支总账本。
- **多模型无缝切换**：
  - 对话窗口内一键切换已启用的 AI 模型（DeepSeek、GPT-4o、Claude、本地 Ollama 等），不绑定单一模型服务商。

---

## 模块全景一览

| 模块 | 说明 | 现状 |
| :--- | :--- | :--- |
| **AI 对话中枢** | 统一助手、流式 SSE、多模型切换、结果卡片、双向可撤销 | ✅ 生产就绪 |
| **语音转文字 (STT)** | 浏览器 Web Speech API 实时识别 + whisper.cpp 本地离线备选 | ✅ 生产就绪 |
| **智能搜索** | DuckDuckGo / 维基百科免 Key 实时全网检索 + 权威信源卡片 | ✅ 生产就绪 |
| **Durable 运行时** | 检查点恢复、日志压缩、三栏无限思维拓扑画布、双向 Ack | ✅ 生产就绪 |
| **跨域调度中心** | 多步骤任务编排、拓扑依赖图、前置放行条件判定 | ✅ 生产就绪 |
| **家庭财务** | 收入记账、支出记账、月度预算对比、收支综合看板 | ✅ 生产就绪 |
| **出行爱车** | 油费/电费/停车/保养/高速记录，里程记录，自动入账 | ✅ 生产就绪 |
| **家务分工** | 任务库、指派认领、打卡打标、催办提醒 | ✅ 生产就绪 |
| **家庭用餐** | 晚餐建议推荐、家庭报饭统计、菜谱与买菜联动 | ✅ 生产就绪 |
| **知识库与扩展** | 个人与家庭知识库 RAG、Agent Skills 扩展、MCP 协议网关 | 规划中（Phase E） |

---

## 技术架构

| 分层 | 选型与说明 |
| :--- | :--- |
| **后端语言** | **Go 1.24+**（高并发 goroutine，天然契合流式 SSE、长连接与后台调度） |
| **Web 框架** | **Gin**（轻量健壮，单向分层依赖 `api/v1 → domain → store`） |
| **ORM / 存储** | **ent**（类型安全，Schema 即代码）+ **SQLite**（纯 Go 驱动 `modernc.org/sqlite`，无 gcc 依赖） |
| **契约驱动** | **OpenAPI 3.0 先行**：`api/openapi.yaml` 驱动双端代码自动生成（oapi-codegen + openapi-typescript） |
| **前端框架** | **React 19 + TypeScript 5.9** + **Vite 8** + **Zustand**（PWA 移动端优先） |
| **设计规范** | **Apple iOS 18 玻璃拟态设计令牌** + **Google Stitch** 视觉一致性 |
| **测试套件** | **Vitest**（组件与状态测试）+ **MSW**（契约 Mock）+ **Go Test**（领域与工具测试） |

---

## 规范与路线图

- **项目总纲与规范地图**：[`AGENTS.md`](AGENTS.md)
- **产品需求规范**：[`docs/AI-PRD.md`](docs/AI-PRD.md)
- **能力扩展主计划**：[`docs/exec-plans/active/capabilities-expansion.md`](docs/exec-plans/active/capabilities-expansion.md)
- **设计技术规格**：
  - [语音转文字深度设计](docs/specs/01-voice-stt-design.md)
  - [智能搜索深度设计](docs/specs/02-smart-search-design.md)
  - [个人与家庭知识库设计](docs/specs/03-knowledge-base-design.md)
  - [Agent Skills 扩展体系设计](docs/specs/04-agent-skills-design.md)
  - [MCP 协议集成设计](docs/specs/05-mcp-integration-design.md)
