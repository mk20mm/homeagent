// Package tool 定义 Agent 工具层的核心抽象。
//
// 设计原则（见 docs/CONVENTIONS-backend.md §3）：
//   - 写操作必须实现 Undo，由 WriteTool 接口在编译期强制（ADR-004）
//   - 参数在可信代码用 JSON Schema 校验，LLM 输出不可直接透传
//   - 执行链路顺序固定：权限校验→参数校验→执行→undo_log→审计，由 Executor 统一包办
package tool

import (
	"context"
	"encoding/json"
)

// RiskLevel 危险分级，控制 ReAct 循环上限与确认策略（ARCHITECTURE §8）。
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"    // 查询类，循环上限 15
	RiskMedium RiskLevel = "medium" // 创建/修改，上限 8
	RiskHigh   RiskLevel = "high"   // 财务/删除/权限，上限 3
)

// MaxTurns 危险分级对应的循环步数上限（防死循环 + 控成本）。
var MaxTurns = map[RiskLevel]int{
	RiskLow:    15,
	RiskMedium: 8,
	RiskHigh:   3,
}

// Spec 是工具的声明信息，供 LLM 选择与权限过滤使用。
type Spec struct {
	Name        string          // snake_case，动词开头，如 record_expense
	Description string          // 何时该用/何时不该用
	InputSchema json.RawMessage // JSON Schema 参数定义（LLM 生成参数后用它校验）
	Risk        RiskLevel
	Permission  string // 所需权限键，与权限矩阵联动（如 "expense.write"）
	Module      string // 所属业务模块：chore/expense/meal/system...
	Idempotency string // 幂等维度说明（写工具必填，如 "user+amount+hint+day"）
}

// Result 是工具执行返回，会被转成结果卡片回显给用户。
type Result struct {
	Summary  string          // 一句话总结，给 LLM 继续生成
	Card     json.RawMessage // 结构化卡片数据（前端渲染 + 撤销按钮）
	UndoData json.RawMessage // 撤销所需数据，写入 undo_log（写操作必填）
	UndoID   string          // 撤销记录 id（Executor 写完 undo_log 后回填，透传给前端撤销按钮）
}

// Tool 是所有工具的基础接口。
type Tool interface {
	Spec() Spec
	Execute(ctx context.Context, input json.RawMessage) (Result, error)
}

// WriteTool 是写操作必须实现的接口（编译期强制可撤销，ADR-004）。
//
// 一个写工具若不实现 Undo()，注册时类型断言会失败、无法注册到 Registry。
type WriteTool interface {
	Tool
	Undo(ctx context.Context, undoData json.RawMessage) error
}
