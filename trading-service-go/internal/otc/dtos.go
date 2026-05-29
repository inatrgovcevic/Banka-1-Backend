package otc

import (
	"banka1/trading-service-go/internal/api"

	"github.com/shopspring/decimal"
)

// These are the service-layer DTOs returned to the HTTP handler. JSON tags match
// the Java OTC DTO field names exactly (camelCase); the frontend depends on them
// being byte-compatible. LocalDate / LocalDateTime use the Jackson-style
// marshalers in internal/api so dates render "2026-05-26" and datetimes
// "2026-05-26T16:59:55.337" with no timezone.

// OtcOfferDto mirrors OtcOfferDto (offer response). modifiedBy is nullable.
type OtcOfferDto struct {
	ID             int64             `json:"id"`
	StockTicker    string            `json:"stockTicker"`
	BuyerID        int64             `json:"buyerId"`
	SellerID       int64             `json:"sellerId"`
	Amount         int               `json:"amount"`
	PricePerStock  decimal.Decimal   `json:"pricePerStock"`
	Premium        decimal.Decimal   `json:"premium"`
	SettlementDate api.LocalDate     `json:"settlementDate"`
	Status         string            `json:"status"`
	ModifiedBy     *string           `json:"modifiedBy"`
	LastModified   api.LocalDateTime `json:"lastModified"`
}

// OptionContractDto mirrors OptionContractDto (signed-contract response).
// exercisedAt is null until the buyer triggers exercise.
type OptionContractDto struct {
	ID             int64             `json:"id"`
	OfferID        int64             `json:"offerId"`
	StockTicker    string            `json:"stockTicker"`
	BuyerID        int64             `json:"buyerId"`
	SellerID       int64             `json:"sellerId"`
	Amount         int               `json:"amount"`
	PricePerStock  decimal.Decimal   `json:"pricePerStock"`
	SettlementDate api.LocalDate     `json:"settlementDate"`
	Status         string            `json:"status"`
	CreatedAt      api.LocalDateTime `json:"createdAt"`
	ExercisedAt    api.LocalDateTime `json:"exercisedAt"`
}

// OtcPositionDto mirrors OtcPositionDto (an OTC-exposed STOCK position).
// stockTicker is nullable (market lookup may fail to resolve the listing).
type OtcPositionDto struct {
	ID                int64   `json:"id"`
	ListingID         int64   `json:"listingId"`
	StockTicker       *string `json:"stockTicker"`
	TotalQuantity     int     `json:"totalQuantity"`
	ReservedQuantity  int     `json:"reservedQuantity"`
	PublicQuantity    int     `json:"publicQuantity"`
	AvailableQuantity int     `json:"availableQuantity"`
}

// PublicStockDto mirrors PublicStockDto — public OTC stocks grouped by ticker.
type PublicStockDto struct {
	Ticker  string                 `json:"ticker"`
	Sellers []PublicStockSellerDto `json:"sellers"`
}

// PublicStockSellerDto mirrors PublicStockSellerDto. availableQuantity carries the
// seller's advertised publicQuantity (matching Java's PublicStockSellerDto(userId,
// name, qty) where qty = publicQuantity). sellerName is nullable.
type PublicStockSellerDto struct {
	SellerID          int64   `json:"sellerId"`
	SellerName        *string `json:"sellerName"`
	AvailableQuantity int     `json:"availableQuantity"`
}

// OtcNegotiationHistoryResponse mirrors OtcNegotiationHistoryResponse. The old_*
// fields are null for the initial CREATED event. eventType / status enums render
// as their name strings.
type OtcNegotiationHistoryResponse struct {
	ID                int64             `json:"id"`
	OfferID           int64             `json:"offerId"`
	BuyerID           int64             `json:"buyerId"`
	SellerID          int64             `json:"sellerId"`
	ActorID           *int64            `json:"actorId"`
	ActorName         *string           `json:"actorName"`
	EventType         string            `json:"eventType"`
	StockTicker       string            `json:"stockTicker"`
	OldAmount         *int              `json:"oldAmount"`
	NewAmount         *int              `json:"newAmount"`
	OldPricePerStock  *decimal.Decimal  `json:"oldPricePerStock"`
	NewPricePerStock  *decimal.Decimal  `json:"newPricePerStock"`
	OldPremium        *decimal.Decimal  `json:"oldPremium"`
	NewPremium        *decimal.Decimal  `json:"newPremium"`
	OldSettlementDate api.LocalDate     `json:"oldSettlementDate"`
	NewSettlementDate api.LocalDate     `json:"newSettlementDate"`
	OldStatus         *string           `json:"oldStatus"`
	NewStatus         *string           `json:"newStatus"`
	ChangedAt         api.LocalDateTime `json:"changedAt"`
}

// ReservationResponse mirrors StockReservationService.Reservation record
// {reservationId, status} — the /stocks/internal reserve + release response.
type ReservationResponse struct {
	ReservationID string `json:"reservationId"`
	Status        string `json:"status"`
}

// OwnershipTransferResponse mirrors StockReservationService.OwnershipTransfer
// record {ownershipTransferId, status} — the /stocks/internal transfer response.
type OwnershipTransferResponse struct {
	OwnershipTransferID string `json:"ownershipTransferId"`
	Status              string `json:"status"`
}

// NegotiationHistoryFilter carries the GET /otc/offers/history query params
// (bound from status / otherPartyId / dateFrom / dateTo). All optional.
type NegotiationHistoryFilter struct {
	Status       *string
	OtherPartyID *int64
	DateFrom     *string // ISO yyyy-MM-dd
	DateTo       *string // ISO yyyy-MM-dd
}
