package funds

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"sort"

	"banka1/trading-service-go/internal/api"
	"banka1/trading-service-go/internal/clients"

	"github.com/shopspring/decimal"
)

// LiquidationService mirrors FundLiquidationService. Two entry points:
//   - LiquidateForFund: saga FUND_LIQUIDATION_FOR_REDEMPTION step 1 — sell down
//     holdings until cumulative proceeds (RSD) ≥ targetAmount.
//   - SellHolding: supervisor-initiated single-ticker sell from the supervisor
//     UI on /funds/{id}/securities/{ticker}/sell.
//
// In both cases the "sell" is simulated: live market price (avgUnitPrice
// fallback), reduce the holding row, credit fund liquidity + bank account.
// No real exchange order is placed (TBD: real matching engine integration).
type LiquidationService struct {
	repo     *Repository
	holding  *HoldingService
	market   *clients.MarketClient
	account  *clients.AccountClient
	snapshot *SnapshotService
	logger   *slog.Logger
}

func NewLiquidationService(repo *Repository, holding *HoldingService, market *clients.MarketClient, account *clients.AccountClient, snapshot *SnapshotService, logger *slog.Logger) *LiquidationService {
	return &LiquidationService{repo: repo, holding: holding, market: market, account: account, snapshot: snapshot, logger: logger}
}

// LiquidateResult mirrors FundLiquidationService.Result. liquidationId is a
// random UUID stamped per-call (matches Java).
type LiquidateResult struct {
	LiquidationID    string          `json:"liquidationId"`
	LiquidatedAmount decimal.Decimal `json:"liquidatedAmount"`
	HoldingsSold     int             `json:"holdingsSold"`
}

// SellResult mirrors FundLiquidationService.SellResult.
type SellResult struct {
	Ticker       string          `json:"ticker"`
	QuantitySold int             `json:"quantitySold"`
	UnitPrice    decimal.Decimal `json:"unitPrice"`
	Proceeds     decimal.Decimal `json:"proceeds"`
}

// LiquidateForFund mirrors FundLiquidationService.liquidateForFund. Algorithm
// matches Java: largest holdings first, sell-down until targetAmount RSD is
// reached; partial-sell within a holding when liquidating the whole position
// would overshoot. Each round is RSD-aware (cumulative is converted from USD
// before the comparison) so currency-mixed funds settle correctly.
func (s *LiquidationService) LiquidateForFund(ctx context.Context, fundID int64, target decimal.Decimal, correlationID string) (*LiquidateResult, error) {
	fund, err := s.repo.FindFundByIDForUpdate(ctx, s.repo.Pool(), fundID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, api.NewOtcError(http.StatusNotFound, "Fond "+itoa(fundID)+" ne postoji.")
		}
		return nil, err
	}

	holdings, err := s.holding.List(ctx, fundID)
	if err != nil {
		return nil, err
	}
	sort.Slice(holdings, func(i, j int) bool {
		return holdings[i].Quantity > holdings[j].Quantity
	})

	livePrices := s.market.CurrentPrices(ctx, uniqueTickers(holdings))

	liquidatedUsd := decimal.Zero
	holdingsSold := 0
	for _, h := range holdings {
		soFarRsd := convertUsdToRsd(ctx, s.market, liquidatedUsd)
		if soFarRsd.Cmp(target) >= 0 {
			break
		}
		unitPrice, ok := livePrices[h.StockTicker]
		if !ok {
			unitPrice = h.AvgUnitPrice
		}
		if unitPrice.Sign() <= 0 {
			continue
		}
		stillNeededRsd := target.Sub(soFarRsd)
		stillNeededUsd := convertRsdToUsd(ctx, s.market, stillNeededRsd)
		positionValueUsd := unitPrice.Mul(decimal.NewFromInt(int64(h.Quantity)))

		var sellQty int
		var sellAmount decimal.Decimal
		if positionValueUsd.Cmp(stillNeededUsd) <= 0 {
			sellQty = h.Quantity
			sellAmount = positionValueUsd.Round(2)
		} else {
			qtyD := stillNeededUsd.Div(unitPrice).Ceil()
			qty := int(qtyD.IntPart())
			if qty > h.Quantity {
				qty = h.Quantity
			}
			sellQty = qty
			sellAmount = unitPrice.Mul(decimal.NewFromInt(int64(qty))).Round(2)
		}
		if sellQty <= 0 {
			continue
		}
		if _, err := s.holding.Reduce(ctx, s.repo.Pool(), fundID, h.StockTicker, sellQty); err != nil {
			return nil, err
		}
		liquidatedUsd = liquidatedUsd.Add(sellAmount)
		holdingsSold++
	}

	liquidatedRsd := convertUsdToRsd(ctx, s.market, liquidatedUsd).Round(2)
	newLiquidity := fund.LikvidnaSredstva.Add(liquidatedRsd)
	if err := s.repo.UpdateFundLiquidity(ctx, s.repo.Pool(), fundID, newLiquidity); err != nil {
		return nil, err
	}
	if err := s.creditFundAccount(ctx, fund, liquidatedRsd, correlationID); err != nil {
		return nil, err
	}
	s.snapshot.RecordSilently(ctx, fundID)

	if liquidatedRsd.Cmp(target) < 0 {
		s.logger.Warn("fund liquidation: partial fill",
			"fundId", fundID, "target", target, "liquidated", liquidatedRsd)
	}
	return &LiquidateResult{
		LiquidationID:    newUUIDv4(),
		LiquidatedAmount: liquidatedRsd,
		HoldingsSold:     holdingsSold,
	}, nil
}

