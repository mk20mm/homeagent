// evals 入口：make eval 执行（go test ./evals/...）。
//
// 输出（AI-PRD §6）：
//   - 用例级 PASS/FAIL 明细
//   - 五项指标通过率 vs §6.1 阈值
//   - §6.2 安全否决项命中 → 整套 0 分，测试失败
package evals

import (
	"context"
	"testing"

	"github.com/mk20mm/homeagent/internal/domain/expense"
)

// allCases 汇总金标准任务集（capability / tool / safety 三类）。
func allCases() []Case {
	var out []Case
	out = append(out, expenseCases()...)
	out = append(out, taskCases()...)
	out = append(out, mealCases()...)
	out = append(out, hubCases()...)
	out = append(out, safetyCases()...)
	return out
}

func TestGoldenSuite(t *testing.T) {
	ctx, cancel := timeoutCtx()
	defer cancel()

	res := runSuite(ctx, allCases())
	t.Log(summarize(res))

	if res.SafetyHit {
		t.Fatal("安全否决项命中，整套 0 分（AI-PRD §6.2）")
	}
	if !res.Pass {
		t.Fatal("指标未达 §6.1 阈值")
	}
}

// TestCaseCount 防止用例集被误删/缩水（§10.14：每模块 5–10 条）。
func TestCaseCount(t *testing.T) {
	cases := allCases()
	if len(cases) < 20 {
		t.Fatalf("金标准任务集应 >= 20 条（§6），当前 %d", len(cases))
	}

	byModule := map[string]int{}
	for _, c := range cases {
		byModule[c.Module]++
	}
	for _, m := range []string{"expense", "task", "meal", "hub"} {
		if byModule[m] < 5 {
			t.Errorf("模块 %s 用例数 %d < 5（§10.14 每模块 5–10 条）", m, byModule[m])
		}
	}
	if byModule["safety"] < 6 {
		t.Errorf("安全否决项 %d < 6（§6.2 六类）", byModule["safety"])
	}
}

// TestSandboxIsolation 两个沙箱互不影响（用例隔离的前提）。
func TestSandboxIsolation(t *testing.T) {
	ctx := context.Background()

	sb1 := newSandbox()
	sb2 := newSandbox()

	// sb1 记一笔
	_, err := sb1.Expenses.Create(ctx, "m-parent", recordCmd(12000, "买菜"), "k1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if len(sb2.Expenses.activeExpenses()) != 0 {
		t.Fatal("沙箱应互相隔离")
	}
	if len(sb1.Expenses.activeExpenses()) != 1 {
		t.Fatal("沙箱内应有一条账单")
	}
}

// recordCmd 构造记账命令（沙箱隔离测试用）。
func recordCmd(cents int64, hint string) expense.RecordExpenseCmd {
	return expense.RecordExpenseCmd{AmountCents: cents, Hint: hint}
}
