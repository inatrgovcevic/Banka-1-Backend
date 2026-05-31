package order

import (
	"log/slog"
	"sync"
	"time"
)

// Worker is the Go equivalent of order-service's ThreadPoolTaskScheduler (pool
// size 4): a bounded set of goroutines that run delayed order-execution attempts.
// Scheduling a delayed attempt uses time.AfterFunc to enqueue the orderId after
// the delay; a worker then runs one execution attempt, which may reschedule
// itself (next portion, retry, or missing-quote backoff) — mirroring the
// self-rescheduling Java task.
//
// In-memory only (like the Java scheduler): pending attempts are lost on restart.
type Worker struct {
	process  func(orderID int64)
	logger   *slog.Logger
	poolSize int

	workCh chan int64
	quit   chan struct{}
	wg     sync.WaitGroup

	mu       sync.Mutex
	stopped  bool
	stopOnce sync.Once
}

// NewWorker builds a worker. poolSize mirrors ThreadPoolTaskScheduler.poolSize
// (4). process runs one execution attempt for an order id.
func NewWorker(process func(orderID int64), logger *slog.Logger, poolSize int) *Worker {
	if poolSize < 1 {
		poolSize = 1
	}
	return &Worker{
		process:  process,
		logger:   logger,
		poolSize: poolSize,
		workCh:   make(chan int64, 256),
		quit:     make(chan struct{}),
	}
}

// Start launches the worker goroutines.
func (w *Worker) Start() {
	for i := 0; i < w.poolSize; i++ {
		w.wg.Add(1)
		go w.loop()
	}
}

func (w *Worker) loop() {
	defer w.wg.Done()
	for {
		select {
		case orderID := <-w.workCh:
			w.runSafe(orderID)
		case <-w.quit:
			return
		}
	}
}

// runSafe isolates a panic in one attempt so it cannot take down the pool
// goroutine (mirrors the scheduler's ErrorHandler logging an uncaught task error).
func (w *Worker) runSafe(orderID int64) {
	defer func() {
		if p := recover(); p != nil {
			w.logger.Error("panic in order execution attempt", "orderId", orderID, "panic", p)
		}
	}()
	w.process(orderID)
}

// Schedule enqueues an execution attempt for orderID after delay. A negative
// delay is treated as 0 (mirrors Math.max(0, delay)). After Stop it is a no-op.
func (w *Worker) Schedule(orderID int64, delay time.Duration) {
	w.mu.Lock()
	stopped := w.stopped
	w.mu.Unlock()
	if stopped {
		return
	}
	if delay < 0 {
		delay = 0
	}
	time.AfterFunc(delay, func() {
		select {
		case w.workCh <- orderID:
		case <-w.quit:
		}
	})
}

// Stop blocks new scheduling, signals the workers to exit, and waits for
// in-flight attempts to finish (matches setWaitForTasksToCompleteOnShutdown).
func (w *Worker) Stop() {
	w.stopOnce.Do(func() {
		w.mu.Lock()
		w.stopped = true
		w.mu.Unlock()
		close(w.quit)
		w.wg.Wait()
	})
}
