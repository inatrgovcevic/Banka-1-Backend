package http

import (
	"net/http"
	"testing"

	authpkg "banka1/trading-service-go/internal/auth"
	"banka1/trading-service-go/internal/platform"
)

// TestRegisterRoutesNoConflict registers every P1–P6 route into a fresh ServeMux.
// Go 1.22 ServeMux panics at registration time on a duplicate or conflicting
// pattern, so this guards against route overlaps (e.g. the /otc and
// /stocks/internal additions colliding with each other or earlier phases) — a
// failure mode `go build`/`go vet` cannot catch. Registration does not touch any
// service field, so an empty *App is sufficient.
func TestRegisterRoutesNoConflict(t *testing.T) {
	mux := http.NewServeMux()
	handle := func(method, path string, h http.Handler) {
		mux.Handle(method+" "+path, h)
	}
	jwt := authpkg.NewJWTService(platform.JWTConfig{
		Secret:           "test-secret",
		Issuer:           "banka1",
		IDClaim:          "id",
		RolesClaim:       "roles",
		PermissionsClaim: "permissions",
	})
	// Panics on a conflicting/duplicate pattern; reaching the end means clean.
	registerRoutes(handle, &App{}, jwt)
}
