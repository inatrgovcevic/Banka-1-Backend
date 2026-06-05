package clients

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

// routeDoer routes requests to a handler keyed by "METHOD path" (no query).
type routeDoer struct {
	fn func(req *http.Request) (*http.Response, error)
}

func (d routeDoer) Do(req *http.Request) (*http.Response, error) { return d.fn(req) }

func jsonResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

// captureDoer returns a fixed response and records the last request. A fresh
// response body is built per call so multi-call methods don't read a drained body.
type captureDoer struct {
	resp    *http.Response
	err     error
	lastReq *http.Request
	body    string
}

func (d *captureDoer) Do(req *http.Request) (*http.Response, error) {
	d.lastReq = req
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		d.body = string(b)
	}
	if d.err != nil {
		return nil, d.err
	}
	body, _ := io.ReadAll(d.resp.Body)
	d.resp.Body = io.NopCloser(strings.NewReader(string(body)))
	return &http.Response{
		StatusCode: d.resp.StatusCode,
		Body:       io.NopCloser(strings.NewReader(string(body))),
		Header:     d.resp.Header,
	}, nil
}

// ---- MarketClient ----------------------------------------------------------

func TestMarketClient_GetListing(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `{"listingId":5,"ticker":"AAPL"}`)}
	c := NewMarketClient("http://m", nil, d)
	l, err := c.GetListing(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if l.ID != 5 || l.Ticker == nil || *l.Ticker != "AAPL" {
		t.Errorf("listing wrong: %+v", l)
	}
	if !strings.Contains(d.lastReq.URL.Path, "/api/listings/5") {
		t.Errorf("path = %s", d.lastReq.URL.Path)
	}
}

func TestMarketClient_GetListing_Error(t *testing.T) {
	d := &captureDoer{resp: jsonResp(500, `err`)}
	c := NewMarketClient("http://m", nil, d)
	if _, err := c.GetListing(context.Background(), 5); err == nil {
		t.Error("expected 500 error")
	}
}

func TestMarketClient_Calculate(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `{"convertedAmount":"120.5"}`)}
	c := NewMarketClient("http://m", nil, d)
	r, err := c.Calculate(context.Background(), "USD", "RSD", dec("100"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Converted() == nil || !r.Converted().Equal(dec("120.5")) {
		t.Errorf("rate wrong: %+v", r)
	}
}

func TestMarketClient_CalculateWithoutCommission(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `{"toAmount":"99"}`)}
	c := NewMarketClient("http://m", nil, d)
	r, err := c.CalculateWithoutCommission(context.Background(), "USD", "RSD", dec("100"))
	if err != nil || r.Converted() == nil {
		t.Fatalf("err=%v r=%+v", err, r)
	}
}

func TestMarketClient_GetExchangeStatus(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `{}`)}
	c := NewMarketClient("http://m", nil, d)
	if _, err := c.GetExchangeStatus(context.Background(), 3); err != nil {
		t.Fatal(err)
	}
}

func TestMarketClient_RefreshListing_SwallowsError(t *testing.T) {
	d := &captureDoer{resp: jsonResp(500, `boom`)}
	c := NewMarketClient("http://m", nil, d)
	c.RefreshListing(context.Background(), 5) // must not panic
}

func TestMarketClient_FetchSnapshots(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `[{"ticker":"AAPL","currentPrice":"190.5"},{"ticker":"","currentPrice":"1"},{"ticker":"AAPL","currentPrice":"200"}]`)}
	c := NewMarketClient("http://m", nil, d)
	out := c.FetchSnapshots(context.Background(), []string{"AAPL"})
	if len(out) != 1 {
		t.Errorf("expected 1 (dedup + skip blank ticker), got %d", len(out))
	}
}

func TestMarketClient_FetchSnapshots_Empty(t *testing.T) {
	c := NewMarketClient("http://m", nil, &captureDoer{resp: jsonResp(200, `[]`)})
	if len(c.FetchSnapshots(context.Background(), nil)) != 0 {
		t.Error("empty tickers -> empty map")
	}
}

func TestMarketClient_FetchSnapshots_UpstreamError(t *testing.T) {
	c := NewMarketClient("http://m", nil, &captureDoer{resp: jsonResp(500, `x`)})
	if len(c.FetchSnapshots(context.Background(), []string{"AAPL"})) != 0 {
		t.Error("upstream error -> empty map")
	}
}

func TestMarketClient_CurrentPrices(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `[{"ticker":"AAPL","currentPrice":"190.5"}]`)}
	c := NewMarketClient("http://m", nil, d)
	out := c.CurrentPrices(context.Background(), []string{"AAPL"})
	if !out["AAPL"].Equal(dec("190.5")) {
		t.Errorf("prices = %v", out)
	}
}

func TestMarketClient_CurrentPrice(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `[{"ticker":"AAPL","currentPrice":"190.5"}]`)}
	c := NewMarketClient("http://m", nil, d)
	p, ok := c.CurrentPrice(context.Background(), "AAPL")
	if !ok || !p.Equal(dec("190.5")) {
		t.Errorf("price = %v ok=%v", p, ok)
	}
	if _, ok := c.CurrentPrice(context.Background(), "MSFT"); ok {
		t.Error("missing ticker should be !ok")
	}
}

