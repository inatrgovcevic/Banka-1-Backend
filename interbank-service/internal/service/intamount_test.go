package service

import (
	"encoding/json"
	"testing"
)

// IntAmount must tolerate the scaled / quoted forms Banka 2 may emit for an OTC
// amount (BigDecimal serialized as 10.00, 10, "10", 10.0000) — all decode to 10
// — while rejecting a genuinely fractional value, and always marshalling back as
// a bare integer so Banka 1's own outbound offers stay int-shaped.
func TestIntAmount_DecodeScaledRejectFractional(t *testing.T) {
	for _, in := range []string{`10`, `10.00`, `"10"`, `10.0000`} {
		var a IntAmount
		if err := json.Unmarshal([]byte(in), &a); err != nil {
			t.Errorf("unmarshal %s: %v", in, err)
			continue
		}
		if a != 10 {
			t.Errorf("unmarshal %s = %d, want 10", in, int(a))
		}
	}

	var a IntAmount
	if err := json.Unmarshal([]byte(`10.5`), &a); err == nil {
		t.Errorf("expected error decoding fractional 10.5, got %d", int(a))
	}

	b, err := json.Marshal(IntAmount(7))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != "7" {
		t.Errorf("marshal IntAmount(7) = %s, want bare 7", b)
	}
}
