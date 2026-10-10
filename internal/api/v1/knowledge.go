package v1

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mk20mm/homeagent/internal/apperr"
	domknow "github.com/mk20mm/homeagent/internal/domain/knowledge"
)

type KnowledgeService interface {
	ListSpaces(ctx context.Context, memberID string) ([]domknow.Space, error)
	ListDocuments(ctx context.Context, memberID string, visibility string, category string, keyword string) ([]domknow.Document, error)
	CreateDocument(ctx context.Context, memberID string, authorName string, cmd domknow.CreateDocumentCmd) (domknow.Document, error)
	GetDocument(ctx context.Context, memberID string, id string) (domknow.Document, error)
	DeleteDocument(ctx context.Context, memberID string, id string) (string, error)
	RestoreDocument(ctx context.Context, id string) error
	Query(ctx context.Context, memberID string, query string, category string, topK int) ([]domknow.QueryResult, error)
}

// ListKnowledgeSpaces 处理 GET /knowledge/spaces
func ListKnowledgeSpaces(svc KnowledgeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}

		spaces, err := svc.ListSpaces(c.Request.Context(), memberID)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"spaces": spaces,
		})
	}
}

// ListKnowledgeDocuments 处理 GET /knowledge/documents
func ListKnowledgeDocuments(svc KnowledgeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}

		visibility := c.Query("visibility")
		category := c.Query("category")
		keyword := c.Query("keyword")

		docs, err := svc.ListDocuments(c.Request.Context(), memberID, visibility, category, keyword)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"documents": docs,
		})
	}
}

type createDocumentReq struct {
	Title      string `json:"title" binding:"required"`
	Category   string `json:"category" binding:"required"`
	Visibility string `json:"visibility" binding:"required"`
	Content    string `json:"content" binding:"required"`
}

// CreateKnowledgeDocument 处理 POST /knowledge/documents
func CreateKnowledgeDocument(svc KnowledgeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}

		var req createDocumentReq
		if err := c.ShouldBindJSON(&req); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "请求参数校验失败", err))
			return
		}

		doc, err := svc.CreateDocument(c.Request.Context(), memberID, "家庭成员", domknow.CreateDocumentCmd{
			Title:      req.Title,
			Category:   req.Category,
			Visibility: req.Visibility,
			Content:    req.Content,
		})
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		c.JSON(http.StatusCreated, doc)
	}
}

// GetKnowledgeDocument 处理 GET /knowledge/documents/:id
func GetKnowledgeDocument(svc KnowledgeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}

		id := c.Param("id")
		doc, err := svc.GetDocument(c.Request.Context(), memberID, id)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		c.JSON(http.StatusOK, doc)
	}
}

// DeleteKnowledgeDocument 处理 DELETE /knowledge/documents/:id
func DeleteKnowledgeDocument(svc KnowledgeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}

		id := c.Param("id")
		undoID, err := svc.DeleteDocument(c.Request.Context(), memberID, id)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"undo_id": undoID,
		})
	}
}

type queryKnowledgeReq struct {
	Query    string `json:"query" binding:"required"`
	Category string `json:"category"`
	TopK     int    `json:"top_k"`
}

// QueryKnowledge 处理 POST /knowledge/query
func QueryKnowledge(svc KnowledgeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}

		var req queryKnowledgeReq
		if err := c.ShouldBindJSON(&req); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "请求参数校验失败", err))
			return
		}

		topK := req.TopK
		if topK <= 0 {
			topK = 3
		}

		results, err := svc.Query(c.Request.Context(), memberID, req.Query, req.Category, topK)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"query":   req.Query,
			"results": results,
		})
	}
}
