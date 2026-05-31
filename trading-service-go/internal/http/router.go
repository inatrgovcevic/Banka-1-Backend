package http

import (
	"log/slog"
	"net/http"

	"banka1/trading-service-go/internal/platform"

	gpauth "banka1/go-platform/auth"
	gpdb "banka1/go-platform/db"
	gphealth "banka1/go-platform/health"
	"banka1/go-platform/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewRouter wires the HTTP server: the go-platform middleware stack (correlation,
// recover, request log, CORS) + the public /actuator/* endpoints + the per-phase
// domain routes (auth-gated inside registerRoutes). Behavior matches the other
// Go services.
func NewRouter(cfg platform.Config, logger *slog.Logger, db *pgxpool.Pool, jwtService *gpauth.Service, app *App) http.Handler {
	mux := http.NewServeMux()
	handle := func(method, path string, handler http.Handler) {
		mux.Handle(method+" "+path, handler)
	}

	// Public actuator endpoints (no auth).
	healthH := gphealth.NewHandler()
	if db != nil {
		healthH.Register(gpdb.Checker("postgres", db))
	}
	handle(http.MethodGet, "/actuator/health", http.HandlerFunc(healthH.Aggregate))
	handle(http.MethodGet, "/actuator/health/liveness", http.HandlerFunc(healthH.Liveness))
	handle(http.MethodGet, "/actuator/health/readiness", http.HandlerFunc(healthH.Readiness))
	handle(http.MethodGet, "/actuator/info", http.HandlerFunc(healthH.Info))

	registerRoutes(handle, app, jwtService)

	cors := httpx.CORSConfig{
		AllowedOrigins:   cfg.CORS.AllowedOrigins,
		AllowedMethods:   cfg.CORS.AllowedMethods,
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Accept", "X-Requested-With", httpx.HeaderCorrelationID},
		ExposedHeaders:   []string{"Location", httpx.HeaderCorrelationID},
		AllowCredentials: true,
	}
	return httpx.Default(logger, cors)(mux)
}
