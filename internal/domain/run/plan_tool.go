package run

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/store/ent/schema"
)

// 确保 SubmitTaskPlanTool 实现了 tool.WriteTool 接口（ADR-004 必须可撤销）。
var _ tool.WriteTool = (*SubmitTaskPlanTool)(nil)

// PlanStepInput 计划步骤入参。
type PlanStepInput struct {
	Position           int            `json:"position"`
	ToolName           string         `json:"tool_name"`
	Input              map[string]any `json:"input"`
	DependsOnPositions []int          `json:"depends_on_positions,omitempty"`
	WaitFor            string         `json:"wait_for,omitempty"`
	CompletionKind     string         `json:"completion_kind,omitempty"`
	OperationKey       string         `json:"operation_key,omitempty"`
}

// SubmitTaskPlanInput 复合计划提交入参。
type SubmitTaskPlanInput struct {
	Goal             string          `json:"goal"`
	RequestedEffects []string        `json:"requested_effects,omitempty"`
	Steps            []PlanStepInput `json:"steps"`
}

// SubmitTaskPlanTool 调度中心复合任务规划工具（agent-dispatch-center.md §6）。
type SubmitTaskPlanTool struct {
	svc Service
}

// NewSubmitTaskPlanTool 创建复合计划工具。
func NewSubmitTaskPlanTool(svc Service) *SubmitTaskPlanTool {
	return &SubmitTaskPlanTool{svc: svc}
}

