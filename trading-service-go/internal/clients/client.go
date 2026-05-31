package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrNotFound signals an upstream 404. The actuary service maps it to its own
// ResourceNotFoundException (404) — mirroring order-service catching
// HttpClientErrorException.NotFound.
var ErrNotFound = errors.New("clients: upstream returned 404")

// HTTPDoer is the subset of *http.Client the base client needs (swappable in tests).
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// baseClient performs JSON requests against one upstream base URL, attaching a
// SERVICE bearer token to every request.
type baseClient struct {
	baseURL string
	http    HTTPDoer
	tokens  *ServiceTokenProvider
}

func newBaseClient(baseURL string, tokens *ServiceTokenProvider, doer HTTPDoer) *baseClient {
	if doer == nil {
		doer = &http.Client{Timeout: 10 * time.Second}
	}
	return &baseClient{baseURL: strings.TrimRight(baseURL, "/"), http: doer, tokens: tokens}
}

// callerAuthKey tags a context with the caller's raw Authorization header.
type callerAuthKey struct{}

// WithCallerAuth returns a context carrying the caller's raw Authorization header
// ("Bearer <jwt>"). When present, doJSON forwards it on outbound calls instead of
// minting a SERVICE token. Used by the /actuaries/* handlers: user-service
// /employees rejects the SERVICE role, so those reads must carry the caller's
// (SUPERVISOR) bearer. Empty input leaves the context unchanged (→ SERVICE mint).
func WithCallerAuth(ctx context.Context, authorization string) context.Context {
	if strings.TrimSpace(authorization) == "" {
		return ctx
	}
	return context.WithValue(ctx, callerAuthKey{}, authorization)
}

func callerAuthFromContext(ctx context.Context) string {
	v, _ := ctx.Value(callerAuthKey{}).(string)
	return v
}

// doJSON issues method baseURL+path(?query) with an optional JSON body, attaches
// the SERVICE token, and decodes a 2xx JSON response into out (skipped when out
// is nil). A 404 returns ErrNotFound; any other non-2xx returns an error.
func (c *baseClient) doJSON(ctx context.Context, method, path string, query url.Values, body, out any) error {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("clients: marshal body: %w", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if auth := callerAuthFromContext(ctx); auth != "" {
		// /actuaries/* forwards the caller's own bearer (clients.WithCallerAuth):
		// user-service /employees rejects the SERVICE role, so it must see the
		// caller's (SUPERVISOR) token instead of a minted SERVICE token.
		req.Header.Set("Authorization", auth)
	} else if c.tokens != nil {
		token, err := c.tokens.Token()
		if err != nil {
			return fmt.Errorf("clients: mint service token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("clients: %s %s -> %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("clients: decode %s %s: %w", method, path, err)
	}
	return nil
}
