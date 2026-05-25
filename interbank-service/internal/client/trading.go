package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
	"github.com/raf-si-2025/banka-1-go/shared/auth"
)

// PublicStockEntry represents one row from GET /internal/interbank/public-stocks.
type PublicStockEntry struct {
	Ticker   string      `json:"ticker"`
	Sellers  []SellerRef `json:"sellers"`
	Quantity int         `json:"quantity"`
}

// SellerRef identifies one seller of a public stock.
type SellerRef struct {
	RoutingNumber int    `json:"routingNumber"`
	ID            string `json:"id"`
}

// TradingClient calls the Java trading-service internal endpoints.
// All methods honour context cancellation and deadlines.
type TradingClient struct {
	baseURL string
	issuer  *auth.S2SIssuer
	hc      *http.Client
}

// NewTradingClient constructs a client with a per-request backstop timeout.
func NewTradingClient(baseURL string, issuer *auth.S2SIssuer, timeout time.Duration) *TradingClient {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &TradingClient{
		baseURL: baseURL,
		issuer:  issuer,
		hc:      &http.Client{Timeout: timeout},
	}
}

// GetPublicStocks returns all publicly-listed stocks with their sellers.
func (c *TradingClient) GetPublicStocks(ctx context.Context) ([]PublicStockEntry, error) {
	u := c.baseURL + "/internal/interbank/public-stocks"
	var out []PublicStockEntry
	if err := c.do(ctx, http.MethodGet, u, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ReserveStock places a stock reservation for a pending inter-bank transaction.
// Returns the reservation UUID; idempotent per (txIDRouting, txIDLocal) on the server.
func (c *TradingClient) ReserveStock(ctx context.Context, sellerUserID int64, ticker string, quantity int, txIDRouting int, txIDLocal string) (string, error) {
	body := map[string]any{
		"sellerUserId": sellerUserID,
		"ticker":       ticker,
		"quantity":     quantity,
		"txIdRouting":  txIDRouting,
		"txIdLocal":    txIDLocal,
	}
	var resp struct {
		ReservationID string `json:"reservationId"`
	}
	if err := c.do(ctx, http.MethodPost, c.baseURL+"/internal/interbank/reserve-stock", body, &resp); err != nil {
		return "", err
	}
	return resp.ReservationID, nil
}

// CommitStock permanently transfers the reserved stock. Returns nil on 204.
func (c *TradingClient) CommitStock(ctx context.Context, reservationID string) error {
	u := c.baseURL + "/internal/interbank/reservations/" + url.PathEscape(reservationID) + "/commit-stock"
	return c.do(ctx, http.MethodPost, u, nil, nil)
}

// ReleaseStock frees the stock reservation. Returns nil on 204.
func (c *TradingClient) ReleaseStock(ctx context.Context, reservationID string) error {
	u := c.baseURL + "/internal/interbank/reservations/" + url.PathEscape(reservationID)
	return c.do(ctx, http.MethodDelete, u, nil, nil)
}

// ReserveOption marks an option contract as reserved (idempotent per spec §3.6).
// negotiationID.Id is URL-path-escaped to support ids with hyphens (e.g. "neg-handshake-s9").
// Only our own negotiations are targeted; routing is implicit (trading-service holds our data).
func (c *TradingClient) ReserveOption(ctx context.Context, negotiationID protocol.ForeignBankId, sellerForeignID, ticker string, quantity int) error {
	u := c.baseURL + "/internal/interbank/options/" + url.PathEscape(negotiationID.Id) + "/reserve"
	body := map[string]any{
		"sellerForeignId": sellerForeignID,
		"ticker":          ticker,
		"quantity":        quantity,
	}
	return c.do(ctx, http.MethodPost, u, body, nil)
}

// ExerciseOption marks an option contract as exercised (idempotent). Returns nil on 204.
func (c *TradingClient) ExerciseOption(ctx context.Context, negotiationID protocol.ForeignBankId) error {
	u := c.baseURL + "/internal/interbank/options/" + url.PathEscape(negotiationID.Id) + "/exercise"
	return c.do(ctx, http.MethodPost, u, nil, nil)
}

// ReleaseOption frees an option reservation (idempotent). Returns nil on 204.
func (c *TradingClient) ReleaseOption(ctx context.Context, negotiationID protocol.ForeignBankId) error {
	u := c.baseURL + "/internal/interbank/options/" + url.PathEscape(negotiationID.Id) + "/release"
	return c.do(ctx, http.MethodDelete, u, nil, nil)
}

func (c *TradingClient) do(ctx context.Context, method, rawURL string, body, out any) error {
	req, err := buildRequest(ctx, method, rawURL, body)
	if err != nil {
		return err
	}
	tok, err := c.issuer.IssueToken()
	if err != nil {
		return fmt.Errorf("client: issue S2S token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	return execRequest(c.hc, req, out)
}
