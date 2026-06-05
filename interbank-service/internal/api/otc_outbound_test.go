package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/service"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/store"
	"github.com/raf-si-2025/banka-1-go/shared/auth"
)

// ---------------------------------------------------------------------------
// testJWTSecret and token helpers
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// fake store + client for API-layer tests
// ---------------------------------------------------------------------------

type fakeOutboundStore struct {
	mu   sync.Mutex
	rows map[string]*store.Negotiation
}

func newFakeOutboundStoreAPI() *fakeOutboundStore {
	return &fakeOutboundStore{rows: make(map[string]*store.Negotiation)}
}

func (f *fakeOutboundStore) Insert(_ context.Context, n *store.Negotiation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *n
	cp.CreatedAt = time.Now()
	cp.LastModifiedAt = time.Now()
	f.rows[n.ID] = &cp
	return nil
}

func (f *fakeOutboundStore) FindByID(_ context.Context, id string) (*store.Negotiation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if n, ok := f.rows[id]; ok {
		cp := *n
		return &cp, nil
	}
	return nil, nil
}

func (f *fakeOutboundStore) FindByAuthoritativeRef(_ context.Context, _ int, id string) (*store.Negotiation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if n, ok := f.rows[id]; ok {
		cp := *n
		return &cp, nil
	}
	for _, n := range f.rows {
		if n.RemoteNegotiationID != nil && *n.RemoteNegotiationID == id {
			cp := *n
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeOutboundStore) UpdateCounter(_ context.Context, n *store.Negotiation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.rows[n.ID]; !ok {
		return store.ErrNotFound
	}
	cp := *n
	f.rows[n.ID] = &cp
	return nil
}

func (f *fakeOutboundStore) MarkClosed(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if n, ok := f.rows[id]; ok {
		n.IsOngoing = false
	}
	return nil
}

func (f *fakeOutboundStore) ListForUser(_ context.Context, userForeignID string, includeAll bool) ([]*store.Negotiation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*store.Negotiation
	for _, n := range f.rows {
		if includeAll || n.BuyerID == userForeignID || n.SellerID == userForeignID {
			cp := *n
			out = append(out, &cp)
		}
	}
	return out, nil
}

type fakeOutboundClientAPI struct {
	createID  *protocol.ForeignBankId
	createErr error
}

func (f *fakeOutboundClientAPI) OutboundCreateNegotiation(_ context.Context, _ int, _ service.OtcOfferDto) (*protocol.ForeignBankId, error) {
	return f.createID, f.createErr
}
func (f *fakeOutboundClientAPI) OutboundPutCounter(_ context.Context, _ int, _ protocol.ForeignBankId, _ service.OtcOfferDto) (int, error) {
	return 204, nil
}
func (f *fakeOutboundClientAPI) OutboundAccept(_ context.Context, _ int, _ protocol.ForeignBankId) (int, error) {
	return 204, nil
}
func (f *fakeOutboundClientAPI) OutboundDelete(_ context.Context, _ int, _ protocol.ForeignBankId) error {
	return nil
}
func (f *fakeOutboundClientAPI) OutboundFetchPublicStock(_ context.Context, _ int) ([]protocol.PublicStockEntry, error) {
	return []protocol.PublicStockEntry{
		{Stock: protocol.StockDescription{Ticker: "AAPL"}, Sellers: []protocol.PublicStockSellerRef{{SellerID: protocol.ForeignBankId{RoutingNumber: 222, Id: "C-2"}, Quantity: decimal.NewFromInt(50)}}},
	}, nil
}

type staticPartnerNamesAPI struct{}

func (s *staticPartnerNamesAPI) DisplayName(routing int) string {
	if routing == 222 {
		return "Banka 2"
	}
	return ""
}

// ---------------------------------------------------------------------------
// Router builder — bypasses RequireJWT by injecting claims directly.
// ---------------------------------------------------------------------------

// buildOutboundRouter creates a chi router with the outbound handler
// and a claims-injector middleware for tests.
func buildOutboundRouter(
	claims *auth.Claims, // nil → no claims (simulates missing auth)
	negStore service.NegotiationStoreForOutbound,
	client service.OtcOutboundClient,
) http.Handler {
	svc := service.NewOtcOutboundService(
		111,
		negStore,
		client,
		nil, // no intra-bank otcSvc in API-layer tests
		&staticPartnerNamesAPI{},
		nil,
	)
	h := NewOtcOutboundHandler(svc, nil)

	r := chi.NewRouter()
	// Inject claims or simulate no-auth.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if claims != nil {
				r = r.WithContext(auth.PutClaims(r.Context(), claims))
			}
			next.ServeHTTP(w, r)
		})
	})
	// Mount wrapped in a permission check.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission("OTC_TRADE", "ROLE_ADMIN", "ROLE_SUPERVISOR"))
		r.Post("/api/interbank/otc/negotiations", h.Create)
		r.Get("/api/interbank/otc/negotiations", h.List)
		r.Get("/api/interbank/otc/negotiations/{id}", h.Get)
		r.Put("/api/interbank/otc/negotiations/{id}/counter", h.Counter)
		r.Post("/api/interbank/otc/negotiations/{id}/accept", h.Accept)
		r.Delete("/api/interbank/otc/negotiations/{id}", h.Delete)
		r.Get("/api/interbank/otc/public-stock", h.PartnerPublicStock)
	})
	return r
}

