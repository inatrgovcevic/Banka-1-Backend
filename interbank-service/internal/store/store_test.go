package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/shopspring/decimal"
)

// testPool boots a pgxpool against TEST_DATABASE_URL and applies migrations.
// Each invocation resets (drops+recreates) the schema so test isolation is guaranteed.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL=postgres://... to run store integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	// Apply migrations using database/sql (goose requires *sql.DB)
	sqlDB, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer sqlDB.Close()
	goose.SetDialect("postgres")
	// Reset first (ignore error on first run when tables don't exist yet)
	if err := goose.Reset(sqlDB, "../../migrations"); err != nil {
		t.Logf("goose reset (ok if first run): %v", err)
	}
	if err := goose.Up(sqlDB, "../../migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

// ─────────────────────────── MessageStore ───────────────────────────

func TestMessageStore_InsertAndLookup(t *testing.T) {
	pool := testPool(t)
	s := NewMessageStore(pool)
	ctx := context.Background()

	body := `{"messageType":"NEW_TX"}`
	m := &Message{
		Direction:           DirectionInbound,
		SenderRoutingNumber: 222,
		LocallyGeneratedKey: "test-insert-001",
		MessageType:         "NEW_TX",
		Status:              MessageStatusProcessed,
		RequestBody:         body,
		HttpStatus:          intPtr(200),
	}
	if err := s.Insert(ctx, m); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if m.ID == 0 {
		t.Errorf("ID not populated after insert")
	}
	if m.CreatedAt.IsZero() {
		t.Errorf("CreatedAt not populated after insert")
	}
	if m.Version != 0 {
		t.Errorf("version=%d want 0 after fresh insert", m.Version)
	}

	got, err := s.Lookup(ctx, DirectionInbound, 222, "test-insert-001")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got == nil {
		t.Fatal("lookup returned nil, want row")
	}
	if got.LocallyGeneratedKey != "test-insert-001" {
		t.Errorf("key=%q want test-insert-001", got.LocallyGeneratedKey)
	}
	if got.Direction != DirectionInbound {
		t.Errorf("direction=%q want INBOUND", got.Direction)
	}
	if got.MessageType != "NEW_TX" {
		t.Errorf("messageType=%q want NEW_TX", got.MessageType)
	}
}

func TestMessageStore_Lookup_Miss(t *testing.T) {
	pool := testPool(t)
	s := NewMessageStore(pool)
	got, err := s.Lookup(context.Background(), DirectionInbound, 999, "nonexistent-key")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for miss, got %+v", got)
	}
}

func TestMessageStore_UniqueViolation(t *testing.T) {
	pool := testPool(t)
	s := NewMessageStore(pool)
	ctx := context.Background()

	m := &Message{
		Direction:           DirectionInbound,
		SenderRoutingNumber: 222,
		LocallyGeneratedKey: "test-dup-001",
		MessageType:         "NEW_TX",
		Status:              MessageStatusProcessed,
		RequestBody:         `{}`,
		HttpStatus:          intPtr(200),
	}
	if err := s.Insert(ctx, m); err != nil {
		t.Fatalf("insert 1: %v", err)
	}
	// Reset ID so Insert doesn't think it's an update
	m.ID = 0
	err := s.Insert(ctx, m)
	if !IsUniqueViolation(err) {
		t.Errorf("expected unique violation (23505), got %v", err)
	}
}

