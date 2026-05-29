// Command minttoken signs a local HS256 JWT using the same JWT config the service
// reads from the environment (JWT_SECRET / BANKA_SECURITY_ISSUER / *_CLAIM), so the
// token is accepted by BOTH the Java trading-service and trading-service-go. It is a
// parity-sweep aid (companion to cmd/paritycheck): mint a role-exact token to drive
// each endpoint's intended happy path on both services.
//
//	go run ./cmd/minttoken -role SERVICE
//	go run ./cmd/minttoken -role SUPERVISOR -id 1
//
// Run inside the golang image with the service env so the secret/issuer match:
//
//	docker run --rm --env-file setup/.env -v "<repo>:/workspace" \
//	  -v trading_go_modcache:/go/pkg/mod -w /workspace/trading-service-go golang:1.25 \
//	  go run ./cmd/minttoken -role SERVICE
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	authpkg "banka1/trading-service-go/internal/auth"
	"banka1/trading-service-go/internal/platform"
)

func main() {
	role := flag.String("role", "SERVICE", "single-string roles claim (e.g. SERVICE, SUPERVISOR, CLIENT_TRADING)")
	id := flag.Int64("id", 1, "id claim")
	subject := flag.String("subject", "parity@banka1.local", "subject (sub) claim")
	perms := flag.String("perms", "", "comma-separated permissions claim (e.g. FUND_AGENT_MANAGE)")
	ttl := flag.Duration("ttl", time.Hour, "token TTL")
	flag.Parse()

	cfg := platform.LoadConfig()
	svc := authpkg.NewJWTService(cfg.JWT)
	// Generate (not GenerateAccessToken) so the TTL is explicit — the in-service
	// JWTConfig does not set AccessTokenDuration. permissions must be non-nil: the
	// Java security-lib converter calls getClaimAsStringList(permissions) which NPEs
	// on a present-but-null claim (Go's gpauth tolerates null), so an empty slice
	// (claim = []) keeps the token accepted by BOTH services.
	permList := []string{}
	if strings.TrimSpace(*perms) != "" {
		permList = strings.Split(*perms, ",")
	}
	tok, err := svc.Generate(*id, *subject, "", *role, permList, *ttl)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mint token:", err)
		os.Exit(1)
	}
	fmt.Print(tok)
}
