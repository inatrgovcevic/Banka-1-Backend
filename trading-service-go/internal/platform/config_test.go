package platform

import (
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear all relevant env so defaults apply.
	for _, k := range []string{
		"SERVER_PORT", "GRPC_PORT", "TRADING_DB_HOST", "TRADING_DB_PORT",
		"TRADING_DB_NAME", "TRADING_DB_USER", "TRADING_DB_PASSWORD",
		"JWT_SECRET", "BANKA_SECURITY_ISSUER", "BANKA_SECURITY_EXPIRATION_TIME",
		"BANKA_SECURITY_CORS_ALLOWED_ORIGINS", "SERVICES_USER_URL",
		"SERVICES_BANKING_CORE_URL", "SERVICES_MARKET_URL",
		"ORDER_SCHEDULERS_ENABLED", "TAX_CAPITAL_GAINS_RATE",
		"BANKA_TAX_CAPITAL_GAINS_RATE", "SAGA_EVENTS_EXCHANGE",
		"SAGA_RESULTS_EXCHANGE", "OTC_SAGA_CONSUMERS_ENABLED",
		"OTC_CONTRACT_EXPIRATION_NOTIFICATION_DAYS", "BANKA1_ROUTING_NUMBER",
	} {
		t.Setenv(k, "")
	}

	c := LoadConfig()
	if c.ServerPort != "18088" {
		t.Errorf("ServerPort = %q want 18088", c.ServerPort)
	}
	if c.GRPCPort != "19088" {
		t.Errorf("GRPCPort = %q want 19088", c.GRPCPort)
	}
	if c.DBName != "trading" {
		t.Errorf("DBName = %q want trading", c.DBName)
	}
	if c.JWT.ExpirationMillis != 3600000 {
		t.Errorf("ExpirationMillis = %d want 3600000", c.JWT.ExpirationMillis)
	}
	if c.TaxCapitalGainsRate != "0.15" {
		t.Errorf("TaxCapitalGainsRate = %q want 0.15", c.TaxCapitalGainsRate)
	}
	if c.SagaEventsExchange != "saga.events" {
		t.Errorf("SagaEventsExchange = %q", c.SagaEventsExchange)
	}
	if c.SagaResultsExchange != "saga.exchange" {
		t.Errorf("SagaResultsExchange = %q", c.SagaResultsExchange)
	}
	if c.OtcReminderDays != 3 {
		t.Errorf("OtcReminderDays = %d want 3", c.OtcReminderDays)
	}
	if c.RoutingNumber != 111 {
		t.Errorf("RoutingNumber = %d want 111", c.RoutingNumber)
	}
	// Defaults: all gates off.
	if c.OrderSchedulersEnabled || c.OtcSagaConsumersEnabled || c.AuditConsumerEnabled {
		t.Errorf("expected all gate flags off by default")
	}
	if len(c.CORS.AllowedMethods) == 0 {
		t.Errorf("AllowedMethods empty")
	}
	if len(c.CORS.AllowedOrigins) != 1 || c.CORS.AllowedOrigins[0] != "http://localhost:4200" {
		t.Errorf("AllowedOrigins = %v", c.CORS.AllowedOrigins)
	}
}