func claimsOtcTrade(userID int64) *auth.Claims {
	return &auth.Claims{
		ID:          userID,
		Permissions: []string{"OTC_TRADE"},
		Roles:       []string{},
	}
}

func claimsAdmin(userID int64) *auth.Claims {
	return &auth.Claims{
		ID:          userID,
		Permissions: []string{},
		Roles:       []string{"ADMIN"},
	}
}

func claimsNoPerm(userID int64) *auth.Claims {
	return &auth.Claims{
		ID:          userID,
		Permissions: []string{"BANKING_BASIC"},
		Roles:       []string{"CLIENT"},
	}
}

// ---------------------------------------------------------------------------
// Helper: create request body
// ---------------------------------------------------------------------------

func outboundCreateBody() []byte {
	req := map[string]any{
		"stockTicker":         "AAPL",
		"settlementDate":      time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		"priceCurrency":       "USD",
		"pricePerUnit":        "150.00",
		"premiumCurrency":     "USD",
		"premium":             "5.00",
		"sellerRoutingNumber": 222,
		"sellerForeignId":     "C-2",
		"amount":              10,
		"buyerLocalUserId":    15,
	}
	b, _ := json.Marshal(req)
	return b
}

func outboundCounterBody() []byte {
	req := map[string]any{
		"settlementDate":  time.Now().Add(72 * time.Hour).Format(time.RFC3339),
		"priceCurrency":   "USD",
		"pricePerUnit":    "155.00",
		"premiumCurrency": "USD",
		"premium":         "6.00",
		"amount":          10,
	}
	b, _ := json.Marshal(req)
	return b
}

func doOutbound(r http.Handler, method, path string, body []byte) *httptest.ResponseRecorder {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

// ---------------------------------------------------------------------------
// Tests: POST /api/interbank/otc/negotiations
// ---------------------------------------------------------------------------

func TestOutboundCreate_NoAuth_401(t *testing.T) {
	ns := newFakeOutboundStoreAPI()
	client := &fakeOutboundClientAPI{}
	// nil claims → RequirePermission has no Claims in context → 401
	r := buildOutboundRouter(nil, ns, client)
	rr := doOutbound(r, http.MethodPost, "/api/interbank/otc/negotiations", outboundCreateBody())
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rr.Code)
	}
}

func TestOutboundCreate_NoPermission_403(t *testing.T) {
	ns := newFakeOutboundStoreAPI()
	client := &fakeOutboundClientAPI{}
	r := buildOutboundRouter(claimsNoPerm(42), ns, client)
	rr := doOutbound(r, http.MethodPost, "/api/interbank/otc/negotiations", outboundCreateBody())
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without OTC_TRADE, got %d", rr.Code)
	}
}

