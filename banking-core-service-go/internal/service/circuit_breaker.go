package service

import (
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen is returned by CircuitBreaker.Allow when the breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker open")

const (
	cbStateClosed   = 0
	cbStateOpen     = 1
	cbStateHalfOpen = 2
)

// CircuitBreaker is a simple three-state circuit breaker (Closed → Open → HalfOpen → Closed).
// It is thread-safe and counts only "retryable" failures (network errors, 5xx) as failures.
type CircuitBreaker struct {
	mu               sync.Mutex
	state            int
	failures         int
	successes        int
	failureThreshold int
	successThreshold int
	recoveryTimeout  time.Duration
	openedAt         time.Time
}

// NewCircuitBreaker creates a circuit breaker that opens after failureThreshold consecutive
// retryable failures, waits recoveryTimeout before moving to half-open, and closes again
// after successThreshold consecutive successes in the half-open state.
func NewCircuitBreaker(failureThreshold, successThreshold int, recoveryTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		recoveryTimeout:  recoveryTimeout,
	}
}

// Allow returns true when the breaker permits a request.
// Closed → always true.
// Open → false until recoveryTimeout elapses, then transitions to HalfOpen.
// HalfOpen → true (one probe at a time).
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case cbStateClosed:
		return true
	case cbStateOpen:
		if time.Since(cb.openedAt) >= cb.recoveryTimeout {
			cb.state = cbStateHalfOpen
			cb.successes = 0
			return true
		}
		return false
	case cbStateHalfOpen:
		return true
	}
	return false
}

// RecordSuccess notifies the breaker that the last request succeeded.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case cbStateClosed:
		cb.failures = 0
	case cbStateHalfOpen:
		cb.successes++
		if cb.successes >= cb.successThreshold {
			cb.state = cbStateClosed
			cb.failures = 0
			cb.successes = 0
		}
	}
}

// RecordFailure notifies the breaker that the last request failed in a retryable way.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case cbStateClosed:
		cb.failures++
		if cb.failures >= cb.failureThreshold {
			cb.state = cbStateOpen
			cb.openedAt = time.Now()
		}
	case cbStateHalfOpen:
		cb.state = cbStateOpen
		cb.openedAt = time.Now()
		cb.successes = 0
	}
}
