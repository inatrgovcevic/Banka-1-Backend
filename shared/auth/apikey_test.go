package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeStore struct{ partners []Partner }

func (f fakeStore) Partners() []Partner { return f.partners }

func newNext(captured **Partner) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := GetPartner(r.Context()); ok {
			*captured = p
		}
		w.WriteHeader(http.StatusOK)
	})
}

func TestRequireXApiKey_NoHeader_401(t *testing.T) {
	var got *Partner
	h := RequireXApiKey(fakeStore{[]Partner{{Routing: 222, InboundToken: "good"}}})(newNext(&got))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d, want 401", rec.Code)
	}
	if got != nil {
		t.Errorf("partner leaked into ctx: %+v", got)
	}
}

func TestRequireXApiKey_GoodToken_200(t *testing.T) {
	var got *Partner
	h := RequireXApiKey(fakeStore{[]Partner{{Routing: 222, InboundToken: "secret-token-abc"}}})(newNext(&got))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("X-Api-Key", "secret-token-abc")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d, want 200", rec.Code)
	}
	if got == nil || got.Routing != 222 {
		t.Errorf("partner=%+v, want routing=222", got)
	}
}

func TestRequireXApiKey_BadToken_401(t *testing.T) {
	var got *Partner
	h := RequireXApiKey(fakeStore{[]Partner{{Routing: 222, InboundToken: "secret"}}})(newNext(&got))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("X-Api-Key", "wrong")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d, want 401", rec.Code)
	}
	if got != nil {
		t.Errorf("partner leaked")
	}
}

func TestRequireXApiKey_MultiplePartners_PicksRight(t *testing.T) {
	partners := []Partner{
		{Routing: 222, InboundToken: "tok-222"},
		{Routing: 333, InboundToken: "tok-333"},
	}
	var got *Partner
	h := RequireXApiKey(fakeStore{partners})(newNext(&got))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("X-Api-Key", "tok-333")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	if got == nil || got.Routing != 333 {
		t.Errorf("partner=%+v, want routing=333", got)
	}
}

func TestPutPartnerGetPartner_RoundTrip(t *testing.T) {
	p := Partner{Routing: 222, InboundToken: "x"}
	ctx := PutPartner(context.Background(), p)
	got, ok := GetPartner(ctx)
	if !ok {
		t.Fatal("not found")
	}
	if got.Routing != 222 {
		t.Errorf("routing=%d", got.Routing)
	}
}
