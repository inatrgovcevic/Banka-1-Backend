package analytics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// stubAnalyticsRepo stubs AnalyticsRepo for unit tests.
type stubAnalyticsRepo struct {
	latestRun *JobRun
	latestErr error
	segments  []ClientSegment
	segErr    error
	risk      *PortfolioRisk
	riskErr   error
	tickers   []TopTicker
	tickErr   error
}

func (s *stubAnalyticsRepo) LatestCompletedRun(_ context.Context) (*JobRun, error) {
	return s.latestRun, s.latestErr
}
func (s *stubAnalyticsRepo) SegmentsByRun(_ context.Context, _ string) ([]ClientSegment, error) {
	return s.segments, s.segErr
}
func (s *stubAnalyticsRepo) PortfolioRiskByRunAndUser(_ context.Context, _ string, _ int64) (*PortfolioRisk, error) {
	return s.risk, s.riskErr
}
func (s *stubAnalyticsRepo) TopTickersByRun(_ context.Context, _ string) ([]TopTicker, error) {
	return s.tickers, s.tickErr
}

func sampleRun() *JobRun {
	now := time.Now()
	return &JobRun{
		RunID:       "run-1",
		JobName:     "analytics",
		Status:      "COMPLETED",
		StartedAt:   now.Add(-time.Minute),
		CompletedAt: &now,
	}
}

// ----- LatestRun -----

func TestLatestRun_NoRun_ReturnsErrNotFound(t *testing.T) {
	svc := &Service{repo: &stubAnalyticsRepo{latestRun: nil}}
	_, err := svc.LatestRun(context.Background())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestLatestRun_RepoError_Propagates(t *testing.T) {
	boom := errors.New("db boom")
	svc := &Service{repo: &stubAnalyticsRepo{latestErr: boom}}
	_, err := svc.LatestRun(context.Background())
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

func TestLatestRun_Success(t *testing.T) {
	run := sampleRun()
	svc := &Service{repo: &stubAnalyticsRepo{latestRun: run}}
	resp, err := svc.LatestRun(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resp.RunID != run.RunID {
		t.Errorf("RunID = %q, want %q", resp.RunID, run.RunID)
	}
	if resp.JobName != run.JobName {
		t.Errorf("JobName = %q, want %q", resp.JobName, run.JobName)
	}
}

// ----- ClientSegments -----

func TestClientSegments_NoRun_ReturnsEmpty(t *testing.T) {
	svc := &Service{repo: &stubAnalyticsRepo{latestRun: nil}}
	resp, err := svc.ClientSegments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Segments) != 0 {
		t.Errorf("expected empty segments, got %d", len(resp.Segments))
	}
	if resp.RunID != nil {
		t.Error("RunID should be nil when no run exists")
	}
}

func TestClientSegments_RepoLatestError(t *testing.T) {
	boom := errors.New("boom")
	svc := &Service{repo: &stubAnalyticsRepo{latestErr: boom}}
	_, err := svc.ClientSegments(context.Background())
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

func TestClientSegments_SegmentError(t *testing.T) {
	boom := errors.New("seg boom")
	svc := &Service{repo: &stubAnalyticsRepo{latestRun: sampleRun(), segErr: boom}}
	_, err := svc.ClientSegments(context.Background())
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

func TestClientSegments_Success(t *testing.T) {
	run := sampleRun()
	segs := []ClientSegment{
		{UserID: 1, ClusterID: 2, SegmentLabel: "HIGH", RiskScore: decimal.NewFromFloat(0.9)},
		{UserID: 2, ClusterID: 1, SegmentLabel: "LOW", RiskScore: decimal.NewFromFloat(0.1)},
	}
	svc := &Service{repo: &stubAnalyticsRepo{latestRun: run, segments: segs}}
	resp, err := svc.ClientSegments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Segments) != 2 {
		t.Errorf("len(Segments) = %d, want 2", len(resp.Segments))
	}
	if resp.RunID == nil || *resp.RunID != run.RunID {
		t.Errorf("RunID mismatch")
	}
}

// ----- PortfolioRisk -----

func TestPortfolioRisk_NoRun_ErrNotFound(t *testing.T) {
	svc := &Service{repo: &stubAnalyticsRepo{latestRun: nil}}
	_, err := svc.PortfolioRisk(context.Background(), 42)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestPortfolioRisk_NoRecord_ErrNotFound(t *testing.T) {
	svc := &Service{repo: &stubAnalyticsRepo{latestRun: sampleRun(), risk: nil}}
	_, err := svc.PortfolioRisk(context.Background(), 42)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestPortfolioRisk_RepoError(t *testing.T) {
	boom := errors.New("boom")
	svc := &Service{repo: &stubAnalyticsRepo{latestErr: boom}}
	_, err := svc.PortfolioRisk(context.Background(), 42)
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

func TestPortfolioRisk_RiskRepoError(t *testing.T) {
	boom := errors.New("risk boom")
	svc := &Service{repo: &stubAnalyticsRepo{latestRun: sampleRun(), riskErr: boom}}
	_, err := svc.PortfolioRisk(context.Background(), 42)
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

func TestPortfolioRisk_Success(t *testing.T) {
	run := sampleRun()
	risk := &PortfolioRisk{UserID: 42, RiskLevel: "HIGH", RiskScore: decimal.NewFromFloat(0.8)}
	svc := &Service{repo: &stubAnalyticsRepo{latestRun: run, risk: risk}}
	resp, err := svc.PortfolioRisk(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if resp.UserID != 42 {
		t.Errorf("UserID = %d, want 42", resp.UserID)
	}
	if resp.RiskLevel != "HIGH" {
		t.Errorf("RiskLevel = %q, want HIGH", resp.RiskLevel)
	}
}

// ----- TopTickers -----

func TestTopTickers_NoRun_ReturnsEmpty(t *testing.T) {
	svc := &Service{repo: &stubAnalyticsRepo{latestRun: nil}}
	resp, err := svc.TopTickers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Tickers) != 0 {
		t.Errorf("expected empty tickers, got %d", len(resp.Tickers))
	}
}

func TestTopTickers_RepoError(t *testing.T) {
	boom := errors.New("boom")
	svc := &Service{repo: &stubAnalyticsRepo{latestErr: boom}}
	_, err := svc.TopTickers(context.Background())
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

func TestTopTickers_TickerError(t *testing.T) {
	boom := errors.New("tick boom")
	svc := &Service{repo: &stubAnalyticsRepo{latestRun: sampleRun(), tickErr: boom}}
	_, err := svc.TopTickers(context.Background())
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

func TestTopTickers_Success(t *testing.T) {
	run := sampleRun()
	tickers := []TopTicker{
		{Rank: 1, Ticker: "AAPL", TradedQuantity: 1000},
		{Rank: 2, Ticker: "MSFT", TradedQuantity: 500},
	}
	svc := &Service{repo: &stubAnalyticsRepo{latestRun: run, tickers: tickers}}
	resp, err := svc.TopTickers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Tickers) != 2 {
		t.Errorf("len(Tickers) = %d, want 2", len(resp.Tickers))
	}
	if resp.Tickers[0].Ticker != "AAPL" {
		t.Errorf("first ticker = %q, want AAPL", resp.Tickers[0].Ticker)
	}
}

func TestErrNotFound_IsDistinct(t *testing.T) {
	if errors.Is(ErrNotFound, errors.New("other")) {
		t.Error("ErrNotFound should not match arbitrary errors")
	}
}
