package market

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"banka1/market-service-go/internal/api"
	"github.com/jackc/pgx/v5"
)

var supportedImportCurrencies = map[string]struct{}{
	"RSD": {},
	"EUR": {},
	"CHF": {},
	"USD": {},
	"GBP": {},
	"JPY": {},
	"CAD": {},
	"AUD": {},
}

var importCurrencyAliases = map[string]string{
	"us dollar":              "USD",
	"u.s. dollar":            "USD",
	"united states dollar":   "USD",
	"american dollar":        "USD",
	"dollar":                 "USD",
	"euro":                   "EUR",
	"euros":                  "EUR",
	"british pound":          "GBP",
	"british pound sterling": "GBP",
	"pound sterling":         "GBP",
	"japanese yen":           "JPY",
	"yen":                    "JPY",
	"swiss franc":            "CHF",
	"canadian dollar":        "CAD",
	"australian dollar":      "AUD",
	"serbian dinar":          "RSD",
	"dinar":                  "RSD",
}

type stockExchangeCSVRow struct {
	ExchangeName        string
	ExchangeAcronym     string
	ExchangeMICCode     string
	Polity              string
	Currency            string
	TimeZone            string
	OpenTime            string
	CloseTime           string
	PreMarketOpenTime   *string
	PreMarketCloseTime  *string
	PostMarketOpenTime  *string
	PostMarketCloseTime *string
	IsActive            bool
}

func (s *Service) ImportStockExchanges(ctx context.Context, csvPath string) (*api.StockExchangeImportResponse, error) {
	if strings.TrimSpace(csvPath) == "" {
		return &api.StockExchangeImportResponse{Source: csvPath}, nil
	}
	file, source, err := openExchangeCSV(csvPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	rows, err := parseStockExchangeCSV(file, source)
	if err != nil {
		return nil, err
	}

	created := 0
	updated := 0
	unchanged := 0
	for _, row := range rows {
		existing, err := s.repo.GetStockExchangeByMIC(ctx, row.ExchangeMICCode)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		entity := buildImportedExchange(row)
		if existing == nil {
			if err := s.repo.InsertStockExchange(ctx, entity); err != nil {
				return nil, err
			}
			created++
			continue
		}
		entity.ID = existing.ID
		if stockExchangeEquals(*existing, entity) {
			unchanged++
			continue
		}
		if err := s.repo.UpdateStockExchange(ctx, entity); err != nil {
			return nil, err
		}
		updated++
	}

	return &api.StockExchangeImportResponse{
		Source:         source,
		ProcessedRows:  len(rows),
		CreatedCount:   created,
		UpdatedCount:   updated,
		UnchangedCount: unchanged,
	}, nil
}

func openExchangeCSV(csvPath string) (*os.File, string, error) {
	candidates := []string{csvPath}
	if strings.HasPrefix(csvPath, "classpath:") {
		trimmed := strings.TrimPrefix(csvPath, "classpath:")
		trimmed = strings.TrimPrefix(trimmed, "/")
		candidates = append(candidates,
			trimmed,
			filepath.Join("resources", trimmed),
			filepath.Join("market-service-go", "resources", trimmed),
			"..\\stock-service\\src\\main\\resources\\"+trimmed,
		)
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		file, err := os.Open(candidate)
		if err == nil {
			return file, csvPath, nil
		}
	}
	return nil, "", fmt.Errorf("stock exchange CSV resource does not exist: %s", csvPath)
}

func parseStockExchangeCSV(reader io.Reader, source string) ([]stockExchangeCSVRow, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read stock exchange CSV resource: %s: %w", source, err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("stock exchange CSV is empty: %s", source)
	}
	headerIndexes := indexCSVHeaders(records[0])
	required := []string{"Exchange Name", "Exchange Acronym", "Exchange Mic Code", "Country", "Currency", "Time Zone", "Open Time", "Close Time"}
	for _, name := range required {
		if _, ok := headerIndexes[name]; !ok {
			return nil, fmt.Errorf("missing required CSV header '%s' in %s", name, source)
		}
	}
	seenMIC := map[string]struct{}{}
	rows := make([]stockExchangeCSVRow, 0, len(records)-1)
	for i, record := range records[1:] {
		lineNo := i + 2
		if isBlankRecord(record) {
			continue
		}
		row, err := mapCSVRow(record, headerIndexes, lineNo, source)
		if err != nil {
			return nil, err
		}
		if _, ok := seenMIC[row.ExchangeMICCode]; ok {
			return nil, fmt.Errorf("duplicate MIC code '%s' found in %s on row %d", row.ExchangeMICCode, source, lineNo)
		}
		seenMIC[row.ExchangeMICCode] = struct{}{}
		rows = append(rows, row)
	}
	return rows, nil
}

