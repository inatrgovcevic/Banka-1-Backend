// Package events defines all trigger event types consumed by the saga
// orchestrator (inbound from trading-service, banking-core, etc.) as well as
// result event types published back onto the bus after saga completion or failure.
//
// Routing key ↔ struct mapping:
//
//	otc.exercise.requested                  → OtcExerciseRequested
//	otc.premium.transfer.requested          → OtcPremiumTransferRequested
//	fund.subscribe.requested                → FundSubscribeRequested
//	fund.redeem.requested                   → FundRedeemRequested
//	fund.redeem.with-liquidation.requested  → FundRedeemWithLiquidationRequested
//
// Published result routing keys follow the Java conventions (preserved for
// wire-compat with other Java consumers in trading-service, banking-core):
//
//	saga.OTC_EXERCISE.completed / failed
//	saga.OTC_PREMIUM_TRANSFER.completed / failed
//	saga.FUND_SUBSCRIBE.STEP_1.fund.success / failure
//	saga.FUND_REDEEM.STEP_1.fund.success / failure
//	saga.FUND_LIQUIDATION_FOR_REDEMPTION.STEP_2.fund.success / failure
package events

import "github.com/shopspring/decimal"

// ---------------------------------------------------------------------------
// Trigger events (consumed by the orchestrator from downstream queues)
// ---------------------------------------------------------------------------

// OtcExerciseRequested is published by trading-service when a buyer exercises
// an option contract. correlationId = contractId (string form).
type OtcExerciseRequested struct {
	ContractID      int64           `json:"contractId"`
	BuyerID         int64           `json:"buyerId"`
	SellerID        int64           `json:"sellerId"`
	StockTicker     string          `json:"stockTicker"`
	Amount          int             `json:"amount"`
	PricePerStock   decimal.Decimal `json:"pricePerStock"`
	Premium         decimal.Decimal `json:"premium"`
	PremiumCurrency string          `json:"premiumCurrency"`
}

// OtcPremiumTransferRequested is published by trading-service when an OTC offer
// is accepted, triggering a one-step premium transfer from buyer to seller.
// correlationId = contractId.
type OtcPremiumTransferRequested struct {
	ContractID      int64           `json:"contractId"`
	BuyerID         int64           `json:"buyerId"`
	SellerID        int64           `json:"sellerId"`
	Premium         decimal.Decimal `json:"premium"`
	Currency        string          `json:"currency"`
}

// FundSubscribeRequested is published by trading-service when a client invests
// in an investment fund. correlationId = transactionId.
type FundSubscribeRequested struct {
	TransactionID     string          `json:"transactionId"`
	Amount            decimal.Decimal `json:"amount"`
	FromAccountNumber string          `json:"fromAccountNumber"` // client account
	FundAccountNumber string          `json:"fundAccountNumber"` // fund's RSD account
	FundID            int64           `json:"fundId"`
}

// FundRedeemRequested is published when a fund has sufficient liquid cash to
// cover the redemption — direct transfer without liquidation.
// correlationId = transactionId.
type FundRedeemRequested struct {
	TransactionID     string          `json:"transactionId"`
	Amount            decimal.Decimal `json:"amount"`
	FromAccountNumber string          `json:"fromAccountNumber"` // fund account (debit side)
	ToAccountNumber   string          `json:"toAccountNumber"`   // client account (credit side)
	FundID            int64           `json:"fundId"`
}

// FundRedeemWithLiquidationRequested is published when the fund must sell
// holdings first to cover the redemption (insufficient liquid cash).
// correlationId = transactionId.
type FundRedeemWithLiquidationRequested struct {
	TransactionID     string          `json:"transactionId"`
	Amount            decimal.Decimal `json:"amount"`
	FundID            int64           `json:"fundId"`
	FundAccountNumber string          `json:"fundAccountNumber"` // fund account to transfer from after liquidation
	ToAccountNumber   string          `json:"toAccountNumber"`   // client account (credit)
}

// ---------------------------------------------------------------------------
// Result events (published by the orchestrator after saga completion)
// ---------------------------------------------------------------------------

// OtcExerciseCompleted is published on saga.OTC_EXERCISE.completed.
type OtcExerciseCompleted struct {
	ContractID int64 `json:"contractId"`
}

// OtcExerciseFailed is published on saga.OTC_EXERCISE.failed.
type OtcExerciseFailed struct {
	ContractID    int64  `json:"contractId"`
	FailureReason string `json:"failureReason"`
	FailedAtStep  int    `json:"failedAtStep"`
}

// OtcPremiumTransferCompleted is published on saga.OTC_PREMIUM_TRANSFER.completed.
type OtcPremiumTransferCompleted struct {
	ContractID int64  `json:"contractId"`
	TransferID string `json:"transferId"`
}

// OtcPremiumTransferFailed is published on saga.OTC_PREMIUM_TRANSFER.failed.
type OtcPremiumTransferFailed struct {
	ContractID    int64  `json:"contractId"`
	FailureReason string `json:"failureReason"`
}

// FundSubscribeCompleted is published on saga.FUND_SUBSCRIBE.STEP_1.fund.success.
// It includes all original event fields plus the banking-core transferId so
// trading-service can correlate and flip the ClientFundTransaction to COMPLETED.
type FundSubscribeCompleted struct {
	TransactionID     string          `json:"transactionId"`
	Amount            decimal.Decimal `json:"amount"`
	FromAccountNumber string          `json:"fromAccountNumber"`
	FundAccountNumber string          `json:"fundAccountNumber"`
	FundID            int64           `json:"fundId"`
	TransferID        string          `json:"transferId"`
}

// FundSubscribeFailed is published on saga.FUND_SUBSCRIBE.STEP_1.fund.failure.
type FundSubscribeFailed struct {
	TransactionID string `json:"transactionId"`
	FundID        int64  `json:"fundId"`
	FailureReason string `json:"failureReason"`
}

// FundRedeemCompleted is published on saga.FUND_REDEEM.STEP_1.fund.success.
type FundRedeemCompleted struct {
	TransactionID   string          `json:"transactionId"`
	Amount          decimal.Decimal `json:"amount"`
	FundID          int64           `json:"fundId"`
	ToAccountNumber string          `json:"toAccountNumber"`
	TransferID      string          `json:"transferId"`
}

// FundRedeemFailed is published on saga.FUND_REDEEM.STEP_1.fund.failure.
type FundRedeemFailed struct {
	TransactionID string `json:"transactionId"`
	FundID        int64  `json:"fundId"`
	FailureReason string `json:"failureReason"`
}

// FundLiquidationCompleted is published on saga.FUND_LIQUIDATION_FOR_REDEMPTION.STEP_2.fund.success.
type FundLiquidationCompleted struct {
	TransactionID   string          `json:"transactionId"`
	Amount          decimal.Decimal `json:"amount"`
	FundID          int64           `json:"fundId"`
	ToAccountNumber string          `json:"toAccountNumber"`
	TransferID      string          `json:"transferId"`
	LiquidationID   string          `json:"liquidationId"`
}

// FundLiquidationFailed is published on saga.FUND_LIQUIDATION_FOR_REDEMPTION.STEP_X.fund.failure.
type FundLiquidationFailed struct {
	TransactionID string `json:"transactionId"`
	FundID        int64  `json:"fundId"`
	FailureReason string `json:"failureReason"`
	FailedAtStep  int    `json:"failedAtStep"`
}
