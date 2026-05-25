package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/raf-si-2025/banka-1-go/shared/auth"
)

// UserDisplayDto is the response from GET /internal/interbank/user/{type}/{id}.
type UserDisplayDto struct {
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	DisplayName string `json:"displayName"`
}

// UserClient calls the Java user-service internal endpoint.
// All methods honour context cancellation and deadlines.
type UserClient struct {
	baseURL string
	issuer  *auth.S2SIssuer
	hc      *http.Client
}

// NewUserClient constructs a client with a per-request backstop timeout.
func NewUserClient(baseURL string, issuer *auth.S2SIssuer, timeout time.Duration) *UserClient {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &UserClient{
		baseURL: baseURL,
		issuer:  issuer,
		hc:      &http.Client{Timeout: timeout},
	}
}

// ResolveUser looks up display info for a user in our bank by type (CLIENT/EMPLOYEE)
// and numeric id. Returns ErrNotFound when the user does not exist (404); wraps
// ErrUpstream for 400 (bad type) and 5xx.
func (c *UserClient) ResolveUser(ctx context.Context, userType string, id int64) (*UserDisplayDto, error) {
	u := fmt.Sprintf("%s/internal/interbank/user/%s/%d",
		c.baseURL,
		url.PathEscape(strings.ToUpper(userType)),
		id,
	)
	req, err := buildRequest(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	tok, err := c.issuer.IssueToken()
	if err != nil {
		return nil, fmt.Errorf("client: issue S2S token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	var out UserDisplayDto
	if err := execRequest(c.hc, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
