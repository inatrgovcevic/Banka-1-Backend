// Package otc serves the /otc (authenticated) and /stocks/internal (public, saga-
// orchestrator-driven) endpoints, the OTC offer + option-contract state machines,
// the OTC_PREMIUM_TRANSFER / OTC_EXERCISE saga publisher + consumers, the daily
// expire + reminder schedulers, and the OTC notifications. It mirrors
// trading-service com.banka1.tradingservice.otc.* over the trading-service-owned
// tables `otc_offers`, `option_contracts`, `otc_negotiation_history`,
// `otc_contract_expiry_reminders`, `stock_reservations`,
// `stock_ownership_transfers`, plus the shared `portfolio` table.
//
// The `trading` schema is owned by Java Liquibase; this service runs no
// migrations. NUMERIC columns (price_per_stock / premium, scale 2 RSD) are read
// via ::text into shopspring/decimal to preserve scale, matching P3/P4/P5.
package otc

import (
	"time"

	"github.com/shopspring/decimal"
)

// OtcOfferStatus values mirror the Java enum (stored as the name string).
const (
	OfferPendingSeller = "PENDING_SELLER"
	OfferPendingBuyer  = "PENDING_BUYER"
	OfferAccepted      = "ACCEPTED"
	OfferRejected      = "REJECTED"
	OfferWithdrawn     = "WITHDRAWN"
	OfferExpired       = "EXPIRED"
)

// OptionContractStatus values mirror the Java enum.
const (
	ContractPendingPremium = "PENDING_PREMIUM"
	ContractActive         = "ACTIVE"
	ContractExercised      = "EXERCISED"
	ContractExpired        = "EXPIRED"
	ContractCanceled       = "CANCELED"
)

// OtcNegotiationEventType values mirror the Java enum.
const (
	EventCreated        = "CREATED"
	EventCounterOffered = "COUNTER_OFFERED"
	EventAccepted       = "ACCEPTED"
	EventRejected       = "REJECTED"
	EventWithdrawn      = "WITHDRAWN"
	EventExpired        = "EXPIRED"
)

// Stock reservation / ownership-transfer lifecycle statuses (stored in
// stock_reservations.status / stock_ownership_transfers.status).
const (
	ReservationHeld      = "HELD"
	ReservationReleased  = "RELEASED"
	ReservationCommitted = "COMMITTED"
	ReservationUnknown   = "UNKNOWN"

	TransferCompleted = "COMPLETED"
	TransferReversed  = "REVERSED"
)

// OtcOffer mirrors a row of `otc_offers`. modified_by is nullable (the Java
// String column). version is the JPA @Version optimistic-lock counter.
type OtcOffer struct {
	ID             int64
	StockTicker    string
	BuyerID        int64
	SellerID       int64
	Amount         int
	PricePerStock  decimal.Decimal
	Premium        decimal.Decimal
	SettlementDate time.Time
	Status         string
	ModifiedBy     *string
	LastModified   time.Time
	CreatedAt      time.Time
	Version        int64
}

// OptionContract mirrors a row of `option_contracts`. exercised_at is nullable
// (set when the buyer triggers exercise; the contract stays ACTIVE until the
// saga completion listener flips it to EXERCISED).
type OptionContract struct {
	ID             int64
	OfferID        int64
	StockTicker    string
	BuyerID        int64
	SellerID       int64
	Amount         int
	PricePerStock  decimal.Decimal
	SettlementDate time.Time
	Status         string
	CreatedAt      time.Time
	ExercisedAt    *time.Time
	Version        int64
}

// NegotiationHistory mirrors a row of `otc_negotiation_history`. The old_* fields
// are nil on the initial CREATED event (before == null in Java). old_status /
// new_status carry the OtcOfferStatus name string.
type NegotiationHistory struct {
	ID                int64
	OfferID           int64
	BuyerID           int64
	SellerID          int64
	ActorID           *int64
	ActorName         *string
	EventType         string
	StockTicker       string
	OldAmount         *int
	NewAmount         *int
	OldPricePerStock  *decimal.Decimal
	NewPricePerStock  *decimal.Decimal
	OldPremium        *decimal.Decimal
	NewPremium        *decimal.Decimal
	OldSettlementDate *time.Time
	NewSettlementDate *time.Time
	OldStatus         *string
	NewStatus         *string
	ChangedAt         time.Time
}

// --- Saga events (publish, on saga.events) ---------------------------------

// PremiumTransferRequestedEvent mirrors OtcService.OtcPremiumTransferRequestedEvent.
// Published on saga.events / otc.premium.transfer.requested AFTER the accept
// transaction commits.
type PremiumTransferRequestedEvent struct {
	ContractID int64           `json:"contractId"`
	BuyerID    int64           `json:"buyerId"`
	SellerID   int64           `json:"sellerId"`
	Premium    decimal.Decimal `json:"premium"`
}

// ExerciseRequestedEvent mirrors OtcService.OtcExerciseRequestedEvent. Published
// on saga.events / otc.exercise.requested AFTER the exercise transaction commits.
type ExerciseRequestedEvent struct {
	ContractID    int64           `json:"contractId"`
	BuyerID       int64           `json:"buyerId"`
	SellerID      int64           `json:"sellerId"`
	StockTicker   string          `json:"stockTicker"`
	Amount        int             `json:"amount"`
	PricePerStock decimal.Decimal `json:"pricePerStock"`
}

// sagaContractEvent is the loose-shape payload the three OTC saga-result
// consumers receive on saga.events. The Java listener records carry only
// contractId (+ reason for the premium-failed event); contractId may arrive as
// int64 or float64 depending on the publisher, so it is an optional pointer.
type sagaContractEvent struct {
	ContractID *int64  `json:"contractId"`
	Reason     *string `json:"reason"`
}
