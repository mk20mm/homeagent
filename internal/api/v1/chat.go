package v1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/agent/runtime"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// chatRequest 对齐 openapi ChatRequest（手写绑定，P2 统一改 strict server）。
type chatRequest struct {
	Content        string `json:"content" binding:"required"`
	ConversationID string `json:"conversation_id,omitempty"`
	ModelID        string `json:"model_id,omitempty"`
}

// Chat POST /chat：SSE 流式回复（token/tool_call/done/error）。
// 客户端断开时 c.Request.Context() 自动取消，runtime 终止生成。
func Chat(rt *runtime.Runtime) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}

		var req chatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "消息格式错误", err))
			return
		}

		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("X-Accel-Buffering", "no") // Nginx 不缓冲
		c.Writer.WriteHeader(http.StatusOK)

		flusher, _ := c.Writer.(http.Flusher)
		hasError := false
		writeEvent := func(e runtime.Event) {
			if e.Type == "error" {
				hasError = true
			}
			b, _ := json.Marshal(e)
			_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", b)
			if flusher != nil {
				flusher.Flush()
			}
		}

		// ctx 取消 = 客户端断开，runtime 内部停止推送
		err := rt.Run(c.Request.Context(), runtime.RunRequest{
			ConversationID: req.ConversationID,
			MemberID:       memberID,
			ModelID:        req.ModelID,
			Content:        req.Content,
			TraceID:        c.GetString("trace_id"),
			OnEvent:        writeEvent,
		})
		if err != nil && !hasError {
			writeEvent(runtime.Event{Type: "error", Error: err.Error()})
		}
	}
}
