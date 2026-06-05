package otc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"banka1/go-platform/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/shopspring/decimal"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ---- RabbitSagaPublisher ----------------------------------------------------

func TestRabbitSagaPublisher_PublishPremiumTransfer(t *testing.T) {
	pub := &sagaCapturePublisher{}
	p := NewRabbitSagaPublisher(pub, discardLogger())
	ev := PremiumTransferRequestedEvent{ContractID: 5, BuyerID: 1, SellerID: 2, Premium: decimal.RequireFromString("10")}
	if err := p.PublishPremiumTransferRequested(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(pub.keys) != 1 || pub.keys[0] != RoutingPremiumTransferRequested {
		t.Fatalf("routingKey = %v want %q", pub.keys, RoutingPremiumTransferRequested)
	}
}

func TestRabbitSagaPublisher_PublishExercise(t *testing.T) {
	pub := &sagaCapturePublisher{}
	p := NewRabbitSagaPublisher(pub, discardLogger())
	ev := ExerciseRequestedEvent{ContractID: 9, StockTicker: "AAPL", Amount: 3}
	if err := p.PublishExerciseRequested(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(pub.keys) != 1 || pub.keys[0] != RoutingExerciseRequested {
		t.Fatalf("routingKey = %v want %q", pub.keys, RoutingExerciseRequested)
	}
}

func TestRabbitSagaPublisher_PropagatesError(t *testing.T) {
	pub := &sagaCapturePublisher{err: errors.New("down")}
	p := NewRabbitSagaPublisher(pub, discardLogger())
	if err := p.PublishPremiumTransferRequested(context.Background(), PremiumTransferRequestedEvent{}); err == nil {
		t.Error("expected error to propagate")
	}
	if err := p.PublishExerciseRequested(context.Background(), ExerciseRequestedEvent{}); err == nil {
		t.Error("expected error to propagate")
	}
}

// ---- NoopSagaPublisher ------------------------------------------------------

func TestNoopSagaPublisher_DropsBoth(t *testing.T) {
	p := NewNoopSagaPublisher(discardLogger())
	if err := p.PublishPremiumTransferRequested(context.Background(), PremiumTransferRequestedEvent{ContractID: 1}); err != nil {
		t.Errorf("premium noop returned err: %v", err)
	}
	if err := p.PublishExerciseRequested(context.Background(), ExerciseRequestedEvent{ContractID: 1}); err != nil {
		t.Errorf("exercise noop returned err: %v", err)
	}
}

// ---- buildSagaHandler decode branches --------------------------------------

func TestBuildSagaHandler_RejectsBadJSON(t *testing.T) {
	b := otcConsumerBindings[0]
	h := buildSagaHandler(nil, b, discardLogger())
	res := h(context.Background(), rabbitmq.Envelope{Body: []byte("{not json")}, amqp.Delivery{})
	if res != rabbitmq.Reject {
		t.Errorf("bad JSON should Reject, got %v", res)
	}
}

func TestBuildSagaHandler_AcksMissingContractID(t *testing.T) {
	b := otcConsumerBindings[0]
	h := buildSagaHandler(nil, b, discardLogger())
	// Empty body -> evt zero value -> ContractID nil -> Ack (skip).
	res := h(context.Background(), rabbitmq.Envelope{Body: nil}, amqp.Delivery{RoutingKey: "x"})
	if res != rabbitmq.Ack {
		t.Errorf("missing contractId should Ack, got %v", res)
	}
	// Non-empty body without contractId -> Ack.
	res = h(context.Background(), rabbitmq.Envelope{Body: []byte(`{"reason":"x"}`)}, amqp.Delivery{RoutingKey: "x"})
	if res != rabbitmq.Ack {
		t.Errorf("body without contractId should Ack, got %v", res)
	}
}

// ---- binding table ----------------------------------------------------------

func TestOtcConsumerBindings_AreDistinct(t *testing.T) {
	if len(otcConsumerBindings) != 4 {
		t.Fatalf("expected 4 bindings, got %d", len(otcConsumerBindings))
	}
	queues := map[string]bool{}
	keys := map[string]bool{}
	for _, b := range otcConsumerBindings {
		if queues[b.queue] {
			t.Errorf("duplicate queue %q", b.queue)
		}
		if keys[b.bindingKey] {
			t.Errorf("duplicate binding key %q", b.bindingKey)
		}
		queues[b.queue] = true
		keys[b.bindingKey] = true
	}
}

// sagaCapturePublisher records routing keys for the raw saga events (the publisher
// receives the typed event struct, not a notificationRequest).
type sagaCapturePublisher struct {
	keys []string
	err  error
}

func (p *sagaCapturePublisher) Publish(_ context.Context, routingKey string, _ any) error {
	p.keys = append(p.keys, routingKey)
	return p.err
}

func (p *sagaCapturePublisher) PublishWithID(_ context.Context, routingKey, _ string, _ any) error {
	p.keys = append(p.keys, routingKey)
	return p.err
}

func (p *sagaCapturePublisher) Close() error { return nil }
