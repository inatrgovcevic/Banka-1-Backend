package rabbit_test

import (
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/rabbit"
)

// ---------------------------------------------------------------------------
// Mock channel for unit tests — does not require a running broker.
// ---------------------------------------------------------------------------

type mockChannel struct {
	exchanges []exchangeDecl
	queues    []queueDecl
	bindings  []bindingDecl
}

type exchangeDecl struct{ name, kind string }
type queueDecl struct{ name string }
type bindingDecl struct{ queue, key, exchange string }

func (m *mockChannel) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	m.exchanges = append(m.exchanges, exchangeDecl{name, kind})
	return nil
}

func (m *mockChannel) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	m.queues = append(m.queues, queueDecl{name})
	return amqp.Queue{Name: name}, nil
}

func (m *mockChannel) QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
	m.bindings = append(m.bindings, bindingDecl{name, key, exchange})
	return nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestDeclareTopology_ExchangeDeclared(t *testing.T) {
	ch := &mockChannel{}
	if err := rabbit.DeclareTopology(ch); err != nil {
		t.Fatalf("DeclareTopology error: %v", err)
	}
	if len(ch.exchanges) != 1 {
		t.Fatalf("expected 1 exchange, got %d", len(ch.exchanges))
	}
	if ch.exchanges[0].name != rabbit.ExchangeSaga {
		t.Errorf("exchange name %q, want %q", ch.exchanges[0].name, rabbit.ExchangeSaga)
	}
	if ch.exchanges[0].kind != "topic" {
		t.Errorf("exchange kind %q, want topic", ch.exchanges[0].kind)
	}
}

func TestDeclareTopology_DLQDeclared(t *testing.T) {
	ch := &mockChannel{}
	if err := rabbit.DeclareTopology(ch); err != nil {
		t.Fatalf("DeclareTopology error: %v", err)
	}
	found := false
	for _, q := range ch.queues {
		if q.name == rabbit.DLQName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("DLQ %q not declared; queues: %v", rabbit.DLQName, ch.queues)
	}
}

func TestDeclareTopology_AllTriggerQueuesDeclared(t *testing.T) {
	ch := &mockChannel{}
	if err := rabbit.DeclareTopology(ch); err != nil {
		t.Fatalf("DeclareTopology error: %v", err)
	}

	expected := []string{
		rabbit.QueueOtcExercise,
		rabbit.QueueOtcPremium,
		rabbit.QueueFundSubscribe,
		rabbit.QueueFundRedeem,
		rabbit.QueueFundRedeemWithLiquidation,
	}
	queueSet := make(map[string]struct{}, len(ch.queues))
	for _, q := range ch.queues {
		queueSet[q.name] = struct{}{}
	}
	for _, name := range expected {
		if _, ok := queueSet[name]; !ok {
			t.Errorf("expected queue %q to be declared", name)
		}
	}
}

func TestDeclareTopology_AllBindingsDeclared(t *testing.T) {
	ch := &mockChannel{}
	if err := rabbit.DeclareTopology(ch); err != nil {
		t.Fatalf("DeclareTopology error: %v", err)
	}

	type queueRK struct{ queue, rk string }
	expected := []queueRK{
		{rabbit.QueueOtcExercise, rabbit.RKOtcExerciseRequested},
		{rabbit.QueueOtcPremium, rabbit.RKOtcPremiumRequested},
		{rabbit.QueueFundSubscribe, rabbit.RKFundSubscribeRequested},
		{rabbit.QueueFundRedeem, rabbit.RKFundRedeemRequested},
		{rabbit.QueueFundRedeemWithLiquidation, rabbit.RKFundRedeemWithLiquidationRequested},
	}

	bindSet := make(map[queueRK]struct{}, len(ch.bindings))
	for _, b := range ch.bindings {
		bindSet[queueRK{b.queue, b.key}] = struct{}{}
	}

	for _, want := range expected {
		if _, ok := bindSet[want]; !ok {
			t.Errorf("expected binding queue=%q rk=%q", want.queue, want.rk)
		}
	}
}

func TestDeclareTopology_AllBindingsUseCorrectExchange(t *testing.T) {
	ch := &mockChannel{}
	if err := rabbit.DeclareTopology(ch); err != nil {
		t.Fatalf("DeclareTopology error: %v", err)
	}
	for _, b := range ch.bindings {
		if b.exchange != rabbit.ExchangeSaga {
			t.Errorf("binding %q/%q uses exchange %q, want %q", b.queue, b.key, b.exchange, rabbit.ExchangeSaga)
		}
	}
}

func TestDeclareTopology_Idempotent(t *testing.T) {
	// Calling DeclareTopology twice should succeed (mock always returns nil).
	ch := &mockChannel{}
	for i := 0; i < 2; i++ {
		if err := rabbit.DeclareTopology(ch); err != nil {
			t.Fatalf("DeclareTopology call %d error: %v", i+1, err)
		}
	}
}
