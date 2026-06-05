package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"Banka1Back/credit-service-go/internal/service"
)

func TestStartInstallmentScheduler_ContextCancel_Stops(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	ctx, cancel := context.WithCancel(context.Background())

	service.StartInstallmentScheduler(ctx, svc)

	// Allow initial run to execute
	time.Sleep(20 * time.Millisecond)
	cancel()
	// Allow goroutine to process context cancellation
	time.Sleep(20 * time.Millisecond)
}

func TestStartInstallmentScheduler_InitialRunError_DoesNotPanic(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()
	installRepo.findDueErr = errors.New("db unavailable")
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	service.StartInstallmentScheduler(ctx, svc)
	<-ctx.Done()
}

func TestStartInstallmentScheduler_ImmediateCancel_DoesNotBlock(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately before starting

	service.StartInstallmentScheduler(ctx, svc)
	time.Sleep(20 * time.Millisecond)
}
