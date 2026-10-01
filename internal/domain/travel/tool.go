package travel

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
)

type RecordTripInput struct {
	VehicleID  string  `json:"vehicle_id,omitempty"`
	TripType   string  `json:"trip_type"` // gas | charging | parking | toll | maintenance | ride
	Amount     float64 `json:"amount,omitempty"`
	Mileage    int     `json:"mileage,omitempty"`
	Note       string  `json:"note,omitempty"`
	OccurredAt string  `json:"occurred_at,omitempty"`
}

type RecordTripTool struct {
	svc Service
}

func NewRecordTripTool(svc Service) *RecordTripTool {
	return &RecordTripTool{svc: svc}
}

func (t *RecordTripTool) Spec() tool.Spec {
	return tool.Spec{
		Name:        "record_trip",
		Description: "记录家庭出行或用车开销（加油/充电/停车/保养/高速通行/打车）。当用户说『今天加油300元』、『给车充了50块电』、『停车花了15』等出行场景时调用。有费用时会自动同步到家庭账本。",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "required": ["trip_type"],
  "properties": {
    "trip_type": {
      "type": "string",
      "enum": ["gas", "charging", "parking", "toll", "maintenance", "ride"],
      "description": "出行类型：gas(加油), charging(充电), parking(停车), toll(高速路桥), maintenance(保养修车), ride(打车公共出行)"
    },
    "amount": {
      "type": "number",
      "minimum": 0,
      "description": "费用金额（元），如 300 或 15.5"
    },
    "mileage": {
      "type": "integer",
      "description": "当前总里程（公里）"
    },
    "note": {
      "type": "string",
      "description": "备注说明，如 万达地下停车场、京港澳高速"
    }
  }
}`),
		Risk:        tool.RiskMedium,
		Permission:  "expense.write",
		Module:      "travel",
		Idempotency: "vehicle+type+amount+time",
	}
}

func (t *RecordTripTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	in, err := tool.DecodeInput[RecordTripInput](input)
	if err != nil {
		return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "参数解析失败", err)
	}
	memberID := tool.MemberIDFrom(ctx)

	cents := int64(in.Amount*100 + 0.5)
	occur := time.Now()
	if in.OccurredAt != "" {
		if pt, perr := time.Parse(time.RFC3339, in.OccurredAt); perr == nil {
			occur = pt
		}
	}

	trip, err := t.svc.RecordTrip(ctx, memberID, RecordTripCmd{
		VehicleID:   in.VehicleID,
		TripType:    in.TripType,
		AmountCents: cents,
		Mileage:     in.Mileage,
		Note:        in.Note,
		OccurredAt:  occur,
	})
	if err != nil {
		return tool.Result{}, err
	}

	undo, _ := json.Marshal(map[string]any{"trip_id": trip.ID})
	card, _ := json.Marshal(map[string]any{
		"type":         "trip",
		"trip_id":      trip.ID,
		"trip_type":    trip.TripType,
		"amount_yuan":  float64(cents) / 100.0,
		"mileage":      trip.Mileage,
		"vehicle_name": trip.VehicleName,
		"note":         trip.Note,
		"time":         occur.Format("15:04"),
	})

	summary := fmt.Sprintf("已记录出行：%s", formatTripHint(trip.TripType, trip.Note))
	if cents > 0 {
		summary += fmt.Sprintf("，费用 %.2f 元并已联动记入账本", float64(cents)/100.0)
	}

	return tool.Result{
		Summary:  summary,
		Card:     card,
		UndoData: undo,
	}, nil
}

func (t *RecordTripTool) Undo(ctx context.Context, undoData json.RawMessage) error {
	var d struct {
		TripID string `json:"trip_id"`
	}
	if err := json.Unmarshal(undoData, &d); err != nil {
		return apperr.New(apperr.CodeInvalidInput, "撤销数据解析失败", err)
	}
	return t.svc.UndoTrip(ctx, d.TripID)
}
