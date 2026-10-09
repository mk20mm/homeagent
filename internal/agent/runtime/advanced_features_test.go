package runtime

import (
	"context"
	"sync"
	"testing"
	"time"

	domrun "github.com/mk20mm/homeagent/internal/domain/run"
	"github.com/mk20mm/homeagent/internal/store/ent/schema"
)

func TestHub_RegisterAndCancel(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	steerCh := make(chan string, 4)

	hub.Register("run-test-1", cancel, steerCh)

	// Verify registration
	if !hub.IsActive("run-test-1") {
		t.Fatal("expected run-test-1 to be active in hub")
	}

	// Test Steer
	ok := hub.Steer("run-test-1", "请先暂停清扫")
	if !ok {
		t.Fatal("steer should return true for active run")
	}

	select {
	case msg := <-steerCh:
		if msg != "请先暂停清扫" {
			t.Fatalf("expected steered instruction '请先暂停清扫', got %q", msg)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for steered message")
	}

	// Test Cancel
	ok = hub.Cancel("run-test-1")
	if !ok {
		t.Fatal("cancel should return true for active run")
	}

	select {
	case <-ctx.Done():
		// Success!
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ctx was not cancelled after hub.Cancel")
	}

	hub.Unregister("run-test-1")
	if hub.IsActive("run-test-1") {
		t.Fatal("run-test-1 should not be active after unregister")
	}
}

func TestHub_LiveWatchersBroadcast(t *testing.T) {
	hub := NewHub()

	// 模拟 3 个客户端订阅 run-live-1
	ch1, unsub1 := hub.Subscribe("run-live-1")
	defer unsub1()
	ch2, unsub2 := hub.Subscribe("run-live-1")
	defer unsub2()
	ch3, unsub3 := hub.Subscribe("run-live-1")
	defer unsub3()

	testEvent := domrun.RunEventItem{
		ID:        "ev-1",
		Seq:       1,
		EventType: "tool_executed",
		Payload:   `{"tool":"vacuum_cleaner"}`,
		CreatedAt: time.Now(),
	}

	// 广播事件
	hub.Broadcast("run-live-1", testEvent)

	var wg sync.WaitGroup
	wg.Add(3)

	checkChan := func(ch <-chan domrun.RunEventItem, name string) {
		defer wg.Done()
		select {
		case ev := <-ch:
			if ev.ID != "ev-1" || ev.EventType != "tool_executed" {
				t.Errorf("%s received wrong event: %+v", name, ev)
			}
		case <-time.After(500 * time.Millisecond):
			t.Errorf("%s timed out waiting for broadcast event", name)
		}
	}

	go checkChan(ch1, "client1")
	go checkChan(ch2, "client2")
	go checkChan(ch3, "client3")

	wg.Wait()
}

func TestRuntime_SteerAndCancelIntegration(t *testing.T) {
	rt, _, _, _, _ := newTestRuntime(t, map[string]bool{"expense.write": true})

	// 模拟一个运行中的 Run 注册到 Runtime Hub
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	steerCh := make(chan string, 4)

	rt.Hub().Register("run-react-1", cancel, steerCh)
	defer rt.Hub().Unregister("run-react-1")

	// 外部调用 rt.Steer
	ok := rt.Steer("run-react-1", "优先洗衣服")
	if !ok {
		t.Fatal("rt.Steer should succeed for active run")
	}

	select {
	case instr := <-steerCh:
		if instr != "优先洗衣服" {
			t.Fatalf("expected '优先洗衣服', got %s", instr)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("steer instruction not received")
	}

	// 外部调用 rt.Cancel
	ok = rt.Cancel("run-react-1")
	if !ok {
		t.Fatal("rt.Cancel should succeed for active run")
	}

	select {
	case <-ctx.Done():
		// Cancelled successfully
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ctx was not cancelled")
	}
}

func TestCheckpoint_BroadcastsToHub(t *testing.T) {
	mockSvc := newRecoveryMockRunService()
	mockSvc.runs["run-ckpt-1"] = domrun.RunDetail{
		ID:      "run-ckpt-1",
		Status:  string(schema.RunStatusRunning),
		Version: 1,
	}
	hub := NewHub()

	ch, unsub := hub.Subscribe("run-ckpt-1")
	defer unsub()

	// 记录检查点
	checkpoint(context.Background(), mockSvc, hub, "run-ckpt-1", "", EventChildRunSpawned, `{"child_run_id":"child-1"}`)

	select {
	case ev := <-ch:
		if ev.EventType != EventChildRunSpawned {
			t.Fatalf("expected event type %s, got %s", EventChildRunSpawned, ev.EventType)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("hub subscriber did not receive checkpoint event")
	}
}
