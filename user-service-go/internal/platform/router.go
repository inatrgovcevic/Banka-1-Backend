package platform

import (
	"log/slog"
	"net/http"

	gpdb "banka1/go-platform/db"
	gphealth "banka1/go-platform/health"
	"banka1/go-platform/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RouterDeps wires the HTTP server. Same shape as before so cmd/server/main.go
// keeps compiling.
type RouterDeps struct {
	Config        Config
	Logger        *slog.Logger
	DB            *pgxpool.Pool
	Authenticator *JWTService
	Register      func(*Mux, *JWTService)
}

// Mux is a tiny wrapper over http.ServeMux supplying the `METHOD path` syntax
// the handlers already use.
type Mux struct {
	mux *http.ServeMux
}

// NewRouter assembles the HTTP handler using the go-platform middleware stack:
//
//	correlation -> recover -> request-log -> CORS
//
// Health endpoints are mounted via go-platform/health so /actuator/* shapes
// stay identical to other Go services.
func NewRouter(deps RouterDeps) http.Handler {
	m := &Mux{mux: http.NewServeMux()}
	registerPlatformRoutes(m, deps.DB)
	deps.Register(m, deps.Authenticator)

	cors := httpx.CORSConfig{
		AllowedOrigins:   deps.Config.CORS.AllowedOrigins,
		AllowedMethods:   deps.Config.CORS.AllowedMethods,
		AllowedHeaders:   []string{"Authorization", "Content-Type", httpx.HeaderCorrelationID},
		ExposedHeaders:   []string{"Location", httpx.HeaderCorrelationID},
		AllowCredentials: true,
	}
	return httpx.Default(deps.Logger, cors)(m.mux)
}

// Handle registers a `method path` route.
func (m *Mux) Handle(method, pattern string, handler http.Handler) {
	m.mux.Handle(method+" "+pattern, handler)
}

// HandleFunc registers a `method path` handlerfunc.
func (m *Mux) HandleFunc(method, pattern string, handler http.HandlerFunc) {
	m.Handle(method, pattern, handler)
}

func registerPlatformRoutes(m *Mux, db *pgxpool.Pool) {
	h := gphealth.NewHandler()
	if db != nil {
		h.Register(gpdb.Checker("postgres", db))
	}
	m.HandleFunc(http.MethodGet, "/actuator/health", h.Aggregate)
	m.HandleFunc(http.MethodGet, "/actuator/health/liveness", h.Liveness)
	m.HandleFunc(http.MethodGet, "/actuator/health/readiness", h.Readiness)
	m.HandleFunc(http.MethodGet, "/actuator/info", h.Info)
}
