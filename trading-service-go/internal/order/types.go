package order

import "strings"

// Order types (OrderType enum).
const (
	TypeMarket    = "MARKET"
	TypeLimit     = "LIMIT"
	TypeStop      = "STOP"
	TypeStopLimit = "STOP_LIMIT"
)

// Order statuses (OrderStatus enum).
const (
	StatusPendingConfirmation = "PENDING_CONFIRMATION"
	StatusPending             = "PENDING"
	StatusApproved            = "APPROVED"
	StatusDeclined            = "DECLINED"
	StatusDone                = "DONE"
	StatusCancelled           = "CANCELLED"
)

// Order directions (OrderDirection enum).
const (
	DirectionBuy  = "BUY"
	DirectionSell = "SELL"
)

// purchaseFor values (PurchaseFor enum).
const (
	PurchaseForBank           = "BANK"
	PurchaseForInvestmentFund = "INVESTMENT_FUND"
)

// approvedBy sentinels (OrderCreationServiceImpl).
const (
	noApprovalRequired int64 = -1 // NO_APPROVAL_REQUIRED
	systemApproval     int64 = -2 // SYSTEM_APPROVAL
)

const (
	limitCurrency = "RSD"
	usd           = "USD"
)

// validOverviewFilters mirrors OrderOverviewStatusFilter (the supervisor portal
// status filter): ALL plus the lifecycle statuses.
var validOverviewFilters = map[string]bool{
	"ALL": true, StatusPending: true, StatusApproved: true,
	StatusDeclined: true, StatusDone: true, StatusCancelled: true,
}

// ParseStatusFilter validates the GET /orders ?status param. Empty -> "ALL"
// (defaultValue). An unknown value returns ok=false so the handler can reproduce
// Spring's MethodArgumentTypeMismatchException (400).
func ParseStatusFilter(raw string) (string, bool) {
	if raw == "" {
		return "ALL", true
	}
	upper := strings.ToUpper(raw)
	if validOverviewFilters[upper] {
		return upper, true
	}
	return "", false
}

// AuthUser is the order-module view of the caller, mirroring order-service
// AuthenticatedUser. Built from the JWT principal in the HTTP layer. Roles is a
// set (the platform principal carries a single role string; the handler seeds
// the slice with it).
type AuthUser struct {
	UserID      int64
	Roles       []string
	Permissions []string
}

// marginPermissions mirrors AuthenticatedUser.MARGIN_PERMISSIONS.
var marginPermissions = map[string]bool{
	"MARGIN_TRADE": true, "SECURITIES_TRADE_MARGIN": true, "MARGIN": true,
}

// tradingPermissions mirrors AuthenticatedUser.TRADING_PERMISSIONS.
var tradingPermissions = map[string]bool{
	"SECURITIES_TRADE": true, "SECURITIES_TRADE_LIMITED": true, "SECURITIES_TRADE_UNLIMITED": true,
	"TRADING_BASIC": true, "TRADING_ADVANCED": true,
}

// HasRole mirrors AuthenticatedUser.hasRole (case-insensitive, exact — no role
// hierarchy; the @PreAuthorize gate already applied hierarchy upstream).
func (u AuthUser) HasRole(role string) bool {
	for _, r := range u.Roles {
		if strings.EqualFold(r, role) {
			return true
		}
	}
	return false
}

// IsClient mirrors AuthenticatedUser.isClient.
func (u AuthUser) IsClient() bool {
	return u.HasRole("CLIENT_BASIC") || u.HasRole("CLIENT_TRADING") || u.HasRole("CLIENT")
}

// IsAgent mirrors AuthenticatedUser.isAgent.
func (u AuthUser) IsAgent() bool {
	return u.HasRole("AGENT") || u.HasRole("SUPERVISOR") || u.HasRole("ADMIN")
}

// HasTradingPermission mirrors AuthenticatedUser.hasTradingPermission.
func (u AuthUser) HasTradingPermission() bool {
	if u.HasRole("CLIENT_TRADING") {
		return true
	}
	for _, p := range u.Permissions {
		if tradingPermissions[strings.ToUpper(p)] {
			return true
		}
	}
	return false
}

// HasMarginPermission mirrors AuthenticatedUser.hasMarginPermission.
func (u AuthUser) HasMarginPermission() bool {
	for _, p := range u.Permissions {
		if marginPermissions[strings.ToUpper(p)] {
			return true
		}
	}
	return false
}
