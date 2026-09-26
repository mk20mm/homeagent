---
name: init-spec-map
description: >-
  Bootstrap a contract-first, AI-agent-friendly repository specification map from scratch for any new project.
  Generates AGENTS.md, OpenAPI contracts, structured JSON tool result cards, domain models, and AI-PRDs.
---

# 项目 Spec 规范与地图生成器 (Init Spec Map)

本技能提炼自 **HomeAgent** 仓库的架构落地经验。用于在任何新项目启动时，快速初始化符合 **AI-STD-006（仓库 Harness 标准）** 的智能体协作蓝图。

核心理念：**OpenAPI 契约先行 + 结构化 JSON 任务卡回显 + 确定性边界防护 + 全量可撤销**。

---

## 4 阶段初始化执行流 (Workflow)

```
[阶段 1: 业务定义与领域挖掘] ──> [阶段 2: AGENTS.md 顶层地图构建]
                                          │
[阶段 4: AI-PRD 与确定性架构] <── [阶段 3: OpenAPI 契约与结构化 JSON 任务卡]
```

---

## 阶段 1：业务定义与领域挖掘 (Domain Discovery)

在创建代码前，首先向用户收集（或自推导）以下基础信息：
1. **项目一句话**：谁在什么场景用它解决什么核心痛点？
2. **核心业务支柱（3~5 个限界上下文）**：划分 Core Domain（核心域）与 Supporting Domain（支撑域）。
3. **统一语言（Ubiquitous Language）**：梳理中英文术语对照表，禁止同名异义。
4. **黄金评测任务（Golden Eval Samples S1~S10）**：设计 10 个最具代表性的真实场景输入、期望工具调用与风险等级（A0~A3）。

*输出物*：`docs/DOMAIN/<领域名称>领域模型.md`。

---

## 阶段 2：构建根目录 `AGENTS.md` (百行地图)

根据 [references/agents-md-template.md](./references/agents-md-template.md)，在项目根目录生成 `AGENTS.md`。必须严格控制在 **100~120 行**：

### 必须包含的 6 大板块：
1. **项目一句话与当前阶段**：明确说明当前完成度与当前开发重点。
2. **不可违反的 8 条架构不变量**：
   - 契约先行：禁止手改生成文件。
   - 主键与金额规范：如全 UUID、金额全分（禁止 float）。
   - 写操作全量可撤销（24h 逆向日志）。
   - 权限双保险（提示词过滤 + 运行时硬校验）。
   - 依赖单向流动（API ➔ Domain ➔ Store）。
   - 错误三段式标准（`{code, message, trace_id}`）。
3. **目录地图**：列出最关键的代码和文档位置。
4. **常用验证命令**：编译、单测、类型检查、启动命令。
5. **环境与踩坑警示（踩过的坑）**：记录本地特定的环境陷阱（编码、代理、端口占用等）。
6. **领域知识索引表**：遇到哪类问题去哪找文档。

---

## 阶段 3：OpenAPI 契约与 JSON 结构化任务卡设计

智能体与业务系统的交互契约是全系统的生命线。

### 1. 契约定义 (`api/openapi.yaml`)
- 接口全部通过 OpenAPI 3.0+ 描述。
- 后端使用代码生成工具生成服务桩（如 Go 的 `oapi-codegen`，Java 的 `openapi-generator`）。
- 前端使用 `openapi-typescript` 自动同步强类型，禁止手写类型声明。

### 2. 结构化 JSON 任务卡验收标准 (Result Card Pattern)
工具（Tools）执行完成后，**严禁仅返回一段自然语言文本**，必须以结构化 JSON 回显，满足双端渲染与状态回滚：

```json
{
  "type": "tool_call",
  "tool": "record_expense",
  "card": {
    "type": "expense",
    "expense_id": "87ee0710-4301-4b88-a531-421ea2b9e7be",
    "amount": 35,
    "category": "食材",
    "hint": "买菜",
    "time": "23:10",
    "duplicated": false
  },
  "undo_id": "b10f8c3e-cd25-48a2-ae98-8c31f5a0d148"
}
```

详细规范与设计模式见 [references/result-card-pattern.md](./references/result-card-pattern.md)。

---

## 阶段 4：AI-PRD 与确定性架构 (`docs/AI-PRD.md` & `docs/ARCHITECTURE.md`)

建立 Agentic 软件系统的两道防火墙：

### 1. 确定性边界（Deterministic Boundary）
明确划分 **代码硬控制 vs 模型软推理**：
- **模型负责**：意图理解、参数抽取、文案润色。
- **代码硬防**：金额计算、权限拦截、幂等查重、事务写入、撤销记录、循环步数上限。
- **人机协同（HITL）**：高危或大额操作强制二次确认。

### 2. 工具自治级别分级 (Autonomy A0~A4)
- **A0（只读建议）**：无副作用（如灵感推荐）。
- **A1（自动读取）**：查询本人数据（如查账单、查待办）。
- **A2（可逆内部写）**：带 undo 的普通写操作（如打卡、记笔记）。
- **A3（难逆高危写）**：记账、删任务、改权限（必须 Preview + 可撤销）。
- **A4（安全关键）**：对外打款、永久物理删除（生产环境需强授权或不做）。

---

## 初始化完成后的自检验收清单

- [ ] 根目录是否存在紧凑精炼的 `AGENTS.md`？
- [ ] `api/openapi.yaml` 是否定义完整并配置了自动化 Codegen 脚本？
- [ ] 工具调用是否全量支持结构化 JSON 结果卡片与 `undo_id`？
- [ ] `docs/DOMAIN/` 是否包含统一语言、实体关系与 S1~S10 真实任务样本？
- [ ] 是否配置了自动化门禁脚本（编译 + 类型 + 单测 + 端到端走查）？
