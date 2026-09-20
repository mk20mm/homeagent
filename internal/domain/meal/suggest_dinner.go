package meal

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// suggestDinnerMenus 建议池（确定性规则，不调 LLM 生成菜单，避免不可控输出）。
var suggestDinnerMenus = []string{
	"番茄炒蛋 + 清炒时蔬 + 米饭",
	"红烧排骨 + 紫菜蛋花汤",
	"青椒土豆丝 + 香煎鸡胸",
	"西红柿牛腩 + 拍黄瓜",
	"香菇滑鸡 + 蒜蓉西兰花",
	"酸辣土豆丝 + 煎蛋",
}

// SuggestDinnerTool A0 只读工具：晚餐建议，无任何写操作、全员可用。
type SuggestDinnerTool struct {
	svc Service
}

func NewSuggestDinnerTool(svc Service) *SuggestDinnerTool {
	return &SuggestDinnerTool{svc: svc}
}

func (t *SuggestDinnerTool) Spec() tool.Spec {
	return tool.Spec{
		Name:        "suggest_dinner",
		Description: "晚餐建议。家人问『今晚吃什么』时使用，结合今晚在家人数给出建议。只读，不产生任何记录。",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {}
}`),
		Risk:       tool.RiskLow,
		Permission: "", // 无需权限，全员可用
		Module:     "meal",
	}
}

func (t *SuggestDinnerTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	memberID := tool.MemberIDFrom(ctx)
	if memberID == "" {
		return tool.Result{}, apperr.New(apperr.CodePermission, "缺少成员身份", nil)
	}

	summary, err := t.svc.Summary(ctx, today())
	if err != nil {
		return tool.Result{}, err
	}

	atHome := make([]string, 0, len(summary.AtHome))
	for _, m := range summary.AtHome {
		atHome = append(atHome, m.Name)
	}
	notAtHome := make([]string, 0, len(summary.NotAtHome))
	for _, m := range summary.NotAtHome {
		notAtHome = append(notAtHome, m.Name)
	}

	menu := suggestDinnerMenus[rand.Intn(len(suggestDinnerMenus))]

	summaryText := ""
	if len(atHome) == 0 {
		summaryText = "今晚还没人报饭。"
	} else {
		summaryText = fmt.Sprintf("今晚 %s 在家吃（%d 人）。", strings.Join(atHome, "、"), len(atHome))
	}
	if len(notAtHome) > 0 {
		summaryText += strings.Join(notAtHome, "、") + " 不回来吃。"
	}
	summaryText += " 建议做「" + menu + "」。"

	card, _ := json.Marshal(map[string]any{
		"type":    "dinner",
		"menu":    menu,
		"at_home": atHome,
		"count":   len(atHome),
	})

	return tool.Result{Summary: summaryText, Card: card}, nil
}
