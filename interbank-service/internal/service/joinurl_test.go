package service

import "testing"

// joinURL must produce exactly one slash between a partner BaseURL and the
// protocol path segment regardless of trailing/leading slashes, so the same
// code works for "https://banka-2.radenkovic.rs/api" and ".../api/".
// Regression guard for the BaseURL+segment concatenation footgun that produced
// ".../apiinterbank" (404) when BaseURL had no trailing slash.
func TestJoinURL(t *testing.T) {
	cases := []struct{ base, path, want string }{
		{"https://banka-2.radenkovic.rs/api", "interbank", "https://banka-2.radenkovic.rs/api/interbank"},
		{"https://banka-2.radenkovic.rs/api/", "interbank", "https://banka-2.radenkovic.rs/api/interbank"},
		{"https://banka-2.radenkovic.rs/api", "/interbank", "https://banka-2.radenkovic.rs/api/interbank"},
		{"https://banka-2.radenkovic.rs/api/", "negotiations/222/neg-1", "https://banka-2.radenkovic.rs/api/negotiations/222/neg-1"},
		{"http://localhost:9999", "public-stock", "http://localhost:9999/public-stock"},
	}
	for _, c := range cases {
		if got := joinURL(c.base, c.path); got != c.want {
			t.Errorf("joinURL(%q, %q) = %q, want %q", c.base, c.path, got, c.want)
		}
	}
}
