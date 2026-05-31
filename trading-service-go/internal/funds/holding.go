package funds

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"banka1/trading-service-go/internal/api"
	"banka1/trading-service-go/internal/clients"

	"github.com/shopspring/decimal"
)

// HoldingService mirrors FundHoldingService: weighted-average add/update,
// quantity reduce with soft-delete on zero, enriched read with live snapshot,
// and totalHoldingsValue (USD priced, converted to RSD).
type HoldingService struct {
	repo   *Repository
	market *clients.MarketClient
	logger *slog.Logger
}

func NewHoldingService(repo *Repository, market *clients.MarketClient, logger *slog.Logger) *HoldingService {
	return &HoldingService{repo: repo, market: market, logger: logger}
}

// AddOrUpdate mirrors FundHoldingService.addOrUpdate. Weighted-average formula
// at scale 4 HALF_UP: newAvg = (oldQty*oldAvg + addedQty*unitPrice) / (oldQty +
// addedQty). The Repository ensures (fund_id, stock_ticker) uniqueness on the
// non-soft-deleted row. Bad input (≤0 qty or ≤0 price) returns an OTC 404 like
// IllegalArgumentException.
func (s *HoldingService) AddOrUpdate(ctx context.Context, q Querier, fundID int64, ticker string, addedQty int, unitPrice decimal.Decimal) (*FundHolding, error) {
	if addedQty <= 0 || unitPrice.Sign() <= 0 {
		return nil, api.NewOtcError(http.StatusNotFound, "addedQuantity i unitPrice moraju biti > 0.")
	}
	existing, err := s.repo.FindHolding(ctx, q, fundID, ticker)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	now := time.Now().UTC()
	if existing != nil {
		oldQty := decimal.NewFromInt(int64(existing.Quantity))
		addedQtyD := decimal.NewFromInt(int64(addedQty))
		newQty := existing.Quantity + addedQty
		totalValue := existing.AvgUnitPrice.Mul(oldQty).
			Add(unitPrice.Mul(addedQtyD))
		newAvg := totalValue.Div(decimal.NewFromInt(int64(newQty))).Round(4)
		existing.Quantity = newQty
		existing.AvgUnitPrice = newAvg
		existing.UpdatedAt = &now
		if err := s.repo.UpdateHolding(ctx, q, existing); err != nil {
			return nil, err
		}
		s.logger.Info("fund holding upserted", "fundId", fundID, "ticker", ticker,
			"qty", existing.Quantity, "avg", existing.AvgUnitPrice)
		return existing, nil
	}
	h := &FundHolding{
		FundID:       fundID,
		StockTicker:  ticker,
		Quantity:     addedQty,
		AvgUnitPrice: unitPrice.Round(4),
		CreatedAt:    now,
	}
	if err := s.repo.InsertHolding(ctx, q, h); err != nil {
		return nil, err
	}
	s.logger.Info("fund holding created", "fundId", fundID, "ticker", ticker,
		"qty", h.Quantity, "avg", h.AvgUnitPrice)
	return h, nil
}

// Reduce mirrors FundHoldingService.reduce. Soft-delete when quantity hits 0.
// "no holding" maps to IllegalStateException → OTC 409 (parity with Java).
// "insufficient quantity" same.
func (s *HoldingService) Reduce(ctx context.Context, q Querier, fundID int64, ticker string, reduceBy int) (*FundHolding, error) {
	if reduceBy <= 0 {
		return nil, api.NewOtcError(http.StatusNotFound, "reduceBy mora biti > 0.")
	}
	h, err := s.repo.FindHolding(ctx, q, fundID, ticker)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, api.NewOtcError(http.StatusConflict,
				"Fond "+itoa(fundID)+" ne poseduje "+ticker+" — reduce odbijen.")
		}
		return nil, err
	}
	if h.Quantity < reduceBy {
		return nil, api.NewOtcError(http.StatusConflict,
			"Nedovoljno hartija "+ticker+" u fondu "+itoa(fundID)+
				" (raspolozivo="+itoa(int64(h.Quantity))+", trazeno="+itoa(int64(reduceBy))+").")
	}
	now := time.Now().UTC()
	newQty := h.Quantity - reduceBy
	h.Quantity = newQty
	h.UpdatedAt = &now
	if newQty == 0 {
		h.Deleted = true
	}
	if err := s.repo.UpdateHolding(ctx, q, h); err != nil {
		return nil, err
	}
	s.logger.Info("fund holding reduced", "fundId", fundID, "ticker", ticker,
		"reduceBy", reduceBy, "newQty", newQty, "deleted", h.Deleted)
	return h, nil
}

