package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/shared/auth"
)

// TradingServiceClient calls the trading-service internal endpoints used by
// OTC exercise and fund liquidation saga handlers.
//
// Endpoints:
//   - POST /stocks/internal/reserve                         → ReserveStocks
//   - DELETE /stocks/internal/reservations/{id}            → ReleaseStocks
//   - POST /stocks/internal/reservations/{id}/transfer     → TransferOwnership
//   - POST /stocks/internal/ownership-transfers/{id}/reverse → ReverseOwnership
//   - POST /funds/internal/{fundId}/liquidate              → LiquidateForFund
type TradingServiceClient struct {
	baseURL string
	issuer  *auth.S2SIssuer
	hc      *http.Client
}

// NewTradingServiceClient builds a client. If timeout <= 0 it defaults to 30s.
func NewTradingServiceClient(baseURL string, issuer *auth.S2SIssuer, timeout time.Duration) *TradingServiceClient {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &TradingServiceClient{
		baseURL: baseURL,
		issuer:  issuer,
		hc:      &http.Client{Timeout: timeout},
	}
}

// StockReservationResult is the response from ReserveStocks.
type StockReservationResult struct {
	ReservationID string `json:"reservationId"`
	Status        string `json:"status"`
}

// OwnershipTransferResult is the response from TransferOwnership.
type OwnershipTransferResult struct {
	OwnershipTransferID string `json:"ownershipTransferId"`
	Status              string `json:"status"`
}

// LiquidationResult is the response from LiquidateForFund.
type LiquidationResult struct {
	LiquidationID    string          `json:"liquidationId"`
	LiquidatedAmount decimal.Decimal `json:"liquidatedAmount"`
	HoldingsSold     int             `json:"holdingsSold"`
}

// ReserveStocks checks that the seller owns at least amount of stockTicker and
// creates a reservation. Returns the reservationID.
func (c *TradingServiceClient) ReserveStocks(
	ctx context.Context, ownerID int64, ticker string, amount int, correlationID string,
) (string, error) {
	body := map[string]any{
		"ownerId":     ownerID,
		"stockTicker": ticker,
		"amount":      amount,
	}
	var result StockReservationResult
	if err := c.do(ctx, http.MethodPost,
		c.baseURL+"/stocks/internal/reserve",
		correlationID, body, &result,
	); err != nil {
		return "", fmt.Errorf("trading.ReserveStocks: %w", err)
	}
	return result.ReservationID, nil
}

// ReleaseStocks frees a stock reservation (compensation for OtcExercise step 2).
func (c *TradingServiceClient) ReleaseStocks(ctx context.Context, reservationID, correlationID string) error {
	u := c.baseURL + "/stocks/internal/reservations/" + url.PathEscape(reservationID)
	if err := c.do(ctx, http.MethodDelete, u, correlationID, nil, nil); err != nil {
		return fmt.Errorf("trading.ReleaseStocks(%s): %w", reservationID, err)
	}
	return nil
}

// TransferOwnership completes the stock transfer from the seller's reservation
// to the buyer. Returns the ownershipTransferID for potential reversal.
func (c *TradingServiceClient) TransferOwnership(
	ctx context.Context, reservationID string, buyerID int64, correlationID string,
) (string, error) {
	body := map[string]any{"buyerId": buyerID}
	var result OwnershipTransferResult
	u := c.baseURL + "/stocks/internal/reservations/" + url.PathEscape(reservationID) + "/transfer"
	if err := c.do(ctx, http.MethodPost, u, correlationID, body, &result); err != nil {
		return "", fmt.Errorf("trading.TransferOwnership(%s): %w", reservationID, err)
	}
	return result.OwnershipTransferID, nil
}

// ReverseOwnership reverses a completed ownership transfer (compensation for
// OtcExercise step 4).
func (c *TradingServiceClient) ReverseOwnership(ctx context.Context, ownershipTransferID, correlationID string) error {
	u := c.baseURL + "/stocks/internal/ownership-transfers/" + url.PathEscape(ownershipTransferID) + "/reverse"
	if err := c.do(ctx, http.MethodPost, u, correlationID, nil, nil); err != nil {
		return fmt.Errorf("trading.ReverseOwnership(%s): %w", ownershipTransferID, err)
	}
	return nil
}

// LiquidateForFund instructs the trading-service to sell enough fund holdings
// to cover targetAmount (in RSD). Used by FundRedeemWithLiquidationSaga step 1.
// Returns the liquidationID string (satisfies saga.TradingActions interface).
func (c *TradingServiceClient) LiquidateForFund(
	ctx context.Context, fundID int64, targetAmount decimal.Decimal, correlationID string,
) (string, error) {
	body := map[string]any{"targetAmount": targetAmount.String()}
	var result LiquidationResult
	u := fmt.Sprintf("%s/funds/internal/%d/liquidate", c.baseURL, fundID)
	if err := c.do(ctx, http.MethodPost, u, correlationID, body, &result); err != nil {
		return "", fmt.Errorf("trading.LiquidateForFund(fund=%d): %w", fundID, err)
	}
	return result.LiquidationID, nil
}

func (c *TradingServiceClient) do(
	ctx context.Context,
	method, rawURL, correlationID string,
	body, out any,
) error {
	req, err := buildReq(ctx, method, rawURL, body)
	if err != nil {
		return err
	}
	tok, err := c.issuer.IssueToken()
	if err != nil {
		return fmt.Errorf("issue S2S token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if correlationID != "" {
		req.Header.Set("X-Correlation-Id", correlationID)
	}
	return execReq(c.hc, req, out)
}