func indexCSVHeaders(record []string) map[string]int {
	indexes := make(map[string]int, len(record))
	for i, raw := range record {
		indexes[strings.TrimSpace(raw)] = i
	}
	return indexes
}

func mapCSVRow(record []string, headers map[string]int, lineNo int, source string) (stockExchangeCSVRow, error) {
	exchangeName, err := requiredCSVValue(record, headers, "Exchange Name", lineNo, source)
	if err != nil {
		return stockExchangeCSVRow{}, err
	}
	exchangeAcronym, err := requiredCSVValue(record, headers, "Exchange Acronym", lineNo, source)
	if err != nil {
		return stockExchangeCSVRow{}, err
	}
	micCode, err := requiredCSVValue(record, headers, "Exchange Mic Code", lineNo, source)
	if err != nil {
		return stockExchangeCSVRow{}, err
	}
	polity, err := requiredCSVValue(record, headers, "Country", lineNo, source)
	if err != nil {
		return stockExchangeCSVRow{}, err
	}
	currency, err := requiredCSVValue(record, headers, "Currency", lineNo, source)
	if err != nil {
		return stockExchangeCSVRow{}, err
	}
	timeZone, err := requiredCSVValue(record, headers, "Time Zone", lineNo, source)
	if err != nil {
		return stockExchangeCSVRow{}, err
	}
	openTime, err := parseRequiredCSVTime(record, headers, "Open Time", lineNo, source)
	if err != nil {
		return stockExchangeCSVRow{}, err
	}
	closeTime, err := parseRequiredCSVTime(record, headers, "Close Time", lineNo, source)
	if err != nil {
		return stockExchangeCSVRow{}, err
	}
	preOpen, err := parseOptionalCSVTime(optionalCSVValue(record, headers, "Pre Market Open Time"), "Pre Market Open Time", lineNo, source)
	if err != nil {
		return stockExchangeCSVRow{}, err
	}
	preClose, err := parseOptionalCSVTime(optionalCSVValue(record, headers, "Pre Market Close Time"), "Pre Market Close Time", lineNo, source)
	if err != nil {
		return stockExchangeCSVRow{}, err
	}
	postOpen, err := parseOptionalCSVTime(optionalCSVValue(record, headers, "Post Market Open Time"), "Post Market Open Time", lineNo, source)
	if err != nil {
		return stockExchangeCSVRow{}, err
	}
	postClose, err := parseOptionalCSVTime(optionalCSVValue(record, headers, "Post Market Close Time"), "Post Market Close Time", lineNo, source)
	if err != nil {
		return stockExchangeCSVRow{}, err
	}
	return stockExchangeCSVRow{
		ExchangeName:        exchangeName,
		ExchangeAcronym:     exchangeAcronym,
		ExchangeMICCode:     micCode,
		Polity:              polity,
		Currency:            currency,
		TimeZone:            timeZone,
		OpenTime:            openTime,
		CloseTime:           closeTime,
		PreMarketOpenTime:   preOpen,
		PreMarketCloseTime:  preClose,
		PostMarketOpenTime:  postOpen,
		PostMarketCloseTime: postClose,
		IsActive:            parseOptionalCSVBool(optionalCSVValue(record, headers, "Is Active")),
	}, nil
}

func requiredCSVValue(record []string, headers map[string]int, name string, lineNo int, source string) (string, error) {
	value := optionalCSVValue(record, headers, name)
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("missing value for column [%s] on row %d in %s", name, lineNo, source)
	}
	return strings.TrimSpace(value), nil
}

func optionalCSVValue(record []string, headers map[string]int, name string) string {
	index, ok := headers[name]
	if !ok || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}

func parseRequiredCSVTime(record []string, headers map[string]int, name string, lineNo int, source string) (string, error) {
	value := optionalCSVValue(record, headers, name)
	if value == "" {
		return "", fmt.Errorf("missing value for column [%s] on row %d in %s", name, lineNo, source)
	}
	return normalizeCSVTime(value, name, lineNo, source)
}

