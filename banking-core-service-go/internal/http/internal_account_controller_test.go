package http

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"banka1/banking-core-service-go/internal/config"
	"banka1/banking-core-service-go/internal/service"
)

func TestInternalDefaultAccountReturnsRSDAccountNumber(t *testing.T) {
	h, cleanup := internalAccountTestHandler(t, []internalAccountFixture{
		{id: 701, ownerID: 7, currency: "RSD", accountNumber: "111000110000000701"},
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/accounts/internal/default/7", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}
	var got struct {
		OwnerID       string `json:"ownerId"`
		AccountNumber string `json:"accountNumber"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.OwnerID != "7" || got.AccountNumber != "111000110000000701" {
		t.Fatalf("response = %+v, want ownerId=7 accountNumber=111000110000000701", got)
	}
}

func TestInternalAccountByOwnerAndCurrencyReturnsAccountWhenPresent(t *testing.T) {
	h, cleanup := internalAccountTestHandler(t, []internalAccountFixture{
		{id: 801, ownerID: 7, currency: "USD", accountNumber: "111000110000000801"},
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/accounts/internal/by-owner/7/currency/USD", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}
	var got struct {
		ID            int64  `json:"id"`
		AccountNumber string `json:"accountNumber"`
		Currency      string `json:"currency"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.ID != 801 || got.AccountNumber != "111000110000000801" || got.Currency != "USD" {
		t.Fatalf("response = %+v, want USD account payload", got)
	}
}

func TestInternalAccountByOwnerAndCurrencyReturns404WhenMissing(t *testing.T) {
	h, cleanup := internalAccountTestHandler(t, nil)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/accounts/internal/by-owner/7/currency/USD", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404\nbody: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("body length = %d, want empty 404 body: %q", rec.Body.Len(), rec.Body.String())
	}
}

type internalAccountFixture struct {
	id            int64
	ownerID       int64
	currency      string
	accountNumber string
}

func internalAccountTestHandler(t *testing.T, fixtures []internalAccountFixture) (*Handler, func()) {
	t.Helper()
	db, dsn := openInternalAccountFixtureDB(t, fixtures)
	accounts := service.NewAccountService(db, config.Config{}, nil)
	return &Handler{
			cfg: testAuthConfig(),
			services: &service.Container{
				Accounts: accounts,
			},
		}, func() {
			deleteInternalAccountFixtures(dsn)
			_ = db.Close()
		}
}

const internalAccountFixtureDriverName = "internal-account-fixture"

var (
	internalAccountFixtureDriverOnce sync.Once
	internalAccountFixtureDBSeq      atomic.Int64
	internalAccountFixtureMu         sync.Mutex
	internalAccountFixturesByDSN     = map[string]map[string]internalAccountFixture{}
)

func openInternalAccountFixtureDB(t *testing.T, fixtures []internalAccountFixture) (*sql.DB, string) {
	t.Helper()
	internalAccountFixtureDriverOnce.Do(func() {
		sql.Register(internalAccountFixtureDriverName, internalAccountFixtureDriver{})
	})

	dsn := "internal-account-fixture-" + strconv.FormatInt(internalAccountFixtureDBSeq.Add(1), 10)
	byKey := map[string]internalAccountFixture{}
	for _, fixture := range fixtures {
		byKey[internalAccountFixtureKey(fixture.ownerID, fixture.currency)] = fixture
	}
	internalAccountFixtureMu.Lock()
	internalAccountFixturesByDSN[dsn] = byKey
	internalAccountFixtureMu.Unlock()

	db, err := sql.Open(internalAccountFixtureDriverName, dsn)
	if err != nil {
		t.Fatal(err)
	}
	return db, dsn
}

func deleteInternalAccountFixtures(dsn string) {
	internalAccountFixtureMu.Lock()
	delete(internalAccountFixturesByDSN, dsn)
	internalAccountFixtureMu.Unlock()
}

func internalAccountFixtureKey(ownerID int64, currency string) string {
	return strconv.FormatInt(ownerID, 10) + ":" + currency
}

type internalAccountFixtureDriver struct{}

func (internalAccountFixtureDriver) Open(name string) (driver.Conn, error) {
	return internalAccountFixtureConn{dsn: name}, nil
}

type internalAccountFixtureConn struct {
	dsn string
}

func (c internalAccountFixtureConn) Prepare(string) (driver.Stmt, error) {
	return nil, driver.ErrSkip
}

func (c internalAccountFixtureConn) Close() error {
	internalAccountFixtureMu.Lock()
	delete(internalAccountFixturesByDSN, c.dsn)
	internalAccountFixtureMu.Unlock()
	return nil
}

func (c internalAccountFixtureConn) Begin() (driver.Tx, error) {
	return nil, driver.ErrSkip
}

func (c internalAccountFixtureConn) QueryContext(_ context.Context, _ string, args []driver.NamedValue) (driver.Rows, error) {
	var ownerID int64
	var currency string
	if len(args) >= 2 {
		if v, ok := args[0].Value.(int64); ok {
			ownerID = v
		}
		if v, ok := args[1].Value.(string); ok {
			currency = v
		}
	}

	internalAccountFixtureMu.Lock()
	fixture, ok := internalAccountFixturesByDSN[c.dsn][internalAccountFixtureKey(ownerID, currency)]
	internalAccountFixtureMu.Unlock()
	if !ok {
		return &internalAccountFixtureRows{}, nil
	}
	return &internalAccountFixtureRows{row: fixture, hasRow: true}, nil
}

type internalAccountFixtureRows struct {
	row     internalAccountFixture
	hasRow  bool
	scanned bool
}

func (r *internalAccountFixtureRows) Columns() []string {
	return []string{
		"id", "broj_racuna", "vlasnik", "oznaka", "raspolozivo_stanje",
		"stanje", "status", "account_type", "email", "username",
		"dnevni_limit", "mesecni_limit", "dnevna_potrosnja", "mesecna_potrosnja",
		"has_daily_limit", "has_monthly_limit", "datum_isteka",
		"daily_limit_remaining", "has_daily_limit_remaining",
	}
}

func (r *internalAccountFixtureRows) Close() error {
	return nil
}

func (r *internalAccountFixtureRows) Next(dest []driver.Value) error {
	if !r.hasRow || r.scanned {
		return io.EOF
	}
	r.scanned = true
	values := []driver.Value{
		r.row.id,
		r.row.accountNumber,
		r.row.ownerID,
		r.row.currency,
		"0",
		"0",
		"ACTIVE",
		"CHECKING",
		"",
		"",
		"0",
		"0",
		"0",
		"0",
		false,
		false,
		nil,
		"0",
		false,
	}
	copy(dest, values)
	return nil
}

var _ driver.QueryerContext = internalAccountFixtureConn{}
