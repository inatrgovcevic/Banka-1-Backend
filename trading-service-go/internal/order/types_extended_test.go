package order

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// HasTradingPermission - additional branches
// ---------------------------------------------------------------------------

func TestHasTradingPermission_ViaPermissions_SECURITIES_TRADE(t *testing.T) {
	t.Parallel()
	u := AuthUser{Roles: []string{"BASIC"}, Permissions: []string{"SECURITIES_TRADE"}}
	assert.True(t, u.HasTradingPermission())
}

func TestHasTradingPermission_ViaPermissions_SECURITIES_TRADE_LIMITED(t *testing.T) {
	t.Parallel()
	u := AuthUser{Roles: []string{"BASIC"}, Permissions: []string{"SECURITIES_TRADE_LIMITED"}}
	assert.True(t, u.HasTradingPermission())
}

func TestHasTradingPermission_ViaPermissions_TRADING_BASIC(t *testing.T) {
	t.Parallel()
	u := AuthUser{Roles: []string{"AGENT"}, Permissions: []string{"TRADING_BASIC"}}
	assert.True(t, u.HasTradingPermission())
}

func TestHasTradingPermission_NoTradingPerm_ReturnsFalse(t *testing.T) {
	t.Parallel()
	u := AuthUser{Roles: []string{"BASIC"}, Permissions: []string{"BANKING_BASIC"}}
	assert.False(t, u.HasTradingPermission())
}

func TestHasTradingPermission_CaseInsensitive(t *testing.T) {
	t.Parallel()
	u := AuthUser{Roles: []string{"BASIC"}, Permissions: []string{"securities_trade_unlimited"}}
	assert.True(t, u.HasTradingPermission())
}

// ---------------------------------------------------------------------------
// HasMarginPermission
// ---------------------------------------------------------------------------

func TestHasMarginPermission_ViaMarginPermission(t *testing.T) {
	t.Parallel()
	u := AuthUser{Permissions: []string{"SECURITIES_TRADE_MARGIN"}}
	assert.True(t, u.HasMarginPermission())
}

func TestHasMarginPermission_MARGIN_Permission(t *testing.T) {
	t.Parallel()
	u := AuthUser{Permissions: []string{"MARGIN"}}
	assert.True(t, u.HasMarginPermission())
}

// ---------------------------------------------------------------------------
// HasRole
// ---------------------------------------------------------------------------

func TestHasRole_CaseInsensitive(t *testing.T) {
	t.Parallel()
	u := AuthUser{Roles: []string{"client_basic"}}
	assert.True(t, u.HasRole("CLIENT_BASIC"))
}

func TestHasRole_NotPresent_ReturnsFalse(t *testing.T) {
	t.Parallel()
	u := AuthUser{Roles: []string{"AGENT"}}
	assert.False(t, u.HasRole("ADMIN"))
}

// ---------------------------------------------------------------------------
// ParseStatusFilter - all variants
// ---------------------------------------------------------------------------

func TestParseStatusFilter_AllStatuses(t *testing.T) {
	t.Parallel()
	for _, status := range []string{StatusPending, StatusApproved, StatusDeclined, StatusDone, StatusCancelled, "ALL"} {
		result, ok := ParseStatusFilter(status)
		assert.True(t, ok, "status=%s", status)
		assert.Equal(t, status, result)
	}
}

func TestParseStatusFilter_CaseInsensitive(t *testing.T) {
	t.Parallel()
	result, ok := ParseStatusFilter("done")
	assert.True(t, ok)
	assert.Equal(t, StatusDone, result)
}
