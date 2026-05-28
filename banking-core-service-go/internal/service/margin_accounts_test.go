package service

import (
	"context"
	"testing"

	"banka1/banking-core-service-go/internal/account"
	"banka1/banking-core-service-go/internal/decimal"
)

func TestCreateMarginAccountRequiresNotNullIDs(t *testing.T) {
	svc := NewMarginAccountService(nil, account.NumberGenerator{})
	_, err := svc.CreateForUser(context.Background(), CreateUserMarginAccountRequest{
		InitialMargin:     decimal.MustParse("100.00"),
		MaintenanceMargin: decimal.MustParse("50.00"),
		BankParticipation: decimal.MustParse("0.50"),
	})
	if err == nil {
		t.Fatal("expected missing employeeId/userId to fail")
	}
}
