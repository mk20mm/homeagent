// Package gateway · OpenAI 适配器：兼容 OpenAI / DeepSeek / 任何 OpenAI 兼容端点。
//
// 设计：
//   - 流式 StreamChat + function calling；tool_call 在网关内聚合（分片 → 完整调用）
//   - ctx 取消即终止生成（runtime 约束）：stream.Close 在 defer 里执行
//   - 用量：StreamOptions.IncludeUsage，流结束的最后一个 chunk 带 usage
//   - 密钥不入库不入日志，由 infra/config 环境变量注入
package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// OpenAIProvider 兼容 OpenAI 协议的供应商。
type OpenAIProvider struct {
	client    *openai.Client
	model     string // 供应商侧模型名，如 gpt-4o / deepseek-chat
	provider  string // 供应商标识（用量埋点用）
}

// NewOpenAIProvider apiKey 必填；baseURL 为空用 OpenAI 默认；model 必填。
func NewOpenAIProvider(apiKey, baseURL, model, providerName string) *OpenAIProvider {
	cfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	return &OpenAIProvider{
		client:   openai.NewClientWithConfig(cfg),
		model:    model,
		provider: providerName,
	}
}

func (p *OpenAIProvider) Name() string {
	if p.provider != "" {
		return p.provider
	}
	return "openai"
}

// buildRequest 把供应商无关请求转成 OpenAI 协议（纯函数，可单测）。
func (p *OpenAIProvider) buildRequest(req ChatRequest) openai.ChatCompletionRequest {
	messages := make([]openai.ChatCompletionMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		switch m.Role {
		case RoleSystem:
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleSystem,
				Content: m.Content,
			})
		case RoleUser:
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: m.Content,
			})
		case RoleAssistant:
			msg := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant}
			if m.Content != "" {
				msg.Content = m.Content
			}
			for _, tc := range m.ToolCalls {
				msg.ToolCalls = append(msg.ToolCalls, openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Name,
						Arguments: string(tc.Args),
					},
				})
			}
			messages = append(messages, msg)
		case RoleTool:
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    m.Content,
				ToolCallID: m.ToolCallID,
				Name:       m.Name,
			})
		}
	}

	tools := make([]openai.Tool, 0, len(req.Tools))
	for _, t := range req.Tools {
		params := json.RawMessage(t.Parameters)
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		tools = append(tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}

	out := openai.ChatCompletionRequest{
		Model:    p.model,
		Messages: messages,
		Stream:   true,
		StreamOptions: &openai.StreamOptions{
			IncludeUsage: true,
		},
	}
	if req.Temperature > 0 {
		out.Temperature = req.Temperature
	}
	if len(tools) > 0 {
		out.Tools = tools
	}
	return out
}

func (p *OpenAIProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	streamReq := p.buildRequest(req)
	stream, err := p.client.CreateChatCompletionStream(ctx, streamReq)
	if err != nil {
		return nil, err
	}

	events := make(chan StreamEvent, 32)
	go func() {
		defer close(events)
		defer stream.Close()

		// tool_call 分片聚合：index → 累积中的调用
		type aggCall struct {
			id      strings.Builder
			name    strings.Builder
			args    strings.Builder
		}
		aggregating := map[int]*aggCall{}

		emitToolCalls := func() {
			for i, ac := range aggregating {
				tc := ToolCall{
					ID:   ac.id.String(),
					Name: ac.name.String(),
					Args: json.RawMessage(ac.args.String()),
				}
				if tc.Name != "" {
					events <- StreamEvent{ToolCall: &tc}
				}
				delete(aggregating, i)
			}
		}

		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				// ctx 取消表现为错误，不记为故障
				if ctx.Err() != nil {
					slog.Debug("stream cancelled", "err", err)
					return
				}
				events <- StreamEvent{Err: err}
				return
			}

			if len(resp.Choices) > 0 {
				delta := resp.Choices[0].Delta
				if delta.Content != "" {
					events <- StreamEvent{Delta: delta.Content}
				}
				for _, tc := range delta.ToolCalls {
					idx := 0
					if tc.Index != nil {
						idx = *tc.Index
					}
					ac, ok := aggregating[idx]
					if !ok {
						ac = &aggCall{}
						aggregating[idx] = ac
					}
					if tc.ID != "" {
						ac.id.WriteString(tc.ID)
					}
					if tc.Function.Name != "" {
						ac.name.WriteString(tc.Function.Name)
					}
					if tc.Function.Arguments != "" {
						ac.args.WriteString(tc.Function.Arguments)
					}
				}
				if resp.Choices[0].FinishReason == openai.FinishReasonToolCalls {
					emitToolCalls()
				}
			}
			// usage 在流末尾的独立 chunk（Choices 可能为空）
			if resp.Usage != nil {
				events <- StreamEvent{
					Usage: &Usage{
						PromptTokens:     int(resp.Usage.PromptTokens),
						CompletionTokens: int(resp.Usage.CompletionTokens),
						TotalTokens:      int(resp.Usage.TotalTokens),
					},
				}
			}
		}
		emitToolCalls()
		events <- StreamEvent{Done: true}
	}()

	return events, nil
}
