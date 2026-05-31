package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"Banka1Back/credit-service-go/internal/model"

	"github.com/shopspring/decimal"
)

type AccountClient struct {
	baseURL string
	http    *http.Client
}

func NewAccountClient() *AccountClient {
	baseURL := os.Getenv("SERVICES_ACCOUNT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8084"
	}

	return &AccountClient{
		baseURL: baseURL,
		http:    &http.Client{},
	}
}

type AccountDetailsResponse struct {
	OwnerID  int64              `json:"ownerId"`
	Currency model.CurrencyCode `json:"currency"`
	Email    string             `json:"email"`
	Username string             `json:"username"`
}

type BankPaymentRequest struct {
	FromBankNumber *string         `json:"fromBankNumber"`
	ToBankNumber   *string         `json:"toBankNumber"`
	Amount         decimal.Decimal `json:"amount"`
}

func (c *AccountClient) GetDetails(accountNumber string) (AccountDetailsResponse, error) {
	url := fmt.Sprintf("%s/internal/accounts/%s/details", c.baseURL, accountNumber)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return AccountDetailsResponse{}, err
	}

	if token := os.Getenv("INTERNAL_AUTH_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return AccountDetailsResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return AccountDetailsResponse{}, errors.New("account-service get details failed")
	}

	var result AccountDetailsResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return AccountDetailsResponse{}, err
	}

	return result, nil
}

func (c *AccountClient) TransactionFromBank(toBankNumber string, amount decimal.Decimal) error {
	body := BankPaymentRequest{
		FromBankNumber: nil,
		ToBankNumber:   &toBankNumber,
		Amount:         amount,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/internal/accounts/transactionFromBank", c.baseURL)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	if token := os.Getenv("INTERNAL_AUTH_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("account-service transactionFromBank failed")
	}

	return nil
}

func (c *AccountClient) TransactionToBank(fromBankNumber string, amount decimal.Decimal) error {
	body := BankPaymentRequest{
		FromBankNumber: &fromBankNumber,
		ToBankNumber:   nil,
		Amount:         amount,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/internal/accounts/transactionFromBank", c.baseURL)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	if token := os.Getenv("INTERNAL_AUTH_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("account-service transactionToBank failed")
	}

	return nil
}
