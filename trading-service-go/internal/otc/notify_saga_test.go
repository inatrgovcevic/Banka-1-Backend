package otc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"banka1/trading-service-go/internal/clients"

	"banka1/go-platform/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/shopspring/decimal"
)

// ---- shared fakes ----------------------------------------------------------

// recordingPublisher captures rabbitmq publishes (notifier + saga publisher).
type recordingPublisher struct {
	keys     []string
	payloads []any
	err      error
}

func (p *recordingPublisher) Publish(_ context.Context, key string, payload any) error {
	p.keys = append(p.keys, key)
	p.payloads = append(p.payloads, payload)
	return p.err
}
func (p *recordingPublisher) PublishWithID(_ context.Context, key, _ string, payload any) error {
	p.keys = append(p.keys, key)
	p.payloads = append(p.payloads, payload)
	return p.err
}
func (p *recordingPublisher) Close() error { return nil }

// doerFunc adapts a function into clients.HTTPDoer.
type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }

// jsonResponse builds a 200 JSON response from v.
func jsonResponse(v any) *http.Response {
	b, _ := json.Marshal(v)
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(string(b))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func customerClientReturning(cust *clients.Customer, status int) *clients.CustomerClient {
	doer := doerFunc(func(_ *http.Request) (*http.Response, error) {
		if status == 404 {
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader(""))}, nil
		}
		return jsonResponse(cust), nil
	})
	return clients.NewCustomerClient("http://user", nil, doer)
}

// ---- RabbitSagaPublisher ---------------------------------------------------

