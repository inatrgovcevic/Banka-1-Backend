package market

import (
	"strings"
	"testing"
)

func TestParseStockExchangeCSVSupportsOptionalColumnsAndNormalization(t *testing.T) {
	payload := strings.NewReader("Exchange Name,Exchange Acronym,Exchange Mic Code,Country,Currency,Time Zone,Open Time,Close Time,Is Active\nNasdaq,NASDAQ,XNAS,USA,United States Dollar,America/New_York,09:30,16:00,yes\n")
	rows, err := parseStockExchangeCSV(payload, "memory")
	if err != nil {
		t.Fatalf("parseStockExchangeCSV returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	entity := buildImportedExchange(rows[0])
	if entity.Currency != "USD" || !entity.IsActive || entity.OpenTime != "09:30:00" || entity.CloseTime != "16:00:00" {
		t.Fatalf("unexpected imported exchange: %+v", entity)
	}
}

func TestNormalizeImportCurrencyFallsBackToOriginalForUnknownCodes(t *testing.T) {
	if got := normalizeImportCurrency("Euro"); got != "EUR" {
		t.Fatalf("expected EUR, got %s", got)
	}
	if got := normalizeImportCurrency("MXN"); got != "MXN" {
		t.Fatalf("expected MXN passthrough, got %s", got)
	}
}
