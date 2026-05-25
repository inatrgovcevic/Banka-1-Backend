package service_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/service"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/store"
	"github.com/raf-si-2025/banka-1-go/shared/auth"
)

// ---------------------------------------------------------------------------
// Fake MessageStoreSeam (no Postgres required)
// ---------------------------------------------------------------------------

type fakeMessageStore struct {
	messages []*store.Message
	nextID   int64
}

func (s *fakeMessageStore) Insert(_ context.Context, m *store.Message) error {
	s.nextID++
	m.ID = s.nextID
	m.Version = 0
	m.CreatedAt = time.Now()
	cp := *m
	s.messages = append(s.messages, &cp)
	return nil
}

func (s *fakeMessageStore) UpdateOptimistic(_ context.Context, m *store.Message) error {
	for _, stored := range s.messages {
		if stored.ID == m.ID && stored.Version == m.Version {
			*stored = *m
			stored.Version++
			m.Version++
			return nil
		}
	}
	return store.ErrOptimisticLockConflict
}

func (s *fakeMessageStore) FindPending(_ context.Context, maxRetries int, cutoff time.Time, limit int) ([]store.Message, error) {
	var out []store.Message
	for _, m := range s.messages {
		if m.Status == store.MessageStatusPendingSend &&
			m.RetryCount < maxRetries &&
			(m.LastAttemptAt == nil || m.LastAttemptAt.Before(cutoff)) {
			out = append(out, *m)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Fake PartnerLookup
// ---------------------------------------------------------------------------

type fakePartnerLookup struct {
	partner auth.Partner
}

func (f *fakePartnerLookup) FindByRouting(routing int) (*auth.Partner, error) {
	if f.partner.Routing == routing {
		p := f.partner
		return &p, nil
	}
	return nil, service.ErrPartnerNotFound
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestPartner(baseURL string) *fakePartnerLookup {
	return &fakePartnerLookup{partner: auth.Partner{
		Routing:       222,
		DisplayName:   "Banka 2",
		BaseURL:       baseURL + "/",
		InboundToken:  "inbound-tok",
		OutboundToken: "outbound-tok",
	}}
}

func sampleTx() protocol.InterbankTransactionPayload {
	return protocol.InterbankTransactionPayload{
		TransactionId: protocol.ForeignBankId{RoutingNumber: 111, Id: "tx-test-01"},
		Postings: []protocol.Posting{
			{
				Account: &protocol.RealAccount{Num: "111000000000000011"},
				Amount:  decimal.NewFromFloat(-100),
				Asset:   &protocol.MonasAsset{Currency: "USD"},
			},
			{
				Account: &protocol.RealAccount{Num: "222000000000000022"},
				Amount:  decimal.NewFromFloat(100),
				Asset:   &protocol.MonasAsset{Currency: "USD"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Tests: SendNewTx
// ---------------------------------------------------------------------------

func TestSendNewTx_XApiKeyAndBodyAndVote(t *testing.T) {
	var receivedAPIKey string
	var receivedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAPIKey = r.Header.Get("X-Api-Key")
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"vote":"YES"}`))
	}))
	defer srv.Close()

	fms := &fakeMessageStore{}
	client := service.NewInterbankClientWithStore(111, newTestPartner(srv.URL), fms, nil, nil)

	vote, err := client.SendNewTx(context.Background(), 222, sampleTx())
	if err != nil {
		t.Fatalf("SendNewTx error: %v", err)
	}

	// Verify vote.
	if vote.Vote != protocol.VoteYes {
		t.Errorf("expected YES vote, got %q", vote.Vote)
	}

	// Verify X-Api-Key header.
	if receivedAPIKey != "outbound-tok" {
		t.Errorf("expected X-Api-Key=outbound-tok, got %q", receivedAPIKey)
	}

	// Verify envelope structure.
	var env map[string]interface{}
	if err := json.Unmarshal(receivedBody, &env); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if env["messageType"] != "NEW_TX" {
		t.Errorf("expected messageType=NEW_TX, got %v", env["messageType"])
	}

	// Verify idempotency key shape: 32 lowercase hex chars.
	ikey, ok := env["idempotenceKey"].(map[string]interface{})
	if !ok {
		t.Fatal("missing idempotenceKey in body")
	}
	localKey, _ := ikey["locallyGeneratedKey"].(string)
	if len(localKey) != 32 {
		t.Errorf("idempotency key should be 32 hex chars, got len=%d val=%q", len(localKey), localKey)
	}
	for _, ch := range localKey {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Errorf("idempotency key contains non-hex char in %q", localKey)
			break
		}
	}
}

func TestSendNewTx_SuccessTransitionsToSent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"vote":"YES"}`))
	}))
	defer srv.Close()

	fms := &fakeMessageStore{}
	client := service.NewInterbankClientWithStore(111, newTestPartner(srv.URL), fms, nil, nil)

	_, err := client.SendNewTx(context.Background(), 222, sampleTx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fms.messages) == 0 {
		t.Fatal("expected a persisted message row")
	}
	msg := fms.messages[0]
	if msg.Status != store.MessageStatusSent {
		t.Errorf("expected SENT after success, got %q", msg.Status)
	}
	if msg.HttpStatus == nil || *msg.HttpStatus != 200 {
		t.Errorf("expected httpStatus=200, got %v", msg.HttpStatus)
	}
}

func TestSendNewTx_5xxLeavesMessagePendingSend(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	fms := &fakeMessageStore{}
	client := service.NewInterbankClientWithStore(111, newTestPartner(srv.URL), fms, nil, nil)

	_, err := client.SendNewTx(context.Background(), 222, sampleTx())
	if err == nil {
		t.Fatal("expected error on 500 response")
	}

	if len(fms.messages) == 0 {
		t.Fatal("expected a persisted message row even on failure")
	}
	// Message should stay PENDING_SEND — retry scheduler will pick it up.
	if fms.messages[0].Status != store.MessageStatusPendingSend {
		t.Errorf("expected PENDING_SEND after failure, got %q", fms.messages[0].Status)
	}
}

// ---------------------------------------------------------------------------
// Tests: SendCommitTx
// ---------------------------------------------------------------------------

func TestSendCommitTx_SuccessTransitionsToSent(t *testing.T) {
	var receivedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	fms := &fakeMessageStore{}
	client := service.NewInterbankClientWithStore(111, newTestPartner(srv.URL), fms, nil, nil)

	txID := protocol.ForeignBankId{RoutingNumber: 111, Id: "tx-commit-01"}
	if err := client.SendCommitTx(context.Background(), 222, txID); err != nil {
		t.Fatalf("SendCommitTx error: %v", err)
	}

	// Verify body messageType.
	var env map[string]interface{}
	if err := json.Unmarshal(receivedBody, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env["messageType"] != "COMMIT_TX" {
		t.Errorf("expected COMMIT_TX, got %v", env["messageType"])
	}

	// Message transitions to SENT.
	if len(fms.messages) == 0 {
		t.Fatal("expected persisted row")
	}
	if fms.messages[0].Status != store.MessageStatusSent {
		t.Errorf("expected SENT, got %q", fms.messages[0].Status)
	}
}

// ---------------------------------------------------------------------------
// Tests: SendRollbackTx
// ---------------------------------------------------------------------------

func TestSendRollbackTx_SuccessTransitionsToSent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	fms := &fakeMessageStore{}
	client := service.NewInterbankClientWithStore(111, newTestPartner(srv.URL), fms, nil, nil)

	txID := protocol.ForeignBankId{RoutingNumber: 111, Id: "tx-rb-01"}
	if err := client.SendRollbackTx(context.Background(), 222, txID); err != nil {
		t.Fatalf("SendRollbackTx error: %v", err)
	}

	if len(fms.messages) == 0 {
		t.Fatal("expected persisted row")
	}
	if fms.messages[0].Status != store.MessageStatusSent {
		t.Errorf("expected SENT, got %q", fms.messages[0].Status)
	}
}

// ---------------------------------------------------------------------------
// Tests: OTC §3 methods
// ---------------------------------------------------------------------------

func TestOutboundDelete_404IsIdempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	fms := &fakeMessageStore{}
	client := service.NewInterbankClientWithStore(111, newTestPartner(srv.URL), fms, nil, nil)

	negID := protocol.ForeignBankId{RoutingNumber: 222, Id: "neg-gone-01"}
	if err := client.OutboundDelete(context.Background(), 222, negID); err != nil {
		t.Errorf("expected nil (idempotent 404), got %v", err)
	}
}

func TestOutboundFetchPublicStock_Graceful5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	fms := &fakeMessageStore{}
	client := service.NewInterbankClientWithStore(111, newTestPartner(srv.URL), fms, nil, nil)

	entries, err := client.OutboundFetchPublicStock(context.Background(), 222)
	if err != nil {
		t.Errorf("expected nil error (graceful degradation on 5xx), got %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(entries))
	}
}

func TestOutboundFetchPublicStock_401IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	fms := &fakeMessageStore{}
	client := service.NewInterbankClientWithStore(111, newTestPartner(srv.URL), fms, nil, nil)

	_, err := client.OutboundFetchPublicStock(context.Background(), 222)
	if err == nil {
		t.Fatal("expected error for 401 (token misconfigured)")
	}
}

func TestOutboundFetchPublicStock_DecodesEntries(t *testing.T) {
	cannedResp := `[{"stock":{"ticker":"TSLA"},"sellers":[{"sellerId":{"routingNumber":222,"id":"C-99"},"quantity":25}]}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "outbound-tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(cannedResp))
	}))
	defer srv.Close()

	fms := &fakeMessageStore{}
	client := service.NewInterbankClientWithStore(111, newTestPartner(srv.URL), fms, nil, nil)

	entries, err := client.OutboundFetchPublicStock(context.Background(), 222)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Stock.Ticker != "TSLA" {
		t.Errorf("expected TSLA, got %q", entries[0].Stock.Ticker)
	}
}

func TestOutboundCreateNegotiation_ReturnsId(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/negotiations" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"routingNumber":222,"id":"mock-neg-abc12345"}`))
	}))
	defer srv.Close()

	fms := &fakeMessageStore{}
	client := service.NewInterbankClientWithStore(111, newTestPartner(srv.URL), fms, nil, nil)

	offer := service.OtcOfferDto{
		Stock:  protocol.StockDescription{Ticker: "AAPL"},
		Amount: 10,
		BuyerID: protocol.ForeignBankId{RoutingNumber: 222, Id: "C-2"},
		SellerID: protocol.ForeignBankId{RoutingNumber: 111, Id: "C-15"},
		LastModifiedBy: protocol.ForeignBankId{RoutingNumber: 111, Id: "C-15"},
	}

	id, err := client.OutboundCreateNegotiation(context.Background(), 222, offer)
	if err != nil {
		t.Fatalf("OutboundCreateNegotiation error: %v", err)
	}
	if id.RoutingNumber != 222 {
		t.Errorf("expected routingNumber=222, got %d", id.RoutingNumber)
	}
	if id.Id != "mock-neg-abc12345" {
		t.Errorf("expected id=mock-neg-abc12345, got %q", id.Id)
	}
}

