package session

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mk20mm/homeagent/internal/agent/gateway"
)

// Compaction 策略与阈值常量（T-DRC-17）。
const (
	CompactThreshold = 50 // 历史消息数触发阈值
	RetainRecent     = 10 // 保留最近的原文条数
	SummaryPrefix    = "[会话摘要] "
	MaxSummaryLength = 500 // 摘要最长字符数
)

// EffectiveHistory 从全量历史中提取有效窗口（包含最近一条会话摘要及之后的消息）。
// 原始历史在存储层永久保留，而传给 LLM 的窗口按摘要剪裁（不变量 4）。
func EffectiveHistory(history []Message) []Message {
	lastSummaryIdx := -1
	for i, m := range history {
		if m.Role == gateway.RoleSystem && strings.HasPrefix(m.Content, SummaryPrefix) {
			lastSummaryIdx = i
		}
	}
	if lastSummaryIdx >= 0 {
		return history[lastSummaryIdx:]
	}
	return history
}

// Compact 将超出阈值的历史消息压缩为摘要与保留近期原文（T-DRC-18）。
// 规则：
// 1. 若 len(history) <= CompactThreshold，直接返回原列表。
// 2. 将 threshold 之前的消息格式化为提示词，调用 LLM 生成 <=500 字摘要。
// 3. 替换为一条 {role: system, content: "[会话摘要] ..."} + 最近 RetainRecent 条原文。
func Compact(ctx context.Context, provider gateway.Provider, modelID string, history []Message) ([]Message, error) {
	if len(history) <= CompactThreshold || provider == nil {
		return history, nil
	}

	splitIdx := len(history) - RetainRecent
	if splitIdx <= 0 {
		return history, nil
	}

	toSummarize := history[:splitIdx]
	recent := history[splitIdx:]

	var sb strings.Builder
	for _, m := range toSummarize {
		switch m.Role {
		case gateway.RoleUser:
			sb.WriteString("用户: ")
			sb.WriteString(m.Content)
			sb.WriteString("\n")
		case gateway.RoleAssistant:
			sb.WriteString("助手: ")
			sb.WriteString(m.Content)
			sb.WriteString("\n")
		case gateway.RoleTool:
			sb.WriteString("工具[")
			sb.WriteString(m.Name)
			sb.WriteString("]: ")
			sb.WriteString(m.Content)
			sb.WriteString("\n")
		case gateway.RoleSystem:
			sb.WriteString(m.Content)
			sb.WriteString("\n")
		}
	}

	prompt := fmt.Sprintf(`请为以下家庭会话的历史对话生成一份简洁、完备的摘要（字数不超过 500 字）。
重点保留：家庭成员偏好、支出与账务记录、已做出的约定与决策、未完成的待办事项。

对话记录：
%s

请直接输出摘要，无需前缀或开场白：`, sb.String())

	chatReq := gateway.ChatRequest{
		Model: modelID,
		Messages: []gateway.Message{
			{Role: gateway.RoleUser, Content: prompt},
		},
	}

	ch, err := provider.StreamChat(ctx, chatReq)
	if err != nil {
		slog.Error("compaction stream chat failed", "err", err)
		return history, err
	}

	var summaryBuilder strings.Builder
	for e := range ch {
		if e.Err != nil {
			slog.Error("compaction stream delta error", "err", e.Err)
			return history, e.Err
		}
		if e.Delta != "" {
			summaryBuilder.WriteString(e.Delta)
		}
	}

	summaryText := strings.TrimSpace(summaryBuilder.String())
	if summaryText == "" {
		summaryText = "会话历史摘要生成为空。"
	}

	// 截断至最多 MaxSummaryLength 字符
	runes := []rune(summaryText)
	if len(runes) > MaxSummaryLength {
		summaryText = string(runes[:MaxSummaryLength])
	}

	summaryMsg := Message{
		Role:    gateway.RoleSystem,
		Content: SummaryPrefix + summaryText,
	}

	compacted := make([]Message, 0, 1+len(recent))
	compacted = append(compacted, summaryMsg)
	compacted = append(compacted, recent...)

	return compacted, nil
}
