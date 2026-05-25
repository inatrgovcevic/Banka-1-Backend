// Package client contains REST clients for the downstream Java services called
// by saga handlers. All clients use a shared S2SIssuer for authorization and
// stdlib net/http for transport (no external HTTP frameworks).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/shared/auth"
)

// Sentinel errors.
var (
	ErrNotFound = errors.New("client: not found (404)")
	ErrUpstream = errors.New("client: upstream error")
)

// BankingCoreClient calls the banking-core-service internal endpoints used by
// saga handlers. It is the Go equivalent of the Java BankingCoreClient.
//
// Endpoints:
//   - POST /transactions/internal/reserve-funds       → ReserveFunds
//   - DELETE /transactions/internal/reservations/{id} → ReleaseFunds
//   - POST /transactions/internal/reservations/{id}/commit → CommitReservation
//   - POST /transactions/internal/transfer            → InternalTransfer
//   - POST /transactions/internal/transfers/{id}/reverse → ReverseTransfer
//   - GET  /accounts/internal/default/{ownerId}       → ResolveDefaultAccountNumber
type BankingCoreClient struct {
	baseURL string
	issuer  *auth.S2SIssuer
	hc      *http.Client
}

// NewBankingCoreClient builds a client. If timeout <= 0 it defaults to 10s.
func NewBankingCoreClient(baseURL string, issuer *auth.S2SIssuer, timeout time.Duration) *BankingCoreClient {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &BankingCoreClient{
		baseURL: baseURL,
		issuer:  issuer,
		hc:      &http.Client{Timeout: timeout},
	}
}

// ReservationResult is returned by ReserveFunds.
type ReservationResult struct {
	ReservationID string `json:"reservationId"`
	Status        string `json:"status"`
}

// TransferResult is returned by InternalTransfer.
type TransferResult struct {
	TransferID string `json:"transferId"`
	Status     string `json:"status"`
}

// ReserveFunds debits the owner's account by amount and creates a HELD
// reservation. Returns the reservation ID for later commit or release.
// correlationID is forwarded as X-Correlation-Id header.
func (c *BankingCoreClient) ReserveFunds(
	ctx context.Context, ownerID int64, amount decimal.Decimal, correlationID string,
) (string, error) {
	body := map[string]any{"ownerId": ownerID, "amount": amount.String()}
	var result ReservationResult
	if err := c.do(ctx, http.MethodPost,
		c.baseURL+"/transactions/internal/reserve-funds",
		correlationID, body, &result,
	); err != nil {
		return "", fmt.Errorf("banking-core.ReserveFunds: %w", err)
	}
	return result.ReservationID, nil
}

// ReleaseFunds frees a HELD reservation back to available balance (compensation
// for a failed saga at step 1 or early in step 3).
func (c *BankingCoreClient) ReleaseFunds(ctx context.Context, reservationID, correlationID string) error {
	u := c.baseURL + "/transactions/internal/reservations/" + url.PathEscape(reservationID)
	if err := c.do(ctx, http.MethodDelete, u, correlationID, nil, nil); err != nil {
		return fmt.Errorf("banking-core.ReleaseFunds(%s): %w", reservationID, err)
	}
	return nil
}

// CommitReservation permanently commits a prepared reservation (used in 2PC
// flows; present here for completeness — saga handlers prefer InternalTransfer).
func (c *BankingCoreClient) CommitReservation(ctx context.Context, reservationID, correlationID string) error {
	u := c.baseURL + "/transactions/internal/reservations/" + url.PathEscape(reservationID) + "/commit"
	if err := c.do(ctx, http.MethodPost, u, correlationID, nil, nil); err != nil {
		return fmt.Errorf("banking-core.CommitReservation(%s): %w", reservationID, err)
	}
	return nil
}

// InternalTransfer moves amount from fromAccount to toAccount. Both are 18-digit
// account numbers. Returns the transferID for potential reversal.
func (c *BankingCoreClient) InternalTransfer(
	ctx context.Context,
	fromAccount, toAccount string,
	amount decimal.Decimal,
	correlationID string,
) (string, error) {
	body := map[string]any{
		"fromAccountNumber": fromAccount,
		"toAccountNumber":   toAccount,
		"amount":            amount.String(),
	}
	var result TransferResult
	if err := c.do(ctx, http.MethodPost,
		c.baseURL+"/transactions/internal/transfer",
		correlationID, body, &result,
	); err != nil {
		return "", fmt.Errorf("banking-core.InternalTransfer: %w", err)
	}
	return result.TransferID, nil
}

// ReverseTransfer reverses a previously completed internal transfer (compensation
// for step 3 of OtcExercise).
func (c *BankingCoreClient) ReverseTransfer(ctx context.Context, transferID, correlationID string) error {
	u := c.baseURL + "/transactions/internal/transfers/" + url.PathEscape(transferID) + "/reverse"
	if err := c.do(ctx, http.MethodPost, u, correlationID, nil, nil); err != nil {
		return fmt.Errorf("banking-core.ReverseTransfer(%s): %w", transferID, err)
	}
	return nil
}

// ResolveDefaultAccountNumber returns the default RSD account number for the
// given user or company ID.
func (c *BankingCoreClient) ResolveDefaultAccountNumber(ctx context.Context, ownerID int64) (string, error) {
	u := fmt.Sprintf("%s/accounts/internal/default/%d", c.baseURL, ownerID)
	var result struct {
		AccountNumber string `json:"accountNumber"`
	}
	if err := c.do(ctx, http.MethodGet, u, "", nil, &result); err != nil {
		return "", fmt.Errorf("banking-core.ResolveDefaultAccountNumber(%d): %w", ownerID, err)
	}
	return result.AccountNumber, nil
}

// do is the shared HTTP helper.  It mints an S2S bearer token, sets optional
// X-Correlation-Id header, marshals body (if non-nil), executes the request,
// and unmarshals out (if non-nil).
func (c *BankingCoreClient) do(
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

// ---------------------------------------------------------------------------
// Low-level shared helpers (unexported)
// ---------------------------------------------------------------------------

func buildReq(ctx context.Context, method, rawURL string, body any) (*http.Request, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, r)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func execReq(hc *http.Client, req *http.Request, out any) error {
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: %s %s", ErrNotFound, req.Method, req.URL.Path)
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: status=%d body=%s", ErrUpstream, resp.StatusCode, b)
	}
	if out == nil {
		return nil
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	if len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, out)
}
