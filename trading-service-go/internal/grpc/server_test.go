package grpc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"banka1/trading-service-go/internal/analytics"
	httpapi "banka1/trading-service-go/internal/http"
	tradingv1 "banka1/trading-service-go/proto/trading/v1"
)

func TestNewServer_ReturnsNonNil(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(nil, logger)
	if srv == nil {
		t.Error("NewServer returned nil")
	}
}

func TestPing_ReturnsStatusUp(t *testing.T) {
	svc := &service{app: nil}
	resp, err := svc.Ping(context.Background(), &tradingv1.PingRequest{})
	if err != nil {
		t.Fatalf("Ping() error: %v", err)
	}
	if resp.JsonPayload != `{"status":"UP"}` {
		t.Errorf("JsonPayload = %q, want {\"status\":\"UP\"}", resp.JsonPayload)
	}
}

type stubAnalyticsRepo struct {
	run *analytics.JobRun
	err error
}

func (s *stubAnalyticsRepo) LatestCompletedRun(_ context.Context) (*analytics.JobRun, error) {
	return s.run, s.err
}
func (s *stubAnalyticsRepo) SegmentsByRun(_ context.Context, _ string) ([]analytics.ClientSegment, error) {
	return nil, nil
}
func (s *stubAnalyticsRepo) PortfolioRiskByRunAndUser(_ context.Context, _ string, _ int64) (*analytics.PortfolioRisk, error) {
	return nil, nil
}
func (s *stubAnalyticsRepo) TopTickersByRun(_ context.Context, _ string) ([]analytics.TopTicker, error) {
	return nil, nil
}

func TestGetLatestAnalyticsRun_ErrNotFound(t *testing.T) {
	analyticsSvc := analytics.NewServiceWithRepo(&stubAnalyticsRepo{run: nil})
	app := &httpapi.App{Analytics: analyticsSvc}
	svc := &service{app: app}
	_, err := svc.GetLatestAnalyticsRun(context.Background(), &tradingv1.GetLatestAnalyticsRunRequest{})
	if err == nil {
		t.Error("expected gRPC NotFound error")
	}
}

func TestGetLatestAnalyticsRun_RepoError(t *testing.T) {
	boom := errors.New("db error")
	analyticsSvc := analytics.NewServiceWithRepo(&stubAnalyticsRepo{err: boom})
	app := &httpapi.App{Analytics: analyticsSvc}
	svc := &service{app: app}
	_, err := svc.GetLatestAnalyticsRun(context.Background(), &tradingv1.GetLatestAnalyticsRunRequest{})
	if err == nil {
		t.Error("expected gRPC Internal error")
	}
}

func TestGetLatestAnalyticsRun_Success(t *testing.T) {
	now := time.Now()
	run := &analytics.JobRun{
		RunID:       "r-1",
		JobName:     "analytics",
		Status:      "COMPLETED",
		StartedAt:   now.Add(-time.Minute),
		CompletedAt: &now,
	}
	analyticsSvc := analytics.NewServiceWithRepo(&stubAnalyticsRepo{run: run})
	app := &httpapi.App{Analytics: analyticsSvc}
	svc := &service{app: app}
	resp, err := svc.GetLatestAnalyticsRun(context.Background(), &tradingv1.GetLatestAnalyticsRunRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.JsonPayload == "" {
		t.Error("expected non-empty JSON payload")
	}
}
