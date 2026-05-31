package clients

import (
	"time"

	gpauth "banka1/go-platform/auth"
)

// Clients bundles the outbound service clients the domains use. They share one
// SERVICE token provider (one cached token across all upstreams), matching the
// single shared interceptor in order-service RestClientConfig.
type Clients struct {
	Market   *MarketClient
	Account  *AccountClient
	Employee *EmployeeClient
	Customer *CustomerClient
}

// New wires the clients. marketURL/bankingCoreURL/userURL come from the
// SERVICES_* env (see internal/platform Config). tokenTTL mirrors
// banka.security.expiration-time. Customer shares the user-service base URL with
// Employee (the consolidated user-service serves both /employees and
// /clients/customers).
func New(jwt *gpauth.Service, marketURL, bankingCoreURL, userURL string, tokenTTL time.Duration) *Clients {
	tokens := NewServiceTokenProvider(jwt, "trading-service-go", tokenTTL)
	return &Clients{
		Market:   NewMarketClient(marketURL, tokens, nil),
		Account:  NewAccountClient(bankingCoreURL, tokens, nil),
		Employee: NewEmployeeClient(userURL, tokens, nil),
		Customer: NewCustomerClient(userURL, tokens, nil),
	}
}
