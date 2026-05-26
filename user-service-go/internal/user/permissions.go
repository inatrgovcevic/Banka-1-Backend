package user

func employeePermissions(role string) []string {
	switch role {
	case "ADMIN":
		return []string{"EMPLOYEE_MANAGE_ALL"}
	case "SUPERVISOR":
		return []string{"SECURITIES_TRADE_UNLIMITED", "TRADE_UNLIMITED", "OTC_TRADE", "FUND_AGENT_MANAGE", "MARGIN_TRADE"}
	case "AGENT":
		return []string{"SECURITIES_TRADE_LIMITED"}
	case "BASIC":
		return []string{"BANKING_BASIC", "CLIENT_MANAGE"}
	case "SERVICE":
		return []string{}
	default:
		return []string{}
	}
}

func clientPermissions(role string) []string {
	switch role {
	case "CLIENT_TRADING":
		return []string{"CLIENT_SECURITIES_TRADE", "CLIENT_OTC_TRADE"}
	case "CLIENT_BASIC":
		return []string{"CLIENT_ACCOUNT_ACCESS"}
	default:
		return []string{"CLIENT_ACCOUNT_ACCESS"}
	}
}