func parseOptionalCSVTime(value string, name string, lineNo int, source string) (*string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	normalized, err := normalizeCSVTime(value, name, lineNo, source)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func normalizeCSVTime(value string, name string, lineNo int, source string) (string, error) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("invalid time value '%s' for column '%s' on row %d in %s. Expected HH:mm format", value, name, lineNo, source)
	}
	return parsed.Format("15:04:05"), nil
}

func parseOptionalCSVBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return true
	}
}

func isBlankRecord(record []string) bool {
	for _, value := range record {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func buildImportedExchange(row stockExchangeCSVRow) StockExchange {
	currency := normalizeImportCurrency(row.Currency)
	_, supported := supportedImportCurrencies[currency]
	return StockExchange{
		ExchangeName:        row.ExchangeName,
		ExchangeAcronym:     row.ExchangeAcronym,
		ExchangeMICCode:     row.ExchangeMICCode,
		Polity:              row.Polity,
		Currency:            currency,
		TimeZone:            row.TimeZone,
		OpenTime:            row.OpenTime,
		CloseTime:           row.CloseTime,
		PreMarketOpenTime:   row.PreMarketOpenTime,
		PreMarketCloseTime:  row.PreMarketCloseTime,
		PostMarketOpenTime:  row.PostMarketOpenTime,
		PostMarketCloseTime: row.PostMarketCloseTime,
		IsActive:            row.IsActive && supported,
	}
}

func normalizeImportCurrency(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	upper := strings.ToUpper(trimmed)
	if _, ok := supportedImportCurrencies[upper]; ok {
		return upper
	}
	if alias, ok := importCurrencyAliases[strings.ToLower(trimmed)]; ok {
		return alias
	}
	return trimmed
}

func stockExchangeEquals(left, right StockExchange) bool {
	return left.ExchangeName == right.ExchangeName &&
		left.ExchangeAcronym == right.ExchangeAcronym &&
		left.ExchangeMICCode == right.ExchangeMICCode &&
		left.Polity == right.Polity &&
		left.Currency == right.Currency &&
		left.TimeZone == right.TimeZone &&
		left.OpenTime == right.OpenTime &&
		left.CloseTime == right.CloseTime &&
		stringPtrEqual(left.PreMarketOpenTime, right.PreMarketOpenTime) &&
		stringPtrEqual(left.PreMarketCloseTime, right.PreMarketCloseTime) &&
		stringPtrEqual(left.PostMarketOpenTime, right.PostMarketOpenTime) &&
		stringPtrEqual(left.PostMarketCloseTime, right.PostMarketCloseTime) &&
		left.IsActive == right.IsActive
}

func stringPtrEqual(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func (s *Service) RefreshAllStocks(ctx context.Context) ([]api.StockMarketDataRefreshResponse, error) {
	items, err := s.repo.ListStockListings(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]api.StockMarketDataRefreshResponse, 0, len(items))
	for idx, item := range items {
		resp, err := s.RefreshStockByTicker(ctx, item.Ticker)
		if err == nil {
			out = append(out, *resp)
		} else {
			s.logger.Warn("stock refresh failed", "ticker", item.Ticker, "error", err.Error())
		}
		if idx < len(items)-1 {
			select {
			case <-ctx.Done():
				return out, ctx.Err()
			case <-time.After(12 * time.Second):
			}
		}
	}
	return out, nil
}

func (s *Service) RefreshOpenListings(ctx context.Context) error {
	items, err := s.repo.ListAllListings(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	forexOpen := now.Weekday() != time.Saturday && now.Weekday() != time.Sunday
	openByExchange := map[int64]bool{}
	for _, item := range items {
		switch item.ListingType {
		case ListingTypeForex:
			if !forexOpen {
				continue
			}
		case ListingTypeStock, ListingTypeFutures:
			open, ok := openByExchange[item.StockExchangeID]
			if !ok {
				status, err := s.GetExchangeStatus(ctx, item.StockExchangeID)
				if err != nil {
					continue
				}
				open = status.Open
				openByExchange[item.StockExchangeID] = open
			}
			if !open || item.ListingType == ListingTypeFutures {
				continue
			}
		}
		_, _ = s.RefreshListing(ctx, item.ID)
	}
	return nil
}
