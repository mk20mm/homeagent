package tool

import (
	"context"
	"encoding/json"
	"testing"
)

// stubTool 只读工具。
type stubTool struct{ spec Spec }

func (s *stubTool) Spec() Spec { return s.spec }
func (s *stubTool) Execute(context.Context, json.RawMessage) (Result, error) {
	return Result{Summary: "ok"}, nil
}

// stubWriteTool 写工具，实现了 Undo。
type stubWriteTool struct{ stubTool }

func (w *stubWriteTool) Undo(context.Context, json.RawMessage) error { return nil }

// stubNoUndoWriteTool 写工具但不实现 Undo —— 必须注册失败（ADR-004 运行时兜底）。
type stubNoUndoWriteTool struct{ stubTool }

func TestRegisterWriteToolRequiresUndo(t *testing.T) {
	reg := NewRegistry()

	// 只读工具：允许
	if err := reg.Register(&stubTool{spec: Spec{Name: "query", Risk: RiskLow}}); err != nil {
		t.Fatalf("只读工具应注册成功: %v", err)
	}

	// 写工具 + 实现 Undo：允许
	if err := reg.Register(&stubWriteTool{stubTool{spec: Spec{
		Name: "write_ok", Risk: RiskMedium, Idempotency: "user+x",
	}}}); err != nil {
		t.Fatalf("实现 Undo 的写工具应注册成功: %v", err)
	}

	// 写工具但没实现 Undo：必须失败
	err := reg.Register(&stubNoUndoWriteTool{stubTool{spec: Spec{
		Name: "write_bad", Risk: RiskHigh, Idempotency: "user+x",
	}}})
	if err == nil {
		t.Fatal("未实现 Undo 的写工具必须注册失败")
	}

	// 写工具但没声明幂等维度：必须失败
	err = reg.Register(&stubWriteTool{stubTool{spec: Spec{Name: "write_noidem", Risk: RiskMedium}}})
	if err == nil {
		t.Fatal("未声明幂等维度的写工具必须注册失败")
	}
}

func TestSpecsWithPermissionFilters(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&stubTool{spec: Spec{Name: "a", Risk: RiskLow, Permission: "expense.write"}})
	_ = reg.Register(&stubTool{spec: Spec{Name: "b", Risk: RiskLow, Permission: "chore.write"}})

	got := reg.SpecsWithPermission(map[string]bool{"expense.write": true})
	if len(got) != 1 || got[0].Name != "a" {
		t.Fatalf("权限过滤应只返回有权限的工具，got %d", len(got))
	}
}

func TestExecutorRejectsNoPermission(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&stubWriteTool{stubTool{spec: Spec{
		Name: "danger", Risk: RiskHigh, Permission: "expense.write", Idempotency: "x",
	}}})
	exec := NewExecutor(reg, nil, nil)

	_, err := exec.Execute(context.Background(), "danger", json.RawMessage(`{}`), "m1", map[string]bool{})
	if err == nil {
		t.Fatal("无权限调用必须被拒绝")
	}
}

func TestMaxTurnsByRisk(t *testing.T) {
	if MaxTurns[RiskHigh] != 3 || MaxTurns[RiskMedium] != 8 || MaxTurns[RiskLow] != 15 {
		t.Fatalf("危险分级步数错误: %v", MaxTurns)
	}
}
