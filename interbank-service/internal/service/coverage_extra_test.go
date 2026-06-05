package service_test

import (
	"context"
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
// PartnerLookup / NewPartnerLookup / FindByRouting
// ---------------------------------------------------------------------------

type staticPS struct{ partners []auth.Partner }

func (s *staticPS) Partners() []auth.Partner { return s.partners }

func TestNewPartnerLookup_FindByRouting(t *testing.T) {
	ps := &staticPS{partners: []auth.Partner{{Routing: 222, DisplayName: "Banka 2", BaseURL: "http://x/"}}}
	lk := service.NewPartnerLookup(ps)
	p, err := lk.FindByRouting(222)
	if err != nil || p == nil || p.Routing != 222 {
		t.Fatalf("FindByRouting(222) → %+v %v", p, err)
	}
	if _, err := lk.FindByRouting(999); err == nil {
		t.Error("expected error for unknown routing")
	}
}

func TestNewInterbankClient_Constructs(t *testing.T) {
	// NewInterbankClient accepts a *store.MessageStore; passing nil is fine since
	// we only verify construction (no method call that touches the pool).
	ps := &staticPS{partners: []auth.Partner{{Routing: 222}}}
	lk := service.NewPartnerLookup(ps)
	c := service.NewInterbankClient(111, lk, nil, nil, nil)
	if c == nil {
		t.Fatal("expected non-nil InterbankClient")
	}
}

// ---------------------------------------------------------------------------
// PartnerStoreAdapter.DisplayName
// ---------------------------------------------------------------------------

func TestPartnerStoreAdapter_DisplayName(t *testing.T) {
	ps := &staticPS{partners: []auth.Partner{{Routing: 222, DisplayName: "Banka 2"}}}
	a := service.NewPartnerStoreAdapter(ps)
	if got := a.DisplayName(222); got != "Banka 2" {
		t.Errorf("expected Banka 2, got %q", got)
	}
	if got := a.DisplayName(999); got != "" {
		t.Errorf("expected empty for unknown, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// OutboundPutCounter / OutboundAccept (HTTP status propagation)
// ---------------------------------------------------------------------------

func TestOutboundPutCounter_StatusPropagated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	fms := &fakeMessageStore{}
	client := service.NewInterbankClientWithStore(111, newTestPartner(srv.URL), fms, nil, nil)
	offer := service.OtcOfferDto{
		Stock:          protocol.StockDescription{Ticker: "AAPL"},
		SettlementDate: time.Now().Add(48 * time.Hour),
		PricePerUnit:   protocol.MonetaryValue{Currency: "USD", Amount: decimal.NewFromFloat(150)},
		Premium:        protocol.MonetaryValue{Currency: "USD", Amount: decimal.NewFromFloat(5)},
		Amount:         10,
	}
	negID := protocol.ForeignBankId{RoutingNumber: 222, Id: "neg-1"}
	status, err := client.OutboundPutCounter(context.Background(), 222, negID, offer)
	if err != nil {
		t.Fatalf("OutboundPutCounter: %v", err)
	}
	if status != http.StatusNoContent {
		t.Errorf("expected 204, got %d", status)
	}
}

func TestOutboundPutCounter_PartnerNotFound(t *testing.T) {
	fms := &fakeMessageStore{}
	client := service.NewInterbankClientWithStore(111, newTestPartner("http://unused"), fms, nil, nil)
	_, err := client.OutboundPutCounter(context.Background(), 999, protocol.ForeignBankId{RoutingNumber: 999, Id: "x"}, service.OtcOfferDto{})
	if err == nil {
		t.Error("expected partner-not-found error")
	}
}

func TestOutboundAccept_StatusPropagated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	fms := &fakeMessageStore{}
	client := service.NewInterbankClientWithStore(111, newTestPartner(srv.URL), fms, nil, nil)
	negID := protocol.ForeignBankId{RoutingNumber: 222, Id: "neg-1"}
	status, err := client.OutboundAccept(context.Background(), 222, negID)
	if err != nil {
		t.Fatalf("OutboundAccept: %v", err)
	}
	if status != http.StatusNoContent {
		t.Errorf("expected 204, got %d", status)
	}
}

func TestOutboundAccept_Conflict409(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	fms := &fakeMessageStore{}
	client := service.NewInterbankClientWithStore(111, newTestPartner(srv.URL), fms, nil, nil)
	negID := protocol.ForeignBankId{RoutingNumber: 222, Id: "neg-1"}
	status, err := client.OutboundAccept(context.Background(), 222, negID)
	// doRequestFull returns ErrOutboundConflict for 409.
	if err == nil {
		t.Error("expected conflict error")
	}
	if status != http.StatusConflict {
		t.Errorf("expected 409, got %d", status)
	}
}

func TestOutboundAccept_PartnerNotFound(t *testing.T) {
	fms := &fakeMessageStore{}
	client := service.NewInterbankClientWithStore(111, newTestPartner("http://unused"), fms, nil, nil)
	_, err := client.OutboundAccept(context.Background(), 999, protocol.ForeignBankId{RoutingNumber: 999, Id: "x"})
	if err == nil {
		t.Error("expected partner-not-found error")
	}
}

// ---------------------------------------------------------------------------
// GetOne + findByLocalOrRemote (via OtcOutboundService)
// ---------------------------------------------------------------------------

func outboundSvcForGet(ns service.NegotiationStoreForOutbound) *service.OtcOutboundService {
	return service.NewOtcOutboundService(111, ns, &outboundClientStub{}, nil,
		&partnerNamesStub{names: map[int]string{222: "Banka 2"}}, nil)
}

type outboundClientStub struct {
	counterStatus int
	counterErr    error
	acceptStatus  int
	acceptErr     error
	deleteErr     error
}

func (c *outboundClientStub) OutboundCreateNegotiation(_ context.Context, _ int, _ service.OtcOfferDto) (*protocol.ForeignBankId, error) {
	return &protocol.ForeignBankId{RoutingNumber: 222, Id: "remote-1"}, nil
}
func (c *outboundClientStub) OutboundPutCounter(_ context.Context, _ int, _ protocol.ForeignBankId, _ service.OtcOfferDto) (int, error) {
	return c.counterStatus, c.counterErr
}
func (c *outboundClientStub) OutboundAccept(_ context.Context, _ int, _ protocol.ForeignBankId) (int, error) {
	return c.acceptStatus, c.acceptErr
}
func (c *outboundClientStub) OutboundDelete(_ context.Context, _ int, _ protocol.ForeignBankId) error {
	return c.deleteErr
}
func (c *outboundClientStub) OutboundFetchPublicStock(_ context.Context, _ int) ([]protocol.PublicStockEntry, error) {
	return nil, nil
}

type partnerNamesStub struct{ names map[int]string }

func (p *partnerNamesStub) DisplayName(routing int) string { return p.names[routing] }

// memNegStore is a minimal NegotiationStoreForOutbound backed by a map.
type memNegStore struct{ rows map[string]*store.Negotiation }

func newMemNegStore() *memNegStore { return &memNegStore{rows: map[string]*store.Negotiation{}} }

func (m *memNegStore) Insert(_ context.Context, n *store.Negotiation) error {
	cp := *n
	m.rows[n.ID] = &cp
	return nil
}
func (m *memNegStore) FindByID(_ context.Context, id string) (*store.Negotiation, error) {
	if n, ok := m.rows[id]; ok {
		cp := *n
		return &cp, nil
	}
	return nil, nil
}
func (m *memNegStore) FindByAuthoritativeRef(_ context.Context, _ int, id string) (*store.Negotiation, error) {
	if n, ok := m.rows[id]; ok {
		cp := *n
		return &cp, nil
	}
	for _, n := range m.rows {
		if n.RemoteNegotiationID != nil && *n.RemoteNegotiationID == id {
			cp := *n
			return &cp, nil
		}
	}
	return nil, nil
}
func (m *memNegStore) UpdateCounter(_ context.Context, n *store.Negotiation) error {
	m.rows[n.ID] = n
	return nil
}
func (m *memNegStore) MarkClosed(_ context.Context, id string) error {
	if n, ok := m.rows[id]; ok {
		n.IsOngoing = false
	}
	return nil
}
func (m *memNegStore) ListForUser(_ context.Context, userForeignID string, includeAll bool) ([]*store.Negotiation, error) {
	var out []*store.Negotiation
	for _, n := range m.rows {
		if includeAll || n.BuyerID == userForeignID || n.SellerID == userForeignID {
			cp := *n
			out = append(out, &cp)
		}
	}
	return out, nil
}

func TestGetOne_ByLocalID(t *testing.T) {
	ns := newMemNegStore()
	ns.rows["neg-1"] = &store.Negotiation{
		ID: "neg-1", BuyerRouting: 111, BuyerID: "C-15", SellerRouting: 222, SellerID: "C-2",
		StockTicker: "AAPL", IsOngoing: true, IsAuthoritative: true,
	}
	svc := outboundSvcForGet(ns)
	v, err := svc.GetOne(context.Background(), "neg-1")
	if err != nil {
		t.Fatalf("GetOne: %v", err)
	}
	if v.LocalID != "neg-1" || v.StockTicker != "AAPL" {
		t.Errorf("view: %+v", v)
	}
	if v.CounterpartyBankName != "Banka 2" {
		t.Errorf("counterparty name: %q", v.CounterpartyBankName)
	}
}

func TestGetOne_ByRemoteID(t *testing.T) {
	remote := "remote-xyz"
	ns := newMemNegStore()
	ns.rows["neg-2"] = &store.Negotiation{
		ID: "neg-2", BuyerRouting: 111, BuyerID: "C-15", SellerRouting: 222, SellerID: "C-2",
		RemoteNegotiationID: &remote, IsOngoing: true,
	}
	svc := outboundSvcForGet(ns)
	v, err := svc.GetOne(context.Background(), "remote-xyz")
	if err != nil {
		t.Fatalf("GetOne by remote: %v", err)
	}
	if v.LocalID != "neg-2" {
		t.Errorf("expected neg-2, got %s", v.LocalID)
	}
}

func TestGetOne_NotFound(t *testing.T) {
	ns := newMemNegStore()
	svc := outboundSvcForGet(ns)
	_, err := svc.GetOne(context.Background(), "ghost")
	if err == nil {
		t.Error("expected ErrNegotiationNotFound")
	}
}

// ---------------------------------------------------------------------------
// AcceptOutbound — mirror path (we are buyer-bank) → routes to partner
// ---------------------------------------------------------------------------

func TestAcceptOutbound_MirrorBuyerBank(t *testing.T) {
	remote := "remote-acc"
	ns := newMemNegStore()
	ns.rows["neg-acc"] = &store.Negotiation{
		ID: "neg-acc", BuyerRouting: 111, BuyerID: "C-15", SellerRouting: 222, SellerID: "C-2",
		IsOngoing: true, IsAuthoritative: false, RemoteNegotiationID: &remote,
		LastModifiedByRouting: 222, LastModifiedByID: "C-2",
		SettlementDate: time.Now().Add(48 * time.Hour),
	}
	svc := service.NewOtcOutboundService(111, ns, &outboundClientStub{acceptStatus: 204}, nil,
		&partnerNamesStub{names: map[int]string{222: "Banka 2"}}, nil)
	status, err := svc.AcceptOutbound(context.Background(), "neg-acc")
	if err != nil {
		t.Fatalf("AcceptOutbound: %v", err)
	}
	if status != 204 {
		t.Errorf("expected 204, got %d", status)
	}
}

func TestAcceptOutbound_Closed(t *testing.T) {
	ns := newMemNegStore()
	ns.rows["neg-closed"] = &store.Negotiation{
		ID: "neg-closed", BuyerRouting: 111, SellerRouting: 222, IsOngoing: false,
	}
	svc := outboundSvcForGet(ns)
	_, err := svc.AcceptOutbound(context.Background(), "neg-closed")
	if err == nil {
		t.Error("expected closed error")
	}
}

func TestAcceptOutbound_TurnViolation(t *testing.T) {
	ns := newMemNegStore()
	ns.rows["neg-turn"] = &store.Negotiation{
		ID: "neg-turn", BuyerRouting: 111, SellerRouting: 222, IsOngoing: true,
		LastModifiedByRouting: 111, // our bank modified last → cannot accept
		SettlementDate: time.Now().Add(48 * time.Hour),
	}
	svc := outboundSvcForGet(ns)
	_, err := svc.AcceptOutbound(context.Background(), "neg-turn")
	if err == nil {
		t.Error("expected turn violation")
	}
}

// ---------------------------------------------------------------------------
// CounterOutbound — mirror path (outbound PUT) success + conflict
// ---------------------------------------------------------------------------

func counterReq() service.OutboundCounterRequest {
	return service.OutboundCounterRequest{
		SettlementDate:  time.Now().Add(72 * time.Hour),
		PriceCurrency:   "USD",
		PricePerUnit:    decimal.NewFromFloat(155),
		PremiumCurrency: "USD",
		Premium:         decimal.NewFromFloat(6),
		Amount:          10,
	}
}

func TestCounterOutbound_MirrorSuccess(t *testing.T) {
	remote := "remote-c"
	ns := newMemNegStore()
	ns.rows["neg-c"] = &store.Negotiation{
		ID: "neg-c", BuyerRouting: 111, BuyerID: "C-15", SellerRouting: 222, SellerID: "C-2",
		StockTicker: "AAPL", IsOngoing: true, IsAuthoritative: false, RemoteNegotiationID: &remote,
		LastModifiedByRouting: 222, LastModifiedByID: "C-2",
		SettlementDate: time.Now().Add(48 * time.Hour),
	}
	svc := service.NewOtcOutboundService(111, ns, &outboundClientStub{counterStatus: 204}, nil,
		&partnerNamesStub{names: map[int]string{222: "Banka 2"}}, nil)
	res, err := svc.CounterOutbound(context.Background(), 15, "neg-c", counterReq())
	if err != nil {
		t.Fatalf("CounterOutbound: %v", err)
	}
	if res.StatusCode != 204 {
		t.Errorf("expected 204, got %d", res.StatusCode)
	}
}

func TestCounterOutbound_AuthoritativeLocal(t *testing.T) {
	ns := newMemNegStore()
	ns.rows["neg-auth"] = &store.Negotiation{
		ID: "neg-auth", BuyerRouting: 222, BuyerID: "C-2", SellerRouting: 111, SellerID: "C-15",
		StockTicker: "AAPL", IsOngoing: true, IsAuthoritative: true,
		PriceCurrency: "USD", PriceAmount: decimal.NewFromFloat(150),
		PremiumCurrency: "USD", PremiumAmount: decimal.NewFromFloat(5), Amount: 10,
		LastModifiedByRouting: 222, LastModifiedByID: "C-2", // partner last → our turn
		SettlementDate: time.Now().Add(48 * time.Hour),
	}
	// otcSvc shares the same store so UpdateCounter mutates the authoritative row.
	otcSvc := service.NewOtcNegotiationService(111, ns, nil, nil)
	svc := service.NewOtcOutboundService(111, ns, &outboundClientStub{}, otcSvc,
		&partnerNamesStub{names: map[int]string{222: "Banka 2"}}, nil)

	res, err := svc.CounterOutbound(context.Background(), 15, "neg-auth", counterReq())
	if err != nil {
		t.Fatalf("CounterOutbound authoritative: %v", err)
	}
	if res.StatusCode != 204 || res.View == nil {
		t.Errorf("expected 204 + view, got %d view=%v", res.StatusCode, res.View)
	}
}

func TestCounterOutbound_TurnViolation(t *testing.T) {
	ns := newMemNegStore()
	ns.rows["neg-t"] = &store.Negotiation{
		ID: "neg-t", BuyerRouting: 111, SellerRouting: 222, IsOngoing: true,
		LastModifiedByRouting: 111, // our turn already used
		SettlementDate: time.Now().Add(48 * time.Hour),
	}
	svc := outboundSvcForGet(ns)
	_, err := svc.CounterOutbound(context.Background(), 15, "neg-t", counterReq())
	if err == nil {
		t.Error("expected turn violation")
	}
}

func TestCounterOutbound_ZeroPrincipal(t *testing.T) {
	ns := newMemNegStore()
	svc := outboundSvcForGet(ns)
	_, err := svc.CounterOutbound(context.Background(), 0, "x", counterReq())
	if err == nil {
		t.Error("expected error for zero principal")
	}
}

func TestCounterOutbound_Closed(t *testing.T) {
	ns := newMemNegStore()
	ns.rows["neg-cl"] = &store.Negotiation{ID: "neg-cl", BuyerRouting: 111, SellerRouting: 222, IsOngoing: false}
	svc := outboundSvcForGet(ns)
	_, err := svc.CounterOutbound(context.Background(), 15, "neg-cl", counterReq())
	if err == nil {
		t.Error("expected closed error")
	}
}

func TestAcceptOutbound_NotBuyerBank(t *testing.T) {
	// Mirror row where we are NEITHER buyer nor (authoritative) → not buyer-bank.
	remote := "r"
	ns := newMemNegStore()
	ns.rows["neg-nb"] = &store.Negotiation{
		ID: "neg-nb", BuyerRouting: 222, SellerRouting: 333, IsOngoing: true,
		IsAuthoritative: false, RemoteNegotiationID: &remote,
		LastModifiedByRouting: 222,
		SettlementDate: time.Now().Add(48 * time.Hour),
	}
	svc := outboundSvcForGet(ns)
	_, err := svc.AcceptOutbound(context.Background(), "neg-nb")
	if err == nil {
		t.Error("expected not-buyer-bank error")
	}
}
