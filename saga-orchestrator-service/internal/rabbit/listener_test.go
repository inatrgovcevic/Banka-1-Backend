package rabbit_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/rabbit"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeConsumeChannel implements rabbit.ConsumeChannel.
type fakeConsumeChannel struct {
	ch         chan amqp.Delivery
	consumeErr error
	consumed   bool
}

func (f *fakeConsumeChannel) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
	f.consumed = true
	if f.consumeErr != nil {
		return nil, f.consumeErr
	}
	return f.ch, nil
}

// fakeAck records ack/nack calls so tests can verify dispatch behaviour.
type fakeAck struct {
	mu     sync.Mutex
	acks   int
	nacks  int
	ackErr error
}

func (a *fakeAck) Ack(tag uint64, multiple bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.acks++
	return a.ackErr
}

func (a *fakeAck) Nack(tag uint64, multiple, requeue bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.nacks++
	return nil
}

func (a *fakeAck) Reject(tag uint64, requeue bool) error { return nil }

func (a *fakeAck) snapshot() (int, int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.acks, a.nacks
}

func okDecoder(b []byte) (any, error) { return string(b), nil }

func TestListener_Run_ConsumeError(t *testing.T) {
	fc := &fakeConsumeChannel{consumeErr: errors.New("consume boom")}
	l := rabbit.NewListener(fc, "q", okDecoder, func(context.Context, any) error { return nil }, quietLogger())

	err := l.Run(context.Background())
	if err == nil {
		t.Fatal("expected error from Consume failure")
	}
}

func TestListener_Run_StopsOnContextCancel(t *testing.T) {
	fc := &fakeConsumeChannel{ch: make(chan amqp.Delivery)}
	l := rabbit.NewListener(fc, "q", okDecoder, func(context.Context, any) error { return nil }, quietLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- l.Run(ctx) }()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned error on cancel: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop on context cancel")
	}
}

func TestListener_Run_StopsWhenChannelClosed(t *testing.T) {
	deliveries := make(chan amqp.Delivery)
	fc := &fakeConsumeChannel{ch: deliveries}
	l := rabbit.NewListener(fc, "q", okDecoder, func(context.Context, any) error { return nil }, quietLogger())

	done := make(chan error, 1)
	go func() { done <- l.Run(context.Background()) }()

	close(deliveries) // simulate broker closing the consumer channel
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned error on channel close: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop when channel closed")
	}
}

func TestListener_Dispatch_Success_Acks(t *testing.T) {
	deliveries := make(chan amqp.Delivery, 1)
	fc := &fakeConsumeChannel{ch: deliveries}

	handled := make(chan struct{}, 1)
	handler := func(_ context.Context, payload any) error {
		if payload.(string) != "body" {
			t.Errorf("payload = %v, want body", payload)
		}
		handled <- struct{}{}
		return nil
	}

	l := rabbit.NewListener(fc, "q", okDecoder, handler, quietLogger())
	ack := &fakeAck{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- l.Run(ctx) }()

	deliveries <- amqp.Delivery{Acknowledger: ack, RoutingKey: "rk", Body: []byte("body")}
	<-handled
	// give dispatch time to ack
	waitFor(t, func() bool { a, _ := ack.snapshot(); return a == 1 })

	cancel()
	<-done

	a, n := ack.snapshot()
	if a != 1 || n != 0 {
		t.Errorf("acks=%d nacks=%d, want acks=1 nacks=0", a, n)
	}
}

func TestListener_Dispatch_DecodeError_Nacks(t *testing.T) {
	deliveries := make(chan amqp.Delivery, 1)
	fc := &fakeConsumeChannel{ch: deliveries}

	badDecoder := func(b []byte) (any, error) { return nil, errors.New("bad payload") }
	l := rabbit.NewListener(fc, "q", badDecoder, func(context.Context, any) error { return nil }, quietLogger())
	ack := &fakeAck{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- l.Run(ctx) }()

	deliveries <- amqp.Delivery{Acknowledger: ack, Body: []byte("garbage")}
	waitFor(t, func() bool { _, n := ack.snapshot(); return n == 1 })

	cancel()
	<-done

	a, n := ack.snapshot()
	if a != 0 || n != 1 {
		t.Errorf("acks=%d nacks=%d, want acks=0 nacks=1", a, n)
	}
}

func TestListener_Dispatch_HandlerError_Nacks(t *testing.T) {
	deliveries := make(chan amqp.Delivery, 1)
	fc := &fakeConsumeChannel{ch: deliveries}

	handler := func(context.Context, any) error { return errors.New("handler failed") }
	l := rabbit.NewListener(fc, "q", okDecoder, handler, quietLogger())
	ack := &fakeAck{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- l.Run(ctx) }()

	deliveries <- amqp.Delivery{Acknowledger: ack, Body: []byte("body")}
	waitFor(t, func() bool { _, n := ack.snapshot(); return n == 1 })

	cancel()
	<-done

	a, n := ack.snapshot()
	if a != 0 || n != 1 {
		t.Errorf("acks=%d nacks=%d, want acks=0 nacks=1", a, n)
	}
}

func TestListener_Dispatch_AckError_Logged(t *testing.T) {
	deliveries := make(chan amqp.Delivery, 1)
	fc := &fakeConsumeChannel{ch: deliveries}

	l := rabbit.NewListener(fc, "q", okDecoder, func(context.Context, any) error { return nil }, quietLogger())
	ack := &fakeAck{ackErr: errors.New("ack failed")}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- l.Run(ctx) }()

	deliveries <- amqp.Delivery{Acknowledger: ack, Body: []byte("body")}
	waitFor(t, func() bool { a, _ := ack.snapshot(); return a == 1 })

	cancel()
	<-done
}

// waitFor polls cond until true or a short timeout elapses.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}
