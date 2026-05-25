package service

import "errors"

// Domain error sentinels for the OTC negotiation and coordinator layer.
// Handlers map these to appropriate HTTP status codes:
//   - ErrNegotiationNotFound   → 404
//   - ErrNegotiationClosed     → 409
//   - ErrTurnViolation         → 409
//   - ErrNegotiationInvalid    → 400
//   - ErrSenderNotParty        → 403

var (
	// ErrNegotiationNotFound is returned when a negotiation row cannot be
	// located by the {routing, id} authoritative reference pair.
	ErrNegotiationNotFound = errors.New("service: negotiation not found")

	// ErrNegotiationClosed is returned when an operation requires an open
	// negotiation but the negotiation has is_ongoing=false or its settlement
	// date has passed.
	ErrNegotiationClosed = errors.New("service: negotiation closed")

	// ErrTurnViolation is returned per Tim 2 §6.3 when the sender's routing
	// matches the last_modified_by routing — it is not the sender's turn.
	ErrTurnViolation = errors.New("service: turn violation")

	// ErrNegotiationInvalid is returned for malformed payloads (past
	// settlement date, negative/zero amounts, routing number mismatches, etc.).
	ErrNegotiationInvalid = errors.New("service: negotiation payload invalid")

	// ErrSenderNotParty is returned when the authenticated sender is not
	// either the buyer or seller in the negotiation (multi-bank safety guard).
	ErrSenderNotParty = errors.New("service: sender is not a party to this negotiation")

	// ErrInterbankProtocol signals a protocol-level failure during 2PC
	// (e.g. local prepare failed, partner rejected, catastrophic commit).
	ErrInterbankProtocol = errors.New("service: inter-bank protocol failure")
)
