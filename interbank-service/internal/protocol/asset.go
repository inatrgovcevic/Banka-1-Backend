package protocol

import (
	"encoding/json"
	"fmt"
)

// AssetType is the discriminator value for the Asset oneof.
// Java: @JsonTypeInfo property="type" with @JsonSubTypes MONAS/STOCK/OPTION
type AssetType string

const (
	AssetTypeMonas  AssetType = "MONAS"
	AssetTypeStock  AssetType = "STOCK"
	AssetTypeOption AssetType = "OPTION"
)

// Asset is one of *MonasAsset / *StockAsset / *OptionAsset.
//
// Wire form matches the Java sealed interface with records:
//
//	Asset.Monas(MonetaryAsset asset)   → {"type":"MONAS","asset":{"currency":"USD"}}
//	Asset.Stock(StockDescription asset) → {"type":"STOCK","asset":{"ticker":"AAPL"}}
//	Asset.Option(OptionDescription asset) → {"type":"OPTION","asset":{...nested...}}
//
// The field name "asset" in each record is preserved by Jackson as the JSON key.
type Asset interface {
	Type() AssetType
	isAsset()
}

// MonasAsset is a money-of-account asset (specific currency).
// Java: record Monas(MonetaryAsset asset) where MonetaryAsset is record MonetaryAsset(CurrencyCode currency)
// Wire: {"type":"MONAS","asset":{"currency":"USD"}}
type MonasAsset struct {
	Currency string `json:"currency" validate:"required,oneof=RSD EUR USD CHF JPY AUD CAD GBP"`
}

func (MonasAsset) Type() AssetType { return AssetTypeMonas }
func (MonasAsset) isAsset()        {}

// StockAsset references a tradeable stock by ticker.
// Java: record Stock(StockDescription asset) where StockDescription is record StockDescription(String ticker)
// Wire: {"type":"STOCK","asset":{"ticker":"AAPL"}}
type StockAsset struct {
	Ticker string `json:"ticker" validate:"required,max=16"`
}

func (StockAsset) Type() AssetType { return AssetTypeStock }
func (StockAsset) isAsset()        {}

// OptionAsset wraps an OptionDescription.
// Java: record Option(OptionDescription asset)
// Wire: {"type":"OPTION","asset":{negotiationId, stock, pricePerUnit, settlementDate, amount}}
type OptionAsset struct {
	OptionDescription
}

func (OptionAsset) Type() AssetType { return AssetTypeOption }
func (OptionAsset) isAsset()        {}

// UnmarshalAsset reads the {"type":"X","asset":{...}} envelope and dispatches
// to the appropriate concrete type. This mirrors Jackson's @JsonTypeInfo behavior.
func UnmarshalAsset(raw json.RawMessage) (Asset, error) {
	var env struct {
		Type  AssetType       `json:"type"`
		Asset json.RawMessage `json:"asset"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("asset envelope: %w", err)
	}
	switch env.Type {
	case AssetTypeMonas:
		var inner struct {
			Currency string `json:"currency"`
		}
		if err := json.Unmarshal(env.Asset, &inner); err != nil {
			return nil, fmt.Errorf("monas asset inner: %w", err)
		}
		return &MonasAsset{Currency: inner.Currency}, nil
	case AssetTypeStock:
		var inner struct {
			Ticker string `json:"ticker"`
		}
		if err := json.Unmarshal(env.Asset, &inner); err != nil {
			return nil, fmt.Errorf("stock asset inner: %w", err)
		}
		return &StockAsset{Ticker: inner.Ticker}, nil
	case AssetTypeOption:
		var desc OptionDescription
		if err := json.Unmarshal(env.Asset, &desc); err != nil {
			return nil, fmt.Errorf("option asset inner: %w", err)
		}
		return &OptionAsset{OptionDescription: desc}, nil
	default:
		return nil, fmt.Errorf("unknown asset type %q", env.Type)
	}
}

// MarshalJSON for each variant emits the {"type":"X","asset":{...}} envelope,
// matching the Java Jackson output where the record field "asset" becomes the
// JSON key alongside the injected "type" discriminator.

func (m *MonasAsset) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type  AssetType `json:"type"`
		Asset struct {
			Currency string `json:"currency"`
		} `json:"asset"`
	}{
		Type: AssetTypeMonas,
		Asset: struct {
			Currency string `json:"currency"`
		}{Currency: m.Currency},
	})
}

func (s *StockAsset) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type  AssetType `json:"type"`
		Asset struct {
			Ticker string `json:"ticker"`
		} `json:"asset"`
	}{
		Type: AssetTypeStock,
		Asset: struct {
			Ticker string `json:"ticker"`
		}{Ticker: s.Ticker},
	})
}

func (o *OptionAsset) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type  AssetType         `json:"type"`
		Asset OptionDescription `json:"asset"`
	}{Type: AssetTypeOption, Asset: o.OptionDescription})
}
