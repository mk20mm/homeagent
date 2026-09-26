# 家事 Agent · 文档地图（docs）

> 对齐 **AI-STD-006（仓库 Harness）**：docs 应含文档地图、ARCHITECTURE、ADR/、DOMAIN/。
> 项目当前阶段：**Phase 02–03（AI PRD / 架构）**，对齐 AI-STD-001 生命周期 G2→G3 门禁。

---

## 目录结构

```
docs/
├── README.md                  # 本文件：文档地图与阅读顺序
├── HARNESS.md                 # Agent 自主研发与知识沉淀使用说明书（Spec Map+操作SOP+沉淀法则）
├── AI-PRD.md                  # 需求契约：六份契约 + §10 V1 功能需求（四模块深耕）
├── ARCHITECTURE.md            # 架构设计（对齐 AI-STD-003）
├── CONVENTIONS-backend.md     # 后端代码规范（Go）
├── CONVENTIONS-frontend.md    # 前端代码规范（React+TS，移动端+管理端）
├── agent-调度器设计.md         # Agent 运行时详细设计（核心，保留）
├── tech-debt.md               # 技术债登记（发现即偿还）
├── e2e-issues.md              # E2E 测试轮次记录与问题清单
├── product/                   # 产品设计研究（理念层 + 灵感层 + 扩展层 + UX 走查层）
│   ├── product-design-research-01.md  #   理念层：产品设计研究与演进方案（15 章 + 附录 A/B）
│   ├── product-design-research-02.md  #   扩展层：功能模块全景（七层）+ 四模块之外的增量模块论证（C17–C20）
│   ├── product-inspiration-01.md      #   灵感层：市面能力调研 + 四模块深耕标尺与能力候选
│   └── ux-exploration-01.md   #   第 1 轮（走查层）：记账闭环与「对话—页面孤岛」
├── exec-plans/                # 执行计划（一等公民，带状态与决策日志）
│   ├── README.md              #   状态约定与索引
│   ├── active/                #   正在推进
│   └── completed/             #   已归档（保留决策日志）
├── ADR/                       # 架构决策记录（对齐 AI-STD-003/006）
│   ├── README.md              #   ADR 索引与写作规范
│   ├── ADR-001-技术栈选型Go.md
│   ├── ADR-002-自建Agent调度器而非框架.md
│   ├── ADR-003-SQLite起步可迁移Postgres.md
│   ├── ADR-004-写操作全量可撤销.md
│   └── ADR-005-权限双保险.md
├── DOMAIN/                    # 领域知识（家庭领域模型）
│   └── 家庭领域模型.md
└── ui/                        # 可视化图表（mermaid 源码 + 渲染 PNG）
    ├── design-system.md       #   UI 设计系统
    ├── architecture-system.md #   系统分层架构图  (.png)
    ├── architecture-agent.md  #   Agent 调度器内部架构图 (.png)
    ├── sequence-dialog.md     #   对话执行时序图 (.png)
    └── roadmap-iterations.md  #   迭代边界图 V1 → 阶段 A–D (.png)
```

根目录另有 **`AGENTS.md`**——约 100 行的地图与不可变不变量，会被注入智能体上下文。本文档与它的分工：`AGENTS.md` 是「入口地图」，本文件是「文档内部详图」。

---

## 每个文件的作用

