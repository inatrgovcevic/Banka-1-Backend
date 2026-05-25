package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/store"
)

// ---------------------------------------------------------------------------
// fakeNegotiationStore — in-memory fake for tests
// ---------------------------------------------------------------------------

type fakeNegotiationStore struct {
	mu   sync.Mutex
	rows map[string]*store.Negotiation
}

func newFakeNegotiationStore() *fakeNegotiationStore {
	return &fakeNegotiationStore{rows: make(map[string]*store.Negotiation)}
}

func (f *fakeNegotiationStore) Insert(_ context.Context, n *store.Negotiation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *n
	f.rows[n.ID] = &cp
	n.CreatedAt = time.Now()
	n.LastModifiedAt = time.Now()
	return nil
}

func (f *fakeNegotiationStore) FindByAuthoritativeRef(_ context.Context, _ int, id string) (*store.Negotiation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, ok := f.rows[id]
	if !ok {
		return nil, nil
	}
	cp := *n
	return &cp, nil
}

func (f *fakeNegotiationStore) UpdateCounter(_ context.Context, n *store.Negotiation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	existing, ok := f.rows[n.ID]
	if !ok {
		return store.ErrNotFound
	}
	existing.Amount = n.Amount
	existing.PriceAmount = n.PriceAmount
	existing.PriceCurrency = n.PriceCurrency
	existing.PremiumAmount = n.PremiumAmount
	existing.PremiumCurrency = n.PremiumCurrency
	existing.SettlementDate = n.SettlementDate
	existing.LastModifiedByRouting = n.LastModifiedByRouting
	existing.LastModifiedByID = n.LastModifiedByID
	existing.Version++
	return nil
}

func (f *fakeNegotiationStore) MarkClosed(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, ok := f.rows[id]
	if !ok {
		return nil // idempotent
	}
	n.IsOngoing = false
	return nil
}

// ---------------------------------------------------------------------------
// fakeCoordinator — minimal fake for accept tests
// ---------------------------------------------------------------------------

type fakeCoordinator struct {
	called bool
	err    error
}

