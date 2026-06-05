package analytics

import (
	"context"
	"errors"

	"banka1/trading-service-go/internal/api"
)

// ErrNotFound signals a missing resource; the HTTP layer maps it to 404.
// Mirrors the Java ResponseStatusException(NOT_FOUND, ...) cases.
var ErrNotFound = errors.New("analytics: resource not found")

// AnalyticsRepo is the repository interface Service depends on. Exported so
// external packages (grpc tests) can inject stubs via NewServiceWithRepo.
type AnalyticsRepo interface {
	LatestCompletedRun(ctx context.Context) (*JobRun, error)
	SegmentsByRun(ctx context.Context, runID string) ([]ClientSegment, error)
	PortfolioRiskByRunAndUser(ctx context.Context, runID string, userID int64) (*PortfolioRisk, error)
	TopTickersByRun(ctx context.Context, runID string) ([]TopTicker, error)
}

type Service struct {
	repo AnalyticsRepo
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// NewServiceWithRepo allows tests to inject a stub AnalyticsRepo without a real DB.
func NewServiceWithRepo(repo AnalyticsRepo) *Service {
	return &Service{repo: repo}
}

// LatestRun returns the latest COMPLETED run, or ErrNotFound when none exists.
func (s *Service) LatestRun(ctx context.Context) (api.AnalyticsRunResponse, error) {
	run, err := s.repo.LatestCompletedRun(ctx)
	if err != nil {
		return api.AnalyticsRunResponse{}, err
	}
	if run == nil {
		return api.AnalyticsRunResponse{}, ErrNotFound
	}
	return api.AnalyticsRunResponse{
		RunID:       run.RunID,
		JobName:     run.JobName,
		Status:      run.Status,
		StartedAt:   api.NewLocalDateTime(run.StartedAt),
		CompletedAt: api.LocalDateTimeFromPtr(run.CompletedAt),
		Message:     run.Message,
	}, nil
}

// ClientSegments returns the latest run's client segments. When no run exists it
// returns {runId:null, computedAt:null, segments:[]} (200), matching Java.
func (s *Service) ClientSegments(ctx context.Context) (api.ClientSegmentsResponse, error) {
	run, err := s.repo.LatestCompletedRun(ctx)
	if err != nil {
		return api.ClientSegmentsResponse{}, err
	}
	if run == nil {
		return api.ClientSegmentsResponse{Segments: []api.ClientSegmentItemResponse{}}, nil
	}
	rows, err := s.repo.SegmentsByRun(ctx, run.RunID)
	if err != nil {
		return api.ClientSegmentsResponse{}, err
	}
	items := make([]api.ClientSegmentItemResponse, 0, len(rows))
	for _, seg := range rows {
		items = append(items, api.ClientSegmentItemResponse{
			UserID:              seg.UserID,
			ClusterID:           seg.ClusterID,
			SegmentLabel:        seg.SegmentLabel,
			TotalPortfolioValue: seg.TotalPortfolioValue,
			TotalCostBasis:      seg.TotalCostBasis,
			UnrealizedPnl:       seg.UnrealizedPnl,
			HoldingsCount:       seg.HoldingsCount,
			MaxHoldingPercent:   seg.MaxHoldingPercent,
			OrderCount:          seg.OrderCount,
			AverageOrderValue:   seg.AverageOrderValue,
			BuySellRatio:        seg.BuySellRatio,
			RiskScore:           seg.RiskScore,
		})
	}
	runID := run.RunID
	return api.ClientSegmentsResponse{
		RunID:      &runID,
		ComputedAt: api.LocalDateTimeFromPtr(run.CompletedAt),
		Segments:   items,
	}, nil
}

// PortfolioRisk returns the risk record for userId in the latest run, or
// ErrNotFound when there is no run or no record for that user.
func (s *Service) PortfolioRisk(ctx context.Context, userID int64) (api.PortfolioRiskResponse, error) {
	run, err := s.repo.LatestCompletedRun(ctx)
	if err != nil {
		return api.PortfolioRiskResponse{}, err
	}
	if run == nil {
		return api.PortfolioRiskResponse{}, ErrNotFound
	}
	risk, err := s.repo.PortfolioRiskByRunAndUser(ctx, run.RunID, userID)
	if err != nil {
		return api.PortfolioRiskResponse{}, err
	}
	if risk == nil {
		return api.PortfolioRiskResponse{}, ErrNotFound
	}
	return api.PortfolioRiskResponse{
		RunID:                run.RunID,
		ComputedAt:           api.LocalDateTimeFromPtr(run.CompletedAt),
		UserID:               risk.UserID,
		TotalMarketValue:     risk.TotalMarketValue,
		TotalCostBasis:       risk.TotalCostBasis,
		UnrealizedPnl:        risk.UnrealizedPnl,
		HoldingsCount:        risk.HoldingsCount,
		MaxHoldingPercent:    risk.MaxHoldingPercent,
		DiversificationScore: risk.DiversificationScore,
		RiskScore:            risk.RiskScore,
		RiskLevel:            risk.RiskLevel,
	}, nil
}

// TopTickers returns the latest run's top tickers. When no run exists it returns
// {runId:null, computedAt:null, tickers:[]} (200), matching Java.
func (s *Service) TopTickers(ctx context.Context) (api.TopTickersResponse, error) {
	run, err := s.repo.LatestCompletedRun(ctx)
	if err != nil {
		return api.TopTickersResponse{}, err
	}
	if run == nil {
		return api.TopTickersResponse{Tickers: []api.TopTickerItemResponse{}}, nil
	}
	rows, err := s.repo.TopTickersByRun(ctx, run.RunID)
	if err != nil {
		return api.TopTickersResponse{}, err
	}
	items := make([]api.TopTickerItemResponse, 0, len(rows))
	for _, t := range rows {
		items = append(items, api.TopTickerItemResponse{
			Rank:             t.Rank,
			ListingID:        t.ListingID,
			Ticker:           t.Ticker,
			TradedQuantity:   t.TradedQuantity,
			TradedNotional:   t.TradedNotional,
			OrderCount:       t.OrderCount,
			TransactionCount: t.TransactionCount,
		})
	}
	runID := run.RunID
	return api.TopTickersResponse{
		RunID:      &runID,
		ComputedAt: api.LocalDateTimeFromPtr(run.CompletedAt),
		Tickers:    items,
	}, nil
}