// List mirrors FundHoldingService.listByFund (active holdings only).
func (s *HoldingService) List(ctx context.Context, fundID int64) ([]FundHolding, error) {
	return s.repo.FindHoldingsActive(ctx, nil, fundID)
}

// EnrichedView is the per-holding projection returned to /funds/{id}/securities.
// Mirrors FundHoldingDto (initialMarginCost = avgUnitPrice * quantity).
type EnrichedView struct {
	ID                int64
	Ticker            string
	Quantity          int
	AvgUnitPrice      decimal.Decimal
	InitialMarginCost decimal.Decimal
	Price             *decimal.Decimal
	Change            *decimal.Decimal
	Volume            int64
	AcquisitionDate   time.Time
}

// EnrichedHoldings mirrors FundHoldingService.enrichedHoldings. Joins active
// holdings with live market snapshots (or nil price/change/volume when
// market-service did not answer for that ticker).
func (s *HoldingService) EnrichedHoldings(ctx context.Context, fundID int64) ([]EnrichedView, error) {
	holdings, err := s.List(ctx, fundID)
	if err != nil {
		return nil, err
	}
	if len(holdings) == 0 {
		return []EnrichedView{}, nil
	}
	tickers := uniqueTickers(holdings)
	snaps := s.market.FetchSnapshots(ctx, tickers)
	out := make([]EnrichedView, 0, len(holdings))
	for _, h := range holdings {
		snap := snaps[h.StockTicker]
		v := EnrichedView{
			ID:                h.ID,
			Ticker:            h.StockTicker,
			Quantity:          h.Quantity,
			AvgUnitPrice:      h.AvgUnitPrice,
			InitialMarginCost: h.AvgUnitPrice.Mul(decimal.NewFromInt(int64(h.Quantity))),
			AcquisitionDate:   h.CreatedAt,
		}
		v.Price = snap.CurrentPrice
		v.Change = snap.ChangePercent
		if snap.Volume != nil {
			v.Volume = *snap.Volume
		}
		out = append(out, v)
	}
	return out, nil
}

// CalculateHoldingsValue mirrors FundHoldingService.calculateHoldingsValue:
// sum(price * quantity) priced USD via market-service (avgUnitPrice fallback
// per-ticker), then converted USD→RSD no-commission. Tolerant — market
// outages degrade to historical avg.
func (s *HoldingService) CalculateHoldingsValue(ctx context.Context, fundID int64) decimal.Decimal {
	holdings, err := s.repo.FindHoldingsActive(ctx, nil, fundID)
	if err != nil || len(holdings) == 0 {
		return decimal.Zero
	}
	tickers := uniqueTickers(holdings)
	prices := s.market.CurrentPrices(ctx, tickers)
	totalUsd := decimal.Zero
	for _, h := range holdings {
		price, ok := prices[h.StockTicker]
		if !ok {
			price = h.AvgUnitPrice
		}
		if price.Sign() <= 0 {
			continue
		}
		totalUsd = totalUsd.Add(price.Mul(decimal.NewFromInt(int64(h.Quantity))))
	}
	return convertUsdToRsd(ctx, s.market, totalUsd).Round(2)
}

func uniqueTickers(hs []FundHolding) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(hs))
	for _, h := range hs {
		if _, ok := seen[h.StockTicker]; ok {
			continue
		}
		seen[h.StockTicker] = struct{}{}
		out = append(out, h.StockTicker)
	}
	return out
}

func convertUsdToRsd(ctx context.Context, m *clients.MarketClient, amount decimal.Decimal) decimal.Decimal {
	if amount.Sign() == 0 {
		return decimal.Zero
	}
	out, ok := m.ConvertNoCommission(ctx, amount, HoldingPriceCurrency, FundBaseCurrency)
	if !ok {
		return amount
	}
	return out
}

// itoa is a tiny helper for embedding ids in error messages. Using strconv
// across this many files would noise up imports; keeps message construction
// inline.
func itoa(v int64) string { return decimal.NewFromInt(v).String() }
