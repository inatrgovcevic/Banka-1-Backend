// Package client provides HTTP clients for the Java internal helper endpoints
// that interbank-service depends on (banking-core, trading-service, user-service).
// All clients authenticate via S2S JWT signed with the shared jwt.secret env var.
package client

import "errors"

// ErrNotFound signals a 404 from the upstream service. Callers should translate
// this to the appropriate domain reason (NO_SUCH_ACCOUNT, etc.) based on context.
var ErrNotFound = errors.New("client: upstream resource not found")

// ErrUpstream is the fallback for any 5xx or unparseable 4xx response.
var ErrUpstream = errors.New("client: upstream error")