func TestLoadConfig_OverridesFromEnv(t *testing.T) {
	t.Setenv("SERVER_PORT", "9000")
	t.Setenv("TRADING_DB_NAME", "mydb")
	t.Setenv("BANKA_SECURITY_EXPIRATION_TIME", "5000")
	t.Setenv("ORDER_SCHEDULERS_ENABLED", "true")
	t.Setenv("OTC_CONTRACT_EXPIRATION_NOTIFICATION_DAYS", "7")
	t.Setenv("BANKA1_ROUTING_NUMBER", "222")
	t.Setenv("BANKA_SECURITY_CORS_ALLOWED_ORIGINS", "http://a.com, http://b.com")

	c := LoadConfig()
	if c.ServerPort != "9000" {
		t.Errorf("ServerPort = %q", c.ServerPort)
	}
	if c.DBName != "mydb" {
		t.Errorf("DBName = %q", c.DBName)
	}
	if c.JWT.ExpirationMillis != 5000 {
		t.Errorf("ExpirationMillis = %d", c.JWT.ExpirationMillis)
	}
	if !c.OrderSchedulersEnabled {
		t.Errorf("OrderSchedulersEnabled should be true")
	}
	if c.OtcReminderDays != 7 {
		t.Errorf("OtcReminderDays = %d", c.OtcReminderDays)
	}
	if c.RoutingNumber != 222 {
		t.Errorf("RoutingNumber = %d", c.RoutingNumber)
	}
	if len(c.CORS.AllowedOrigins) != 2 {
		t.Errorf("AllowedOrigins = %v", c.CORS.AllowedOrigins)
	}
}

func TestDatabaseURL(t *testing.T) {
	c := Config{DBUser: "u", DBPassword: "p", DBHost: "h", DBPort: "5432", DBName: "db"}
	if got := c.DatabaseURL(); got != "postgres://u:p@h:5432/db" {
		t.Errorf("DatabaseURL = %q", got)
	}
}

func TestGetEnv(t *testing.T) {
	t.Setenv("FOO_X", "bar")
	if getEnv("FOO_X", "def") != "bar" {
		t.Error("getEnv set value")
	}
	t.Setenv("FOO_X", "   ")
	if getEnv("FOO_X", "def") != "def" {
		t.Error("getEnv blank -> fallback")
	}
	if getEnv("FOO_UNSET_ZZZ", "def") != "def" {
		t.Error("getEnv unset -> fallback")
	}
}

func TestGetEnvInt64(t *testing.T) {
	t.Setenv("INT_X", "42")
	if getEnvInt64("INT_X", 1) != 42 {
		t.Error("parse ok")
	}
	t.Setenv("INT_X", "notanint")
	if getEnvInt64("INT_X", 1) != 1 {
		t.Error("bad parse -> fallback")
	}
	if getEnvInt64("INT_UNSET_ZZZ", 9) != 9 {
		t.Error("unset -> fallback")
	}
}

func TestGetEnvBool(t *testing.T) {
	t.Setenv("BOOL_X", "true")
	if !getEnvBool("BOOL_X", false) {
		t.Error("true")
	}
	t.Setenv("BOOL_X", "0")
	if getEnvBool("BOOL_X", true) {
		t.Error("0 -> false")
	}
	t.Setenv("BOOL_X", "garbage")
	if !getEnvBool("BOOL_X", true) {
		t.Error("bad parse -> fallback true")
	}
	if getEnvBool("BOOL_UNSET_ZZZ", false) {
		t.Error("unset -> fallback false")
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV("a, b ,c,,  ")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("splitCSV = %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("splitCSV[%d] = %q want %q", i, got[i], want[i])
		}
	}
	if len(splitCSV("")) != 0 {
		t.Error("empty input -> empty slice")
	}
}

func TestIsDevSeed(t *testing.T) {
	if !isDevSeed("003_devseed_fixtures.sql") {
		t.Error("devseed file should match")
	}
	if isDevSeed("001_init.sql") {
		t.Error("non-devseed should not match")
	}
}

func TestDevSeedEnabled(t *testing.T) {
	t.Setenv("LIQUIBASE_CONTEXTS", "")
	if !devSeedEnabled() {
		t.Error("empty contexts default to dev -> enabled")
	}
	t.Setenv("LIQUIBASE_CONTEXTS", "prod")
	if devSeedEnabled() {
		t.Error("prod context -> disabled")
	}
	t.Setenv("LIQUIBASE_CONTEXTS", "prod,dev,test")
	if !devSeedEnabled() {
		t.Error("list containing dev -> enabled")
	}
	t.Setenv("LIQUIBASE_CONTEXTS", "DEV")
	if !devSeedEnabled() {
		t.Error("uppercase DEV -> enabled (case-insensitive)")
	}
}
