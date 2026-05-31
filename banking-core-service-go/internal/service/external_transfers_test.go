package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"banka1/banking-core-service-go/internal/config"
	"banka1/banking-core-service-go/internal/decimal"
)

func TestIssueClearingHouseTransferRetriesTransientFailures(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if got := r.Header.Get("X-Idempotency-Key"); got != "transfer-42" {
			t.Fatalf("X-Idempotency-Key=%q, want transfer-42", got)
		}
		if attempts == 1 {
			http.Error(w, "temporary", http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(clearingHouseIssueResult{Success: true, ClearingHouseRef: "CH-42"})
	}))
	defer server.Close()

	svc := NewExternalTransferService(nil, config.Config{
		ClearingHouseURL:      server.URL,
		ClearingHouseAPIToken: "token",
	}, nil)
	svc.clearingRetryBackoffs = []time.Duration{0}

	result := svc.issueClearingHouseTransfer(context.Background(), transferRetryEvent{
		TransferID:       42,
		RetryAttempt:     1,
		Amount:           decimal.MustParse("100.00"),
		RecipientAccount: "123",
		Currency:         "RSD",
	})
	if !result.Success {
		t.Fatalf("issueClearingHouseTransfer() success=false reason=%q", result.FailureReason)
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d, want 2", attempts)
	}
}
