package config

import (
	"testing"
	"time"
)

func TestEnvFallback(t *testing.T) {
	t.Setenv("PLATFORM_TEST_EMPTY", "")
	t.Setenv("PLATFORM_TEST_SPACES", "   ")
	t.Setenv("PLATFORM_TEST_SET", "value")

	if got := Env("PLATFORM_TEST_EMPTY", "fb"); got != "fb" {
		t.Fatalf("blank env should return fallback, got %q", got)
	}
	if got := Env("PLATFORM_TEST_SPACES", "fb"); got != "fb" {
		t.Fatalf("whitespace env should return fallback, got %q", got)
	}
	if got := Env("PLATFORM_TEST_SET", "fb"); got != "value" {
		t.Fatalf("set env should return value, got %q", got)
	}
	if got := Env("PLATFORM_TEST_UNSET", "fb"); got != "fb" {
		t.Fatalf("unset env should return fallback, got %q", got)
	}
}

func TestEnvInt(t *testing.T) {
	t.Setenv("PLATFORM_INT_OK", "42")
	t.Setenv("PLATFORM_INT_BAD", "not-a-number")

	if EnvInt("PLATFORM_INT_OK", 1) != 42 {
		t.Fatal("expected 42")
	}
	if EnvInt("PLATFORM_INT_BAD", 7) != 7 {
		t.Fatal("invalid int should fall back")
	}
	if EnvInt("PLATFORM_INT_MISSING", 9) != 9 {
		t.Fatal("missing int should fall back")
	}
}

func TestEnvBool(t *testing.T) {
	t.Setenv("PLATFORM_BOOL_T", "true")
	t.Setenv("PLATFORM_BOOL_F", "0")
	t.Setenv("PLATFORM_BOOL_BAD", "yesplz")

	if !EnvBool("PLATFORM_BOOL_T", false) {
		t.Fatal("true expected")
	}
	if EnvBool("PLATFORM_BOOL_F", true) {
		t.Fatal("false expected")
	}
	if !EnvBool("PLATFORM_BOOL_BAD", true) {
		t.Fatal("invalid bool should fall back to true")
	}
}

func TestEnvDurationAcceptsGoAndMilliseconds(t *testing.T) {
	t.Setenv("PLATFORM_DUR_GO", "2s")
	t.Setenv("PLATFORM_DUR_MS", "1500")
	t.Setenv("PLATFORM_DUR_BAD", "garbage")

	if EnvDuration("PLATFORM_DUR_GO", time.Second) != 2*time.Second {
		t.Fatal("go duration parse failed")
	}
	if EnvDuration("PLATFORM_DUR_MS", time.Second) != 1500*time.Millisecond {
		t.Fatal("numeric should be parsed as milliseconds")
	}
	if EnvDuration("PLATFORM_DUR_BAD", 5*time.Second) != 5*time.Second {
		t.Fatal("bad duration should fall back")
	}
}

func TestSplitCSV(t *testing.T) {
	t.Setenv("PLATFORM_CSV", " a,b ,  ,c  ")
	got := SplitCSV("PLATFORM_CSV", "")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestRequiredPanicsWhenMissing(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for missing required env")
		}
	}()
	Required("PLATFORM_REQ_MISSING")
}

func TestRequiredReturnsValueWhenSet(t *testing.T) {
	t.Setenv("PLATFORM_REQ_OK", "abc")
	if Required("PLATFORM_REQ_OK") != "abc" {
		t.Fatal("expected abc")
	}
}

func TestRedactHidesSecrets(t *testing.T) {
	got := Redact(map[string]string{
		"JWT_SECRET":   "supersecret",
		"DB_PASSWORD":  "hunter2",
		"API_KEY_FOO":  "xyz",
		"TWELVE_DATA_API_KEY": "tk",
		"SERVER_PORT":  "8085",
	})
	if got["JWT_SECRET"] != "***" || got["DB_PASSWORD"] != "***" || got["API_KEY_FOO"] != "***" || got["TWELVE_DATA_API_KEY"] != "***" {
		t.Fatalf("secrets not redacted: %+v", got)
	}
	if got["SERVER_PORT"] != "8085" {
		t.Fatalf("non-secret value should pass through, got %v", got["SERVER_PORT"])
	}
}

func TestIsSecret(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"JWT_SECRET", true},
		{"DB_PASSWORD", true},
		{"DISCORD_BOT_TOKEN", true},
		{"ALPHA_VANTAGE_API_KEY", true},
		{"BANKA_SECURITY_JMBG_AES_KEY", true},
		{"SERVER_PORT", false},
		{"BANKA_SECURITY_ISSUER", false},
	}
	for _, tc := range cases {
		if IsSecret(tc.key) != tc.want {
			t.Errorf("IsSecret(%q) = %v, want %v", tc.key, !tc.want, tc.want)
		}
	}
}
