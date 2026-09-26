# 结构化 JSON 任务卡设计模式 (Result Card Pattern)

> 核心思想：**AI 与客户端交互禁止仅依赖纯文本。必须让每一次工具执行产出标准、类型化、可被前端直接渲染且可撤销的 JSON 任务卡。**

---

## 1. 标准外层信封结构 (Envelope Schema)

每个 SSE 工具回调或 API 响应，统一遵循三段式信封格式：

```typescript
export interface ToolEventEnvelope<T = unknown> {
  /** 事件类型：token (文字增量) | tool_call (工具卡片) | done (结束) | error (报错) */
  type: 'tool_call'
  /** 调用的底层工具唯一标识 */
  tool: string
  /** 任务卡核心负载 (Payload) */
  card: T
  /** 逆向撤销凭据 (仅对写操作产生，24小时内有效) */
  undo_id?: string
}
```

---

## 2. 经典卡片 Payload 范例

### 范例 A：记账任务卡 (`expense`)
```json
{
  "type": "tool_call",
  "tool": "record_expense",
  "card": {
    "type": "expense",
    "expense_id": "87ee0710-4301-4b88-a531-421ea2b9e7be",
    "amount": 35.00,
    "amount_cents": 3500,
    "category": "食材",
    "category_icon": "🥦",
    "hint": "买菜",
    "time": "23:10",
    "duplicated": false
  },
  "undo_id": "b10f8c3e-cd25-48a2-ae98-8c31f5a0d148"
}
```
**前端验收行为**：
- 渲染独立的白色或磨砂卡片，内嵌「🥦 食材 · 买菜」、「¥35.00」。
- 底部附带「撤销」药丸按钮。点击触发 `POST /api/v1/undo/{undo_id}`，卡片即刻变灰并展示「已撤销」。
- 若 `duplicated: true`，高亮显示「今日已记过该账单，未重复记账」。

---

### 范例 B：烹饪/做饭步骤卡 (`cooking_workflow`)
```json
{
  "type": "tool_call",
  "tool": "save_recipe",
  "card": {
    "type": "recipe",
    "recipe_id": "f512a321-9981-4213-aa01-d872b8319082",
    "title": "家常红烧肉",
    "duration_minutes": 50,
    "total_steps": 5,
    "current_step": 2,
    "step_summary": "小火炒糖色至微黄起小细泡",
    "ingredients": ["五花肉 500g", "冰糖 30g", "八角 2个"],
    "pro_tip": "五花肉切块后一定要擦干水分防溅油"
  },
  "undo_id": "c983d02a-9122-4a01-9921-ef770a129031"
}
```
**前端验收行为**：
- 渲染暖橙色下厨卡片，展示当前步骤大字摘要与火候提示。
- 附带「📱 开启做饭大字免脏屏模式」与「查看私房配方」入口。

---

## 3. 设计不变量与反模式

| 检查项 | 正确模式 (Do) | 错误反模式 (Don't) |
|---|---|---|
| **金额表达** | `amount_cents: 3500`（整数分）+ 格式化展示值 | 仅返回 `amount: 35.00` 浮点数 |
| **可读性** | 字段命名直观清晰（`category`, `hint`, `time`） | 返回无解释的内部哈希或脏 UUID 垃圾字段 |
| **幂等反馈** | 重复触发时返回原记录并标记 `duplicated: true` | 重复写入两条记录，或直接抛 500 崩溃 |
| **撤销能力** | 所有修改状态的写操作一律附带 `undo_id` | 写操作不可逆，用户点错无法恢复 |
| **双端同步** | 先落库持久化，再下发卡片，最后回复自然语言 | 先生成文字承诺，落库失败导致“口惠而实不至” |
