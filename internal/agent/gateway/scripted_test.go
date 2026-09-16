package gateway

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mk20mm/homeagent/internal/agent/tool"
)

func TestScriptedProviderTwoRoundScript(t *testing.T) {
	p := NewScriptedProvider(RecordExpenseScript())

	// round 0：应含 tool_call
	ch, err := p.StreamChat(context.Background(), ChatRequest{Model: "fake"})
	if err != nil {
		t.Fatalf("StreamChat: %v", err)
	}
	var sawToolCall bool
	var deltas string
	for e := range ch {
		deltas += e.Delta
		if e.ToolCall != nil {
			sawToolCall = true
			if e.ToolCall.Name != "record_expense" {
				t.Fatalf("工具名错误: %s", e.ToolCall.Name)
			}
		}
		if !e.Done && e.Usage != nil {
			t.Fatal("Usage 只应在 Done 事件携带")
		}
	}
	if !sawToolCall {
		t.Fatal("round 0 应触发 tool_call")
	}
	if deltas != "好的，已记上" {
		t.Fatalf("文本聚合错误: %q", deltas)
	}

	// round 1：纯文本收尾，无 tool_call
	ch, _ = p.StreamChat(context.Background(), ChatRequest{})
	sawToolCall = false
	deltas = ""
	for e := range ch {
		deltas += e.Delta
		if e.ToolCall != nil {
			sawToolCall = true
		}
	}
	if sawToolCall {
		t.Fatal("round 1 不应再有 tool_call（否则死循环）")
	}
	if deltas != "已记账 ¥120，食材类。" {
		t.Fatalf("收尾文本错误: %q", deltas)
	}
	if p.Rounds() != 2 {
		t.Fatalf("应发生 2 轮调用，got %d", p.Rounds())
	}
}

func TestScriptedProviderCancelStopsStream(t *testing.T) {
	p := NewScriptedProvider(NoToolScript())
	p.DelayMS = 50 // 每事件 50ms，中途取消

	ctx, cancel := context.WithCancel(context.Background())
	ch, _ := p.StreamChat(ctx, ChatRequest{})

	go func() {
		time.Sleep(60 * time.Millisecond) // 第一个事件已出，第二个未出
		cancel()
	}()

	n := 0
	for range ch {
		n++
	}
	if n > 2 {
		t.Fatalf("ctx 取消应停止推送，但收到 %d 个事件", n)
	}
	// channel 必须已关闭（range 退出）
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("取消后 channel 应已关闭")
		}
	default:
	}
}

func TestScriptedProviderNilScriptGetsDone(t *testing.T) {
	p := NewScriptedProvider(func(int, ChatRequest) []StreamEvent { return nil })
	ch, _ := p.StreamChat(context.Background(), ChatRequest{})
	gotDone := false
	for e := range ch {
		if e.Done {
			gotDone = true
		}
	}
	if !gotDone {
		t.Fatal("空脚本必须补 Done，否则消费者挂死")
	}
}

func TestToolsFromSpecs(t *testing.T) {
	specs := []tool.Spec{
		{
			Name:        "record_expense",
			Description: "记录一笔家庭支出",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
	}
	defs := ToolsFromSpecs(specs)
	if len(defs) != 1 {
		t.Fatalf("应转换 1 个工具，got %d", len(defs))
	}
	if defs[0].Name != "record_expense" || defs[0].Description != "记录一笔家庭支出" {
		t.Fatalf("字段丢失: %+v", defs[0])
	}
	if string(defs[0].Parameters) != `{"type":"object"}` {
		t.Fatalf("Schema 未透传: %s", defs[0].Parameters)
	}
}
