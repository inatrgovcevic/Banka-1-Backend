package order

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"banka1/trading-service-go/internal/api"
)

// stubPublisher captures Publish calls.
type stubPublisher struct {
	calls   []string
	pubErr  error
}

func (p *stubPublisher) Publish(_ context.Context, routingKey string, _ any) error {
	p.calls = append(p.calls, routingKey)
	return p.pubErr
}
func (p *stubPublisher) PublishWithID(_ context.Context, routingKey, _ string, _ any) error {
	p.calls = append(p.calls, routingKey)
	return p.pubErr
}
func (p *stubPublisher) Close() error { return nil }

func newTestNotifier(pub *stubPublisher) *RabbitNotifier {
	return NewRabbitNotifier(pub, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func samplePayload() api.OrderNotificationPayload {
	return api.OrderNotificationPayload{OrderID: 1}
}

func TestRabbitNotifier_OrderApproved(t *testing.T) {
	pub := &stubPublisher{}
	n := newTestNotifier(pub)
	n.OrderApproved(context.Background(), samplePayload())
	if len(pub.calls) != 1 || pub.calls[0] != routingOrderApproved {
		t.Errorf("calls = %v, want [%s]", pub.calls, routingOrderApproved)
	}
}

func TestRabbitNotifier_OrderDeclined(t *testing.T) {
	pub := &stubPublisher{}
	n := newTestNotifier(pub)
	n.OrderDeclined(context.Background(), samplePayload())
	if len(pub.calls) != 1 || pub.calls[0] != routingOrderDeclined {
		t.Errorf("calls = %v", pub.calls)
	}
}

func TestRabbitNotifier_OrderCreated(t *testing.T) {
	pub := &stubPublisher{}
	n := newTestNotifier(pub)
	n.OrderCreated(context.Background(), samplePayload())
	if pub.calls[0] != routingOrderCreated {
		t.Errorf("routing = %q, want %q", pub.calls[0], routingOrderCreated)
	}
}

func TestRabbitNotifier_OrderDone(t *testing.T) {
	pub := &stubPublisher{}
	n := newTestNotifier(pub)
	n.OrderDone(context.Background(), samplePayload())
	if pub.calls[0] != routingOrderDone {
		t.Errorf("routing = %q, want %q", pub.calls[0], routingOrderDone)
	}
}

func TestRabbitNotifier_OrderPartialFill(t *testing.T) {
	pub := &stubPublisher{}
	n := newTestNotifier(pub)
	n.OrderPartialFill(context.Background(), samplePayload())
	if pub.calls[0] != routingOrderPartialFill {
		t.Errorf("routing = %q", pub.calls[0])
	}
}

func TestRabbitNotifier_OrderAutoCancelled(t *testing.T) {
	pub := &stubPublisher{}
	n := newTestNotifier(pub)
	n.OrderAutoCancelled(context.Background(), samplePayload())
	if pub.calls[0] != routingOrderAutoCancelled {
		t.Errorf("routing = %q", pub.calls[0])
	}
}

func TestRabbitNotifier_RecurringOrderSkipped(t *testing.T) {
	pub := &stubPublisher{}
	n := newTestNotifier(pub)
	n.RecurringOrderSkipped(context.Background(), api.RecurringOrderSkippedNotification{})
	if pub.calls[0] != routingOrderRecurringSkipped {
		t.Errorf("routing = %q", pub.calls[0])
	}
}

func TestRabbitNotifier_PublishError_Swallowed(t *testing.T) {
	pub := &stubPublisher{pubErr: errors.New("broker down")}
	n := newTestNotifier(pub)
	// Must not panic or return error.
	n.OrderApproved(context.Background(), samplePayload())
	n.OrderDeclined(context.Background(), samplePayload())
}

func TestRabbitNotifier_RecurringSkipped_PublishError(t *testing.T) {
	pub := &stubPublisher{pubErr: errors.New("fail")}
	n := newTestNotifier(pub)
	n.RecurringOrderSkipped(context.Background(), api.RecurringOrderSkippedNotification{})
}

// ---- NoopNotifier ----

func TestNoopNotifier_AllMethods(t *testing.T) {
	var n NoopNotifier
	n.OrderApproved(context.Background(), samplePayload())
	n.OrderDeclined(context.Background(), samplePayload())
	n.OrderCreated(context.Background(), samplePayload())
	n.OrderDone(context.Background(), samplePayload())
	n.OrderPartialFill(context.Background(), samplePayload())
	n.OrderAutoCancelled(context.Background(), samplePayload())
	n.RecurringOrderSkipped(context.Background(), api.RecurringOrderSkippedNotification{})
}

// ---- constants ----

func TestRoutingKeyConstants_NonEmpty(t *testing.T) {
	keys := []string{
		routingOrderApproved, routingOrderDeclined, routingOrderCreated,
		routingOrderDone, routingOrderPartialFill, routingOrderAutoCancelled,
		routingOrderRecurringSkipped,
	}
	for _, k := range keys {
		if k == "" {
			t.Errorf("routing key should not be empty")
		}
	}
}

func TestEventConstants_NonEmpty(t *testing.T) {
	events := []string{eventCreated, eventDone, eventPartialFill, eventAutoCancelled}
	for _, e := range events {
		if e == "" {
			t.Errorf("event constant should not be empty")
		}
	}
}
