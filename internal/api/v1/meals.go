package v1

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/apperr"
	dommeal "github.com/mk20mm/homeagent/internal/domain/meal"
)

type MealService interface {
	Report(ctx context.Context, memberID string, cmd dommeal.ReportCmd) error
	Summary(ctx context.Context, date time.Time) (dommeal.MealSummary, error)
}

type MealReportLister interface {
	ListMealReports(ctx context.Context, date time.Time) ([]dommeal.MealMemberReport, error)
}

func ListMeals(lister MealReportLister) gin.HandlerFunc {
	return func(c *gin.Context) {
		dateStr := c.Query("date")
		date := time.Now().Truncate(24 * time.Hour)
		if dateStr != "" {
			if d, err := time.Parse("2006-01-02", dateStr); err == nil {
				date = d
			}
		}

		reports, err := lister.ListMealReports(c.Request.Context(), date)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		atHomeCount := 0
		notAtHomeCount := 0
		type memberInfo struct {
			MemberID string `json:"member_id"`
			Name     string `json:"name"`
			AtHome   bool   `json:"at_home"`
		}
		var members []memberInfo

		for _, r := range reports {
			if r.AtHome {
				atHomeCount++
			} else {
				notAtHomeCount++
			}
			members = append(members, memberInfo{
				MemberID: r.MemberID,
				Name:     r.Name,
				AtHome:   r.AtHome,
			})
		}
		if members == nil {
			members = []memberInfo{}
		}

		c.JSON(http.StatusOK, gin.H{
			"date":              date.Format("2006-01-02"),
			"at_home_count":     atHomeCount,
			"not_at_home_count": notAtHomeCount,
			"members":           members,
		})
	}
}

type reportMealReq struct {
	AtHome bool   `json:"at_home"`
	Date   string `json:"date"`
	Note   string `json:"note"`
}

func ReportMeal(svc MealService, pl PermissionLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "未授权", nil))
			return
		}
		if !permissionOf(c, pl, memberID, "meal.write") {
			abortWith(c, apperr.New(apperr.CodePermission, "无报饭权限", nil))
			return
		}

		var req reportMealReq
		if err := c.ShouldBindJSON(&req); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "参数错误", err))
			return
		}

		cmd := dommeal.ReportCmd{
			AtHome: req.AtHome,
			Note:   req.Note,
		}
		if req.Date != "" {
			if d, err := time.Parse("2006-01-02", req.Date); err == nil {
				cmd.Date = d
			}
		}

		if err := svc.Report(c.Request.Context(), memberID, cmd); err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"member_id": memberID,
			"at_home":   req.AtHome,
			"note":      req.Note,
		})
	}
}
