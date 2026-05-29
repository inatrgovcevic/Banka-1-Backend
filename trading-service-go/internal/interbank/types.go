// Package interbank serves the /internal/interbank* endpoints (all SERVICE-gated)
// that interbank-service calls — via TradingInternalClient — to reserve, commit,
// and release this bank's stock and OTC-option holdings as part of the Tim 2
// inter-bank 2PC protocol. It mirrors com.banka1.tradingservice.interbank.* over
// two tables plus the shared `portfolio` table:
//
//   - interbank_stock_reservations  (order-service Liquibase; reservation_id UUID)
//   - interbank_option_reservations (trading-service Liquibase; negotiation_id PK)
//
// The 2PC protocol itself, the negotiation/retry scheduler, and the mock bank-2
// live in the SEPARATE interbank-service microservice (out of scope here). This
// package only exposes the synchronous primitives that service invokes:
//
//   - reserveStock  — reserved_quantity += qty (quantity untouched), HELD row
//   - commitStock   — quantity -= qty AND reserved_quantity -= qty, COMMITTED
//   - releaseStock  — reserved_quantity -= qty only, RELEASED
//
// plus a thin option lifecycle (reserve/exercise/release) keyed by negotiationId
// and a public-stocks listing tagged with this bank's routing number. There is no
// RabbitMQ, saga, or scheduler on the trading side — every method is one
// gpdb.RunInTx (Java @Transactional). The `trading` schema is owned by Java
// Liquibase; this service runs no migrations.
package interbank

// Stock reservation lifecycle (interbank_stock_reservations.status). HELD on
// reserve; COMMITTED / RELEASED are terminal (a repeated transition into the same
// terminal state is an idempotent no-op).
const (
	StatusHeld      = "HELD"
	StatusCommitted = "COMMITTED"
	StatusReleased  = "RELEASED"
)

// Option reservation lifecycle (interbank_option_reservations.status). RESERVED
// after accept; EXERCISED / RELEASED are terminal.
const (
	OptionReserved  = "RESERVED"
	OptionExercised = "EXERCISED"
	OptionReleased  = "RELEASED"
)

// StockReservation carries the interbank_stock_reservations columns the 2PC logic
// reads on commit/release (the row is resolved by reservation_id). The write-only
// columns (transaction ids, ticker, timestamps) are not scanned.
type StockReservation struct {
	ReservationID string
	PortfolioID   int64
	Quantity      int
	Status        string
}

// OptionReservation carries the interbank_option_reservations columns the option
// lifecycle reads (resolved by negotiation_id PK): the mapped stock reservationId
// and the current status.
type OptionReservation struct {
	NegotiationID string
	ReservationID string
	Status        string
}
