package clients

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// EmployeeClient calls user-service (SERVICES_USER_URL) for employee records.
type EmployeeClient struct {
	base *baseClient
}

// NewEmployeeClient builds an EmployeeClient over baseURL.
func NewEmployeeClient(baseURL string, tokens *ServiceTokenProvider, doer HTTPDoer) *EmployeeClient {
	return &EmployeeClient{base: newBaseClient(baseURL, tokens, doer)}
}

// GetEmployee mirrors EmployeeClient.getEmployee: GET /employees/{id}. A 404
// surfaces as ErrNotFound so callers can map it to their own not-found error.
func (c *EmployeeClient) GetEmployee(ctx context.Context, id int64) (*Employee, error) {
	var out Employee
	if err := c.base.doJSON(ctx, http.MethodGet, fmt.Sprintf("/employees/%d", id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SearchEmployees mirrors EmployeeClient.searchEmployees: GET /employees with the
// filters added only when present (queryParamIfPresent) plus page/size.
func (c *EmployeeClient) SearchEmployees(ctx context.Context, email, ime, prezime, pozicija *string, page, size int) (*EmployeePage, error) {
	q := url.Values{}
	if email != nil {
		q.Set("email", *email)
	}
	if ime != nil {
		q.Set("ime", *ime)
	}
	if prezime != nil {
		q.Set("prezime", *prezime)
	}
	if pozicija != nil {
		q.Set("pozicija", *pozicija)
	}
	q.Set("page", strconv.Itoa(page))
	q.Set("size", strconv.Itoa(size))
	var out EmployeePage
	if err := c.base.doJSON(ctx, http.MethodGet, "/employees", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ActuaryClientIDs mirrors trading-service OTC UserServiceClient.getActuaryClientIds:
// GET /internal/otc/actuary-client-ids -> List<Long>. Used by OTC public-stocks
// supervisor view to filter to stocks advertised by actuaries. Defensive: any
// error (timeout, non-2xx, decode) yields an empty list, exactly like the Java
// client swallowing the exception — supervisor view then shows no stocks rather
// than failing the request (P6).
func (c *EmployeeClient) ActuaryClientIDs(ctx context.Context) []int64 {
	var out []int64
	if err := c.base.doJSON(ctx, http.MethodGet, "/internal/otc/actuary-client-ids", nil, nil, &out); err != nil {
		return []int64{}
	}
	return out
}