func TestRabbitSagaPublisher_Premium(t *testing.T) {
	pub := &recordingPublisher{}
	p := NewRabbitSagaPublisher(pub, discard())
	ev := PremiumTransferRequestedEvent{ContractID: 5, Premium: dec("10")}
	if err := p.PublishPremiumTransferRequested(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(pub.keys) != 1 || pub.keys[0] != RoutingPremiumTransferRequested {
		t.Fatalf("keys = %v", pub.keys)
	}
}

func TestRabbitSagaPublisher_Exercise(t *testing.T) {
	pub := &recordingPublisher{}
	p := NewRabbitSagaPublisher(pub, discard())
	if err := p.PublishExerciseRequested(context.Background(), ExerciseRequestedEvent{ContractID: 9, StockTicker: "AAPL"}); err != nil {
		t.Fatal(err)
	}
	if len(pub.keys) != 1 || pub.keys[0] != RoutingExerciseRequested {
		t.Fatalf("keys = %v", pub.keys)
	}
}

func TestRabbitSagaPublisher_Error(t *testing.T) {
	pub := &recordingPublisher{err: errors.New("down")}
	p := NewRabbitSagaPublisher(pub, discard())
	if err := p.PublishPremiumTransferRequested(context.Background(), PremiumTransferRequestedEvent{}); err == nil {
		t.Error("expected error")
	}
	if err := p.PublishExerciseRequested(context.Background(), ExerciseRequestedEvent{}); err == nil {
		t.Error("expected error")
	}
}

func TestNoopSagaPublisher(t *testing.T) {
	p := NewNoopSagaPublisher(discard())
	if err := p.PublishPremiumTransferRequested(context.Background(), PremiumTransferRequestedEvent{ContractID: 1}); err != nil {
		t.Error(err)
	}
	if err := p.PublishExerciseRequested(context.Background(), ExerciseRequestedEvent{ContractID: 1}); err != nil {
		t.Error(err)
	}
}

// ---- buildSagaHandler ------------------------------------------------------

func TestBuildSagaHandler_BadJSON(t *testing.T) {
	h := buildSagaHandler(nil, otcConsumerBindings[0], discard())
	if res := h(context.Background(), rabbitmq.Envelope{Body: []byte("{bad")}, amqp.Delivery{}); res != rabbitmq.Reject {
		t.Errorf("bad json should Reject, got %v", res)
	}
}

func TestBuildSagaHandler_MissingContractID(t *testing.T) {
	h := buildSagaHandler(nil, otcConsumerBindings[0], discard())
	if res := h(context.Background(), rabbitmq.Envelope{Body: nil}, amqp.Delivery{RoutingKey: "x"}); res != rabbitmq.Ack {
		t.Errorf("missing contractId should Ack, got %v", res)
	}
	if res := h(context.Background(), rabbitmq.Envelope{Body: []byte(`{"reason":"x"}`)}, amqp.Delivery{}); res != rabbitmq.Ack {
		t.Errorf("body without contractId should Ack, got %v", res)
	}
}

func TestBuildSagaHandler_Dispatch(t *testing.T) {
	cases := []struct {
		idx      int
		status   string // initial contract status that makes the transition fire
		expected string
	}{
		{0, ContractPendingPremium, ContractActive},   // premium completed
		{1, ContractPendingPremium, ContractCanceled}, // premium failed
		{2, ContractActive, ContractExercised},        // exercise completed
	}
	for _, c := range cases {
		repo := &stubRepo{contract: &OptionContract{ID: 5, Status: c.status, SellerID: 20, StockTicker: "AAPL", Amount: 1, PricePerStock: dec("0")}}
		pf := &stubPortfolio{}
		svc := newSvc(repo, pf, &stubMarket{}, nil, nil, nil, nil)
		h := buildSagaHandler(svc, otcConsumerBindings[c.idx], discard())
		body := []byte(`{"contractId":5,"reason":"r"}`)
		if res := h(context.Background(), rabbitmq.Envelope{Body: body}, amqp.Delivery{}); res != rabbitmq.Ack {
			t.Fatalf("binding %d should Ack, got %v", c.idx, res)
		}
		if len(repo.updatedStatuses) == 0 || repo.updatedStatuses[0] != c.expected {
			t.Errorf("binding %d: status = %v want %s", c.idx, repo.updatedStatuses, c.expected)
		}
	}
}

func TestBuildSagaHandler_ExerciseFailedRevert(t *testing.T) {
	// kindExerciseFailed -> RevertExercise; non-ACTIVE contract is a no-op (no error -> Ack)
	repo := &stubRepo{contract: &OptionContract{ID: 5, Status: ContractExercised, PricePerStock: dec("0")}}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	h := buildSagaHandler(svc, otcConsumerBindings[3], discard())
	if res := h(context.Background(), rabbitmq.Envelope{Body: []byte(`{"contractId":5}`)}, amqp.Delivery{}); res != rabbitmq.Ack {
		t.Errorf("exercise-failed revert should Ack, got %v", res)
	}
}

func TestBuildSagaHandler_ServiceError(t *testing.T) {
	repo := &stubRepo{contract: &OptionContract{ID: 5, Status: ContractPendingPremium, PricePerStock: dec("0")}, updateStatusErr: errors.New("db")}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	h := buildSagaHandler(svc, otcConsumerBindings[0], discard())
	if res := h(context.Background(), rabbitmq.Envelope{Body: []byte(`{"contractId":5}`)}, amqp.Delivery{}); res != rabbitmq.Reject {
		t.Errorf("service error should Reject, got %v", res)
	}
}

func TestOtcConsumerBindings_Distinct(t *testing.T) {
	if len(otcConsumerBindings) != 4 {
		t.Fatalf("expected 4 bindings, got %d", len(otcConsumerBindings))
	}
	seen := map[string]bool{}
	for _, b := range otcConsumerBindings {
		if seen[b.queue] {
			t.Errorf("dup queue %q", b.queue)
		}
		seen[b.queue] = true
	}
}

// ---- RabbitNotifier --------------------------------------------------------

func sampleOffer() *OtcOffer {
	return &OtcOffer{ID: 1, StockTicker: "AAPL", BuyerID: 10, SellerID: 20, Amount: 5,
		PricePerStock: dec("100"), Premium: dec("5"), Status: OfferAccepted}
}

func TestRabbitNotifier_CounterOffered(t *testing.T) {
	pub := &recordingPublisher{}
	email := "a@b.com"
	cust := &clients.Customer{ID: 20, FirstName: ptr("Jo"), LastName: ptr("Do"), Email: &email}
	n := NewRabbitNotifier(pub, customerClientReturning(cust, 200), discard())
	n.CounterOffered(context.Background(), sampleOffer(), 10)
	if len(pub.keys) != 1 || pub.keys[0] != routingOtcCountered {
		t.Errorf("keys = %v", pub.keys)
	}
}

func TestRabbitNotifier_Accepted(t *testing.T) {
	pub := &recordingPublisher{}
	email := "a@b.com"
	cust := &clients.Customer{ID: 20, Email: &email}
	n := NewRabbitNotifier(pub, customerClientReturning(cust, 200), discard())
	n.Accepted(context.Background(), sampleOffer(), 10)
	if len(pub.keys) != 1 || pub.keys[0] != routingOtcAccepted {
		t.Errorf("keys = %v", pub.keys)
	}
}

func TestRabbitNotifier_Canceled(t *testing.T) {
	pub := &recordingPublisher{}
	email := "a@b.com"
	cust := &clients.Customer{ID: 20, Email: &email}
	n := NewRabbitNotifier(pub, customerClientReturning(cust, 200), discard())
	n.Canceled(context.Background(), sampleOffer(), 10, "REJECTED")
	if len(pub.keys) != 1 || pub.keys[0] != routingOtcCanceled {
		t.Errorf("keys = %v", pub.keys)
	}
}

func TestRabbitNotifier_ExpiryReminder(t *testing.T) {
	pub := &recordingPublisher{}
	email := "a@b.com"
	cust := &clients.Customer{ID: 1, Email: &email}
	n := NewRabbitNotifier(pub, customerClientReturning(cust, 200), discard())
	c := &OptionContract{ID: 1, OfferID: 2, StockTicker: "AAPL", BuyerID: 10, SellerID: 20, Amount: 3, PricePerStock: dec("100")}
	n.ExpiryReminder(context.Background(), c, 3)
	// fires for both buyer and seller
	if len(pub.keys) != 2 {
		t.Errorf("expected 2 publishes (buyer+seller), got %d", len(pub.keys))
	}
}

func TestRabbitNotifier_SkipsWhenNoEmail(t *testing.T) {
	pub := &recordingPublisher{}
	cust := &clients.Customer{ID: 20} // no email
	n := NewRabbitNotifier(pub, customerClientReturning(cust, 200), discard())
	n.Accepted(context.Background(), sampleOffer(), 10)
	if len(pub.keys) != 0 {
		t.Errorf("should skip when email blank, got %v", pub.keys)
	}
}

func TestRabbitNotifier_SkipsWhenCustomerNotFound(t *testing.T) {
	pub := &recordingPublisher{}
	n := NewRabbitNotifier(pub, customerClientReturning(nil, 404), discard())
	n.Accepted(context.Background(), sampleOffer(), 10)
	if len(pub.keys) != 0 {
		t.Errorf("should skip when customer missing, got %v", pub.keys)
	}
}

func TestRabbitNotifier_PublishError(t *testing.T) {
	pub := &recordingPublisher{err: errors.New("broker down")}
	email := "a@b.com"
	cust := &clients.Customer{ID: 20, Email: &email}
	n := NewRabbitNotifier(pub, customerClientReturning(cust, 200), discard())
	// should not panic; error is logged best-effort
	n.Accepted(context.Background(), sampleOffer(), 10)
}

func TestCounterparty(t *testing.T) {
	o := sampleOffer()
	if counterparty(o, o.BuyerID) != o.SellerID {
		t.Error("buyer actor -> seller counterparty")
	}
	if counterparty(o, o.SellerID) != o.BuyerID {
		t.Error("seller actor -> buyer counterparty")
	}
}

func TestDisplayName(t *testing.T) {
	if got := displayName(&clients.Customer{FirstName: ptr("Jo"), LastName: ptr("Do")}); got != "Jo Do" {
		t.Errorf("got %q", got)
	}
	if got := displayName(&clients.Customer{}); got != "" {
		t.Errorf("empty name expected, got %q", got)
	}
}

func TestNoopNotifier(t *testing.T) {
	var n OtcNotifier = NoopNotifier{}
	ctx := context.Background()
	n.CounterOffered(ctx, sampleOffer(), 10)
	n.Accepted(ctx, sampleOffer(), 10)
	n.Canceled(ctx, sampleOffer(), 10, "X")
	n.ExpiryReminder(ctx, &OptionContract{PricePerStock: decimal.Zero}, 3)
}
