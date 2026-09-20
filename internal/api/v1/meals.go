package v1

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/domain/meal"
)

// MealService 报饭领域服务（handler 只用这两个方法，与工具走同一服务）。
type MealService interface {
	Report(ctx context.Context, memberID string, cmd meal.ReportCmd) (id string, err error)
	Summary(ctx context.Context, date time.Time) (meal.MealSummary, error)
}

// ListMeals GET /meals —— 今日报饭汇总（在家/不在家名单，给做饭人）。
func ListMeals(svc MealService, pl PermissionLookup, audit tool.AuditLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		if !permissionOf(c, pl, audit, memberID, "list_meals", tool.RiskLow, "meal.write") {
			abortWith(c, apperr.New(apperr.CodePermission, "无报饭查看权限", nil))
			return
		}

		var date time.Time // 零值 → 领域默认今日
		if d := parseDate(c.Query("date")); d != nil {
			date = *d
		}

		summary, err := svc.Summary(c.Request.Context(), date)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		members := make([]gin.H, 0, len(summary.AtHome)+len(summary.NotAtHome))
		for _, m := range summary.AtHome {
			members = append(members, gin.H{"member_id": m.ID, "name": m.Name, "at_home": true})
		}
		for _, m := range summary.NotAtHome {
			members = append(members, gin.H{"member_id": m.ID, "name": m.Name, "at_home": false})
		}

		unreported := make([]gin.H, 0, len(summary.Unreported))
		for _, m := range summary.Unreported {
			unreported = append(unreported, gin.H{"member_id": m.ID, "name": m.Name})
		}

		c.JSON(200, gin.H{
			"date":              summary.Date.Format("2006-01-02"),
			"at_home_count":     len(summary.AtHome),
			"not_at_home_count": len(summary.NotAtHome),
			"members":           members,
			"unreported_count":  len(summary.Unreported),
			"unreported":        unreported,
		})
	}
}

// ReportMeal POST /meals —— 报饭（report_meal 的 HTTP 出口，人+日期幂等）。
func ReportMeal(svc MealService, undoWriter UndoWriter, pl PermissionLookup, audit tool.AuditLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		if !permissionOf(c, pl, audit, memberID, "report_meal", tool.RiskMedium, "meal.write") {
			abortWith(c, apperr.New(apperr.CodePermission, "无报饭权限", nil))
			return
		}

		var body struct {
			AtHome *bool  `json:"at_home"`
			Date   string `json:"date"`
			Note   string `json:"note"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "请求参数有误", err))
			return
		}
		if body.AtHome == nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "缺少报饭状态（at_home）", nil))
			return
		}

		cmd := meal.ReportCmd{AtHome: *body.AtHome, Note: body.Note}
		if body.Date != "" {
			d, err := meal.ParseDate(body.Date)
			if err != nil {
				abortWith(c, apperr.New(apperr.CodeInvalidInput, "日期格式有误（应为 YYYY-MM-DD）", err))
				return
			}
			cmd.Date = d
		}

		ctx := c.Request.Context()
		id, err := svc.Report(ctx, memberID, cmd)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		// 撤销数据用服务端归一化后的真实日期（空日期 → 今日），否则撤销会扑空
		day := cmd.Date
		if day.IsZero() {
			day = meal.Today()
		}

		undoData, _ := json.Marshal(map[string]any{"date": day.Format("2006-01-02")})
		undoID, err := undoWriter.SaveUndo(ctx, memberID, "report_meal", undoData)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		c.JSON(200, gin.H{
			"id":        id,
			"member_id": memberID,
			"date":      day.Format("2006-01-02"),
			"at_home":   *body.AtHome,
			"note":      body.Note,
			"undoable":  true,
			"undo_id":   undoID,
		})
	}
}
