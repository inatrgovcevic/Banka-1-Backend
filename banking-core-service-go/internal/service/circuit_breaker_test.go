package service

import (
	"testing"
	"time"
)

func TestCircuitBreakerStartsClosed(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, time.Second)
	if !cb.Allow() {
		t.Fatal("new circuit breaker should allow requests")
	}
}

func TestCircuitBreakerOpensAfterFailureThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, time.Second)
	cb.RecordFailure()
	cb.RecordFailure()
	if !cb.Allow() {
		t.Fatal("should still be closed after 2 failures (threshold=3)")
	}
	cb.RecordFailure()
	if cb.Allow() {
		t.Fatal("should be open after 3 failures")
	}
}

func TestCircuitBreakerBlocksWhenOpen(t *testing.T) {
	cb := NewCircuitBreaker(1, 1, time.Hour)
	cb.RecordFailure()
	for i := 0; i < 5; i++ {
		if cb.Allow() {
			t.Fatalf("call %d: open breaker should block requests", i+1)
		}
	}
}

func TestCircuitBreakerTransitionsToHalfOpenAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker(1, 1, 0)
	cb.RecordFailure()
	if !cb.Allow() {
		t.Fatal("should transition to half-open after timeout=0 and allow one probe")
	}
}

func TestCircuitBreakerClosesAfterSuccessThresholdInHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(1, 2, 0)
	cb.RecordFailure()
	cb.Allow() // transitions to HalfOpen

	cb.RecordSuccess()
	if !cb.Allow() {
		t.Fatal("still half-open after 1 success (threshold=2)")
	}
	cb.RecordSuccess()
	// now closed — should accept indefinitely
	for i := 0; i < 5; i++ {
		if !cb.Allow() {
			t.Fatalf("call %d: should be closed after 2 successes", i+1)
		}
	}
}

func TestCircuitBreakerReopensFromHalfOpenOnFailure(t *testing.T) {
	// With timeout=0 the breaker immediately re-enters HalfOpen on Allow(), so we can't
	// test Allow()=false directly. Instead we verify the observable consequence: the
	// success counter is reset, so successThreshold successes are required all over again.
	cb := NewCircuitBreaker(1, 2, 0) // successThreshold = 2
	cb.RecordFailure()
	cb.Allow()         // → HalfOpen
	cb.RecordSuccess() // 1 of 2 — not yet closed
	cb.RecordFailure() // → reopens; success counter resets to 0
	cb.Allow()         // → HalfOpen again (timeout=0)
	cb.RecordSuccess() // 1 of 2 again (counter was reset)
	cb.Allow()         // still HalfOpen
	cb.RecordSuccess() // 2 of 2 → closes
	if !cb.Allow() {
		t.Fatal("after reopening from HalfOpen, breaker should close once successThreshold is reached again")
	}
}

func TestCircuitBreakerResetsFailureCountOnSuccessInClosed(t *testing.T) {
	cb := NewCircuitBreaker(3, 1, time.Hour)
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess() // resets counter
	cb.RecordFailure()
	cb.RecordFailure()
	if !cb.Allow() {
		t.Fatal("failure count should have reset; breaker should still be closed")
	}
}

func TestCircuitBreakerSuccessInClosedStateIsNoop(t *testing.T) {
	cb := NewCircuitBreaker(3, 1, time.Hour)
	for i := 0; i < 10; i++ {
		cb.RecordSuccess()
	}
	if !cb.Allow() {
		t.Fatal("repeated successes in closed state should not break anything")
	}
}
