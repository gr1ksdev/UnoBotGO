package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/mymmrac/telego"
)

const (
	ChatWorkerCount       = 8
	ChatQueueCapacity     = 32
	InlineWorkerCount     = 4
	InlineQueueCap        = 64
	RecoveryWorkerCount   = 2
	RecoveryQueueCapacity = 16
)

type chatTask struct {
	chatID     game.ChatID
	generation uint64
	ctx        context.Context
	fn         func(ctx context.Context)
}

type recoveryTask struct {
	chatID game.ChatID
	fn     func(ctx context.Context)
}

type chatGeneration struct {
	value  uint64
	ctx    context.Context
	cancel context.CancelFunc
	queue  chan chatTask
}

// Dispatcher manages concurrency, queue partitioning by ChatID, dedicated inline workers,
// backpressure via bounded channels, and orderly shutdown.
type Dispatcher struct {
	logger        *slog.Logger
	chatQueues    [ChatWorkerCount]chan chatTask
	inlineQueue   chan *telego.InlineQuery
	recoveryQueue chan recoveryTask
	inlineHandler func(ctx context.Context, query *telego.InlineQuery)
	chatMu        sync.Mutex
	chatStates    map[game.ChatID]chatGeneration

	ctx    context.Context
	cancel context.CancelFunc

	workersWg sync.WaitGroup
	stopped   sync.Once
}

func NewDispatcher(logger *slog.Logger, inlineHandler func(ctx context.Context, query *telego.InlineQuery)) *Dispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	d := &Dispatcher{
		logger:        logger,
		inlineQueue:   make(chan *telego.InlineQuery, InlineQueueCap),
		recoveryQueue: make(chan recoveryTask, RecoveryQueueCapacity),
		inlineHandler: inlineHandler,
		chatStates:    make(map[game.ChatID]chatGeneration),
		ctx:           ctx,
		cancel:        cancel,
	}

	for i := 0; i < ChatWorkerCount; i++ {
		d.chatQueues[i] = make(chan chatTask, ChatQueueCapacity)
	}

	d.start()
	return d
}

func (d *Dispatcher) start() {
	// Start chat partition workers
	for i := 0; i < ChatWorkerCount; i++ {
		workerID := i
		queue := d.chatQueues[i]
		d.workersWg.Add(1)
		go func() {
			defer d.workersWg.Done()
			for task := range queue {
				if !d.isCurrent(task.chatID, task.generation) {
					d.logger.Debug("discarded stale chat task", "chat_id", task.chatID, "generation", task.generation)
					continue
				}
				d.runSafely("chat", task.chatID, func() { task.fn(task.ctx) })
			}
		}()
		d.logger.Debug("started chat worker", "worker_id", workerID)
	}

	// Start inline query workers
	for i := 0; i < InlineWorkerCount; i++ {
		workerID := i
		d.workersWg.Add(1)
		go func() {
			defer d.workersWg.Done()
			for query := range d.inlineQueue {
				if d.inlineHandler != nil {
					d.runSafely("inline", 0, func() { d.inlineHandler(d.ctx, query) })
				}
			}
		}()
		d.logger.Debug("started inline worker", "worker_id", workerID)
	}

	for i := 0; i < RecoveryWorkerCount; i++ {
		workerID := i
		d.workersWg.Add(1)
		go func() {
			defer d.workersWg.Done()
			for task := range d.recoveryQueue {
				d.runSafely("recovery", task.chatID, func() { task.fn(d.ctx) })
			}
		}()
		d.logger.Debug("started recovery worker", "worker_id", workerID)
	}
}

func (d *Dispatcher) runSafely(kind string, chatID game.ChatID, fn func()) {
	defer func() {
		if recovered := recover(); recovered != nil {
			d.logger.Error("recovered worker panic",
				"worker_kind", kind,
				"chat_id", chatID,
				"panic", fmt.Sprint(recovered),
				"stack", string(debug.Stack()),
			)
		}
	}()
	fn()
}

func chatQueueIndex(chatID game.ChatID) int {
	val := int64(chatID)
	if val < 0 {
		val = -val
	}
	return int(val % ChatWorkerCount)
}

