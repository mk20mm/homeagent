package task

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// ListMyTasksTool 低风险只读：查我的待办。
type ListMyTasksTool struct {
	svc Service
}

func NewListMyTasksTool(svc Service) *ListMyTasksTool {
	return &ListMyTasksTool{svc: svc}
}

func (t *ListMyTasksTool) Spec() tool.Spec {
	return tool.Spec{
		Name:        "list_my_tasks",
		Description: "查我名下的家务待办。用户问『我有什么任务/还有什么没做』时使用。只读。",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {}
}`),
		Risk:       tool.RiskLow,
		Permission: "task.read",
		Module:     "task",
	}
}

func (t *ListMyTasksTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	memberID := tool.MemberIDFrom(ctx)
	if memberID == "" {
		return tool.Result{}, apperr.New(apperr.CodePermission, "缺少成员身份", nil)
	}

	tasks, err := t.svc.ListMyTasks(ctx, memberID)
	if err != nil {
		return tool.Result{}, err
	}

	items := make([]map[string]any, 0, len(tasks))
	for _, tk := range tasks {
		items = append(items, map[string]any{
			"id":     tk.ID,
			"title":  tk.Title,
			"risk":   RiskLabel(tk.Risk),
			"due_at": FormatDueAt(tk.DueAt),
			"points": tk.Points,
		})
	}
	card, _ := json.Marshal(map[string]any{
		"type":  "task_list",
		"items": items,
		"count": len(items),
	})

	summary := "没有待办任务"
	if len(tasks) > 0 {
		summary = "有 " + strconv.Itoa(len(tasks)) + " 项待办："
		for i, tk := range tasks {
			if i > 0 {
				summary += "、"
			}
			summary += tk.Title
		}
	}

	return tool.Result{Summary: summary, Card: card}, nil
}
