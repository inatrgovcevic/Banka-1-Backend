package mock_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/mock"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
)

// ---------------------------------------------------------------------------
// Test setup
// ---------------------------------------------------------------------------

func newMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	r := chi.NewRouter()
	mock.RegisterMockRoutes(r)
	return httptest.NewServer(r)
}

func do(t *testing.T, srv *httptest.Server, method, path, apiKey, body string) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, srv.URL+path, bodyReader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if apiKey != "" {
		req.Header.Set("X-Api-Key", apiKey)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return b
}

// ---------------------------------------------------------------------------
// Auth middleware tests
// ---------------------------------------------------------------------------

func TestMockBanka2_NoAPIKey_Returns401(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	resp := do(t, srv, http.MethodPost, "/_mock/banka2/negotiations", "", `{}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestMockBanka2_BlankAPIKey_Returns401(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	resp := do(t, srv, http.MethodPost, "/_mock/banka2/negotiations", "   ", `{}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for blank key, got %d", resp.StatusCode)
	}
}

func TestMockBanka2_AnyNonBlankAPIKey_Passes(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/_mock/banka2/public-stock", "whatever-token", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// POST /interbank — NEW_TX
// ---------------------------------------------------------------------------

func TestMockInterbank_NewTx_DefaultYesVote(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	payload := buildNewTxPayload(t)
	resp := do(t, srv, http.MethodPost, "/_mock/banka2/interbank", "tok", payload)
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d, body=%s", resp.StatusCode, body)
	}

	var vote protocol.TransactionVote
	if err := json.Unmarshal(body, &vote); err != nil {
		t.Fatalf("unmarshal vote: %v", err)
	}
	if vote.Vote != protocol.VoteYes {
		t.Errorf("expected YES vote, got %q", vote.Vote)
	}
}

func TestMockInterbank_NewTx_NoVoteWithReason(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	payload := buildNewTxPayload(t)
	resp := do(t, srv, http.MethodPost,
		"/_mock/banka2/interbank?vote=NO&reason=INSUFFICIENT_ASSET",
		"tok", payload)
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with NO body, got %d, body=%s", resp.StatusCode, body)
	}

	var vote protocol.TransactionVote
	if err := json.Unmarshal(body, &vote); err != nil {
		t.Fatalf("unmarshal vote: %v", err)
	}
	if vote.Vote != protocol.VoteNo {
		t.Errorf("expected NO vote, got %q", vote.Vote)
	}
	if len(vote.Reasons) == 0 {
		t.Fatal("expected at least one reason")
	}
	if vote.Reasons[0].Reason != "INSUFFICIENT_ASSET" {
		t.Errorf("expected INSUFFICIENT_ASSET reason, got %q", vote.Reasons[0].Reason)
	}
}

func TestMockInterbank_NewTx_NoVoteDefaultReason(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	payload := buildNewTxPayload(t)
	resp := do(t, srv, http.MethodPost,
		"/_mock/banka2/interbank?vote=NO",
		"tok", payload)
	body := readBody(t, resp)

	var vote protocol.TransactionVote
	if err := json.Unmarshal(body, &vote); err != nil {
		t.Fatalf("unmarshal vote: %v", err)
	}
	if vote.Vote != protocol.VoteNo {
		t.Errorf("expected NO vote, got %q", vote.Vote)
	}
	// Default reason should be INSUFFICIENT_ASSET.
	if len(vote.Reasons) == 0 || vote.Reasons[0].Reason != "INSUFFICIENT_ASSET" {
		t.Errorf("expected INSUFFICIENT_ASSET default reason, got %v", vote.Reasons)
	}
}

// ---------------------------------------------------------------------------
// POST /interbank — COMMIT_TX / ROLLBACK_TX
// ---------------------------------------------------------------------------

func TestMockInterbank_CommitTx_Returns204(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	payload := buildEnvelopePayload(t, "COMMIT_TX",
		`{"transactionId":{"routingNumber":111,"id":"tx-01"}}`)
	resp := do(t, srv, http.MethodPost, "/_mock/banka2/interbank", "tok", payload)
	readBody(t, resp)

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
}

