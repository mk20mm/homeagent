package tool

import (
	"testing"
)

func TestExecutorIsReplayable(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&stubTool{spec: Spec{
		Name:       "safe_query",
		Risk:       RiskLow,
		Replayable: true,
	}})
	_ = reg.Register(&stubWriteTool{stubTool{spec: Spec{
		Name:        "unsafe_write",
		Risk:        RiskMedium,
		Idempotency: "user+x",
		Replayable:  false,
	}}})

	exec := NewExecutor(reg, nil, nil)

	if !exec.IsReplayable("safe_query") {
		t.Errorf("safe_query 应为 Replayable: true")
	}
	if exec.IsReplayable("unsafe_write") {
		t.Errorf("unsafe_write 应为 Replayable: false")
	}
	if exec.IsReplayable("non_existent") {
		t.Errorf("不存在的工具应为 Replayable: false")
	}
}
