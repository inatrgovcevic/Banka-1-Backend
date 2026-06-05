// Package mock provides a mock Banka 2 HTTP controller for local development
// and end-to-end testing without a real partner bank instance.
//
// Active only when cfg.MockPartner.Enabled == true (env: INTERBANK_MOCK_PARTNER_ENABLED=true).
// In dev mode, BANKA2_BASE_URL defaults to http://localhost:8091/_mock/banka2/ so outbound
// calls "to Banka 2" loop back into our own process through this controller.
//
// Auth: non-blank X-Api-Key is accepted (any value). The real inter-bank auth
// filter (RequireXApiKey) handles production auth; this mock is intentionally
// permissive for local dev loops.
package mock

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
)

const (
	banka2Routing = 222
	banka1Routing = 111
)

// RegisterMockRoutes mounts all /_mock/banka2/* routes on r.
// Call this only when cfg.MockPartner.Enabled is true.
func RegisterMockRoutes(r chi.Router) {
	r.Route("/_mock/banka2", func(r chi.Router) {
		// All mock routes require a non-blank X-Api-Key header.
		r.Use(requireNonBlankAPIKey)

		// POST /interbank — simulate NEW_TX / COMMIT_TX / ROLLBACK_TX.
		r.Post("/interbank", mockInterbank)

		// GET /public-stock — canned 2 tickers.
		r.Get("/public-stock", mockPublicStock)

		// POST /negotiations — return new neg id.
		r.Post("/negotiations", mockCreateNegotiation)

		// PUT /negotiations/{rn}/{id} — counter-offer, always 204.
		r.Put("/negotiations/{rn}/{id}", mockCounterOffer)

		// GET /negotiations/{rn}/{id} — canned AAPL negotiation.
		r.Get("/negotiations/{rn}/{id}", mockGetNegotiation)

		// DELETE /negotiations/{rn}/{id} — idempotent 204.
		r.Delete("/negotiations/{rn}/{id}", mockDeleteNegotiation)

		// GET /negotiations/{rn}/{id}/accept — 204.
		r.Get("/negotiations/{rn}/{id}/accept", mockAcceptNegotiation)

		// GET /interbank/user/{rn}/{id} — canned display name (authoritative path).
		r.Get("/interbank/user/{rn}/{id}", mockUser)

		// GET /user/{rn}/{id} — Tim 2 MINOR-1 alias.
		r.Get("/user/{rn}/{id}", mockUser)
	})
}

// ---------------------------------------------------------------------------
// Middleware
// ---------------------------------------------------------------------------

// requireNonBlankAPIKey returns 401 if X-Api-Key is absent or blank.
// Production auth uses RequireXApiKey with constant-time compare; this mock is
// deliberately lenient (any non-blank value passes) to ease local testing.
func requireNonBlankAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(r.Header.Get("X-Api-Key")) == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// mockInterbank handles POST /_mock/banka2/interbank.
//
// NEW_TX: responds 200 + {"vote":"YES"} by default.
//
//	Pass ?vote=NO&reason=REASON to simulate a NO vote.
//
// COMMIT_TX / ROLLBACK_TX: responds 204 No Content.
func mockInterbank(w http.ResponseWriter, r *http.Request) {
	var msg protocol.InterbankMessagePayload
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	switch msg.MessageType {
	case protocol.MessageTypeNewTx:
		vote := r.URL.Query().Get("vote")
		reason := r.URL.Query().Get("reason")

		var resp protocol.TransactionVote
		if strings.EqualFold(vote, "NO") {
			reasonStr := reason
			if reasonStr == "" {
				reasonStr = protocol.ReasonInsufficientAsset
			}
			resp = protocol.TransactionVote{
				Vote: protocol.VoteNo,
				Reasons: []protocol.NoVoteReason{
					{Reason: reasonStr},
				},
			}
		} else {
			resp = protocol.TransactionVote{Vote: protocol.VoteYes}
		}
		writeJSON(w, http.StatusOK, resp)

	case protocol.MessageTypeCommitTx, protocol.MessageTypeRollbackTx:
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "unknown messageType", http.StatusBadRequest)
	}
}

// mockPublicStock handles GET /_mock/banka2/public-stock.
// Returns 2 canned entries: TSLA (seller C-99, qty 25) and MSFT (seller C-100, qty 50).
func mockPublicStock(w http.ResponseWriter, r *http.Request) {
	entries := []protocol.PublicStockEntry{
		{
			Stock: protocol.StockDescription{Ticker: "TSLA"},
			Sellers: []protocol.PublicStockSellerRef{
				{
					SellerID: protocol.ForeignBankId{RoutingNumber: banka2Routing, Id: "C-99"},
					Quantity: decimal.NewFromInt(25),
				},
			},
		},
		{
			Stock: protocol.StockDescription{Ticker: "MSFT"},
			Sellers: []protocol.PublicStockSellerRef{
				{
					SellerID: protocol.ForeignBankId{RoutingNumber: banka2Routing, Id: "C-100"},
					Quantity: decimal.NewFromInt(50),
				},
			},
		},
	}
	writeJSON(w, http.StatusOK, entries)
}

// mockCreateNegotiation handles POST /_mock/banka2/negotiations.
// Returns a new ForeignBankId with routing 222 and a random 8-char UUID suffix.
func mockCreateNegotiation(w http.ResponseWriter, r *http.Request) {
	id := protocol.ForeignBankId{
		RoutingNumber: banka2Routing,
		Id:            "mock-neg-" + randHex8(),
	}
	writeJSON(w, http.StatusOK, id)
}

// mockCounterOffer handles PUT /_mock/banka2/negotiations/{rn}/{id}.
// Always returns 204 No Content.
func mockCounterOffer(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// mockGetNegotiation handles GET /_mock/banka2/negotiations/{rn}/{id}.
// Returns a canned AAPL negotiation.
func mockGetNegotiation(w http.ResponseWriter, r *http.Request) {
	rn := chi.URLParam(r, "rn")
	_ = rn
	negotiation := map[string]interface{}{
		"stock":          map[string]string{"ticker": "AAPL"},
		"settlementDate": "2026-06-15T00:00:00+02:00",
		"pricePerUnit":   map[string]interface{}{"currency": "USD", "amount": "195.00"},
		"premium":        map[string]interface{}{"currency": "USD", "amount": "500.00"},
		"buyerId":        map[string]interface{}{"routingNumber": banka2Routing, "id": "C-99"},
		"sellerId":       map[string]interface{}{"routingNumber": banka1Routing, "id": "C-5"},
		"amount":         10,
		"lastModifiedBy": map[string]interface{}{"routingNumber": banka1Routing, "id": "C-5"},
		"isOngoing":      true,
	}
	writeJSON(w, http.StatusOK, negotiation)
}

// mockDeleteNegotiation handles DELETE /_mock/banka2/negotiations/{rn}/{id}.
// Always returns 204 No Content.
func mockDeleteNegotiation(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// mockAcceptNegotiation handles GET /_mock/banka2/negotiations/{rn}/{id}/accept.
// Always returns 204 No Content (simulates successful 2PC from partner side).
func mockAcceptNegotiation(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// mockUser handles GET /_mock/banka2/user/{rn}/{id} and /interbank/user/{rn}/{id}.
// Returns a canned display name for Banka 2.
func mockUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	resp := map[string]string{
		"bankDisplayName": "Banka 2",
		"displayName":     fmt.Sprintf("Mock %s", id),
	}
	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// randHex8 returns 8 random lowercase hex characters.
func randHex8() string {
	return fmt.Sprintf("%08x", rand.Uint32()) //nolint:gosec // mock only, no crypto needed
}