func TestMockInterbank_RollbackTx_Returns204(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	payload := buildEnvelopePayload(t, "ROLLBACK_TX",
		`{"transactionId":{"routingNumber":111,"id":"tx-01"}}`)
	resp := do(t, srv, http.MethodPost, "/_mock/banka2/interbank", "tok", payload)
	readBody(t, resp)

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// GET /public-stock
// ---------------------------------------------------------------------------

func TestMockPublicStock_ReturnsTwoEntries(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/_mock/banka2/public-stock", "tok", "")
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var entries []protocol.PublicStockEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		t.Fatalf("unmarshal entries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	tickers := map[string]bool{}
	for _, e := range entries {
		tickers[e.Stock.Ticker] = true
	}
	if !tickers["TSLA"] || !tickers["MSFT"] {
		t.Errorf("expected TSLA and MSFT, got %v", tickers)
	}
	// Sellers should have routing 222.
	for _, e := range entries {
		for _, s := range e.Sellers {
			if s.SellerID.RoutingNumber != 222 {
				t.Errorf("expected seller routing 222, got %d", s.SellerID.RoutingNumber)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// POST /negotiations — create
// ---------------------------------------------------------------------------

func TestMockCreateNegotiation_ReturnsForeignBankId(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	resp := do(t, srv, http.MethodPost, "/_mock/banka2/negotiations", "tok", `{"amount":10}`)
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d, body=%s", resp.StatusCode, body)
	}

	var id protocol.ForeignBankId
	if err := json.Unmarshal(body, &id); err != nil {
		t.Fatalf("unmarshal id: %v", err)
	}
	if id.RoutingNumber != 222 {
		t.Errorf("expected routingNumber=222, got %d", id.RoutingNumber)
	}
	if !strings.HasPrefix(id.Id, "mock-neg-") {
		t.Errorf("expected id to start with 'mock-neg-', got %q", id.Id)
	}
}

// ---------------------------------------------------------------------------
// PUT /negotiations/{rn}/{id} — counter-offer
// ---------------------------------------------------------------------------

func TestMockCounterOffer_Returns204(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	resp := do(t, srv, http.MethodPut, "/_mock/banka2/negotiations/111/neg-01", "tok", `{"amount":5}`)
	readBody(t, resp)

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// GET /negotiations/{rn}/{id} — get state
// ---------------------------------------------------------------------------

func TestMockGetNegotiation_ReturnsAAPL(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/_mock/banka2/negotiations/111/neg-01", "tok", "")
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var neg map[string]interface{}
	if err := json.Unmarshal(body, &neg); err != nil {
		t.Fatalf("unmarshal neg: %v", err)
	}
	stock, _ := neg["stock"].(map[string]interface{})
	if stock == nil || stock["ticker"] != "AAPL" {
		t.Errorf("expected AAPL stock, got %v", neg["stock"])
	}
}

// ---------------------------------------------------------------------------
// DELETE /negotiations/{rn}/{id}
// ---------------------------------------------------------------------------

func TestMockDeleteNegotiation_Returns204(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	resp := do(t, srv, http.MethodDelete, "/_mock/banka2/negotiations/111/neg-01", "tok", "")
	readBody(t, resp)

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// GET /negotiations/{rn}/{id}/accept
// ---------------------------------------------------------------------------

func TestMockAcceptNegotiation_Returns204(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/_mock/banka2/negotiations/111/neg-01/accept", "tok", "")
	readBody(t, resp)

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// GET /user/{rn}/{id} and /interbank/user/{rn}/{id}
// ---------------------------------------------------------------------------

func TestMockUser_ReturnsDisplayName(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/_mock/banka2/user/111/C-5", "tok", "")
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var u map[string]string
	if err := json.Unmarshal(body, &u); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if u["bankDisplayName"] != "Banka 2" {
		t.Errorf("expected bankDisplayName='Banka 2', got %q", u["bankDisplayName"])
	}
	if u["displayName"] != "Mock C-5" {
		t.Errorf("expected displayName='Mock C-5', got %q", u["displayName"])
	}
}

func TestMockUser_InterbankPathAlias(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/_mock/banka2/interbank/user/111/C-15", "tok", "")
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from /interbank/user alias, got %d", resp.StatusCode)
	}
	var u map[string]string
	if err := json.Unmarshal(body, &u); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if u["displayName"] != "Mock C-15" {
		t.Errorf("expected 'Mock C-15', got %q", u["displayName"])
	}
}

// ---------------------------------------------------------------------------
// Helper builders
// ---------------------------------------------------------------------------

func buildNewTxPayload(t *testing.T) string {
	t.Helper()
	// Minimal valid NEW_TX envelope.
	body := map[string]interface{}{
		"idempotenceKey": map[string]interface{}{
			"routingNumber":       111,
			"locallyGeneratedKey": "test-key-12345678901234567890abcd",
		},
		"messageType": "NEW_TX",
		"message": map[string]interface{}{
			"transactionId": map[string]interface{}{
				"routingNumber": 111,
				"id":            "tx-snap-01",
			},
			"postings": []interface{}{
				map[string]interface{}{
					"account": map[string]interface{}{"type": "ACCOUNT", "num": "111000000000000011"},
					"amount":  "-100.00",
					"asset":   map[string]interface{}{"type": "MONAS", "asset": map[string]interface{}{"currency": "USD"}},
				},
				map[string]interface{}{
					"account": map[string]interface{}{"type": "ACCOUNT", "num": "222000000000000022"},
					"amount":  "100.00",
					"asset":   map[string]interface{}{"type": "MONAS", "asset": map[string]interface{}{"currency": "USD"}},
				},
			},
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return string(b)
}

func buildEnvelopePayload(t *testing.T, msgType, msgBody string) string {
	t.Helper()
	return `{"idempotenceKey":{"routingNumber":111,"locallyGeneratedKey":"test-key-commit"},"messageType":"` +
		msgType + `","message":` + msgBody + `}`
}