func TestOutboundCreate_HappyPath(t *testing.T) {
	ns := newFakeOutboundStoreAPI()
	partnerID := &protocol.ForeignBankId{RoutingNumber: 222, Id: "neg-partner-42"}
	client := &fakeOutboundClientAPI{createID: partnerID}
	r := buildOutboundRouter(claimsOtcTrade(15), ns, client)
	rr := doOutbound(r, http.MethodPost, "/api/interbank/otc/negotiations", outboundCreateBody())
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body: %s", rr.Code, rr.Body.String())
	}
	var id protocol.ForeignBankId
	if err := json.Unmarshal(rr.Body.Bytes(), &id); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if id.Id != "neg-partner-42" {
		t.Errorf("expected neg-partner-42, got %s", id.Id)
	}
}

func TestOutboundCreate_AdminHappyPath(t *testing.T) {
	ns := newFakeOutboundStoreAPI()
	partnerID := &protocol.ForeignBankId{RoutingNumber: 222, Id: "neg-admin-1"}
	client := &fakeOutboundClientAPI{createID: partnerID}
	r := buildOutboundRouter(claimsAdmin(1), ns, client)
	rr := doOutbound(r, http.MethodPost, "/api/interbank/otc/negotiations", outboundCreateBody())
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin, got %d body: %s", rr.Code, rr.Body.String())
	}
}

