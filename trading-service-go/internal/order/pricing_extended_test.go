package order

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"banka1/trading-service-go/internal/clients"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// referencePricePerUnit
// ---------------------------------------------------------------------------

func TestReferencePricePerUnit_MarketBuy_NilAsk_ReturnsError(t *testing.T) {
	t.Parallel()
	listing := &clients.StockListing{ID: 1, Ask: nil}
	_, err := referencePricePerUnit(TypeMarket, DirectionBuy, listing, nil, nil)
	require.Error(t, err)
}

func TestReferencePricePerUnit_MarketSell_NilBid_ReturnsError(t *testing.T) {
	t.Parallel()
	listing := &clients.StockListing{ID: 1, Bid: nil}
	_, err := referencePricePerUnit(TypeMarket, DirectionSell, listing, nil, nil)
	require.Error(t, err)
}

func TestReferencePricePerUnit_Limit_NilLimit_ReturnsError(t *testing.T) {
	t.Parallel()
	listing := &clients.StockListing{}
	_, err := referencePricePerUnit(TypeLimit, DirectionBuy, listing, nil, nil)
	require.Error(t, err)
}

func TestReferencePricePerUnit_Stop_NilStop_ReturnsError(t *testing.T) {
	t.Parallel()
	listing := &clients.StockListing{}
	_, err := referencePricePerUnit(TypeStop, DirectionBuy, listing, nil, nil)
	require.Error(t, err)
}

func TestReferencePricePerUnit_Stop_WithValue(t *testing.T) {
	t.Parallel()
	listing := &clients.StockListing{}
	stop := decimal.NewFromInt(95)
	result, err := referencePricePerUnit(TypeStop, DirectionBuy, listing, nil, &stop)
	require.NoError(t, err)
	assert.True(t, result.Equal(decimal.NewFromInt(95)))
}

func TestReferencePricePerUnit_StopLimit_WithLimitValue(t *testing.T) {
	t.Parallel()
	listing := &clients.StockListing{}
	limit := decimal.NewFromInt(50)
	result, err := referencePricePerUnit(TypeStopLimit, DirectionBuy, listing, &limit, nil)
	require.NoError(t, err)
	assert.True(t, result.Equal(decimal.NewFromInt(50)))
}

// ---------------------------------------------------------------------------
// calculateApproximatePrice - nil contract size
// ---------------------------------------------------------------------------

func TestCalculateApproximatePrice_NilContractSize_ReturnsError(t *testing.T) {
	t.Parallel()
	ask := decimal.NewFromInt(10)
	listing := &clients.StockListing{ID: 1, Ask: &ask, ContractSize: nil}
	_, err := calculateApproximatePrice(TypeMarket, DirectionBuy, listing, 1, nil, nil)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// calculateInitialMarginCost - various listing types
// ---------------------------------------------------------------------------

func TestCalculateInitialMarginCost_NilPrice_ReturnsError(t *testing.T) {
	t.Parallel()
	listing := &clients.StockListing{ID: 1, Price: nil}
	_, err := calculateInitialMarginCost(listing, 1)
	require.Error(t, err)
}

func TestCalculateInitialMarginCost_ForexType(t *testing.T) {
	t.Parallel()
	price := decimal.NewFromInt(1)
	listing := &clients.StockListing{
		ID:           1,
		Price:        &price,
		ContractSize: ip(1000),
	}
	// We need to set listing type to FOREX
	listing.ListingType = sp("FOREX")
	result, err := calculateInitialMarginCost(listing, 1)
	require.NoError(t, err)
	assert.False(t, result.IsNegative())
}

func TestCalculateInitialMarginCost_OptionType_WithUnderlyingPrice(t *testing.T) {
	t.Parallel()
	price := decimal.NewFromInt(5)
	underlying := decimal.NewFromInt(100)
	listing := &clients.StockListing{
		ID:              1,
		Price:           &price,
		ContractSize:    ip(1),
		UnderlyingPrice: &underlying,
	}
	listing.ListingType = sp("OPTION")
	result, err := calculateInitialMarginCost(listing, 1)
	require.NoError(t, err)
	// underlying 100 * 1 * 0.50 * 1.10 * 1 = 55
	assert.True(t, result.Equal(decimal.NewFromFloat(55)))
}

func TestCalculateInitialMarginCost_UnknownType_FallsBackToStockCalc(t *testing.T) {
	t.Parallel()
	price := decimal.NewFromInt(100)
	listing := &clients.StockListing{
		ID:           1,
		Price:        &price,
		ContractSize: ip(1),
	}
	listing.ListingType = sp("UNKNOWN_TYPE")
	result, err := calculateInitialMarginCost(listing, 1)
	require.NoError(t, err)
	// price 100 * 0.5 * 1.1 * 1 = 55
	assert.True(t, result.Equal(decimal.NewFromFloat(55)))
}

// ---------------------------------------------------------------------------
// convertAmount / convertAmountNoComm
// ---------------------------------------------------------------------------

func TestConvertAmount_SameCurrency_ReturnsInput(t *testing.T) {
	t.Parallel()
	svc := &Service{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	amount := decimal.NewFromInt(100)
	result, err := svc.convertAmount(context.Background(), "USD", "USD", amount)
	require.NoError(t, err)
	assert.True(t, result.Equal(amount))
}

func TestConvertAmount_EmptyCurrency_ReturnsInput(t *testing.T) {
	t.Parallel()
	svc := &Service{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	amount := decimal.NewFromInt(50)
	result, err := svc.convertAmount(context.Background(), "", "USD", amount)
	require.NoError(t, err)
	assert.True(t, result.Equal(amount))
}

func TestConvertAmountNoComm_SameCurrency_ReturnsInput(t *testing.T) {
	t.Parallel()
	svc := &Service{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	amount := decimal.NewFromInt(200)
	result, err := svc.convertAmountNoComm(context.Background(), "EUR", "EUR", amount)
	require.NoError(t, err)
	assert.True(t, result.Equal(amount))
}

// ---------------------------------------------------------------------------
// isMarketFamily
// ---------------------------------------------------------------------------

func TestIsMarketFamily_MarketAndStop_True(t *testing.T) {
	t.Parallel()
	assert.True(t, isMarketFamily(TypeMarket))
	assert.True(t, isMarketFamily(TypeStop))
	assert.False(t, isMarketFamily(TypeLimit))
	assert.False(t, isMarketFamily(TypeStopLimit))
}

// ---------------------------------------------------------------------------
// orderPricingFamily
// ---------------------------------------------------------------------------

func TestOrderPricingFamily_StopLimit_ReturnsLimit(t *testing.T) {
	t.Parallel()
	assert.Equal(t, TypeLimit, orderPricingFamily(TypeStopLimit))
	assert.Equal(t, TypeMarket, orderPricingFamily(TypeMarket))
	assert.Equal(t, TypeStop, orderPricingFamily(TypeStop))
}

// ---------------------------------------------------------------------------
// decimalMin / decimalMax / decimalMaxZero
// ---------------------------------------------------------------------------

func TestDecimalMaxZero_Negative_ReturnsZero(t *testing.T) {
	t.Parallel()
	result := decimalMaxZero(decimal.NewFromInt(-5))
	assert.True(t, result.IsZero())
}

func TestDecimalMaxZero_Positive_ReturnsValue(t *testing.T) {
	t.Parallel()
	result := decimalMaxZero(decimal.NewFromInt(10))
	assert.True(t, result.Equal(decimal.NewFromInt(10)))
}