func TestMarketClient_ConvertNoCommission_SameCurrency(t *testing.T) {
	c := NewMarketClient("http://m", nil, &captureDoer{resp: jsonResp(200, `{}`)})
	got, ok := c.ConvertNoCommission(context.Background(), dec("100"), "USD", "USD")
	if !ok || !got.Equal(dec("100")) {
		t.Errorf("same currency should pass through: %v %v", got, ok)
	}
}

func TestMarketClient_ConvertNoCommission_Converts(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `{"convertedAmount":"11700"}`)}
	c := NewMarketClient("http://m", nil, d)
	got, ok := c.ConvertNoCommission(context.Background(), dec("100"), "USD", "RSD")
	if !ok || !got.Equal(dec("11700")) {
		t.Errorf("convert = %v ok=%v", got, ok)
	}
}

func TestMarketClient_ConvertNoCommission_Failure(t *testing.T) {
	c := NewMarketClient("http://m", nil, &captureDoer{resp: jsonResp(500, `x`)})
	if _, ok := c.ConvertNoCommission(context.Background(), dec("100"), "USD", "RSD"); ok {
		t.Error("upstream failure should be !ok")
	}
}

func TestMarketClient_FetchDividendData(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `[{"listingId":1,"ticker":"AAPL"}]`)}
	c := NewMarketClient("http://m", nil, d)
	out := c.FetchDividendData(context.Background())
	if len(out) != 1 || out[0].Ticker != "AAPL" {
		t.Errorf("dividend data wrong: %+v", out)
	}
}

func TestMarketClient_FetchDividendData_Error(t *testing.T) {
	c := NewMarketClient("http://m", nil, &captureDoer{resp: jsonResp(500, `x`)})
	if len(c.FetchDividendData(context.Background())) != 0 {
		t.Error("error -> empty slice")
	}
}

// ---- CustomerClient --------------------------------------------------------

func TestCustomerClient_GetCustomer(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `{"id":10,"firstName":"Jo"}`)}
	c := NewCustomerClient("http://u", nil, d)
	cust, err := c.GetCustomer(context.Background(), 10)
	if err != nil || cust.ID != 10 {
		t.Fatalf("err=%v cust=%+v", err, cust)
	}
}

func TestCustomerClient_GetCustomer_NotFound(t *testing.T) {
	c := NewCustomerClient("http://u", nil, &captureDoer{resp: jsonResp(404, ``)})
	if _, err := c.GetCustomer(context.Background(), 10); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCustomerClient_SearchCustomers(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `{"content":[],"totalPages":0,"totalElements":0}`)}
	c := NewCustomerClient("http://u", nil, d)
	ime := "Jo"
	if _, err := c.SearchCustomers(context.Background(), &ime, nil, 0, 20); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.lastReq.URL.RawQuery, "ime=Jo") {
		t.Errorf("query = %s", d.lastReq.URL.RawQuery)
	}
}

// ---- EmployeeClient --------------------------------------------------------

func TestEmployeeClient_GetEmployee(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `{"id":7}`)}
	c := NewEmployeeClient("http://u", nil, d)
	if _, err := c.GetEmployee(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
}

func TestEmployeeClient_SearchEmployees(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `{"content":[]}`)}
	c := NewEmployeeClient("http://u", nil, d)
	email := "a@b.com"
	if _, err := c.SearchEmployees(context.Background(), &email, nil, nil, nil, 0, 10); err != nil {
		t.Fatal(err)
	}
}

func TestEmployeeClient_ActuaryClientIDs(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `[1,2,3]`)}
	c := NewEmployeeClient("http://u", nil, d)
	ids := c.ActuaryClientIDs(context.Background())
	if len(ids) != 3 {
		t.Errorf("ids = %v", ids)
	}
}

func TestEmployeeClient_ActuaryClientIDs_Error(t *testing.T) {
	c := NewEmployeeClient("http://u", nil, &captureDoer{resp: jsonResp(500, `x`)})
	if len(c.ActuaryClientIDs(context.Background())) != 0 {
		t.Error("error -> empty slice")
	}
}

// ---- AccountClient ---------------------------------------------------------

func TestAccountClient_GetAccountDetailsByNumber(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `{}`)}
	c := NewAccountClient("http://b", nil, d)
	if _, err := c.GetAccountDetailsByNumber(context.Background(), "123"); err != nil {
		t.Fatal(err)
	}
}

func TestAccountClient_GetAccountDetailsByID_LongTreatedAsNumber(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `{}`)}
	c := NewAccountClient("http://b", nil, d)
	if _, err := c.GetAccountDetailsByID(context.Background(), 123456789012345678); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.lastReq.URL.Path, "/details") {
		t.Errorf("path = %s", d.lastReq.URL.Path)
	}
}

