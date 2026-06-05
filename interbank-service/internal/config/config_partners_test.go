package config_test

import (
	"testing"

	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/config"
)

func TestLoad_PartnersJSON_Parsed(t *testing.T) {
	setEnv(t,
		"INTERBANK_DB_URL", "postgres://test:test@localhost:5432/test",
		"INTERBANK_JWT_SECRET", "supersecret",
		"INTERBANK_PROFILE", "dev",
		"INTERBANK_PARTNERS_JSON", `[{"Routing":222,"DisplayName":"Banka 2","BaseURL":"http://localhost:8081/","InboundToken":"in-tok","OutboundToken":"out-tok"}]`,
	)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Interbank.Partners) != 1 {
		t.Fatalf("partners=%d want 1", len(cfg.Interbank.Partners))
	}
	p := cfg.Interbank.Partners[0]
	if p.Routing != 222 || p.DisplayName != "Banka 2" || p.InboundToken != "in-tok" {
		t.Errorf("partner: %+v", p)
	}
}

func TestLoad_PartnersJSON_Invalid(t *testing.T) {
	setEnv(t,
		"INTERBANK_DB_URL", "postgres://test:test@localhost:5432/test",
		"INTERBANK_JWT_SECRET", "supersecret",
		"INTERBANK_PROFILE", "dev",
		"INTERBANK_PARTNERS_JSON", `{not valid json`,
	)
	if _, err := config.Load(); err == nil {
		t.Fatal("expected error for invalid PARTNERS_JSON")
	}
}

func TestLoad_ProdProfile_DevToken_Rejected(t *testing.T) {
	setEnv(t,
		"INTERBANK_DB_URL", "postgres://test:test@localhost:5432/test",
		"INTERBANK_JWT_SECRET", "supersecret",
		"INTERBANK_PROFILE", "prod",
		"INTERBANK_PARTNERS_JSON", `[{"Routing":222,"InboundToken":"dev-xxx","OutboundToken":"out-tok"}]`,
	)
	if _, err := config.Load(); err == nil {
		t.Fatal("expected prod profile to reject dev- inbound token")
	}
}

func TestLoad_ProdProfile_EmptyOutboundToken_Rejected(t *testing.T) {
	setEnv(t,
		"INTERBANK_DB_URL", "postgres://test:test@localhost:5432/test",
		"INTERBANK_JWT_SECRET", "supersecret",
		"INTERBANK_PROFILE", "prod",
		"INTERBANK_PARTNERS_JSON", `[{"Routing":222,"InboundToken":"good-in","OutboundToken":""}]`,
	)
	if _, err := config.Load(); err == nil {
		t.Fatal("expected prod profile to reject empty outbound token")
	}
}

func TestLoad_ProdProfile_GoodTokens_Accepted(t *testing.T) {
	setEnv(t,
		"INTERBANK_DB_URL", "postgres://test:test@localhost:5432/test",
		"INTERBANK_JWT_SECRET", "supersecret",
		"INTERBANK_PROFILE", "prod",
		"INTERBANK_PARTNERS_JSON", `[{"Routing":222,"InboundToken":"good-in","OutboundToken":"good-out"}]`,
	)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load with good tokens: %v", err)
	}
	if len(cfg.Interbank.Partners) != 1 {
		t.Errorf("partners=%d want 1", len(cfg.Interbank.Partners))
	}
}

func TestLoad_DefaultProfile_IsProd(t *testing.T) {
	// No INTERBANK_PROFILE → defaults to prod; with a dev- token it must fail.
	setEnv(t,
		"INTERBANK_DB_URL", "postgres://test:test@localhost:5432/test",
		"INTERBANK_JWT_SECRET", "supersecret",
		"INTERBANK_PROFILE", "",
		"INTERBANK_PARTNERS_JSON", `[{"Routing":222,"InboundToken":"dev-x","OutboundToken":"dev-y"}]`,
	)
	if _, err := config.Load(); err == nil {
		t.Fatal("empty profile should default to prod and reject dev- token")
	}
}