func TestMessageStore_UpdateOptimistic(t *testing.T) {
	pool := testPool(t)
	s := NewMessageStore(pool)
	ctx := context.Background()

	m := &Message{
		Direction:           DirectionOutbound,
		SenderRoutingNumber: 111,
		LocallyGeneratedKey: "test-update-001",
		MessageType:         "NEW_TX",
		Status:              MessageStatusPendingSend,
		RequestBody:         `{"messageType":"NEW_TX"}`,
	}
	if err := s.Insert(ctx, m); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Happy-path update
	resp := `{"vote":"YES"}`
	m.Status = MessageStatusSent
	m.HttpStatus = intPtr(200)
	m.ResponseBody = &resp
	now := time.Now()
	m.LastAttemptAt = &now

	if err := s.UpdateOptimistic(ctx, m); err != nil {
		t.Fatalf("update: %v", err)
	}
	if m.Version != 1 {
		t.Errorf("version=%d want 1 after first update", m.Version)
	}

	// Stale-version update must return ErrOptimisticLockConflict
	stale := *m
	stale.Version = 0 // deliberately stale
	if err := s.UpdateOptimistic(ctx, &stale); err == nil {
		t.Error("expected ErrOptimisticLockConflict on stale version, got nil")
	}
}

func TestMessageStore_FindPending(t *testing.T) {
	pool := testPool(t)
	s := NewMessageStore(pool)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		m := &Message{
			Direction:           DirectionOutbound,
			SenderRoutingNumber: 111,
			LocallyGeneratedKey: fmt.Sprintf("pending-%d", i),
			MessageType:         "NEW_TX",
			Status:              MessageStatusPendingSend,
			RequestBody:         `{}`,
		}
		if err := s.Insert(ctx, m); err != nil {
			t.Fatalf("insert pending %d: %v", i, err)
		}
	}
	// Also insert a SENT message — should not appear in FindPending
	sent := &Message{
		Direction:           DirectionOutbound,
		SenderRoutingNumber: 111,
		LocallyGeneratedKey: "already-sent",
		MessageType:         "NEW_TX",
		Status:              MessageStatusSent,
		RequestBody:         `{}`,
	}
	if err := s.Insert(ctx, sent); err != nil {
		t.Fatalf("insert sent: %v", err)
	}

	cutoff := time.Now().Add(time.Minute) // generous cutoff — catches all
	rows, err := s.FindPending(ctx, 5, cutoff, 10)
	if err != nil {
		t.Fatalf("FindPending: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("FindPending returned %d rows, want 3", len(rows))
	}
	for _, r := range rows {
		if r.Status != MessageStatusPendingSend {
			t.Errorf("unexpected status %q in pending results", r.Status)
		}
	}
}

// ─────────────────────────── NegotiationStore ───────────────────────────

func TestNegotiationStore_InsertAndFindByID(t *testing.T) {
	pool := testPool(t)
	s := NewNegotiationStore(pool)
	ctx := context.Background()

	n := sampleNegotiation("test-neg-001")
	if err := s.Insert(ctx, n); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if n.CreatedAt.IsZero() {
		t.Errorf("CreatedAt not populated")
	}

	got, err := s.FindByID(ctx, "test-neg-001")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got == nil {
		t.Fatal("FindByID returned nil, want row")
	}
	if got.StockTicker != "AAPL" {
		t.Errorf("ticker=%q want AAPL", got.StockTicker)
	}
	if !got.PriceAmount.Equal(decimal.RequireFromString("200.00")) {
		t.Errorf("price=%v want 200.00", got.PriceAmount)
	}
}

func TestNegotiationStore_FindByID_Miss(t *testing.T) {
	pool := testPool(t)
	s := NewNegotiationStore(pool)
	got, err := s.FindByID(context.Background(), "nonexistent-neg")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for miss, got %+v", got)
	}
}

func TestNegotiationStore_MarkClosed(t *testing.T) {
	pool := testPool(t)
	s := NewNegotiationStore(pool)
	ctx := context.Background()

	n := sampleNegotiation("test-neg-close")
	if err := s.Insert(ctx, n); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := s.MarkClosed(ctx, n.ID); err != nil {
		t.Fatalf("MarkClosed: %v", err)
	}
	got, err := s.FindByID(ctx, n.ID)
	if err != nil {
		t.Fatalf("FindByID after close: %v", err)
	}
	if got == nil {
		t.Fatal("got nil after close")
	}
	if got.IsOngoing {
		t.Errorf("is_ongoing=true after MarkClosed, want false")
	}
	// Idempotent — second call must not error
	if err := s.MarkClosed(ctx, n.ID); err != nil {
		t.Errorf("second MarkClosed should be idempotent, got: %v", err)
	}
}