| 文件                          | 作用                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | 对齐标准       |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------- |
| **HARNESS.md**                | 人机自主研发操作手册与知识沉淀使用说明书：基于 OpenAI 契约先行与结构化 JSON 任务卡验收标准 + 极简使用姿势 + 知识分类、真伪检验、渐进瘦身法则                                                                                                                                                                                                                                                                                                                                                                              | AI-STD-006     |
| **AI-PRD.md**                 | 六份可验收契约：业务定义、能力契约、确定性边界、自治级别 A0–A4、Eval 契约、明确不做；含全部模块 V1/V2/V3 迭代边界；**§10 = V1 功能需求（四模块深耕 PRD，唯一需求真相源）**                                                                                                                                                                                                                                                                                                                                                | AI-STD-002     |
| **ARCHITECTURE.md**           | 七层架构、组件清单与存在理由、数据流、信任边界、工具清单、状态设计、观测与评测埋点                                                                                                                                                                                                                                                                                                                                                                                                                                        | AI-STD-003     |
| **CONVENTIONS-backend.md**    | Go 目录结构、分层依赖、工具层接口约束（WriteTool 编译期强制撤销）、错误三段式、测试与评测命令                                                                                                                                                                                                                                                                                                                                                                                                                             | AI-STD-006     |
| **CONVENTIONS-frontend.md**   | 移动端+管理端 monorepo 结构、组件/状态/样式规范、对话状态机、撤销交互、设计令牌                                                                                                                                                                                                                                                                                                                                                                                                                                           | AI-STD-006     |
| **agent-调度器设计.md**       | Agent 核心实现设计：六组件、工具注册表、危险分级控制、撤销机制、Go 实现要点                                                                                                                                                                                                                                                                                                                                                                                                                                               | AI-STD-003/004 |
| **ADR/**                      | 「为什么这么选」：Go、自建调度器、SQLite、全量可撤销、权限双保险                                                                                                                                                                                                                                                                                                                                                                                                                                                          | AI-STD-003/006 |
| **DOMAIN/家庭领域模型.md**    | 限界上下文、统一语言、核心实体、跨模块联动不变量、Eval 真实任务样本                                                                                                                                                                                                                                                                                                                                                                                                                                                       | AI-STD-002/005 |
| **ui/design-system.md**       | 苹果简约风 UI 规范：色彩、字体、圆角间距、关键视觉元素、一期页面清单                                                                                                                                                                                                                                                                                                                                                                                                                                                      | —              |
| **ui/architecture-system.md** | 系统七层分层架构图（mermaid + PNG）                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | AI-STD-003     |
| **ui/architecture-agent.md**  | Agent 调度器内部流程图（mermaid + PNG）                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | AI-STD-003     |
| **ui/sequence-dialog.md**     | 「买菜 120」对话执行时序图（mermaid + PNG）                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | AI-STD-003     |
| **ui/roadmap-iterations.md**  | 迭代边界图：V1 四模块深耕 → 阶段 A–D 出口判定（mermaid + PNG）                                                                                                                                                                                                                                                                                                                                                                                                                                                            | AI-STD-001     |
| **exec-plans/**               | 执行计划：任务状态 + **决策日志**（含被否方案）。智能体推进任务的唯一入口；**V1 总览（阶段 A–D）在 `exec-plans/README.md`**                                                                                                                                                                                                                                                                                                                                                                                               | AI-STD-006     |
| **tech-debt.md**              | 技术债登记：影响 + 偿还时机。发现坏模式立即登记，定期清偿                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | AI-STD-006     |
| **e2e-issues.md**             | E2E 测试轮次记录：每轮通过率、暴露的真实 bug、已修/待修清单                                                                                                                                                                                                                                                                                                                                                                                                                                                               | AI-STD-006     |
| **product/**                  | 产品设计研究入口，分四层：**定方向先读** `product-design-research-01.md`（理念层：设计原则 8 条 / 用户旅程 / 信息架构 / 交互系统 / AI 能力边界 / 演进路线）；**定模块深度读** `product-inspiration-01.md`（灵感层：市面产品能力调研 + 四模块深耕标尺 + 能力候选）；**定模块边界读** `product-design-research-02.md`（扩展层：功能模块全景七层 + 四模块之外的增量模块论证，含采购囤货/家庭档案/健康关怀与 C17–C20 登记）；**改产品前再读** `ux-exploration-01.md`（走查层：端到端走查 → 体验断点 → 产品机会 A/B/C 三方案） | AI-STD-002     |

---

## 阅读顺序

### 🚀 新人快速理解（30 分钟）

1. 根 `README.md` —— 项目是什么、为什么、技术栈、路线图
2. `AI-PRD.md` §1 业务定义 + §7 明确不做 + §8.10 一期 MVP 边界
3. `ui/architecture-system.md` —— 系统全貌一张图
4. `ui/sequence-dialog.md` —— 一个请求怎么跑完

### 📐 架构评审（Phase 03 / G3）

1. `ARCHITECTURE.md` —— 组件、数据流、信任边界、状态设计
2. `agent-调度器设计.md` —— 核心实现细节
3. `ADR/` —— 逐条审「为什么这么选」
4. `ui/architecture-agent.md` + `ui/architecture-system.md`

### 📋 产品 / 需求评审（Phase 02 / G2）

1. `AI-PRD.md` 全文（重点 §1、§4 自治、§6 Eval 契约、§7 不做）
2. `product/product-design-research-01.md` —— 设计层（产品理念、设计原则、信息架构、交互系统、AI 能力、演进路线）
3. `product/product-inspiration-01.md` —— 灵感层（市面能力调研、四模块深耕标尺 L0–L4、能力候选）
4. `product/product-design-research-02.md` —— 扩展层（功能模块全景七层、四模块之外的增量模块论证、C17–C20 登记）
5. `product/ux-exploration-01.md` —— 走查层（体验断点与产品机会）
6. `DOMAIN/家庭领域模型.md` —— 术语对齐与真实任务样本
7. `ui/roadmap-iterations.md` —— 迭代边界

### 🔒 安全评审（Phase 11 / G7）

1. `AI-PRD.md` §3 确定性边界 + §4 自治级别 + §6.2 安全否决项
2. `ARCHITECTURE.md` §4 信任边界 + §5 横切组件
3. `ADR-004`（撤销）+ `ADR-005`（权限双保险）
4. `DOMAIN/家庭领域模型.md` §5 安全任务样本（S7/S8）

### 💻 开发实现（Phase 08）

1. `ARCHITECTURE.md` §8 工具清单 + §10 埋点
2. `agent-调度器设计.md`「Go 实现要点」
3. `DOMAIN/家庭领域模型.md` §3 实体与不变量
4. `ui/design-system.md` —— 前端规范

---

## 文档约定

- **语言**：全中文（术语保留「中文（English）」标注）
- **图表**：mermaid 源码与渲染 PNG 成对存放于 `ui/`，PNG 由 `mmdc` 生成
- **ADR**：一条决策一篇，固定结构（状态/背景/决策/后果/备选）
- **标准对齐**：每份文档开头标注所对齐的 AI-STD 条款
- **变更**：模块边界改动同步改 `AI-PRD.md §8` 与 `ui/roadmap-iterations.md`；架构改动先写/改 ADR
- **自检**：改完文档跑 `pnpm run docs:check`（等价 `make docs-check`）——检查相对链接、FR 编号在 AI-PRD 与 exec-plans 间双向一致、`ui/` 的 mermaid 与 PNG 成对且不早于源码、计划验收用例 JSON 完整（含 `passes`）、开头 AI-STD 标注、prettier 格式；存在硬性不一致时退出码为 1

## 与根 README 的关系

根 `README.md`（项目级，由主维护者管理）是项目门面；`docs/README.md` 是文档内部地图。
根 README 中引用的 `docs/功能边界与迭代规划.md` 已并入 **`AI-PRD.md` §8**，原文件保留为跳转桩。
