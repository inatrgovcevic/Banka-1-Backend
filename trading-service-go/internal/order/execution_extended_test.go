package order

import (
	"testing"

	"banka1/trading-service-go/internal/clients"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// calculateExecutionPricePerUnit - missing branches
// ---------------------------------------------------------------------------

func TestCalculateExecutionPricePerUnit_MarketSell_NilBid_ReturnsFalse(t *testing.T) {
	t.Parallel()
	order := &Order{OrderType: TypeMarket, Direction: DirectionSell}
	listing := &clients.StockListing{Bid: nil}
	_, ok := calculateExecutionPricePerUnit(order, listing)
	assert.False(t, ok)
}

func TestCalculateExecutionPricePerUnit_Limit_NilLimitValue_ReturnsFalse(t *testing.T) {
	t.Parallel()
	order := &Order{OrderType: TypeLimit, Direction: DirectionBuy, LimitValue: nil}
	ask := decimal.NewFromInt(50)
	listing := &clients.StockListing{Ask: &ask}
	_, ok := calculateExecutionPricePerUnit(order, listing)
	assert.False(t, ok)
}

func TestCalculateExecutionPricePerUnit_LimitBuy_NilAsk_ReturnsFalse(t *testing.T) {
	t.Parallel()
	limit := decimal.NewFromInt(50)
	order := &Order{OrderType: TypeLimit, Direction: DirectionBuy, LimitValue: &limit}
	listing := &clients.StockListing{Ask: nil}
	_, ok := calculateExecutionPricePerUnit(order, listing)
	assert.False(t, ok)
}

func TestCalculateExecutionPricePerUnit_LimitSell_NilBid_ReturnsFalse(t *testing.T) {
	t.Parallel()
	limit := decimal.NewFromInt(50)
	order := &Order{OrderType: TypeLimit, Direction: DirectionSell, LimitValue: &limit}
	listing := &clients.StockListing{Bid: nil}
	_, ok := calculateExecutionPricePerUnit(order, listing)
	assert.False(t, ok)
}

func TestCalculateExecutionPricePerUnit_Stop_ReturnsFalse(t *testing.T) {
	t.Parallel()
	order := &Order{OrderType: TypeStop, Direction: DirectionBuy}
	listing := &clients.StockListing{}
	_, ok := calculateExecutionPricePerUnit(order, listing)
	assert.False(t, ok, "STOP should not be executed directly — returns false")
}

// ---------------------------------------------------------------------------
// decimalMin / decimalMax edge cases
// ---------------------------------------------------------------------------

func TestDecimalMin_Equal_ReturnsA(t *testing.T) {
	t.Parallel()
	a := decimal.NewFromInt(5)
	result := decimalMin(a, decimal.NewFromInt(5))
	assert.True(t, result.Equal(a))
}

func TestDecimalMax_Equal_ReturnsA(t *testing.T) {
	t.Parallel()
	a := decimal.NewFromInt(7)
	result := decimalMax(a, decimal.NewFromInt(7))
	assert.True(t, result.Equal(a))
}

func TestDecimalMax_BGreater_ReturnsB(t *testing.T) {
	t.Parallel()
	result := decimalMax(decimal.NewFromInt(3), decimal.NewFromInt(10))
	assert.True(t, result.Equal(decimal.NewFromInt(10)))
}