func TestNegotiationStore_FindByAuthoritativeRef(t *testing.T) {
	pool := testPool(t)
	s := NewNegotiationStore(pool)
	ctx := context.Background()

	// Authoritative negotiation — found by its own id
	auth := sampleNegotiation("test-neg-auth")
	auth.IsAuthoritative = true
	if err := s.Insert(ctx, auth); err != nil {
		t.Fatalf("insert auth: %v", err)
	}

	// Mirror negotiation — found by remote_negotiation_id
	remote := "remote-neg-777"
	mirror := sampleNegotiation("test-neg-mirror")
	mirror.IsAuthoritative = false
	mirror.RemoteNegotiationID = &remote
	if err := s.Insert(ctx, mirror); err != nil {
		t.Fatalf("insert mirror: %v", err)
	}

	// Look up authoritative by its id
	g1, err := s.FindByAuthoritativeRef(ctx, 111, "test-neg-auth")
	if err != nil {
		t.Fatalf("FindByAuthoritativeRef auth: %v", err)
	}
	if g1 == nil || g1.ID != "test-neg-auth" {
		t.Errorf("expected test-neg-auth, got %v", g1)
	}

	// Look up mirror by remote id
	g2, err := s.FindByAuthoritativeRef(ctx, 222, "remote-neg-777")
	if err != nil {
		t.Fatalf("FindByAuthoritativeRef mirror: %v", err)
	}
	if g2 == nil || g2.ID != "test-neg-mirror" {
		t.Errorf("expected test-neg-mirror, got %v", g2)
	}
}

func TestNegotiationStore_UpdateCounter(t *testing.T) {
	pool := testPool(t)
	s := NewNegotiationStore(pool)
	ctx := context.Background()

	n := sampleNegotiation("test-neg-counter")
	if err := s.Insert(ctx, n); err != nil {
		t.Fatalf("insert: %v", err)
	}

	n.PriceAmount = decimal.RequireFromString("250.00")
	n.Amount = 5
	if err := s.UpdateCounter(ctx, n); err != nil {
		t.Fatalf("UpdateCounter: %v", err)
	}
	if n.Version != 1 {
		t.Errorf("version=%d want 1", n.Version)
	}

	got, _ := s.FindByID(ctx, n.ID)
	if !got.PriceAmount.Equal(decimal.RequireFromString("250.00")) {
		t.Errorf("price after update=%v want 250.00", got.PriceAmount)
	}

	// Stale version must conflict
	stale := *n
	stale.Version = 0
	if err := s.UpdateCounter(ctx, &stale); err == nil {
		t.Error("expected ErrOptimisticLockConflict on stale version")
	}
}

// ─────────────────────────── ContractStore ───────────────────────────

func TestContractStore_InsertAndFind(t *testing.T) {
	pool := testPool(t)
	nstore := NewNegotiationStore(pool)
	cstore := NewContractStore(pool)
	ctx := context.Background()

	// Negotiation must exist (FK constraint)
	n := sampleNegotiation("test-neg-for-contract")
	n.IsOngoing = false
	if err := nstore.Insert(ctx, n); err != nil {
		t.Fatalf("insert neg: %v", err)
	}

	c := sampleContract("test-contract-001", n.ID)
	if err := cstore.Insert(ctx, c); err != nil {
		t.Fatalf("insert contract: %v", err)
	}
	if c.CreatedAt.IsZero() {
		t.Errorf("CreatedAt not populated")
	}

	got, err := cstore.FindByNegotiationID(ctx, n.ID)
	if err != nil {
		t.Fatalf("FindByNegotiationID: %v", err)
	}
	if got == nil {
		t.Fatal("FindByNegotiationID returned nil")
	}
	if got.StockTicker != "AAPL" {
		t.Errorf("ticker=%q want AAPL", got.StockTicker)
	}
	if !got.StrikeAmount.Equal(decimal.NewFromInt(200)) {
		t.Errorf("strikeAmount=%v want 200", got.StrikeAmount)
	}
}