func (t *SubmitTaskPlanTool) Spec() tool.Spec {
	return tool.Spec{
		Name:        "submit_task_plan",
		Description: "提交跨域多步骤复合计划（如『做完两道菜后扫厨房』）。该工具负责保存与校验计划拓扑，不会一次性执行所有动作。",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "required": ["goal", "steps"],
  "properties": {
    "goal": {"type": "string", "description": "复合计划总体目标"},
    "requested_effects": {
      "type": "array",
      "items": {"type": "string"},
      "description": "本次计划声明的影响领域列表（如 cooking, chore, finance）"
    },
    "steps": {
      "type": "array",
      "description": "计划步骤序列（上限 10 步，严格无环有向图）",
      "items": {
        "type": "object",
        "required": ["position", "tool_name", "input"],
        "properties": {
          "position": {"type": "integer", "description": "步骤序号，从 1 开始递增"},
          "tool_name": {"type": "string", "description": "要调用的目标工具名称"},
          "input": {"type": "object", "description": "目标工具的执行参数 JSON 对象"},
          "depends_on_positions": {
            "type": "array",
            "items": {"type": "integer"},
            "description": "前置依赖步骤的 position 序号列表"
          },
          "wait_for": {
            "type": "string",
            "enum": ["none", "entity_completed", "scheduled_at", "user_confirm"],
            "description": "等待放行条件，默认 none"
          },
          "completion_kind": {
            "type": "string",
            "description": "完成判定类型，如 local_committed / observed_device_success / member_confirmed"
          },
          "operation_key": {
            "type": "string",
            "description": "幂等操作键，可选"
          }
        }
      }
    }
  }
}`),
		Risk:        tool.RiskMedium,
		Permission:  "task.write",
		Module:      "dispatch",
		Idempotency: "goal+member+steps",
	}
}

func (t *SubmitTaskPlanTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	in, err := tool.DecodeInput[SubmitTaskPlanInput](input)
	if err != nil {
		return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "计划参数解析失败", err)
	}
	memberID := tool.MemberIDFrom(ctx)
	if memberID == "" {
		return tool.Result{}, apperr.New(apperr.CodePermission, "缺少成员身份", nil)
	}
	if strings.TrimSpace(in.Goal) == "" {
		return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "计划目标不能为空", nil)
	}
	if len(in.Steps) == 0 {
		return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "计划步骤不能为空", nil)
	}
	if len(in.Steps) > 10 {
		return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "计划步骤超过上限（最多10步）", nil)
	}

	// 校验序号与无环有向图 (DAG)
	posSet := make(map[int]bool, len(in.Steps))
	for _, s := range in.Steps {
		if s.Position <= 0 {
			return tool.Result{}, apperr.New(apperr.CodeInvalidInput, fmt.Sprintf("步骤序号非法: %d", s.Position), nil)
		}
		if posSet[s.Position] {
			return tool.Result{}, apperr.New(apperr.CodeInvalidInput, fmt.Sprintf("步骤序号重复: %d", s.Position), nil)
		}
		posSet[s.Position] = true
	}

	// 校验依赖无环：前置依赖步骤必须存在且序号小于当前步骤
	for _, s := range in.Steps {
		for _, dep := range s.DependsOnPositions {
			if !posSet[dep] {
				return tool.Result{}, apperr.New(apperr.CodeInvalidInput, fmt.Sprintf("依赖的步骤 %d 不存在", dep), nil)
			}
			if dep >= s.Position {
				return tool.Result{}, apperr.New(apperr.CodeInvalidInput, fmt.Sprintf("步骤 %d 依赖 %d 构成循环或向前依赖，必须严格无环", s.Position, dep), nil)
			}
		}
	}

	convID := tool.ConversationIDFrom(ctx)
	traceID := tool.TraceIDFrom(ctx)

	// 创建 Run
	runDetail, err := t.svc.CreateRun(ctx, memberID, in.Goal, schema.IntentPlan, traceID, convID, in.RequestedEffects)
	if err != nil {
		return tool.Result{}, err
	}

	// 顺序添加步骤并构建 position -> stepID 映射
	posToStepID := make(map[int]string, len(in.Steps))
	for _, s := range in.Steps {
		inputBytes, _ := json.Marshal(s.Input)
		depIDs := make([]string, 0, len(s.DependsOnPositions))
		for _, depPos := range s.DependsOnPositions {
			if id, ok := posToStepID[depPos]; ok {
				depIDs = append(depIDs, id)
			}
		}

		waitFor := schema.StepWaitFor(s.WaitFor)
		if waitFor == "" {
			waitFor = schema.WaitForNone
		}
		compKind := s.CompletionKind
		if compKind == "" {
			compKind = "local_committed"
		}

		stepItem, err := t.svc.AddRunStep(ctx, runDetail.ID, RunStepInput{
			Position:         s.Position,
			ToolName:         s.ToolName,
			InputJSON:        string(inputBytes),
			OperationKey:     s.OperationKey,
			DependsOnStepIDs: depIDs,
			WaitFor:          waitFor,
			CompletionKind:   compKind,
		})
		if err != nil {
			return tool.Result{}, err
		}
		posToStepID[s.Position] = stepItem.ID
	}

	cardData := map[string]any{
		"type":        "plan",
		"run_id":      runDetail.ID,
		"goal":        in.Goal,
		"status":      "running",
		"total_steps": len(in.Steps),
		"effects":     in.RequestedEffects,
	}
	cardBytes, _ := json.Marshal(cardData)
	undoBytes, _ := json.Marshal(map[string]any{"run_id": runDetail.ID})

	return tool.Result{
		Summary:  fmt.Sprintf("已创建复合计划「%s」（共 %d 步），进入调度编排", in.Goal, len(in.Steps)),
		Card:     cardBytes,
		UndoData: undoBytes,
	}, nil
}

// Undo 撤销任务规划（将 Run 状态变更为 cancelled）。
func (t *SubmitTaskPlanTool) Undo(ctx context.Context, undoData json.RawMessage) error {
	var payload struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(undoData, &payload); err != nil {
		return apperr.New(apperr.CodeInvalidInput, "撤销数据无效", err)
	}
	if payload.RunID == "" {
		return nil
	}
	_, err := t.svc.UpdateRunStatus(ctx, payload.RunID, 0, schema.RunStatusCancelled)
	return err
}
