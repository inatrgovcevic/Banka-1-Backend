package main

import "testing"

func TestNormalizeJSONStripsScaleDifferences(t *testing.T) {
	javaLike := []byte(`{"rate":1.00000000,"commission":0.00,"fromAmount":100.00}`)
	goLike := []byte(`{"rate":1,"commission":0,"fromAmount":100}`)

	if normalizeJSON(javaLike) != normalizeJSON(goLike) {
		t.Fatalf("expected normalizer to compress trailing zeros: java=%s go=%s",
			normalizeJSON(javaLike), normalizeJSON(goLike))
	}
}

func TestNormalizeJSONFiltersVolatileTimestamps(t *testing.T) {
	javaLike := []byte(`{"ticker":"AAPL","timestamp":"2026-05-26T10:00:00Z","lastRefresh":"2026-05-26T09:30:00","createdAt":"2026-05-26T00:00:00Z","price":150.25}`)
	goLike := []byte(`{"ticker":"AAPL","timestamp":"2099-12-31T23:59:59Z","lastRefresh":"2099-12-31T23:59:59","createdAt":"2099-12-31T23:59:59Z","price":150.25}`)

	if normalizeJSON(javaLike) != normalizeJSON(goLike) {
		t.Fatalf("expected normalizer to ignore timestamp/lastRefresh/createdAt: java=%s go=%s",
			normalizeJSON(javaLike), normalizeJSON(goLike))
	}
}

func TestNormalizeJSONDetectsRealFieldDifferences(t *testing.T) {
	javaLike := []byte(`{"open":true,"afterHours":false,"closed":false}`)
	goLike := []byte(`{"open":true,"afterHours":false,"closed":true}`)

	if normalizeJSON(javaLike) == normalizeJSON(goLike) {
		t.Fatalf("expected normalizer to surface real diffs")
	}
}

func TestNormalizeJSONOrdersKeysDeterministically(t *testing.T) {
	a := []byte(`{"b":2,"a":1,"c":3}`)
	b := []byte(`{"c":3,"a":1,"b":2}`)
	if normalizeJSON(a) != normalizeJSON(b) {
		t.Fatalf("expected normalizer to sort keys for stable diffing: a=%s b=%s", normalizeJSON(a), normalizeJSON(b))
	}
}
