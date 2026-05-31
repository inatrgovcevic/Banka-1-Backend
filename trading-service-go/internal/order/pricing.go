package order

import (
	"context"
	"fmt"
	"strings"

	"banka1/trading-service-go/internal/clients"

	"github.com/shopspring/decimal"
)

// determineOrderType mirrors OrderCreationServiceImpl.determineOrderType: the
// type is derived purely from which of limit/stop are present.
func determineOrderType(limit, stop *decimal.Decimal) string {
	switch {
	case limit == nil && stop == nil:
		return TypeMarket
	case limit != nil && stop == nil:
		return TypeLimit
	case limit == nil:
		return TypeStop
	default:
		return TypeStopLimit
	}
}

// isMarketFamily mirrors the 0.14/$7 commission family (MARKET, STOP).
func isMarketFamily(orderType string) bool {
	return orderType == TypeMarket || orderType == TypeStop
}

// orderPricingFamily mirrors orderPricingFamily: STOP_LIMIT prices like LIMIT.
func orderPricingFamily(orderType string) string {
	if orderType == TypeStopLimit {
		return TypeLimit
	}
	return orderType
}

// referencePricePerUnit mirrors getReferencePricePerUnit. For MARKET it is the
// ask (BUY) / bid (SELL); a nil quote is an internal error (Java would NPE → 500).
func referencePricePerUnit(orderType, direction string, listing *clients.StockListing, limit, stop *decimal.Decimal) (decimal.Decimal, error) {
	switch orderType {
	case TypeMarket:
		quote := listing.Ask
		if direction == DirectionSell {
			quote = listing.Bid
		}
		if quote == nil {
			return decimal.Zero, fmt.Errorf("order: market %s quote unavailable for listing %d", direction, listing.ID)
		}
		return *quote, nil
	case TypeLimit, TypeStopLimit:
		if limit == nil {
			return decimal.Zero, fmt.Errorf("order: limit value missing")
		}
		return *limit, nil
	default: // STOP
		if stop == nil {
			return decimal.Zero, fmt.Errorf("order: stop value missing")
		}
		return *stop, nil
	}
}

// calculateApproximatePrice mirrors calculateApproximatePrice:
// referencePrice × contractSize × quantity.
func calculateApproximatePrice(orderType, direction string, listing *clients.StockListing, quantity int, limit, stop *decimal.Decimal) (decimal.Decimal, error) {
	ppu, err := referencePricePerUnit(orderType, direction, listing, limit, stop)
	if err != nil {
		return decimal.Zero, err
	}
	if listing.ContractSize == nil {
		return decimal.Zero, fmt.Errorf("order: listing %d has no contract size", listing.ID)
	}
	return ppu.Mul(decimal.NewFromInt(int64(*listing.ContractSize))).Mul(decimal.NewFromInt(int64(quantity))), nil
}

// commission mirrors calculateFee / calculateCommission (identical in Java): rate
// 0.14 (market/stop) or 0.24 (limit/stop-limit), scale 2 HALF_UP, capped at the
// $7/$12 USD equivalent. A cap-conversion failure is swallowed → uncapped
// commission (matches Java catching the exception).
func (s *Service) commission(ctx context.Context, orderType string, base decimal.Decimal, currency string) decimal.Decimal {
	rate := decimal.RequireFromString("0.24")
	capUSD := decimal.NewFromInt(12)
	if isMarketFamily(orderType) {
		rate = decimal.RequireFromString("0.14")
		capUSD = decimal.NewFromInt(7)
	}
	fee := base.Mul(rate).Round(2)
	capAmt, err := s.convertAmount(ctx, usd, currency, capUSD)
	if err != nil {
		s.logger.Warn("commission cap conversion failed, applying uncapped rate", "currency", currency, "error", err)
		return fee
	}
	if fee.GreaterThan(capAmt) {
		return capAmt
	}
	return fee
}

// convertAmount mirrors convertAmount (with commission). No-op on equal/empty
// currencies; returns the converted amount or the input when the result is missing.
func (s *Service) convertAmount(ctx context.Context, from, to string, amount decimal.Decimal) (decimal.Decimal, error) {
	if from == "" || to == "" || strings.EqualFold(from, to) {
		return amount, nil
	}
	rate, err := s.market.Calculate(ctx, from, to, amount)
	if err != nil {
		return decimal.Zero, err
	}
	if rate == nil || rate.Converted() == nil {
		return amount, nil
	}
	return *rate.Converted(), nil
}

// convertAmountNoComm mirrors convertAmountWithoutCommission.
func (s *Service) convertAmountNoComm(ctx context.Context, from, to string, amount decimal.Decimal) (decimal.Decimal, error) {
	if from == "" || to == "" || strings.EqualFold(from, to) {
		return amount, nil
	}
	rate, err := s.market.CalculateWithoutCommission(ctx, from, to, amount)
	if err != nil {
		return decimal.Zero, err
	}
	if rate == nil || rate.Converted() == nil {
		return amount, nil
	}
	return *rate.Converted(), nil
}

// calculateInitialMarginCost mirrors calculateInitialMarginCost:
// maintenanceMargin × 1.10 × quantity, scale 2 HALF_UP. maintenanceMargin comes
// from the listing, else is derived by listing type.
func calculateInitialMarginCost(listing *clients.StockListing, quantity int) (decimal.Decimal, error) {
	if listing.Price == nil {
		return decimal.Zero, fmt.Errorf("order: listing %d has no price for margin", listing.ID)
	}
	price := *listing.Price
	var maintenance decimal.Decimal
	switch {
	case listing.MaintenanceMargin != nil:
		maintenance = *listing.MaintenanceMargin
	default:
		contractSize := decimal.NewFromInt(int64(listing.ContractSizeOr(1)))
		switch listing.ListingTypeOr("STOCK") {
		case "STOCK":
			maintenance = price.Mul(decimal.RequireFromString("0.50"))
		case "FOREX", "FUTURES":
			maintenance = contractSize.Mul(price).Mul(decimal.RequireFromString("0.10"))
		case "OPTION":
			underlying := price
			if listing.UnderlyingPrice != nil {
				underlying = *listing.UnderlyingPrice
			}
			maintenance = contractSize.Mul(underlying).Mul(decimal.RequireFromString("0.50"))
		default:
			maintenance = price.Mul(decimal.RequireFromString("0.50"))
		}
	}
	return maintenance.Mul(decimal.RequireFromString("1.10")).Mul(decimal.NewFromInt(int64(quantity))).Round(2), nil
}