func TestContractStore_SumActiveBySellerAndTicker(t *testing.T) {
	pool := testPool(t)
	nstore := NewNegotiationStore(pool)
	cstore := NewContractStore(pool)
	ctx := context.Background()

	// Insert 2 active contracts for same seller+ticker
	for i, id := range []string{"test-neg-sum-1", "test-neg-sum-2"} {
		n := sampleNegotiation(id)
		n.IsOngoing = false
		if err := nstore.Insert(ctx, n); err != nil {
			t.Fatalf("insert neg %d: %v", i, err)
		}
		c := sampleContract(fmt.Sprintf("contract-sum-%d", i), id)
		c.Amount = 10
		c.Status = ContractStatusActive
		if err := cstore.Insert(ctx, c); err != nil {
			t.Fatalf("insert contract %d: %v", i, err)
		}
	}
	// Insert one EXERCISED contract — should not be counted
	n3 := sampleNegotiation("test-neg-sum-3")
	n3.IsOngoing = false
	if err := nstore.Insert(ctx, n3); err != nil {
		t.Fatalf("insert neg 3: %v", err)
	}
	c3 := sampleContract("contract-sum-2", n3.ID)
	c3.Amount = 5
	c3.Status = ContractStatusExercised
	if err := cstore.Insert(ctx, c3); err != nil {
		t.Fatalf("insert contract 3: %v", err)
	}

	sum, err := cstore.SumActiveBySellerAndTicker(ctx, 111, "C-5", "AAPL")
	if err != nil {
		t.Fatalf("SumActiveBySellerAndTicker: %v", err)
	}
	if sum != 20 {
		t.Errorf("sum=%d want 20 (2×10 active, 5 exercised excluded)", sum)
	}
}

func TestContractStore_UpdateStatus(t *testing.T) {
	pool := testPool(t)
	nstore := NewNegotiationStore(pool)
	cstore := NewContractStore(pool)
	ctx := context.Background()

	n := sampleNegotiation("test-neg-status")
	n.IsOngoing = false
	if err := nstore.Insert(ctx, n); err != nil {
		t.Fatalf("insert neg: %v", err)
	}
	c := sampleContract("contract-status-001", n.ID)
	if err := cstore.Insert(ctx, c); err != nil {
		t.Fatalf("insert contract: %v", err)
	}

	if err := cstore.UpdateStatus(ctx, c.ID, ContractStatusExercised); err != nil {
		t.Fatalf("UpdateStatus→EXERCISED: %v", err)
	}
	got, err := cstore.FindByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Status != ContractStatusExercised {
		t.Errorf("status=%q want EXERCISED", got.Status)
	}
	if got.ExercisedAt == nil {
		t.Errorf("exercised_at should be set after EXERCISED transition")
	}
}

// ─────────────────────────── TransactionStore ───────────────────────────