// newUUIDv4 generates a random RFC 4122 v4 UUID using crypto/rand. Used for
// the liquidationId stamp on saga step-1 results. Falls back to a high-entropy
// timestamp+random hex if the RNG is somehow unavailable (should never happen).
func newUUIDv4() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString(b[:])
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return hex.EncodeToString(b[0:4]) + "-" +
		hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" +
		hex.EncodeToString(b[8:10]) + "-" +
		hex.EncodeToString(b[10:16])
}

// SellHolding mirrors FundLiquidationService.sellHolding. Used by the
// supervisor UI button on each ticker row. ticker is case-insensitive against
// the fund's holdings (matches Java equalsIgnoreCase).
func (s *LiquidationService) SellHolding(ctx context.Context, fundID int64, ticker string, quantity int) (*SellResult, error) {
	fund, err := s.repo.FindFundByIDForUpdate(ctx, s.repo.Pool(), fundID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, api.NewOtcError(http.StatusNotFound, "Fond "+itoa(fundID)+" ne postoji.")
		}
		return nil, err
	}
	holdings, err := s.holding.List(ctx, fundID)
	if err != nil {
		return nil, err
	}
	var match *FundHolding
	for i := range holdings {
		if strEqualFold(holdings[i].StockTicker, ticker) {
			match = &holdings[i]
			break
		}
	}
	if match == nil {
		return nil, api.NewOtcError(http.StatusNotFound,
			"Fond "+itoa(fundID)+" ne poseduje hartiju "+ticker+".")
	}
	if quantity > match.Quantity {
		return nil, api.NewOtcError(http.StatusNotFound,
			"Trazena kolicina "+itoa(int64(quantity))+
				" veca od raspolozive "+itoa(int64(match.Quantity))+".")
	}
	unitPrice, ok := s.market.CurrentPrice(ctx, ticker)
	if !ok {
		unitPrice = match.AvgUnitPrice
	}
	proceedsUsd := unitPrice.Mul(decimal.NewFromInt(int64(quantity))).Round(2)
	proceedsRsd := convertUsdToRsd(ctx, s.market, proceedsUsd).Round(2)
	if _, err := s.holding.Reduce(ctx, s.repo.Pool(), fundID, match.StockTicker, quantity); err != nil {
		return nil, err
	}
	newLiquidity := fund.LikvidnaSredstva.Add(proceedsRsd)
	if err := s.repo.UpdateFundLiquidity(ctx, s.repo.Pool(), fundID, newLiquidity); err != nil {
		return nil, err
	}
	if err := s.creditFundAccount(ctx, fund, proceedsRsd, "supervisor-sell"); err != nil {
		return nil, err
	}
	s.snapshot.RecordSilently(ctx, fundID)
	s.logger.Info("supervisor sold holding",
		"fundId", fundID, "ticker", ticker, "qty", quantity, "unitPrice", unitPrice,
		"proceedsUsd", proceedsUsd, "proceedsRsd", proceedsRsd)
	return &SellResult{
		Ticker:       match.StockTicker,
		QuantitySold: quantity,
		UnitPrice:    unitPrice,
		Proceeds:     proceedsRsd,
	}, nil
}

func (s *LiquidationService) creditFundAccount(ctx context.Context, fund *InvestmentFund, amount decimal.Decimal, correlationID string) error {
	if amount.Sign() <= 0 {
		return nil
	}
	ownerID := -1000 - fund.ID
	if err := s.account.CreditAccount(ctx, fund.AccountNumber, amount, ownerID); err != nil {
		return api.NewOtcError(http.StatusConflict,
			"AccountServiceClient nije dostupan — credit racuna fonda nije moguc.")
	}
	s.logger.Info("fund account credited from liquidation",
		"fundId", fund.ID, "accountNumber", fund.AccountNumber, "ownerId", ownerID,
		"amount", amount, "correlationId", correlationID)
	return nil
}

func convertRsdToUsd(ctx context.Context, m *clients.MarketClient, amount decimal.Decimal) decimal.Decimal {
	if amount.Sign() == 0 {
		return decimal.Zero
	}
	out, ok := m.ConvertNoCommission(ctx, amount, FundBaseCurrency, HoldingPriceCurrency)
	if !ok {
		return amount
	}
	return out
}

func strEqualFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
