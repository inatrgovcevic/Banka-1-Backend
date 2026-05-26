package platform

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RouterDeps struct {
	Config        Config
	Logger        *slog.Logger
	DB            *pgxpool.Pool
	Authenticator *JWTService
	Register      func(*Mux, *JWTService)
}

type Mux struct {
	mux *http.ServeMux
}

func NewRouter(deps RouterDeps) http.Handler {
	m := &Mux{mux: http.NewServeMux()}
	registerPlatformRoutes(m, deps.DB)
	deps.Register(m, deps.Authenticator)

	var handler http.Handler = m.mux
	handler = recoverMiddleware(deps.Logger, handler)
	handler = requestLogMiddleware(deps.Logger, handler)
	handler = corsMiddleware(deps.Config.CORS, handler)
	return handler
}

func (m *Mux) Handle(method, pattern string, handler http.Handler) {
	m.mux.Handle(method+" "+pattern, handler)
}

func (m *Mux) HandleFunc(method, pattern string, handler http.HandlerFunc) {
	m.Handle(method, pattern, handler)
}

func registerPlatformRoutes(m *Mux, db *pgxpool.Pool) {
	health := func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusOK, map[string]string{"status": "UP"})
	}
	m.HandleFunc(http.MethodGet, "/actuator/health/liveness", health)
	m.HandleFunc(http.MethodGet, "/actuator/health/readiness", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			Error(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "Database is not ready")
			return
		}
		JSON(w, http.StatusOK, map[string]string{"status": "UP"})
	})
	m.HandleFunc(http.MethodGet, "/actuator/info", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusOK, map[string]string{"service": "user-service-go"})
	})
}

func corsMiddleware(cfg CORSConfig, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (slices.Contains(cfg.AllowedOrigins, origin) || slices.Contains(cfg.AllowedOrigins, "*")) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Correlation-Id")
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ","))
		if r.Method == http.MethodOptions {
			NoContent(w, http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestLogMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}

func recoverMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic recovered", "panic", recovered)
				Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
