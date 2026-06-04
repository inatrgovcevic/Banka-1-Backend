package user

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmployeePermissions_Admin_HasAllPerms(t *testing.T) {
	perms := employeePermissions("ADMIN")
	assert.Contains(t, perms, "BANKING_BASIC")
	assert.Contains(t, perms, "EMPLOYEE_MANAGE_ALL")
	assert.Contains(t, perms, "OTC_TRADE")
}

func TestEmployeePermissions_Supervisor(t *testing.T) {
	perms := employeePermissions("SUPERVISOR")
	assert.Contains(t, perms, "FUND_AGENT_MANAGE")
	assert.NotContains(t, perms, "EMPLOYEE_MANAGE_ALL")
}

func TestEmployeePermissions_Agent(t *testing.T) {
	perms := employeePermissions("AGENT")
	assert.Contains(t, perms, "SECURITIES_TRADE_LIMITED")
	assert.NotContains(t, perms, "TRADE_UNLIMITED")
}

func TestEmployeePermissions_Basic(t *testing.T) {
	perms := employeePermissions("BASIC")
	assert.Contains(t, perms, "BANKING_BASIC")
	assert.Contains(t, perms, "CLIENT_MANAGE")
	assert.NotContains(t, perms, "SECURITIES_TRADE_LIMITED")
}

func TestEmployeePermissions_Service_Empty(t *testing.T) {
	perms := employeePermissions("SERVICE")
	assert.Empty(t, perms)
}

func TestEmployeePermissions_Unknown_Empty(t *testing.T) {
	perms := employeePermissions("UNKNOWN_ROLE")
	assert.Empty(t, perms)
}

func TestClientPermissions_ClientTrading(t *testing.T) {
	perms := clientPermissions("CLIENT_TRADING")
	assert.Contains(t, perms, "CLIENT_SECURITIES_TRADE")
	assert.Contains(t, perms, "CLIENT_OTC_TRADE")
}

func TestClientPermissions_ClientBasic(t *testing.T) {
	perms := clientPermissions("CLIENT_BASIC")
	assert.Equal(t, []string{"CLIENT_ACCOUNT_ACCESS"}, perms)
}

func TestClientPermissions_Unknown_DefaultsToBasic(t *testing.T) {
	perms := clientPermissions("UNKNOWN")
	assert.Equal(t, []string{"CLIENT_ACCOUNT_ACCESS"}, perms)
}
