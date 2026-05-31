package market

import (
	"testing"
	"time"

	"banka1/market-service-go/internal/api"
	"github.com/shopspring/decimal"
)

func TestListingSummarySettlementDateFutures(t *testing.T) {
	settlement := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	listings := []Listing{
		{ID: 1, ListingType: ListingTypeFutures, Ticker: "ESZ26", Name: "E-mini S&P", ExchangeMICCode: "XCME", Currency: "USD",
			Price: "5000.00", Ask: "5000.50", Bid: "4999.50", Change: "10.00", Volume: 1000, SettlementDate: &settlement},
		{ID: 2, ListingType: ListingTypeFutures, Ticker: "NQZ26", Name: "Nasdaq 100", ExchangeMICCode: "XCME", Currency: "USD",
			Price: "20000.00", Ask: "20000.50", Bid: "19999.50", Change: "5.00", Volume: 500, SettlementDate: nil},
	}

	resp := buildFuturesSummariesForTest(listings)
	if len(resp) != 2 {
		t.Fatalf("expected 2 summary rows, got %d", len(resp))
	}
	if resp[0].SettlementDate == nil || *resp[0].SettlementDate != "2026-12-31" {
		t.Fatalf("expected futures summary[0] settlementDate 2026-12-31, got %v", resp[0].SettlementDate)
	}
	if resp[1].SettlementDate != nil {
		t.Fatalf("expected futures summary[1] settlementDate nil, got %v", *resp[1].SettlementDate)
	}
}

func buildFuturesSummariesForTest(items []Listing) []api.ListingSummaryResponse {
	out := make([]api.ListingSummaryResponse, 0, len(items))
	for _, item := range items {
		initialMargin := calculateInitialMargin(item.ListingType, item.Price, nil)
		var settlementDate *string
		if item.ListingType == ListingTypeFutures && item.SettlementDate != nil {
			value := item.SettlementDate.Format("2006-01-02")
			settlementDate = &value
		}
		out = append(out, api.ListingSummaryResponse{
			ListingID:         item.ID,
			ListingType:       string(item.ListingType),
			Ticker:            item.Ticker,
			Name:              item.Name,
			ExchangeMICCode:   item.ExchangeMICCode,
			Currency:          item.Currency,
			Price:             dec(item.Price),
			Change:            dec(item.Change),
			Volume:            item.Volume,
			InitialMarginCost: initialMargin,
			SettlementDate:    settlementDate,
		})
	}
	return out
}

func TestInitialMarginForFuturesAndForex(t *testing.T) {
	stock := calculateInitialMargin(ListingTypeStock, "100", nil)
	futures := calculateInitialMargin(ListingTypeFutures, "100", int32ptr(50))
	forex := calculateInitialMargin(ListingTypeForex, "1.20", nil)

	if !stock.Equal(decimal.RequireFromString("55")) {
		t.Fatalf("stock margin expected 100*0.5*1.10 = 55, got %s", stock)
	}
	if !futures.Equal(decimal.RequireFromString("550")) {
		t.Fatalf("futures margin expected 50*100*0.1*1.10 = 550, got %s", futures)
	}
	if !forex.Equal(decimal.RequireFromString("132")) {
		t.Fatalf("forex margin expected 1000*1.20*0.1*1.10 = 132, got %s", forex)
	}
}

