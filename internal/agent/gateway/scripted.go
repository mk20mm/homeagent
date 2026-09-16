package gateway

import (
	"context"
	"sync"
	"time"
)

// ScriptedProvider 是可编程的假供应商：按轮次脚本返回事件，不依赖真实 API。
//
// 用途（C 阶段 P0）：跑通「对话→执行→撤销」全链路，不花一分钱、不等网络。
// 行为：
//   - 第 n 次调用（round 从 0 计）执行 Script(n, req)，事件顺序推入 channel
//   - 若脚本返回 nil 事件，推一个空 Done 结束（防止挂死）
//   - 记录所有请求（Recorded）供测试断言 ReAct 行为
type ScriptedProvider struct {
	Script   func(round int, req ChatRequest) []StreamEvent
	DelayMS  int // 每事件间隔毫秒，模拟流式节奏
	mu       sync.Mutex
	round    int
	Recorded []ChatRequest
}

func NewScriptedProvider(script func(round int, req ChatRequest) []StreamEvent) *ScriptedProvider {
	return &ScriptedProvider{Script: script}
}

func (p *ScriptedProvider) Name() string { return "fake" }

// StreamChat 执行一轮脚本。ctx 取消时立即关闭通道（模拟断线取消）。
func (p *ScriptedProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	p.mu.Lock()
	round := p.round
	p.round++
	p.Recorded = append(p.Recorded, req)
	p.mu.Unlock()

	events := p.Script(round, req)
	if events == nil {
		events = []StreamEvent{{Done: true}}
	}

	ch := make(chan StreamEvent, len(events))
	go func() {
		defer close(ch)
		for _, e := range events {
			if p.DelayMS > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(p.DelayMS) * time.Millisecond):
				}
			}
			select {
			case <-ctx.Done():
				return
			case ch <- e:
			}
		}
		// 脚本没给 Done，补一个（保证消费者能正常退出）
		if len(events) == 0 || !events[len(events)-1].Done {
			select {
			case <-ctx.Done():
			case ch <- StreamEvent{Done: true}:
			}
		}
	}()
	return ch, nil
}

// Reset 重置轮次计数（测试隔离用）。
func (p *ScriptedProvider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.round = 0
	p.Recorded = nil
}

// Round 已发生过的调用次数。
func (p *ScriptedProvider) Rounds() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.round
}
