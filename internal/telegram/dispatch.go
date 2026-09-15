package telegram

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/mymmrac/telego"
)

const (
	ChatWorkerCount   = 8
	ChatQueueCapacity = 32
	InlineWorkerCount = 4
	InlineQueueCap    = 64
)

type chatTask struct {
	chatID game.ChatID
	fn     func(ctx context.Context)
}

// Dispatcher manages concurrency, queue partitioning by ChatID, dedicated inline workers,
// backpressure via bounded channels, and orderly shutdown.
type Dispatcher struct {
	logger        *slog.Logger
	chatQueues    [ChatWorkerCount]chan chatTask
	inlineQueue   chan *telego.InlineQuery
	inlineHandler func(ctx context.Context, query *telego.InlineQuery)

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
		inlineHandler: inlineHandler,
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
				task.fn(d.ctx)
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
					d.inlineHandler(d.ctx, query)
				}
			}
		}()
		d.logger.Debug("started inline worker", "worker_id", workerID)
	}
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
	idx := chatQueueIndex(chatID)
	task := chatTask{chatID: chatID, fn: fn}

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

// EnqueueInline attempts to schedule an inline query.
// Returns false if the inline queue is saturated.
func (d *Dispatcher) EnqueueInline(query *telego.InlineQuery) bool {
	if query == nil {
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
