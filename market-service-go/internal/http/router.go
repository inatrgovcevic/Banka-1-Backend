package http

import (
	"log/slog"
	"net/http"
	"strings"

	"banka1/market-service-go/internal/auth"
	"banka1/market-service-go/internal/platform"

	gpauth "banka1/go-platform/auth"
	gpdb "banka1/go-platform/db"
	gphealth "banka1/go-platform/health"
	"banka1/go-platform/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewRouter wires the HTTP server. Middleware stack and /actuator endpoints
// come from banka1/go-platform so behavior is identical across Go services.
func NewRouter(cfg platform.Config, logger *slog.Logger, db *pgxpool.Pool, jwtService *auth.JWTService, app *App) http.Handler {
	mux := http.NewServeMux()

	handle := func(method, path string, handler http.Handler) {
		mux.Handle(method+" "+path, handler)
	}

	healthH := gphealth.NewHandler()
	if db != nil {
		healthH.Register(gpdb.Checker("postgres", db))
	}
	handle(http.MethodGet, "/actuator/health", http.HandlerFunc(healthH.Aggregate))
	handle(http.MethodGet, "/actuator/health/liveness", http.HandlerFunc(healthH.Liveness))
	handle(http.MethodGet, "/actuator/health/readiness", http.HandlerFunc(healthH.Readiness))
	handle(http.MethodGet, "/actuator/info", http.HandlerFunc(healthH.Info))

	permitAll := gpauth.NewPermitAll([]string{
		"/stocks/public/**",
		"/stocks/internal/**",
		"/internal/calculate/**",
		"/exchange/rates/current",
		"/actuator/info",
		"/actuator/health",
		"/actuator/health/liveness",
		"/actuator/health/readiness",
	})
	withAuth := func(path string, handler http.Handler) http.Handler {
		if permitAll.Matches(path) {
			return handler
		}
		// Legacy prefix passthrough kept for compatibility with the existing
		// public:= []string{...} list in this service.
		for _, prefix := range []string{"/stocks/internal/", "/internal/calculate/", "/actuator/", "/stocks/public/"} {
			if strings.HasPrefix(path, prefix) {
				return handler
			}
		}
		return jwtService.Middleware(handler)
	}

	registerRoutes(handle, cfg, app, jwtService, withAuth)

	cors := httpx.CORSConfig{
		AllowedOrigins:   cfg.CORS.AllowedOrigins,
		AllowedMethods:   cfg.CORS.AllowedMethods,
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Accept", "X-Requested-With", "X-Verification-Code", httpx.HeaderCorrelationID},
		ExposedHeaders:   []string{"Location", httpx.HeaderCorrelationID},
		AllowCredentials: true,
	}
	return httpx.Default(logger, cors)(mux)
}
