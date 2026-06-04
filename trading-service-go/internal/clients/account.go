package clients

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/shopspring/decimal"
)

// AccountClient calls banking-core/account-service (SERVICES_BANKING_CORE_URL).
type AccountClient struct {
	base *baseClient
}

// NewAccountClient builds an AccountClient over baseURL.
func NewAccountClient(baseURL string, tokens *ServiceTokenProvider, doer HTTPDoer) *AccountClient {
	return &AccountClient{base: newBaseClient(baseURL, tokens, doer)}
}

// GetAccountDetailsByNumber mirrors getAccountDetails(String):
// GET /internal/accounts/{accountNumber}/details.
func (c *AccountClient) GetAccountDetailsByNumber(ctx context.Context, accountNumber string) (*AccountDetails, error) {
	var out AccountDetails
	path := "/internal/accounts/" + url.PathEscape(accountNumber) + "/details"
	if err := c.base.doJSON(ctx, http.MethodGet, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAccountDetailsByID mirrors getAccountDetails(Long): treat an 18+ digit id as
// an account number; otherwise resolve by internal id and fall back to number on 404.
func (c *AccountClient) GetAccountDetailsByID(ctx context.Context, accountID int64) (*AccountDetails, error) {
	s := strconv.FormatInt(accountID, 10)
	if len(s) >= 18 {
		return c.GetAccountDetailsByNumber(ctx, s)
	}
	var out AccountDetails
	err := c.base.doJSON(ctx, http.MethodGet, fmt.Sprintf("/internal/accounts/id/%d/details", accountID), nil, nil, &out)
	if errors.Is(err, ErrNotFound) {
		return c.GetAccountDetailsByNumber(ctx, s)
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// GetGovernmentBankAccountRsd mirrors getGovernmentBankAccountRsd:
// GET /internal/accounts/state/RSD (the state's RSD account).
func (c *AccountClient) GetGovernmentBankAccountRsd(ctx context.Context) (*AccountDetails, error) {
	var out AccountDetails
	if err := c.base.doJSON(ctx, http.MethodGet, "/internal/accounts/state/RSD", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetBankAccount mirrors EmployeeClient.getBankAccount (which uses the account
// RestClient): GET /internal/accounts/bank/{currency}.
func (c *AccountClient) GetBankAccount(ctx context.Context, currency string) (*BankAccount, error) {
	var out BankAccount
	if err := c.base.doJSON(ctx, http.MethodGet, "/internal/accounts/bank/"+url.PathEscape(currency), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Transaction mirrors accountClient.transaction(PaymentDto):
// POST /internal/accounts/transaction. The response body is ignored (as in Java).
func (c *AccountClient) Transaction(ctx context.Context, payment Payment) error {
	return c.base.doJSON(ctx, http.MethodPost, "/internal/accounts/transaction", nil, payment, nil)
}

// Transfer mirrors accountClient.transfer(PaymentDto):
// POST /internal/accounts/transfer. Used for same-owner (bank-internal) fee legs
// that the /transaction endpoint rejects. Response body ignored.
func (c *AccountClient) Transfer(ctx context.Context, payment Payment) error {
	return c.base.doJSON(ctx, http.MethodPost, "/internal/accounts/transfer", nil, payment, nil)
}

// ExchangeBuy mirrors accountClient.exchangeBuy(OneSidedTransactionDto):
// POST /internal/accounts/exchange/buy (one-sided debit, GHI #199). Body ignored.
func (c *AccountClient) ExchangeBuy(ctx context.Context, req OneSidedTransaction) error {
	return c.base.doJSON(ctx, http.MethodPost, "/internal/accounts/exchange/buy", nil, req, nil)
}

// StockBuyMarginTransaction calls POST /transactions/stockBuyMarginTransaction.
// Used for margin BUY orders: banking-core debits clientPart from the user's
// margin account initialMargin and loans bankPart from the bank.
func (c *AccountClient) StockBuyMarginTransaction(ctx context.Context, userID int64, amount decimal.Decimal) error {
	body := map[string]any{
		"userId": userID,
		"amount": amount,
	}
	return c.base.doJSON(ctx, http.MethodPost, "/transactions/stockBuyMarginTransaction", nil, body, nil)
}

// StockSellMarginTransaction calls POST /transactions/stockSellMarginTransaction.
// Used for margin SELL orders: banking-core credits back clientPart and reduces
// the loan value by bankPart.
func (c *AccountClient) StockSellMarginTransaction(ctx context.Context, userID int64, amount decimal.Decimal) error {
	body := map[string]any{
		"userId": userID,
		"amount": amount,
	}
	return c.base.doJSON(ctx, http.MethodPost, "/transactions/stockSellMarginTransaction", nil, body, nil)
}

// ExchangeSell mirrors accountClient.exchangeSell(OneSidedTransactionDto):
// POST /internal/accounts/exchange/sell (one-sided credit, GHI #199). Body ignored.
func (c *AccountClient) ExchangeSell(ctx context.Context, req OneSidedTransaction) error {
	return c.base.doJSON(ctx, http.MethodPost, "/internal/accounts/exchange/sell", nil, req, nil)
}

// CreatedSystemAccount mirrors funds AccountServiceClient.CreatedSystemAccount:
// the response of POST /internal/accounts/system. Only the fields the funds
// service consumes are kept.
type CreatedSystemAccount struct {
	ID               int64           `json:"id"`
	AccountNumber    string          `json:"accountNumber"`
	OwnerID          int64           `json:"ownerId"`
	Currency         string          `json:"currency"`
	AvailableBalance decimal.Decimal `json:"availableBalance"`
	Status           string          `json:"status"`
	AccountType      string          `json:"accountType"`
}

// CreateSystemAccount mirrors funds AccountServiceClient.createSystemAccount:
// POST /internal/accounts/system. Idempotent on the upstream (returns the
// existing account if the number is already registered) — funds invokes it
// every invest/redeem to ensure the fund account exists before publishing the
// saga request, matching Java ensureFundAccountExists.
func (c *AccountClient) CreateSystemAccount(ctx context.Context, accountNumber string, ownerID int64, currency, displayName string, initialBalance decimal.Decimal) (*CreatedSystemAccount, error) {
	body := map[string]any{
		"accountNumber":   accountNumber,
		"ownerId":         ownerID,
		"currencyCode":    currency,
		"accountConcrete": "STANDARDNI",
		"displayName":     displayName,
		"initialBalance":  initialBalance,
	}
	var out CreatedSystemAccount
	if err := c.base.doJSON(ctx, http.MethodPost, "/internal/accounts/system", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreditAccount mirrors funds AccountServiceClient.creditAccount: POST
// /internal/accounts/credit. Body ignored. Used by FundDividendService for the
// fund's gross-dividend credit before splitting to clients.
func (c *AccountClient) CreditAccount(ctx context.Context, accountNumber string, amount decimal.Decimal, ownerID int64) error {
	body := map[string]any{
		"accountNumber": accountNumber,
		"amount":        amount,
		"clientId":      ownerID,
	}
	return c.base.doJSON(ctx, http.MethodPost, "/internal/accounts/credit", nil, body, nil)
}

// DebitAccount mirrors funds AccountServiceClient.debitAccount: POST
// /internal/accounts/debit. Body ignored. Used by FundDividendService for the
// REINVEST cash-out leg.
func (c *AccountClient) DebitAccount(ctx context.Context, accountNumber string, amount decimal.Decimal, ownerID int64) error {
	body := map[string]any{
		"accountNumber": accountNumber,
		"amount":        amount,
		"clientId":      ownerID,
	}
	return c.base.doJSON(ctx, http.MethodPost, "/internal/accounts/debit", nil, body, nil)
}

// GetDefaultRsdAccountNumberForOwner mirrors
// AccountClient.getDefaultRsdAccountNumberForOwner: GET
// /accounts/internal/default/{ownerId} -> {"accountNumber": "..."}. Every error
// (transport, 404, decode, missing field) is swallowed and returns "" — matching
// Java returning null after catching Exception, so OTC tax collection skips a
// seller that has no RSD account rather than aborting the run.
func (c *AccountClient) GetDefaultRsdAccountNumberForOwner(ctx context.Context, ownerID int64) string {
	var out struct {
		AccountNumber string `json:"accountNumber"`
	}
	if err := c.base.doJSON(ctx, http.MethodGet, fmt.Sprintf("/accounts/internal/default/%d", ownerID), nil, nil, &out); err != nil {
		return ""
	}
	return out.AccountNumber
}

// OwnerAccount mirrors DividendAccountClient.OwnerAccount — the id /
// accountNumber / ownerId subset the dividend payout records and credits (WP-14).
type OwnerAccount struct {
	ID            *int64  `json:"id"`
	AccountNumber *string `json:"accountNumber"`
	OwnerID       *int64  `json:"ownerId"`
}

// Number returns the account number or "".
func (a OwnerAccount) Number() string {
	if a.AccountNumber == nil {
		return ""
	}
	return *a.AccountNumber
}

// OwnerIDValue returns the owner id or 0.
func (a OwnerAccount) OwnerIDValue() int64 {
	if a.OwnerID == nil {
		return 0
	}
	return *a.OwnerID
}

// GetAccountInCurrency mirrors DividendAccountClient.accountInCurrency: GET
// /accounts/internal/by-owner/{ownerId}/currency/{currencyCode}. A 404 (owner
// has no account in that currency) and every other failure return nil — the
// dividend executor then falls back to the owner's RSD account, matching Java.
func (c *AccountClient) GetAccountInCurrency(ctx context.Context, ownerID int64, currency string) *OwnerAccount {
	var out OwnerAccount
	path := fmt.Sprintf("/accounts/internal/by-owner/%d/currency/%s", ownerID, url.PathEscape(currency))
	if err := c.base.doJSON(ctx, http.MethodGet, path, nil, nil, &out); err != nil {
		return nil
	}
	if out.AccountNumber == nil || *out.AccountNumber == "" {
		return nil
	}
	return &out
}

// GetStateRsdOwnerAccount mirrors DividendAccountClient.stateRsdAccount: GET
// /internal/accounts/state/RSD projected to OwnerAccount. Tolerant (nil on any
// failure) — the payout row is still recorded, only the credit is skipped.
func (c *AccountClient) GetStateRsdOwnerAccount(ctx context.Context) *OwnerAccount {
	return c.ownerAccount(ctx, "/internal/accounts/state/RSD")
}

// GetBankRsdOwnerAccount mirrors DividendAccountClient.bankRsdAccount: GET
// /internal/accounts/bank/RSD projected to OwnerAccount (Profit Banke target).
func (c *AccountClient) GetBankRsdOwnerAccount(ctx context.Context) *OwnerAccount {
	return c.ownerAccount(ctx, "/internal/accounts/bank/RSD")
}

func (c *AccountClient) ownerAccount(ctx context.Context, path string) *OwnerAccount {
	var out OwnerAccount
	if err := c.base.doJSON(ctx, http.MethodGet, path, nil, nil, &out); err != nil {
		return nil
	}
	if out.AccountNumber == nil || *out.AccountNumber == "" {
		return nil
	}
	return &out
}