// ---------------------------------------------------------------------------
// Test: Resend (used by retry scheduler)
// ---------------------------------------------------------------------------

func TestResend_ReusesExistingIdempotencyKey(t *testing.T) {
	var receivedPayloads []map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var env map[string]interface{}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &env)
		receivedPayloads = append(receivedPayloads, env)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"vote":"YES"}`))
	}))
	defer srv.Close()

	fms := &fakeMessageStore{}
	client := service.NewInterbankClientWithStore(111, newTestPartner(srv.URL), fms, nil, nil)

	// First: SendNewTx fails (we'll manually create a PENDING_SEND row).
	// Instead of failing the HTTP, we build the row directly to simulate what
	// the client would have persisted on a previous failed attempt.
	tx := sampleTx()
	payload := protocol.InterbankMessagePayload{
		IdempotenceKey: protocol.IdempotenceKey{RoutingNumber: 111, LocallyGeneratedKey: "original-key-1234567890ab"},
		MessageType:    protocol.MessageTypeNewTx,
		Message:        &tx,
	}
	rawBody, _ := json.Marshal(payload)

	now := time.Now().Add(-5 * time.Minute)
	msg := &store.Message{
		Direction:           store.DirectionOutbound,
		SenderRoutingNumber: 222,
		LocallyGeneratedKey: "original-key-1234567890ab",
		MessageType:         string(protocol.MessageTypeNewTx),
		Status:              store.MessageStatusPendingSend,
		RequestBody:         string(rawBody),
		RetryCount:          0,
		LastAttemptAt:       &now,
	}
	_ = fms.Insert(context.Background(), msg)

	// Now resend.
	if err := client.Resend(context.Background(), msg); err != nil {
		t.Fatalf("Resend error: %v", err)
	}

	if len(receivedPayloads) != 1 {
		t.Fatalf("expected 1 HTTP call, got %d", len(receivedPayloads))
	}

	// The re-sent payload must have the SAME idempotency key.
	sentPayload := receivedPayloads[0]
	sentKey := sentPayload["idempotenceKey"].(map[string]interface{})["locallyGeneratedKey"].(string)
	if sentKey != "original-key-1234567890ab" {
		t.Errorf("Resend must reuse original key; got %q", sentKey)
	}

	// Resend mutates the msg in place (scheduler owns the UpdateOptimistic call).
	// Verify the in-memory status is SENT so the scheduler knows to persist it.
	if msg.Status != store.MessageStatusSent {
		t.Errorf("expected msg.Status=SENT after Resend, got %q", msg.Status)
	}
}
