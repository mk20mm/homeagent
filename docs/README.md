# 家事 Agent · 文档地图（docs）

> 对齐 **AI-STD-006（仓库 Harness）**：docs 应含文档地图、ARCHITECTURE、ADR/、DOMAIN/。
> 项目当前阶段：**Phase 02–03（AI PRD / 架构）**，对齐 AI-STD-001 生命周期 G2→G3 门禁。

---

## 目录结构

```
docs/
├── README.md                  # 本文件：文档地图与阅读顺序
├── HARNESS.md                 # Agent 自主研发与知识沉淀使用说明书（操作SOP+沉淀法则）
├── AI-PRD.md                  # 产品需求契约（对齐 AI-STD-002）
├── ARCHITECTURE.md            # 架构设计（对齐 AI-STD-003）
├── CONVENTIONS-backend.md     # 后端代码规范（Go）
├── CONVENTIONS-frontend.md    # 前端代码规范（React+TS，移动端+管理端）
├── agent-调度器设计.md         # Agent 运行时详细设计（核心，保留）
├── homeagent-v1-architecture.md # 三域架构（财务/家务/厨房）完整设计真相源
├── homeagent-v1-acceptance.md   # 验收规格（80项用例：A12/F12/C18/K15/X5/D18）
├── agent-dispatch-center.md     # Agent 调度中心架构、状态机、多模型路由与异常容灾体系
├── agent-harness-design.md      # Agent 驾驭工程与自动化验收设计
├── gemini-v1-handoff.md         # Gemini 批次实施交接说明
├── tech-debt.md                 # 技术债登记（发现即偿还）
├── e2e-issues.md              # E2E 测试轮次记录与问题清单
├── product/                   # 产品设计探索（UX 走查 + 机会分析）
│   └── ux-exploration-01.md   #   第 1 轮：记账闭环与「对话—页面孤岛」
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
    └── roadmap-iterations.md  #   迭代边界图 V1/V2/V3 (.png)
```

根目录另有 **`AGENTS.md`**——约 100 行的地图与不可变不变量，会被注入智能体上下文。本文档与它的分工：`AGENTS.md` 是「入口地图」，本文件是「文档内部详图」。

---

## 每个文件的作用

| 文件                          | 作用                                                                                                              | 对齐标准       |
| ----------------------------- | ----------------------------------------------------------------------------------------------------------------- | -------------- |
| **HARNESS.md**                | 人机自主研发操作手册与知识沉淀使用说明书：日常极简使用姿势 + 知识分类、真伪检验、渐进瘦身法则                     | AI-STD-006     |
| **AI-PRD.md**                 | 六份可验收契约：业务定义、能力契约、确定性边界、自治级别 A0–A4、Eval 契约、明确不做；含全部模块 V1/V2/V3 迭代边界 | AI-STD-002     |
| **ARCHITECTURE.md**           | 七层架构、组件清单与存在理由、数据流、信任边界、工具清单、状态设计、观测与评测埋点                                | AI-STD-003     |
| **CONVENTIONS-backend.md**    | Go 目录结构、分层依赖、工具层接口约束（WriteTool 编译期强制撤销）、错误三段式、测试与评测命令                     | AI-STD-006     |
| **CONVENTIONS-frontend.md**   | 移动端+管理端 monorepo 结构、组件/状态/样式规范、对话状态机、撤销交互、设计令牌                                   | AI-STD-006     |
| **agent-调度器设计.md**       | Agent 核心实现设计：六组件、工具注册表、危险分级控制、撤销机制、Go 实现要点                                       | AI-STD-003/004 |
| **agent-dispatch-center.md**  | Agent 调度中心架构、状态机、多模型路由与异常容灾体系（执行计划与根因闭环）                                         | AI-STD-003/005 |
| **ADR/**                      | 「为什么这么选」：Go、自建调度器、SQLite、全量可撤销、权限双保险                                                  | AI-STD-003/006 |
| **DOMAIN/家庭领域模型.md**    | 限界上下文、统一语言、核心实体、跨模块联动不变量、Eval 真实任务样本                                               | AI-STD-002/005 |
| **ui/design-system.md**       | 苹果简约风 UI 规范：色彩、字体、圆角间距、关键视觉元素、一期页面清单                                              | —              |
| **ui/architecture-system.md** | 系统七层分层架构图（mermaid + PNG）                                                                               | AI-STD-003     |
| **ui/architecture-agent.md**  | Agent 调度器内部流程图（mermaid + PNG）                                                                           | AI-STD-003     |
| **ui/sequence-dialog.md**     | 「买菜 120」对话执行时序图（mermaid + PNG）                                                                       | AI-STD-003     |
| **ui/roadmap-iterations.md**  | 一/二/三期迭代边界图（mermaid + PNG）                                                                             | AI-STD-001     |
| **exec-plans/**               | 执行计划：任务状态 + **决策日志**（含被否方案）。智能体推进任务的唯一入口                                         | AI-STD-006     |
| **tech-debt.md**              | 技术债登记：影响 + 偿还时机。发现坏模式立即登记，定期清偿                                                         | AI-STD-006     |
| **e2e-issues.md**             | E2E 测试轮次记录：每轮通过率、暴露的真实 bug、已修/待修清单                                                       | AI-STD-006     |
| **product/**                  | UX 设计探索：真实用户视角端到端走查 → 体验断点 → 产品机会（A/B/C 三方案）。**改产品前先读**                        | AI-STD-002     |

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
2. `DOMAIN/家庭领域模型.md` —— 术语对齐与真实任务样本
3. `ui/roadmap-iterations.md` —— 迭代边界

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

## 与根 README 的关系

根 `README.md`（项目级，由主维护者管理）是项目门面；`docs/README.md` 是文档内部地图。
根 README 中引用的 `docs/功能边界与迭代规划.md` 已并入 **`AI-PRD.md` §8**，原文件保留为跳转桩。