func (f *fakeCoordinator) AcceptNegotiation(_ context.Context, _ *store.Negotiation) error {
	f.called = true
	return f.err
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const (
	myRouting = 111
	theirRouting = 222
)

func futureDate() time.Time { return time.Now().Add(24 * time.Hour) }

func validOffer(senderRouting int) OtcOfferDto {
	return OtcOfferDto{
		Stock:          protocol.StockDescription{Ticker: "AAPL"},
		SettlementDate: futureDate(),
		PricePerUnit:   protocol.MonetaryValue{Currency: "USD", Amount: decimal.NewFromFloat(150.0)},
		Premium:        protocol.MonetaryValue{Currency: "USD", Amount: decimal.NewFromFloat(5.0)},
		BuyerID:        protocol.ForeignBankId{RoutingNumber: senderRouting, Id: "C-2"},
		SellerID:       protocol.ForeignBankId{RoutingNumber: myRouting, Id: "C-15"},
		Amount:         10,
		LastModifiedBy: protocol.ForeignBankId{RoutingNumber: senderRouting, Id: "C-2"},
	}
}

func newService(s NegotiationStoreIface, c CoordinatorIface) *OtcNegotiationService {
	return NewOtcNegotiationService(myRouting, s, c, nil)
}

// ---------------------------------------------------------------------------
// CreateNegotiation tests
// ---------------------------------------------------------------------------

func TestCreateNegotiation_Happy(t *testing.T) {
	s := newFakeNegotiationStore()
	svc := newService(s, &fakeCoordinator{})
	id, err := svc.CreateNegotiation(context.Background(), validOffer(theirRouting), theirRouting)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.RoutingNumber != myRouting {
		t.Errorf("expected routing %d, got %d", myRouting, id.RoutingNumber)
	}
	if id.Id == "" {
		t.Error("expected non-empty negotiation id")
	}
}

func TestCreateNegotiation_SellerNotUs(t *testing.T) {
	s := newFakeNegotiationStore()
	svc := newService(s, &fakeCoordinator{})
	offer := validOffer(theirRouting)
	offer.SellerID.RoutingNumber = theirRouting // wrong — seller must be us
	_, err := svc.CreateNegotiation(context.Background(), offer, theirRouting)
	if !errors.Is(err, ErrNegotiationInvalid) {
		t.Fatalf("expected ErrNegotiationInvalid, got %v", err)
	}
}

func TestCreateNegotiation_BuyerMismatch(t *testing.T) {
	s := newFakeNegotiationStore()
	svc := newService(s, &fakeCoordinator{})
	offer := validOffer(theirRouting)
	offer.BuyerID.RoutingNumber = 999 // doesn't match senderRouting
	_, err := svc.CreateNegotiation(context.Background(), offer, theirRouting)
	if !errors.Is(err, ErrNegotiationInvalid) {
		t.Fatalf("expected ErrNegotiationInvalid, got %v", err)
	}
}

func TestCreateNegotiation_PastSettlement(t *testing.T) {
	s := newFakeNegotiationStore()
	svc := newService(s, &fakeCoordinator{})
	offer := validOffer(theirRouting)
	offer.SettlementDate = time.Now().Add(-24 * time.Hour)
	_, err := svc.CreateNegotiation(context.Background(), offer, theirRouting)
	if !errors.Is(err, ErrNegotiationInvalid) {
		t.Fatalf("expected ErrNegotiationInvalid, got %v", err)
	}
}

func TestCreateNegotiation_NegativeAmount(t *testing.T) {
	s := newFakeNegotiationStore()
	svc := newService(s, &fakeCoordinator{})
	offer := validOffer(theirRouting)
	offer.Amount = -1
	_, err := svc.CreateNegotiation(context.Background(), offer, theirRouting)
	if !errors.Is(err, ErrNegotiationInvalid) {
		t.Fatalf("expected ErrNegotiationInvalid, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetNegotiation tests
// ---------------------------------------------------------------------------

func TestGetNegotiation_Happy(t *testing.T) {
	s := newFakeNegotiationStore()
	svc := newService(s, &fakeCoordinator{})
	id, _ := svc.CreateNegotiation(context.Background(), validOffer(theirRouting), theirRouting)
	dto, err := svc.GetNegotiation(context.Background(), myRouting, id.Id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Stock.Ticker != "AAPL" {
		t.Errorf("expected AAPL, got %s", dto.Stock.Ticker)
	}
	if !dto.IsOngoing {
		t.Error("expected isOngoing=true")
	}
}

func TestGetNegotiation_NotFound(t *testing.T) {
	s := newFakeNegotiationStore()
	svc := newService(s, &fakeCoordinator{})
	_, err := svc.GetNegotiation(context.Background(), myRouting, "nonexistent")
	if !errors.Is(err, ErrNegotiationNotFound) {
		t.Fatalf("expected ErrNegotiationNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// UpdateCounter tests
// ---------------------------------------------------------------------------

func TestUpdateCounter_Happy(t *testing.T) {
	s := newFakeNegotiationStore()
	svc := newService(s, &fakeCoordinator{})
	id, _ := svc.CreateNegotiation(context.Background(), validOffer(theirRouting), theirRouting)
	// Now we (myRouting) make a counter. The existing lastModifiedBy=theirRouting, so it's our turn.
	offer := validOffer(theirRouting) // reuse shape but flip lastModifiedBy to us
	offer.LastModifiedBy = protocol.ForeignBankId{RoutingNumber: myRouting, Id: "sys"}
	offer.BuyerID = protocol.ForeignBankId{RoutingNumber: theirRouting, Id: "C-2"}
	offer.SellerID = protocol.ForeignBankId{RoutingNumber: myRouting, Id: "C-15"}
	err := svc.UpdateCounter(context.Background(), myRouting, id.Id, offer, myRouting)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateCounter_TurnViolation(t *testing.T) {
	s := newFakeNegotiationStore()
	svc := newService(s, &fakeCoordinator{})
	id, _ := svc.CreateNegotiation(context.Background(), validOffer(theirRouting), theirRouting)
	// theirRouting was last modifier — they try to modify again → TurnViolation
	offer := validOffer(theirRouting)
	err := svc.UpdateCounter(context.Background(), myRouting, id.Id, offer, theirRouting)
	if !errors.Is(err, ErrTurnViolation) {
		t.Fatalf("expected ErrTurnViolation, got %v", err)
	}
}

func TestUpdateCounter_NotFound(t *testing.T) {
	s := newFakeNegotiationStore()
	svc := newService(s, &fakeCoordinator{})
	offer := validOffer(theirRouting)
	err := svc.UpdateCounter(context.Background(), myRouting, "ghost", offer, theirRouting)
	if !errors.Is(err, ErrNegotiationNotFound) {
		t.Fatalf("expected ErrNegotiationNotFound, got %v", err)
	}
}

func TestUpdateCounter_Closed(t *testing.T) {
	s := newFakeNegotiationStore()
	svc := newService(s, &fakeCoordinator{})
	id, _ := svc.CreateNegotiation(context.Background(), validOffer(theirRouting), theirRouting)
	_ = s.MarkClosed(context.Background(), id.Id)
	offer := validOffer(theirRouting)
	offer.LastModifiedBy = protocol.ForeignBankId{RoutingNumber: myRouting, Id: "sys"}
	err := svc.UpdateCounter(context.Background(), myRouting, id.Id, offer, myRouting)
	if !errors.Is(err, ErrNegotiationClosed) {
		t.Fatalf("expected ErrNegotiationClosed, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Delete tests
// ---------------------------------------------------------------------------

func TestDelete_Happy(t *testing.T) {
	s := newFakeNegotiationStore()
	svc := newService(s, &fakeCoordinator{})
	id, _ := svc.CreateNegotiation(context.Background(), validOffer(theirRouting), theirRouting)
	if err := svc.Delete(context.Background(), myRouting, id.Id, theirRouting); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify closed.
	dto, _ := svc.GetNegotiation(context.Background(), myRouting, id.Id)
	if dto.IsOngoing {
		t.Error("expected isOngoing=false after delete")
	}
}

func TestDelete_Idempotent(t *testing.T) {
	s := newFakeNegotiationStore()
	svc := newService(s, &fakeCoordinator{})
	// Delete of non-existent — must return nil (idempotent).
	if err := svc.Delete(context.Background(), myRouting, "ghost", theirRouting); err != nil {
		t.Fatalf("expected nil for missing neg, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// AcceptNegotiation tests
// ---------------------------------------------------------------------------

func TestAcceptNegotiation_Happy(t *testing.T) {
	s := newFakeNegotiationStore()
	coord := &fakeCoordinator{}
	svc := newService(s, coord)
	id, _ := svc.CreateNegotiation(context.Background(), validOffer(theirRouting), theirRouting)
	// Now we counter so theirRouting can accept (lastModifiedBy=us).
	neg, _ := s.FindByAuthoritativeRef(context.Background(), myRouting, id.Id)
	neg.LastModifiedByRouting = myRouting
	s.mu.Lock()
	s.rows[id.Id] = neg
	s.mu.Unlock()
	err := svc.AcceptNegotiation(context.Background(), myRouting, id.Id, theirRouting)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !coord.called {
		t.Error("expected coordinator to be called")
	}
}

func TestAcceptNegotiation_NotFound(t *testing.T) {
	svc := newService(newFakeNegotiationStore(), &fakeCoordinator{})
	err := svc.AcceptNegotiation(context.Background(), myRouting, "ghost", theirRouting)
	if !errors.Is(err, ErrNegotiationNotFound) {
		t.Fatalf("expected ErrNegotiationNotFound, got %v", err)
	}
}

func TestAcceptNegotiation_TurnViolation(t *testing.T) {
	s := newFakeNegotiationStore()
	svc := newService(s, &fakeCoordinator{})
	id, _ := svc.CreateNegotiation(context.Background(), validOffer(theirRouting), theirRouting)
	// lastModifiedBy = theirRouting, so theirRouting tries to accept → turn violation.
	err := svc.AcceptNegotiation(context.Background(), myRouting, id.Id, theirRouting)
	if !errors.Is(err, ErrTurnViolation) {
		t.Fatalf("expected ErrTurnViolation, got %v", err)
	}
}

func TestAcceptNegotiation_Closed(t *testing.T) {
	s := newFakeNegotiationStore()
	svc := newService(s, &fakeCoordinator{})
	id, _ := svc.CreateNegotiation(context.Background(), validOffer(theirRouting), theirRouting)
	_ = s.MarkClosed(context.Background(), id.Id)
	neg, _ := s.FindByAuthoritativeRef(context.Background(), myRouting, id.Id)
	neg.LastModifiedByRouting = myRouting
	s.mu.Lock()
	s.rows[id.Id] = neg
	s.mu.Unlock()
	err := svc.AcceptNegotiation(context.Background(), myRouting, id.Id, theirRouting)
	if !errors.Is(err, ErrNegotiationClosed) {
		t.Fatalf("expected ErrNegotiationClosed, got %v", err)
	}
}
