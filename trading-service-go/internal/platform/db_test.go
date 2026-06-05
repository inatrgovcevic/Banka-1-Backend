package platform

import (
	"context"
	"testing"
)

func TestOpenPostgres_BadURL_ReturnsError(t *testing.T) {
	// An empty/invalid URL should return an error rather than panicking.
	_, err := OpenPostgres(context.Background(), "not-a-valid-postgres-url")
	if err == nil {
		t.Error("expected error for invalid postgres URL")
	}
}

func TestOpenPostgres_EmptyURL_ReturnsError(t *testing.T) {
	_, err := OpenPostgres(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty URL")
	}
}