// EnqueueChat attempts to schedule work for a specific chat.
// Returns false if the queue is full (backpressure), without executing the action.
func (d *Dispatcher) EnqueueChat(chatID game.ChatID, fn func(ctx context.Context)) bool {
	if fn == nil || d.ctx.Err() != nil {
		return false
	}
	idx := chatQueueIndex(chatID)
	generation, taskCtx, dedicatedQueue := d.currentGeneration(chatID)
	task := chatTask{chatID: chatID, generation: generation, ctx: taskCtx, fn: fn}

	if dedicatedQueue != nil {
		select {
		case <-d.ctx.Done():
			return false
		case dedicatedQueue <- task:
			return true
		default:
			d.logger.Warn("dedicated chat queue saturated, dropping task", "chat_id", chatID, "generation", generation)
			return false
		}
	}
	select {
	case <-d.ctx.Done():
		return false
	case d.chatQueues[idx] <- task:
		return true
	default:
		d.logger.Warn("chat queue saturated, dropping task", "chat_id", chatID, "worker_idx", idx)
		return false
	}
}

// EnqueueRecovery schedules administrative recovery independently from normal
// chat queues, so it remains available when a chat queue is saturated.
func (d *Dispatcher) EnqueueRecovery(chatID game.ChatID, fn func(ctx context.Context)) bool {
	if fn == nil || d.ctx.Err() != nil {
		return false
	}
	task := recoveryTask{chatID: chatID, fn: fn}
	select {
	case <-d.ctx.Done():
		return false
	case d.recoveryQueue <- task:
		return true
	default:
		d.logger.Warn("recovery queue saturated, dropping task", "chat_id", chatID)
		return false
	}
}

// ResetChat cancels the current chat context and advances its generation.
// Queued work from older generations is discarded by workers before execution.
func (d *Dispatcher) ResetChat(chatID game.ChatID) uint64 {
	d.chatMu.Lock()
	state, ok := d.chatStates[chatID]
	if ok {
		state.cancel()
	} else {
		state.value = 0
	}
	state.value++
	state.ctx, state.cancel = context.WithCancel(d.ctx)
	state.queue = make(chan chatTask, ChatQueueCapacity)
	d.chatStates[chatID] = state
	d.chatMu.Unlock()
	d.workersWg.Add(1)
	go d.runDedicatedChat(state.ctx, state.queue)
	d.logger.Info("chat execution generation reset", "chat_id", chatID, "generation", state.value)
	return state.value
}

func (d *Dispatcher) currentGeneration(chatID game.ChatID) (uint64, context.Context, chan chatTask) {
	d.chatMu.Lock()
	defer d.chatMu.Unlock()
	state, ok := d.chatStates[chatID]
	if !ok {
		state.ctx, state.cancel = context.WithCancel(d.ctx)
		d.chatStates[chatID] = state
	}
	return state.value, state.ctx, state.queue
}

func (d *Dispatcher) isCurrent(chatID game.ChatID, generation uint64) bool {
	d.chatMu.Lock()
	defer d.chatMu.Unlock()
	state, ok := d.chatStates[chatID]
	return ok && state.value == generation
}

func (d *Dispatcher) runDedicatedChat(ctx context.Context, queue <-chan chatTask) {
	defer d.workersWg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-queue:
			if !d.isCurrent(task.chatID, task.generation) {
				continue
			}
			d.runSafely("dedicated_chat", task.chatID, func() { task.fn(task.ctx) })
		}
	}
}

// EnqueueInline attempts to schedule an inline query.
// Returns false if the inline queue is saturated.
func (d *Dispatcher) EnqueueInline(query *telego.InlineQuery) bool {
	if query == nil || d.ctx.Err() != nil {
		return false
	}

	select {
	case <-d.ctx.Done():
		return false
	case d.inlineQueue <- query:
		return true
	default:
		d.logger.Warn("inline queue saturated, dropping query", "query_id", query.ID, "user_id", query.From.ID)
		return false
	}
}

// Stop gracefully stops the dispatcher, draining queues up to timeout.
func (d *Dispatcher) Stop(timeout time.Duration) {
	d.stopped.Do(func() {
		// Stop accepting new admissions
		d.cancel()

		// Close ingress channels so workers drain remaining items
		for i := 0; i < ChatWorkerCount; i++ {
			close(d.chatQueues[i])
		}
		close(d.inlineQueue)
		close(d.recoveryQueue)

		done := make(chan struct{})
		go func() {
			d.workersWg.Wait()
			close(done)
		}()

		select {
		case <-done:
			d.logger.Info("all dispatch workers drained cleanly")
		case <-time.After(timeout):
			d.logger.Warn("dispatch workers drain timed out", "timeout", timeout)
		}
	})
}
