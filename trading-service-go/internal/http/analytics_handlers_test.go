package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"banka1/trading-service-go/internal/analytics"

	"github.com/stretchr/testify/assert"
)

// ---- stub analytics repo ----

type httpStubAnalyticsRepo struct {
	run    *analytics.JobRun
	runErr error
	segs   []analytics.ClientSegment
	risk   *analytics.PortfolioRisk
	ticks  []analytics.TopTicker
}

func (s *httpStubAnalyticsRepo) LatestCompletedRun(_ context.Context) (*analytics.JobRun, error) {
	return s.run, s.runErr
}
func (s *httpStubAnalyticsRepo) SegmentsByRun(_ context.Context, _ string) ([]analytics.ClientSegment, error) {
	return s.segs, nil
}
func (s *httpStubAnalyticsRepo) PortfolioRiskByRunAndUser(_ context.Context, _ string, _ int64) (*analytics.PortfolioRisk, error) {
	return s.risk, nil
}
func (s *httpStubAnalyticsRepo) TopTickersByRun(_ context.Context, _ string) ([]analytics.TopTicker, error) {
	return s.ticks, nil
}

func newHandlersWithAnalytics(repo analytics.AnalyticsRepo) *Handlers {
	app := &App{Analytics: analytics.NewServiceWithRepo(repo)}
	return &Handlers{app: app}
}

func sampleJobRun() *analytics.JobRun {
	now := time.Now()
	return &analytics.JobRun{
		RunID: "r-1", JobName: "analytics", Status: "COMPLETED",
		StartedAt: now.Add(-time.Minute), CompletedAt: &now,
	}
}

// ---- AnalyticsLatestRun ----

func TestAnalyticsLatestRun_NoRun_Returns404(t *testing.T) {
	h := newHandlersWithAnalytics(&httpStubAnalyticsRepo{run: nil})
	req := httptest.NewRequest(http.MethodGet, "/analytics/runs/latest", nil)
	w := httptest.NewRecorder()
	h.AnalyticsLatestRun(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAnalyticsLatestRun_RepoError_Returns500(t *testing.T) {
	h := newHandlersWithAnalytics(&httpStubAnalyticsRepo{runErr: errors.New("db error")})
	req := httptest.NewRequest(http.MethodGet, "/analytics/runs/latest", nil)
	w := httptest.NewRecorder()
	h.AnalyticsLatestRun(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAnalyticsLatestRun_Success_Returns200(t *testing.T) {
	h := newHandlersWithAnalytics(&httpStubAnalyticsRepo{run: sampleJobRun()})
	req := httptest.NewRequest(http.MethodGet, "/analytics/runs/latest", nil)
	w := httptest.NewRecorder()
	h.AnalyticsLatestRun(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "r-1")
}

// ---- AnalyticsClientSegments ----

func TestAnalyticsClientSegments_NoRun_ReturnsEmptyList(t *testing.T) {
	h := newHandlersWithAnalytics(&httpStubAnalyticsRepo{run: nil})
	req := httptest.NewRequest(http.MethodGet, "/analytics/segments", nil)
	w := httptest.NewRecorder()
	h.AnalyticsClientSegments(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAnalyticsClientSegments_RepoError_Returns500(t *testing.T) {
	h := newHandlersWithAnalytics(&httpStubAnalyticsRepo{runErr: errors.New("boom")})
	req := httptest.NewRequest(http.MethodGet, "/analytics/segments", nil)
	w := httptest.NewRecorder()
	h.AnalyticsClientSegments(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAnalyticsClientSegments_WithRun_Returns200(t *testing.T) {
	h := newHandlersWithAnalytics(&httpStubAnalyticsRepo{
		run:  sampleJobRun(),
		segs: []analytics.ClientSegment{{UserID: 1}},
	})
	req := httptest.NewRequest(http.MethodGet, "/analytics/segments", nil)
	w := httptest.NewRecorder()
	h.AnalyticsClientSegments(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---- AnalyticsPortfolioRisk ----

func TestAnalyticsPortfolioRisk_InvalidUserID_Returns400(t *testing.T) {
	h := newHandlersWithAnalytics(&httpStubAnalyticsRepo{})
	req := httptest.NewRequest(http.MethodGet, "/analytics/risk/abc", nil)
	req.SetPathValue("userId", "abc")
	w := httptest.NewRecorder()
	h.AnalyticsPortfolioRisk(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAnalyticsPortfolioRisk_NoRun_Returns404(t *testing.T) {
	h := newHandlersWithAnalytics(&httpStubAnalyticsRepo{run: nil})
	req := httptest.NewRequest(http.MethodGet, "/analytics/risk/42", nil)
	req.SetPathValue("userId", "42")
	w := httptest.NewRecorder()
	h.AnalyticsPortfolioRisk(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAnalyticsPortfolioRisk_Success_Returns200(t *testing.T) {
	h := newHandlersWithAnalytics(&httpStubAnalyticsRepo{
		run:  sampleJobRun(),
		risk: &analytics.PortfolioRisk{UserID: 42, RiskLevel: "LOW"},
	})
	req := httptest.NewRequest(http.MethodGet, "/analytics/risk/42", nil)
	req.SetPathValue("userId", "42")
	w := httptest.NewRecorder()
	h.AnalyticsPortfolioRisk(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---- AnalyticsTopTickers ----

func TestAnalyticsTopTickers_NoRun_ReturnsEmptyList(t *testing.T) {
	h := newHandlersWithAnalytics(&httpStubAnalyticsRepo{run: nil})
	req := httptest.NewRequest(http.MethodGet, "/analytics/tickers", nil)
	w := httptest.NewRecorder()
	h.AnalyticsTopTickers(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAnalyticsTopTickers_Success_Returns200(t *testing.T) {
	h := newHandlersWithAnalytics(&httpStubAnalyticsRepo{
		run:   sampleJobRun(),
		ticks: []analytics.TopTicker{{Ticker: "AAPL"}},
	})
	req := httptest.NewRequest(http.MethodGet, "/analytics/tickers", nil)
	w := httptest.NewRecorder()
	h.AnalyticsTopTickers(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "AAPL")
}
