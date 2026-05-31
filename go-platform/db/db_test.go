package db

import (
	"strings"
	"testing"
)

func TestURLBuildsExpectedDSN(t *testing.T) {
	cfg := Config{Host: "h", Port: "5432", Name: "n", User: "u", Password: "p"}
	got := cfg.URL()
	want := "postgres://u:p@h:5432/n?sslmode=disable"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestURLHonoursSSLMode(t *testing.T) {
	cfg := Config{Host: "h", Port: "5432", Name: "n", User: "u", Password: "p", SSLMode: "require"}
	if !strings.HasSuffix(cfg.URL(), "sslmode=require") {
		t.Fatalf("ssl mode lost: %s", cfg.URL())
	}
}

func TestCheckerNilPool(t *testing.T) {
	c := Checker("postgres", nil)
	if c.Name() != "postgres" {
		t.Fatal("name lost")
	}
	if err := c.Check(nil); err == nil {
		t.Fatal("expected error on nil pool")
	}
}
