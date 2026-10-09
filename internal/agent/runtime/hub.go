package runtime

import (
	"context"
	"sync"

	domrun "github.com/mk20mm/homeagent/internal/domain/run"
)

type activeRunContext struct {
	cancel    context.CancelFunc
	steerChan chan string
}

// Hub 管理活跃 Run 的并发控制（Steering/Cancel）与多端观察者广播（Live Watchers）。
type Hub struct {
	mu       sync.RWMutex
	active   map[string]*activeRunContext
	watchers map[string][]chan domrun.RunEventItem
}

// NewHub 创建并发管控与事件广播中心。
func NewHub() *Hub {
	return &Hub{
		active:   make(map[string]*activeRunContext),
		watchers: make(map[string][]chan domrun.RunEventItem),
	}
}

// Register 注册正在执行中的 Run。
func (h *Hub) Register(runID string, cancel context.CancelFunc, steerChan chan string) {
	if runID == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.active[runID] = &activeRunContext{
		cancel:    cancel,
		steerChan: steerChan,
	}
}

// Unregister 移除已结束的活跃 Run。
func (h *Hub) Unregister(runID string) {
	if runID == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.active, runID)
}

// IsActive 检查 Run 是否在活跃运行中。
func (h *Hub) IsActive(runID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.active[runID]
	return ok
}

// Steer 向活跃中的 Run 发送实时人类干预指令。返回是否成功送达活跃循环。
func (h *Hub) Steer(runID string, instruction string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	runCtx, ok := h.active[runID]
	if !ok || runCtx.steerChan == nil {
		return false
	}
	select {
	case runCtx.steerChan <- instruction:
		return true
	default:
		return false
	}
}

// Cancel 紧急中断正在活跃执行中的 Run。
func (h *Hub) Cancel(runID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	runCtx, ok := h.active[runID]
	if !ok || runCtx.cancel == nil {
		return false
	}
	runCtx.cancel()
	return true
}

// Subscribe 订阅指定 Run 的事件广播（Live Watchers）。返回只读事件通道与退订函数。
func (h *Hub) Subscribe(runID string) (<-chan domrun.RunEventItem, func()) {
	ch := make(chan domrun.RunEventItem, 32)
	h.mu.Lock()
	h.watchers[runID] = append(h.watchers[runID], ch)
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		list := h.watchers[runID]
		for i, c := range list {
			if c == ch {
				h.watchers[runID] = append(list[:i], list[i+1:]...)
				close(ch)
				break
			}
		}
		if len(h.watchers[runID]) == 0 {
			delete(h.watchers, runID)
		}
	}

	return ch, unsubscribe
}

// Broadcast 向所有观察该 Run 的客户端广播一条新事件。
func (h *Hub) Broadcast(runID string, event domrun.RunEventItem) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	watchers := h.watchers[runID]
	for _, ch := range watchers {
		select {
		case ch <- event:
		default:
			// 慢客户端不阻塞全局调度循环
		}
	}
}
