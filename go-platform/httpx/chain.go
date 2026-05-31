package httpx

import (
	"log/slog"
	"net/http"
)

// Chain returns a middleware-applying function. Middlewares are applied
// outermost-first: Chain(a, b, c)(h) == a(b(c(h))).
func Chain(middlewares ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(final http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}

// Default returns the canonical Go-platform middleware stack:
//
//	correlation -> recover -> request-log -> CORS -> handler
//
// Correlation is outermost so the id is in context for everything below it
// (including the recover handler that logs panics). CORS is innermost so the
// preflight short-circuit still passes through observability.
func Default(logger *slog.Logger, cors CORSConfig) func(http.Handler) http.Handler {
	return Chain(
		CorrelationMiddleware,
		RecoverMiddleware(logger),
		RequestLogMiddleware(logger),
		CORSMiddleware(cors),
	)
}