func TestAccountClient_GetAccountDetailsByID_FallbackOn404(t *testing.T) {
	calls := 0
	d := routeDoer{fn: func(req *http.Request) (*http.Response, error) {
		calls++
		if strings.Contains(req.URL.Path, "/id/") {
			return jsonResp(404, ``), nil
		}
		return jsonResp(200, `{}`), nil
	}}
	c := NewAccountClient("http://b", nil, d)
	if _, err := c.GetAccountDetailsByID(context.Background(), 42); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("expected fallback (2 calls), got %d", calls)
	}
}

func TestAccountClient_GetGovernmentBankAccountRsd(t *testing.T) {
	c := NewAccountClient("http://b", nil, &captureDoer{resp: jsonResp(200, `{}`)})
	if _, err := c.GetGovernmentBankAccountRsd(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestAccountClient_GetBankAccount(t *testing.T) {
	c := NewAccountClient("http://b", nil, &captureDoer{resp: jsonResp(200, `{}`)})
	if _, err := c.GetBankAccount(context.Background(), "RSD"); err != nil {
		t.Fatal(err)
	}
}

func TestAccountClient_PostMethods(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `{}`)}
	c := NewAccountClient("http://b", nil, d)
	ctx := context.Background()
	if err := c.Transaction(ctx, Payment{}); err != nil {
		t.Fatal(err)
	}
	if err := c.Transfer(ctx, Payment{}); err != nil {
		t.Fatal(err)
	}
	if err := c.ExchangeBuy(ctx, OneSidedTransaction{}); err != nil {
		t.Fatal(err)
	}
	if err := c.ExchangeSell(ctx, OneSidedTransaction{}); err != nil {
		t.Fatal(err)
	}
	if err := c.StockBuyMarginTransaction(ctx, 1, dec("10")); err != nil {
		t.Fatal(err)
	}
	if err := c.StockSellMarginTransaction(ctx, 1, dec("10")); err != nil {
		t.Fatal(err)
	}
	if err := c.CreditAccount(ctx, "123", dec("10"), 5); err != nil {
		t.Fatal(err)
	}
	if err := c.DebitAccount(ctx, "123", dec("10"), 5); err != nil {
		t.Fatal(err)
	}
}

func TestAccountClient_CreateSystemAccount(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `{"id":1,"accountNumber":"123"}`)}
	c := NewAccountClient("http://b", nil, d)
	out, err := c.CreateSystemAccount(context.Background(), "123", 5, "RSD", "Fund", dec("0"))
	if err != nil || out.ID != 1 {
		t.Fatalf("err=%v out=%+v", err, out)
	}
}

func TestAccountClient_GetDefaultRsdAccountNumberForOwner(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `{"accountNumber":"999"}`)}
	c := NewAccountClient("http://b", nil, d)
	if got := c.GetDefaultRsdAccountNumberForOwner(context.Background(), 5); got != "999" {
		t.Errorf("got %q", got)
	}
}

func TestAccountClient_GetDefaultRsdAccountNumberForOwner_Error(t *testing.T) {
	c := NewAccountClient("http://b", nil, &captureDoer{resp: jsonResp(500, `x`)})
	if got := c.GetDefaultRsdAccountNumberForOwner(context.Background(), 5); got != "" {
		t.Errorf("error should give empty, got %q", got)
	}
}

func TestAccountClient_GetAccountInCurrency(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `{"accountNumber":"abc","ownerId":5}`)}
	c := NewAccountClient("http://b", nil, d)
	a := c.GetAccountInCurrency(context.Background(), 5, "RSD")
	if a == nil || a.Number() != "abc" {
		t.Errorf("account wrong: %+v", a)
	}
}

func TestAccountClient_GetAccountInCurrency_NilOnEmpty(t *testing.T) {
	c := NewAccountClient("http://b", nil, &captureDoer{resp: jsonResp(200, `{}`)})
	if c.GetAccountInCurrency(context.Background(), 5, "RSD") != nil {
		t.Error("missing accountNumber -> nil")
	}
}

func TestAccountClient_GetAccountInCurrency_NilOnError(t *testing.T) {
	c := NewAccountClient("http://b", nil, &captureDoer{resp: jsonResp(404, ``)})
	if c.GetAccountInCurrency(context.Background(), 5, "RSD") != nil {
		t.Error("404 -> nil")
	}
}

func TestAccountClient_OwnerAccounts(t *testing.T) {
	d := &captureDoer{resp: jsonResp(200, `{"accountNumber":"st"}`)}
	c := NewAccountClient("http://b", nil, d)
	if c.GetStateRsdOwnerAccount(context.Background()) == nil {
		t.Error("state account nil")
	}
	if c.GetBankRsdOwnerAccount(context.Background()) == nil {
		t.Error("bank account nil")
	}
}

func TestAccountClient_OwnerAccount_NilOnError(t *testing.T) {
	c := NewAccountClient("http://b", nil, &captureDoer{resp: jsonResp(500, `x`)})
	if c.GetStateRsdOwnerAccount(context.Background()) != nil {
		t.Error("error -> nil")
	}
}

// ---- transport error path --------------------------------------------------

func TestDoJSON_TransportError(t *testing.T) {
	c := NewMarketClient("http://m", nil, &captureDoer{err: errors.New("dial")})
	if _, err := c.GetListing(context.Background(), 1); err == nil {
		t.Error("transport error should propagate")
	}
}

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }
