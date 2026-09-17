package task

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// CompleteTaskInput 打卡参数。
type CompleteTaskInput struct {
	TaskID string `json:"task_id"`
	Title  string `json:"title,omitempty"` // task_id 为空时按标题查我的任务
}

// CompleteTaskTool 中风险写工具：家务打卡，幂等防重复。
type CompleteTaskTool struct {
	svc Service
}

func NewCompleteTaskTool(svc Service) *CompleteTaskTool {
	return &CompleteTaskTool{svc: svc}
}

func (t *CompleteTaskTool) Spec() tool.Spec {
	return tool.Spec{
		Name:        "complete_task",
		Description: "完成家务打卡。家人报告『洗完了/做完了』时使用，需指定任务。重复打卡同一任务只计一次。",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "task_id": {"type": "string", "description": "任务 id"},
    "title": {"type": "string", "description": "任务标题（无 id 时按标题匹配我的任务）"}
  }
}`),
		Risk:        tool.RiskMedium,
		Permission:  "task.write",
		Module:      "task",
		Idempotency: "task once",
	}
}

func (t *CompleteTaskTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	in, err := tool.DecodeInput[CompleteTaskInput](input)
	if err != nil {
		return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "参数解析失败", err)
	}
	memberID := tool.MemberIDFrom(ctx)
	if memberID == "" {
		return tool.Result{}, apperr.New(apperr.CodePermission, "缺少成员身份", nil)
	}

	taskID := in.TaskID
	if taskID == "" {
		if in.Title == "" {
			return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "需提供 task_id 或 title", nil)
		}
		tasks, err := t.svc.ListMyTasks(ctx, memberID)
		if err != nil {
			return tool.Result{}, err
		}
		for _, tk := range tasks {
			if tk.Title == in.Title {
				taskID = tk.ID
				break
			}
		}
		if taskID == "" {
			return tool.Result{}, apperr.New(apperr.CodeNotFound, "没找到任务「"+in.Title+"」", nil)
		}
	}

	if err := t.svc.CompleteTask(ctx, taskID, memberID); err != nil {
		return tool.Result{}, err
	}

	undo, _ := json.Marshal(map[string]any{"task_id": taskID})
	card, _ := json.Marshal(map[string]any{
		"type":   "task_done",
		"title":  in.Title,
		"points": "+1",
	})

	return tool.Result{
		Summary:  "已完成「" + in.Title + "」打卡",
		Card:     card,
		UndoData: undo,
	}, nil
}

// Undo 撤销误打卡：状态回退 pending + 清完成时间。
func (t *CompleteTaskTool) Undo(ctx context.Context, undoData json.RawMessage) error {
	var d struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(undoData, &d); err != nil {
		return apperr.New(apperr.CodeInvalidInput, "撤销数据解析失败", err)
	}
	return t.svc.UncompleteTask(ctx, d.TaskID)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
