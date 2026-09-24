package telegram

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/mymmrac/telego"
)

func TestDispatcher_ChatOrder(t *testing.T) {
	d := NewDispatcher(nil, nil)
	defer d.Stop(2 * time.Second)

	chatID := game.ChatID(-100123)
	const count = 20

	var executed []int
	var mu sync.Mutex
	done := make(chan struct{})

	for i := 0; i < count; i++ {
		seq := i
		ok := d.EnqueueChat(chatID, func(ctx context.Context) {
			mu.Lock()
			executed = append(executed, seq)
			if len(executed) == count {
				close(done)
			}
			mu.Unlock()
		})
		if !ok {
			t.Fatalf("failed to enqueue task %d", seq)
		}
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for tasks")
	}

	mu.Lock()
	defer mu.Unlock()
	for i := 0; i < count; i++ {
		if executed[i] != i {
			t.Fatalf("expected order %d, got %d", i, executed[i])
		}
	}
}

func TestDispatcher_ChatBackpressure(t *testing.T) {
	d := NewDispatcher(nil, nil)
	defer d.Stop(2 * time.Second)

	chatID := game.ChatID(-100999)
	blocker := make(chan struct{})
	started := make(chan struct{})

	// Enqueue one task that blocks the worker once popped
	d.EnqueueChat(chatID, func(ctx context.Context) {
		close(started)
		<-blocker
	})

	// Wait for worker to pop the first task and start blocking
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatalf("worker did not start in time")
	}

	// Now channel buffer of 32 tasks can be filled
	for i := 0; i < ChatQueueCapacity; i++ {
		if !d.EnqueueChat(chatID, func(ctx context.Context) {}) {
			close(blocker)
			t.Fatalf("failed to enqueue task %d in buffer", i)
		}
	}

	// 33rd task should be refused due to backpressure
	refused := d.EnqueueChat(chatID, func(ctx context.Context) {})
	close(blocker)

	if refused {
		t.Errorf("expected task to be refused due to queue saturation")
	}
}

func TestDispatcher_InlineWorkers(t *testing.T) {
	var processed atomic.Int32
	done := make(chan struct{})

	handler := func(ctx context.Context, q *telego.InlineQuery) {
		if processed.Add(1) == 10 {
			close(done)
		}
	}

	d := NewDispatcher(nil, handler)
	defer d.Stop(2 * time.Second)

	for i := 0; i < 10; i++ {
		q := &telego.InlineQuery{ID: "q"}
		if !d.EnqueueInline(q) {
			t.Fatalf("failed to enqueue inline query %d", i)
		}
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for inline queries")
	}

	if processed.Load() != 10 {
		t.Errorf("expected 10 processed queries, got %d", processed.Load())
	}
}

func TestDispatcher_ShutdownDrain(t *testing.T) {
	var executed atomic.Int32

	d := NewDispatcher(nil, nil)

	chatID := game.ChatID(42)
	for i := 0; i < 15; i++ {
		d.EnqueueChat(chatID, func(ctx context.Context) {
			time.Sleep(5 * time.Millisecond)
			executed.Add(1)
		})
	}

	// Stop dispatcher and verify all 15 tasks are drained
	d.Stop(5 * time.Second)

	if executed.Load() != 15 {
		t.Errorf("expected all 15 tasks to be drained on shutdown, got %d", executed.Load())
	}
}

func TestDispatcher_ResetCancelsCurrentAndDiscardsStaleTasks(t *testing.T) {
	d := NewDispatcher(nil, nil)
	defer d.Stop(2 * time.Second)
	chatID := game.ChatID(-200)
	started := make(chan struct{})
	canceled := make(chan struct{})
	fresh := make(chan struct{})
	otherChat := make(chan struct{})
	var stale atomic.Int32

	if !d.EnqueueChat(chatID, func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		close(canceled)
	}) {
		t.Fatal("failed to enqueue blocking task")
	}
	waitDispatchSignal(t, started)
	if !d.EnqueueChat(chatID, func(context.Context) { stale.Add(1) }) {
		t.Fatal("failed to enqueue stale task")
	}
	d.ResetChat(chatID)
	if !d.EnqueueChat(chatID, func(context.Context) { close(fresh) }) {
		t.Fatal("failed to enqueue fresh task")
	}
	// -208 hashes to the same worker as -200 and must resume after cancellation.
	if !d.EnqueueChat(-208, func(context.Context) { close(otherChat) }) {
		t.Fatal("failed to enqueue other chat")
	}
	waitDispatchSignal(t, canceled)
	waitDispatchSignal(t, fresh)
	waitDispatchSignal(t, otherChat)
	if stale.Load() != 0 {
		t.Fatal("stale task executed after reset")
	}
}

func TestDispatcher_RecoveryBypassesSaturatedChatQueue(t *testing.T) {
	d := NewDispatcher(nil, nil)
	defer d.Stop(2 * time.Second)
	chatID := game.ChatID(-300)
	blocker := make(chan struct{})
	started := make(chan struct{})
	if !d.EnqueueChat(chatID, func(context.Context) { close(started); <-blocker }) {
		t.Fatal("failed to enqueue blocker")
	}
	waitDispatchSignal(t, started)
	for i := 0; i < ChatQueueCapacity; i++ {
		if !d.EnqueueChat(chatID, func(context.Context) {}) {
			close(blocker)
			t.Fatalf("queue saturated early at %d", i)
		}
	}
	recovered := make(chan struct{})
	if !d.EnqueueRecovery(chatID, func(context.Context) { close(recovered) }) {
		close(blocker)
		t.Fatal("recovery queue rejected task")
	}
	waitDispatchSignal(t, recovered)
	close(blocker)
}

func TestDispatcher_RecoversPanicsInEveryWorkerClass(t *testing.T) {
	var inlineCalls atomic.Int32
	inlineDone := make(chan struct{})
	d := NewDispatcher(nil, func(context.Context, *telego.InlineQuery) {
		if inlineCalls.Add(1) == 1 {
			panic("inline boom")
		}
		close(inlineDone)
	})
	defer d.Stop(2 * time.Second)

	chatDone := make(chan struct{})
	d.EnqueueChat(-400, func(context.Context) { panic("chat boom") })
	d.EnqueueChat(-400, func(context.Context) { close(chatDone) })
	d.EnqueueInline(&telego.InlineQuery{ID: "panic"})
	d.EnqueueInline(&telego.InlineQuery{ID: "after"})
	recoveryDone := make(chan struct{})
	d.EnqueueRecovery(-400, func(context.Context) { panic("recovery boom") })
	d.EnqueueRecovery(-400, func(context.Context) { close(recoveryDone) })

	waitDispatchSignal(t, chatDone)
	waitDispatchSignal(t, inlineDone)
	waitDispatchSignal(t, recoveryDone)
}

func waitDispatchSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for dispatcher signal")
	}
}
