package clients

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// CustomerClient calls user-service (SERVICES_USER_URL) for client/customer
// records. Mirrors order-service ClientClient: the consolidated user-service
// serves /clients/customers alongside /employees, so this shares the user-service
// base URL with EmployeeClient. Net-new for P4 (tax) — only the tax domain needs
// customer names/emails for tracking rows and the tax.collected notification.
type CustomerClient struct {
	base *baseClient
}

// NewCustomerClient builds a CustomerClient over baseURL (the user-service base).
func NewCustomerClient(baseURL string, tokens *ServiceTokenProvider, doer HTTPDoer) *CustomerClient {
	return &CustomerClient{base: newBaseClient(baseURL, tokens, doer)}
}

// GetCustomer mirrors ClientClient.getCustomer: GET /clients/customers/{id}. A 404
// surfaces as ErrNotFound so callers can treat it as "not a client" (the tax
// notification enrichment then falls back to employee-service).
func (c *CustomerClient) GetCustomer(ctx context.Context, id int64) (*Customer, error) {
	var out Customer
	if err := c.base.doJSON(ctx, http.MethodGet, fmt.Sprintf("/clients/customers/%d", id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SearchCustomers mirrors ClientClient.searchCustomers: GET /clients/customers
// with ime/prezime added only when present (queryParamIfPresent) plus page/size.
func (c *CustomerClient) SearchCustomers(ctx context.Context, ime, prezime *string, page, size int) (*CustomerPage, error) {
	q := url.Values{}
	if ime != nil {
		q.Set("ime", *ime)
	}
	if prezime != nil {
		q.Set("prezime", *prezime)
	}
	q.Set("page", strconv.Itoa(page))
	q.Set("size", strconv.Itoa(size))
	var out CustomerPage
	if err := c.base.doJSON(ctx, http.MethodGet, "/clients/customers", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
