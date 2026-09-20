package task

import (
	"context"
	"encoding/json"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// AssignTaskInput 派发参数。
type AssignTaskInput struct {
	Title        string `json:"title"`
	AssigneeName string `json:"assignee_name"`
	Description  string `json:"description,omitempty"`
	Risk         string `json:"risk,omitempty"`
	DueAt        string `json:"due_at,omitempty"`
}

// AssignTaskTool 中风险写工具：派发家务，可撤销。
type AssignTaskTool struct {
	svc Service
}

func NewAssignTaskTool(svc Service) *AssignTaskTool {
	return &AssignTaskTool{svc: svc}
}

func (t *AssignTaskTool) Spec() tool.Spec {
	return tool.Spec{
		Name:        "assign_task",
		Description: "派发家务任务给家人。『让小明洗碗/提醒奶奶吃药』时使用。assignee_name 留空表示待认领。",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "required": ["title"],
  "properties": {
    "title": {"type": "string", "description": "任务标题，如 洗碗"},
    "assignee_name": {"type": "string", "description": "执行人名字（家庭成员），留空=待认领"},
    "description": {"type": "string", "description": "补充说明"},
    "risk": {"type": "string", "enum": ["low", "medium", "high"], "description": "重要程度，默认 medium"},
    "due_at": {"type": "string", "format": "date-time", "description": "截止时间，可选"}
  }
}`),
		Risk:        tool.RiskMedium,
		Permission:  "task.write",
		Module:      "task",
		Idempotency: "assigner+assignee+title+day",
	}
}

func (t *AssignTaskTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	in, err := tool.DecodeInput[AssignTaskInput](input)
	if err != nil {
		return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "参数解析失败", err)
	}
	memberID := tool.MemberIDFrom(ctx)
	if memberID == "" {
		return tool.Result{}, apperr.New(apperr.CodePermission, "缺少成员身份", nil)
	}

	cmd := AssignTaskCmd{
		Title:        in.Title,
		AssigneeName: in.AssigneeName,
		Description:  in.Description,
		Risk:         in.Risk,
	}
	if in.DueAt != "" {
		if d, err := parseTime(in.DueAt); err == nil {
			cmd.DueAt = d
		}
	}

	id, duplicated, err := t.svc.AssignTask(ctx, memberID, cmd)
	if err != nil {
		return tool.Result{}, err
	}

	assignee := in.AssigneeName
	if assignee == "" {
		assignee = "待认领"
	}
	// 幂等命中时本次没有新写入，不生成撤销记录（撤销会误删早先那个任务）
	var undo json.RawMessage
	if !duplicated {
		undo, _ = json.Marshal(map[string]any{"task_id": id})
	}
	summary := "已派发「" + in.Title + "」给 " + assignee
	if duplicated {
		summary = "今天已派过「" + in.Title + "」，未重复派发"
	}
	card, _ := json.Marshal(map[string]any{
		"type":       "task",
		"task_id":    id,
		"title":      in.Title,
		"assignee":   assignee,
		"risk":       RiskLabel(cmd.Risk),
		"due_at":   FormatDueAt(nil),
		"status":     string(StatusPending),
		"duplicated": duplicated,
	})

	return tool.Result{
		Summary:  summary,
		Card:     card,
		UndoData: undo,
	}, nil
}

// Undo 撤销派发（软删除任务）。
func (t *AssignTaskTool) Undo(ctx context.Context, undoData json.RawMessage) error {
	var d struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(undoData, &d); err != nil {
		return apperr.New(apperr.CodeInvalidInput, "撤销数据解析失败", err)
	}
	return t.svc.DeleteTask(ctx, d.TaskID)
}
