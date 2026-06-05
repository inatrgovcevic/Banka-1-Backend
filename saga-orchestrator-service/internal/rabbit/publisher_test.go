package rabbit_test

import (
	"context"
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/rabbit"
)

// fakePublishChannel implements rabbit.PublishChannel for tests.
type fakePublishChannel struct {
	calls   []publishCall
	failErr error
}

type publishCall struct {
	exchange   string
	key        string
	mandatory  bool
	immediate  bool
	msg        amqp.Publishing
}

func (f *fakePublishChannel) Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	f.calls = append(f.calls, publishCall{exchange, key, mandatory, immediate, msg})
	return f.failErr
}

func TestPublisher_Publish_Success(t *testing.T) {
	ch := &fakePublishChannel{}
	p := rabbit.NewPublisher(ch)

	body := []byte(`{"hello":"world"}`)
	if err := p.Publish(context.Background(), rabbit.RKOtcExerciseCompleted, body); err != nil {
		t.Fatalf("Publish error: %v", err)
	}

	if len(ch.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(ch.calls))
	}
	c := ch.calls[0]
	if c.exchange != rabbit.ExchangeSaga {
		t.Errorf("exchange = %q, want %q", c.exchange, rabbit.ExchangeSaga)
	}
	if c.key != rabbit.RKOtcExerciseCompleted {
		t.Errorf("routing key = %q, want %q", c.key, rabbit.RKOtcExerciseCompleted)
	}
	if c.mandatory || c.immediate {
		t.Errorf("mandatory/immediate should be false, got %v/%v", c.mandatory, c.immediate)
	}
	if c.msg.ContentType != "application/json" {
		t.Errorf("content type = %q, want application/json", c.msg.ContentType)
	}
	if c.msg.DeliveryMode != amqp.Persistent {
		t.Errorf("delivery mode = %d, want %d (persistent)", c.msg.DeliveryMode, amqp.Persistent)
	}
	if string(c.msg.Body) != string(body) {
		t.Errorf("body = %q, want %q", c.msg.Body, body)
	}
}

func TestPublisher_Publish_Error(t *testing.T) {
	ch := &fakePublishChannel{failErr: errors.New("broker gone")}
	p := rabbit.NewPublisher(ch)

	err := p.Publish(context.Background(), "rk", []byte("x"))
	if err == nil {
		t.Fatal("expected error when channel.Publish fails")
	}
	if !errors.Is(err, ch.failErr) {
		t.Errorf("error should wrap underlying cause; got %v", err)
	}
}