func TestTransactionStore_PrepareAndCommit(t *testing.T) {
	pool := testPool(t)
	s := NewTransactionStore(pool)
	ctx := context.Background()

	localID := fmt.Sprintf("tx-%d", time.Now().UnixNano())
	tx := &Transaction{
		TransactionIdRouting: 222,
		TransactionIdLocal:   localID,
		PostingsJSON:         `[{"account":{"type":"ACCOUNT","accountNumber":"111000000000000001"},"amount":"-100","asset":{"type":"MONAS","asset":{"currency":"USD"}}}]`,
		ReservationRefs: []ReservationRef{
			{Kind: RefKindMonas, ReservationID: "res-001"},
		},
		MessageMeta: `{"message":"test prepare","callNumber":"","paymentCode":"289","paymentPurpose":"transfer"}`,
	}
	if err := s.PersistPrepared(ctx, tx); err != nil {
		t.Fatalf("PersistPrepared: %v", err)
	}
	if tx.ID == 0 {
		t.Errorf("ID not populated after PersistPrepared")
	}
	if tx.CreatedAt.IsZero() {
		t.Errorf("CreatedAt not populated")
	}

	got, err := s.FindByID(ctx, 222, localID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got == nil {
		t.Fatal("FindByID returned nil, want row")
	}
	if got.Status != TxStatusPrepared {
		t.Errorf("status=%q want PREPARED", got.Status)
	}
	if len(got.ReservationRefs) != 1 {
		t.Errorf("refs len=%d want 1", len(got.ReservationRefs))
	} else if got.ReservationRefs[0].ReservationID != "res-001" {
		t.Errorf("reservationId=%q want res-001", got.ReservationRefs[0].ReservationID)
	}
	if got.FinalizedAt != nil {
		t.Errorf("FinalizedAt should be nil before commit")
	}

	if err := s.UpdateStatus(ctx, 222, localID, TxStatusCommitted); err != nil {
		t.Fatalf("UpdateStatus→COMMITTED: %v", err)
	}

	after, err := s.FindByID(ctx, 222, localID)
	if err != nil {
		t.Fatalf("FindByID after commit: %v", err)
	}
	if after.Status != TxStatusCommitted {
		t.Errorf("status=%q want COMMITTED", after.Status)
	}
	if after.FinalizedAt == nil {
		t.Errorf("FinalizedAt not set after COMMITTED transition")
	}
}

func TestTransactionStore_FindByID_Miss(t *testing.T) {
	pool := testPool(t)
	s := NewTransactionStore(pool)
	got, err := s.FindByID(context.Background(), 999, "nonexistent-tx")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for miss, got %+v", got)
	}
}

func TestTransactionStore_UniqueViolation(t *testing.T) {
	pool := testPool(t)
	s := NewTransactionStore(pool)
	ctx := context.Background()

	localID := fmt.Sprintf("tx-dup-%d", time.Now().UnixNano())
	tx := &Transaction{
		TransactionIdRouting: 222,
		TransactionIdLocal:   localID,
		PostingsJSON:         `[]`,
		MessageMeta:          `{}`,
	}
	if err := s.PersistPrepared(ctx, tx); err != nil {
		t.Fatalf("first PersistPrepared: %v", err)
	}
	tx.ID = 0 // reset so it retries insert
	err := s.PersistPrepared(ctx, tx)
	if !IsUniqueViolation(err) {
		t.Errorf("expected unique violation (23505) on duplicate, got %v", err)
	}
}

// ─────────────────────────── Helpers ───────────────────────────

func sampleNegotiation(id string) *Negotiation {
	return &Negotiation{
		ID:                    id,
		BuyerRouting:          222,
		BuyerID:               "C-2",
		SellerRouting:         111,
		SellerID:              "C-5",
		StockTicker:           "AAPL",
		Amount:                10,
		PriceCurrency:         "USD",
		PriceAmount:           decimal.RequireFromString("200.00"),
		PremiumCurrency:       "USD",
		PremiumAmount:         decimal.RequireFromString("500.00"),
		SettlementDate:        time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		LastModifiedByRouting: 222,
		LastModifiedByID:      "C-2",
		IsOngoing:             true,
		IsAuthoritative:       true,
	}
}

func sampleContract(id, negotiationID string) *Contract {
	return &Contract{
		ID:                       id,
		NegotiationID:            negotiationID,
		BuyerRouting:             222,
		BuyerID:                  "C-2",
		SellerRouting:            111,
		SellerID:                 "C-5",
		StockTicker:              "AAPL",
		Amount:                   10,
		StrikeCurrency:           "USD",
		StrikeAmount:             decimal.NewFromInt(200),
		SettlementDate:           time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		Status:                   ContractStatusActive,
		OptionPseudoOwnerRouting: 222,
		OptionPseudoOwnerID:      "C-2",
	}
}

func intPtr(i int) *int { return &i }