func TestOutboundCreate_PartnerFails_500(t *testing.T) {
	ns := newFakeOutboundStoreAPI()
	client := &fakeOutboundClientAPI{createErr: errors.New("partner unavailable")}
	r := buildOutboundRouter(claimsOtcTrade(15), ns, client)
	rr := doOutbound(r, http.MethodPost, "/api/interbank/otc/negotiations", outboundCreateBody())
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when partner fails, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Tests: GET /api/interbank/otc/negotiations
// ---------------------------------------------------------------------------

func TestOutboundList_FiltersByUserScope(t *testing.T) {
	ns := newFakeOutboundStoreAPI()
	remoteID := "neg-r-1"
	ns.rows["neg-1"] = &store.Negotiation{
		ID:            "neg-1",
		BuyerRouting:  111,
		BuyerID:       "C-15",
		SellerRouting: 222,
		SellerID:      "C-2",
		IsOngoing:     true,
		RemoteNegotiationID: &remoteID,
		SettlementDate: time.Now().Add(24 * time.Hour),
		LastModifiedByRouting: 222,
		LastModifiedByID: "C-2",
	}
	ns.rows["neg-2"] = &store.Negotiation{
		ID:            "neg-2",
		BuyerRouting:  111,
		BuyerID:       "C-99", // different user
		SellerRouting: 222,
		SellerID:      "C-3",
		IsOngoing:     true,
		SettlementDate: time.Now().Add(24 * time.Hour),
	}

	client := &fakeOutboundClientAPI{}
	r := buildOutboundRouter(claimsOtcTrade(15), ns, client)
	rr := doOutbound(r, http.MethodGet, "/api/interbank/otc/negotiations", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body: %s", rr.Code, rr.Body.String())
	}
	var views []service.NegotiationView
	if err := json.Unmarshal(rr.Body.Bytes(), &views); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(views) != 1 || views[0].LocalID != "neg-1" {
		t.Errorf("expected 1 view for user 15, got %d: %v", len(views), views)
	}
}

// ---------------------------------------------------------------------------
// Tests: PUT /api/interbank/otc/negotiations/{id}/counter
// ---------------------------------------------------------------------------

func TestOutboundCounter_HappyPath(t *testing.T) {
	ns := newFakeOutboundStoreAPI()
	remoteID := "neg-counter-r"
	ns.rows["neg-counter"] = &store.Negotiation{
		ID:            "neg-counter",
		BuyerRouting:  111,
		BuyerID:       "C-15",
		SellerRouting: 222,
		SellerID:      "C-2",
		StockTicker:   "AAPL",
		PriceCurrency: "USD",
		PriceAmount:   decimal.NewFromFloat(150),
		PremiumCurrency: "USD",
		PremiumAmount: decimal.NewFromFloat(5),
		Amount:        10,
		IsOngoing:     true,
		IsAuthoritative: false,
		RemoteNegotiationID: &remoteID,
		LastModifiedByRouting: 222, // partner was last — our turn
		LastModifiedByID: "C-2",
		SettlementDate: time.Now().Add(48 * time.Hour),
	}

	client := &fakeOutboundClientAPI{}
	r := buildOutboundRouter(claimsOtcTrade(15), ns, client)
	rr := doOutbound(r, http.MethodPut, "/api/interbank/otc/negotiations/neg-counter/counter", outboundCounterBody())
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Tests: POST /api/interbank/otc/negotiations/{id}/accept
// ---------------------------------------------------------------------------

func TestOutboundAccept_HappyPath(t *testing.T) {
	ns := newFakeOutboundStoreAPI()
	remoteID := "neg-acc-r"
	ns.rows["neg-acc"] = &store.Negotiation{
		ID:            "neg-acc",
		BuyerRouting:  111,
		BuyerID:       "C-15",
		SellerRouting: 222,
		SellerID:      "C-2",
		IsOngoing:     true,
		IsAuthoritative: false,
		RemoteNegotiationID: &remoteID,
		LastModifiedByRouting: 222, // partner was last — we can accept
		LastModifiedByID: "C-2",
		SettlementDate: time.Now().Add(48 * time.Hour),
	}

	client := &fakeOutboundClientAPI{}
	r := buildOutboundRouter(claimsOtcTrade(15), ns, client)
	rr := doOutbound(r, http.MethodPost, "/api/interbank/otc/negotiations/neg-acc/accept", nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Tests: DELETE /api/interbank/otc/negotiations/{id}
// ---------------------------------------------------------------------------

func TestOutboundDelete_HappyPath(t *testing.T) {
	ns := newFakeOutboundStoreAPI()
	remoteID := "neg-del-r"
	ns.rows["neg-del"] = &store.Negotiation{
		ID:            "neg-del",
		BuyerRouting:  111,
		BuyerID:       "C-15",
		SellerRouting: 222,
		SellerID:      "C-2",
		IsOngoing:     true,
		IsAuthoritative: false,
		RemoteNegotiationID: &remoteID,
	}

	client := &fakeOutboundClientAPI{}
	r := buildOutboundRouter(claimsOtcTrade(15), ns, client)
	rr := doOutbound(r, http.MethodDelete, "/api/interbank/otc/negotiations/neg-del", nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body: %s", rr.Code, rr.Body.String())
	}
}

func TestOutboundDelete_NonExistent_NoError(t *testing.T) {
	ns := newFakeOutboundStoreAPI()
	client := &fakeOutboundClientAPI{}
	r := buildOutboundRouter(claimsOtcTrade(15), ns, client)
	rr := doOutbound(r, http.MethodDelete, "/api/interbank/otc/negotiations/neg-ghost", nil)
	// Non-existent negotiation is idempotent — service returns nil, handler returns 204.
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 (idempotent delete), got %d body: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Tests: GET /api/interbank/otc/public-stock
// ---------------------------------------------------------------------------

func TestOutboundPublicStock_HappyPath(t *testing.T) {
	ns := newFakeOutboundStoreAPI()
	client := &fakeOutboundClientAPI{}
	r := buildOutboundRouter(claimsOtcTrade(15), ns, client)
	rr := doOutbound(r, http.MethodGet, "/api/interbank/otc/public-stock?bankCode=222", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body: %s", rr.Code, rr.Body.String())
	}
	var entries []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &entries); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least 1 stock entry")
	}
}

func TestOutboundPublicStock_NoAuth_401(t *testing.T) {
	ns := newFakeOutboundStoreAPI()
	client := &fakeOutboundClientAPI{}
	r := buildOutboundRouter(nil, ns, client)
	rr := doOutbound(r, http.MethodGet, "/api/interbank/otc/public-stock", nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}
