package config

import (
	"os"
	"testing"
)

type TestConfig struct {
	Port     int    `envconfig:"PORT" default:"8080"`
	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`
	Enabled  bool   `envconfig:"ENABLED" default:"true"`
}

func TestLoad_ReadsEnvWithPrefix(t *testing.T) {
	os.Setenv("MYAPP_PORT", "9090")
	defer os.Unsetenv("MYAPP_PORT")
	os.Setenv("MYAPP_ENABLED", "false")
	defer os.Unsetenv("MYAPP_ENABLED")

	var cfg TestConfig
	if err := Load("MYAPP", &cfg); err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port: got %d, want 9090", cfg.Port)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel default: got %q, want info", cfg.LogLevel)
	}
	if cfg.Enabled != false {
		t.Errorf("Enabled override: got %v, want false", cfg.Enabled)
	}
}

func TestLoad_RequiredFieldMissing(t *testing.T) {
	type WithRequired struct {
		DBURL string `envconfig:"DATABASE_URL" required:"true"`
	}
	var cfg WithRequired
	err := Load("MYAPP", &cfg)
	if err == nil {
		t.Fatal("expected error for missing required field")
	}
}
