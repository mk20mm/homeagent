// Package prompt 系统提示词构建（ARCHITECTURE §2.2 组件②）。
//
// 安全核心：工具定义经权限过滤后通过 ChatRequest.Tools 下发（function calling），
// 提示词正文只放人设与行为约束，不内联工具清单（ADR-005 源头过滤在 registry 完成）。
package prompt

import (
	"fmt"
	"strings"
	"time"

	"github.com/mk20mm/homeagent/internal/store/ent/schema"
)

// Options 提示词构建输入。
type Options struct {
	MemberName string
	Role       schema.Role
	Now        time.Time
}

// Build 构建系统提示词：人设 + 家庭信息 + 时间 + 行为约束。
func Build(o Options) string {
	role := schema.Role(o.Role)
	var b strings.Builder

	b.WriteString("你是「家事助手」，一个家庭协作中枢。家人说一句话，你调用后端工具把家务、用餐、账单安排到位。\n\n")

	// 按角色切语气（老人简洁、小孩活泼）
	switch role {
	case schema.RoleElder:
		b.WriteString("说话对象是家中老人：用词简单直接，一次只说一件事，重要操作要重复确认。避免网络用语。\n")
	case schema.RoleChild:
		b.WriteString("说话对象是家中孩子：语气活泼友好，多用鼓励，涉及花钱的操作必须先问家长。\n")
	default:
		b.WriteString("说话对象是家中成年人：简洁高效，直接给结果。\n")
	}

	b.WriteString("\n行为约束：\n")
	b.WriteString("- 只做事务执行，不闲聊；无法理解意图时直接追问，不要猜。\n")
	b.WriteString("- 涉及花钱的操作，执行后必须告知金额和分类，并提示可以撤销。\n")
	b.WriteString("- 不要编造没执行过的结果。工具失败就如实说失败原因。\n")

	// 时间（农历在 family.Lunar 开启时由上层补充）
	b.WriteString("\n当前时间：")
	b.WriteString(o.Now.Format("2006-01-02 15:04 Monday"))
	if o.MemberName != "" {
		fmt.Fprintf(&b, "。说话人：%s。", o.MemberName)
	}

	return b.String()
}
