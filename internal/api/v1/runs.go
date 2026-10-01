package v1

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/apperr"
	domrun "github.com/mk20mm/homeagent/internal/domain/run"
)

// RunService 调度中心运行服务接口（api/v1 契约依赖抽象）。
type RunService interface {
	GetRun(ctx context.Context, memberID string, runID string) (domrun.RunDetail, error)
	ListRuns(ctx context.Context, memberID string, status string, limit int) ([]domrun.RunItem, error)
	ConfirmRunStep(ctx context.Context, memberID string, runID string, stepID string, choice string, nonce string) (domrun.RunDetail, error)
	ReplyRun(ctx context.Context, memberID string, runID string, stepID string, expectedVersion int, answers map[string]any) (domrun.RunDetail, error)
	AmendRun(ctx context.Context, memberID string, runID string, expectedVersion int, modifications []map[string]any) (domrun.RunDetail, error)
}

// ListRuns GET /runs
func ListRuns(svc RunService) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}

		status := c.Query("status")
		limit := 20
		if lStr := c.Query("limit"); lStr != "" {
			if l, err := strconv.Atoi(lStr); err == nil && l > 0 && l <= 100 {
				limit = l
			}
		}

		runs, err := svc.ListRuns(c.Request.Context(), memberID, status, limit)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		c.JSON(http.StatusOK, gin.H{"runs": runs})
	}
}

// GetRun GET /runs/:runId
func GetRun(svc RunService) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}

		runID := c.Param("runId")
		detail, err := svc.GetRun(c.Request.Context(), memberID, runID)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		c.JSON(http.StatusOK, detail)
	}
}

type confirmRunStepReq struct {
	StepID            string `json:"step_id" binding:"required"`
	Choice            string `json:"choice" binding:"required"`
	ConfirmationNonce string `json:"confirmation_nonce"`
}

// ConfirmRunStep POST /runs/:runId/confirm
func ConfirmRunStep(svc RunService) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}

		runID := c.Param("runId")
		var req confirmRunStepReq
		if err := c.ShouldBindJSON(&req); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "参数无效", err))
			return
		}

		detail, err := svc.ConfirmRunStep(c.Request.Context(), memberID, runID, req.StepID, req.Choice, req.ConfirmationNonce)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(http.StatusOK, detail)
	}
}

type replyRunReq struct {
	RequestID       string         `json:"request_id" binding:"required"`
	StepID          string         `json:"step_id" binding:"required"`
	ExpectedVersion int            `json:"expected_version"`
	Answers         map[string]any `json:"answers" binding:"required"`
}

// ReplyRun POST /runs/:runId/reply
func ReplyRun(svc RunService) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}

		runID := c.Param("runId")
		var req replyRunReq
		if err := c.ShouldBindJSON(&req); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "参数无效", err))
			return
		}

		detail, err := svc.ReplyRun(c.Request.Context(), memberID, runID, req.StepID, req.ExpectedVersion, req.Answers)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(http.StatusOK, detail)
	}
}

type amendRunReq struct {
	ExpectedVersion int              `json:"expected_version"`
	Modifications   []map[string]any `json:"modifications" binding:"required"`
}

// AmendRun POST /runs/:runId/amend
func AmendRun(svc RunService) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}

		runID := c.Param("runId")
		var req amendRunReq
		if err := c.ShouldBindJSON(&req); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "参数无效", err))
			return
		}

		detail, err := svc.AmendRun(c.Request.Context(), memberID, runID, req.ExpectedVersion, req.Modifications)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(http.StatusOK, detail)
	}
}
