# 家事 Agent（HomeAgent）

仓库：[https://github.com/mk20mm/homeagent](https://github.com/mk20mm/homeagent)

> **面向现代家庭的 AI 协作中枢**：家人动嘴说一句话，Agent 自动把家务分工、用餐安排、日程提醒、账单收支、爱车出行、个人与家庭知识检索统筹到位，支持多步骤跨域编排、深度语义决策、主动提醒与本地安全撤销。

---

## 📸 核心特性

### 1. ⚡ TypeSafe Jev (System One) 智能决策精排
- **两阶段智能检索架构**：第一阶段免 Key 全网公网检索（DuckDuckGo / 维基百科），第二阶段接入 **TypeSafe Jev System One** 语义概率决策模型，极速完成结果重排序与权威度评级。
- **按需显式触发，零冗余开销**：遵循「高阶模型按需触发」原则，默认零额外成本与延迟；在搜索信源卡片右上角提供明确的 `[⚡ Jev 智能精排]` 交互按钮。
- **垃圾营销强力压制与权威置顶**：实时识别低质 SEO 广告与营销号，将其置信度降至 1% 并沉底；权威干货、官方文档与高可信来源赋予 90%+ 置信度置顶呈现。
- **透明研判证据链与双向对比**：卡片展示直观的置信度百分比徽章与精排判定标签（`权威信源` / `低质营销·降权`），悬停可查看研判证据，支持一键切换「查看原始排序」进行双向还原对比。

### 2. 💬 豆包级交错思维流与结构化排版
- **对齐商业级顶级对话体验**：告别大模型单调晦涩的长文本堆砌，重构为高度结构化、层级分明、视觉呼吸感极佳的现代对话流。
- **交错穿插思维流（Thought Process）**：耗时统计状态栏（搭配优雅细虚线分隔），思考短句与微动作小药丸交错穿插（`🌐 检索全网信源...` / `⚡ Jev 语义研判...` / `📖 查阅家庭菜谱知识库...`），推理过程实时透明。
- **步骤清单卡片化（Structured Cards）**：长文本步骤智能卡片化，每一步骤独立封装为带柔和浅色背景的圆角卡片，左侧搭配深色圆形编号徽章（`1`、`2`、`3`），加粗核心步骤标题，并高亮具体操作导引。
- **核心建议 Callout 盒**：提炼出的重点防坑指南与核心建议以柔和淡青色底色配合左侧高亮边框醒目呈现，关键信息一眼即得。
- **快捷闭环行动胶囊（Action Pills）**：对话末尾自动附带可点击的快捷闭环按钮（如 `📝 计入本月开销`、`📅 生成提醒事项`），一键将对话结论落入具体家庭业务系统。

### 3. 🎙️ 动嘴即办（100% 免费语音转文字）
- **双引擎零成本接入**：集成浏览器原生 `Web Speech API`，毫秒级实时流式出字，无需购买任何云端商业语音 Key；备选纯 C++ `whisper.cpp` 离线私有化服务，家庭私密语音 100% 不离家。
- **iOS 18 拟态磨砂抽屉**：对齐 Google Stitch 现代移动端设计，内置 20 柱 Web Audio FFT 动态频谱动画、60s 防呆计时与一键填入输入框。

### 4. 🧠 Durable 智能体运行时与无限思考画布
- **Pi-Agent 级执行保障**：执行图持久化检查点（Checkpointing）、长上下文自动压缩（Compaction）与系统宕机断点恢复（Crash-Recovery）。
- **动态思考拓扑画布**：三栏式交互、多轮双向 Ack 确认、步骤状态节点可视化与执行轨迹回放。

### 5. 🔀 跨域多步骤调度中心（Dispatch Center）
- **全域多步骤联动编排**：支持“做完红烧肉后提醒洗碗并记账”等复杂跨域生活场景，具备严格的有向无环图（DAG）校验与幂等放行。

### 6. ⏪ 本地写操作全量可撤销（ADR-004）
- **家庭数据安全无忧**：领域本地写操作全量记录 `undo_log`，支持 24 小时内本地极速一键撤销与实时原子回滚。

### 7. 🚗 出行与爱车台账（Travel）
- **车生活无缝协同**：加油、充电、停车、维保、路桥通行一键记录，里程与能耗台账清晰，费用实时自动归集汇入家庭收支总账本。

---

## 📊 模块全景一览

| 模块 | 说明 | 现状 |
| :--- | :--- | :--- |
| **AI 对话中枢** | 豆包级交错思考流、步骤清单卡片化、快捷行动胶囊、多模型无缝切换、双向可撤销 | ✅ 生产就绪 |
| **智能搜索** | DuckDuckGo / 维基全网公网检索 + **TypeSafe Jev (System One) 智能语义精排** | ✅ 生产就绪 |
| **语音转文字 (STT)** | 浏览器 Web Speech API 实时识别 + whisper.cpp 本地离线备选 + Stitch 频谱抽屉 | ✅ 生产就绪 |
| **Durable 运行时** | 检查点恢复、日志压缩、三栏无限思维拓扑画布、双向 Ack | ✅ 生产就绪 |
| **跨域调度中心** | 多步骤任务编排、拓扑依赖图、前置放行条件判定 | ✅ 生产就绪 |
| **家庭财务** | 收入记账、支出记账、月度预算对比、收支综合看板 | ✅ 生产就绪 |
| **出行爱车** | 油费/电费/停车/保养/高速记录，里程记录，自动入账 | ✅ 生产就绪 |
| **家务分工** | 任务库、指派认领、打卡打标、催办提醒 | ✅ 生产就绪 |
| **个人与家庭知识库** | 飞书双空间架构 (家庭共享 🌐 + 个人私密 🔒 物理隔离)、滑动窗口智能分块、SQLite FTS5 混合检索、24h 可撤销 | ✅ 生产就绪 |
| **Skills 与 MCP 网关** | Agent Skills 扩展体系、MCP 标准协议客户端与动态工具接入 | 推进中（Phase E） |

---

## 🛠️ 技术架构

| 分层 | 选型与说明 |
| :--- | :--- |
| **后端语言** | **Go 1.24+**（高并发 goroutine，天然契合流式 SSE、长连接与后台调度） |
| **Web 框架** | **Gin**（轻量健壮，单向分层依赖 `api/v1 → domain → store`） |
| **决策引擎** | **TypeSafe Jev (System One)**（轻量级语义决策打分，赋能实时智能搜索精排） |
| **ORM / 存储** | **ent**（类型安全，Schema 即代码）+ **SQLite**（纯 Go 驱动 `modernc.org/sqlite`，无 gcc 依赖） |
| **契约驱动** | **OpenAPI 3.0 先行**：`api/openapi.yaml` 驱动双端代码自动生成（oapi-codegen + openapi-typescript） |
| **前端框架** | **React 19 + TypeScript 5.9** + **Vite 8** + **Zustand**（PWA 移动端优先） |
| **设计规范** | **Apple iOS 18 玻璃拟态设计令牌** + **Google Stitch** 视觉一致性 |
| **测试套件** | **Vitest**（组件与状态测试）+ **MSW**（契约 Mock）+ **Go Test**（领域与工具测试） |

---

## 🚀 本地极速启动

### 1. 代码拉取与生成

```bash
git clone https://github.com/mk20mm/homeagent.git
cd homeagent

# 1. 契约生成与数据库迁移
make generate   # ent ORM + OpenAPI Go 桩 + 前端 TS Schema 自动生成
make migrate    # 建库 + 种子数据（SQLite 纯 Go 驱动，无需 gcc）
```

### 2. 环境变量配置（可选）

<<<<<<< HEAD
创建根目录 `.env` 文件（已纳入 `.gitignore`，安全不外泄）：

```ini
# 大模型配置（若留空则使用内置脚本供应商）
LLM_API_KEY=your_llm_api_key
LLM_MODEL=deepseek-chat
LLM_BASE_URL=https://api.deepseek.com/v1

# TypeSafe Jev 智能精排密钥（配置后即可体验 500ms 快速语义决策打分）
TYPESAFE_API_KEY=your_typesafe_api_key
```

### 3. 启动服务
=======

>>>>>>> origin/main

```bash
# 启动 Go 后端服务（端口 :8080）
go run ./cmd/homeagent

# 启动前端（pnpm 工作区）
pnpm install
pnpm -r run dev # web 移动端 PWA :5173 / admin 管理端 :3001
```

---

## 📖 规范与路线图

- **项目总纲与规范地图**：[`AGENTS.md`](AGENTS.md)
- **产品需求规范**：[`docs/AI-PRD.md`](docs/AI-PRD.md)
- **能力扩展主计划**：[`docs/exec-plans/active/capabilities-expansion.md`](docs/exec-plans/active/capabilities-expansion.md)
- **设计技术规格**：
  - [语音转文字深度设计](docs/specs/01-voice-stt-design.md)
  - [智能搜索深度设计](docs/specs/02-smart-search-design.md)
  - [个人与家庭知识库设计](docs/specs/03-knowledge-base-design.md)
  - [Agent Skills 扩展体系设计](docs/specs/04-agent-skills-design.md)
  - [MCP 协议集成设计](docs/specs/05-mcp-integration-design.md)
