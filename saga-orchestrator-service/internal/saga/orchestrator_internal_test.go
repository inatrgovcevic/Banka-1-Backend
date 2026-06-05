package saga

import (
	"testing"
)

// ---------------------------------------------------------------------------
// marshalLog / unmarshalLog
// ---------------------------------------------------------------------------

func TestMarshalLog_EmptyMapReturnsNil(t *testing.T) {
	if b := marshalLog(map[string]string{}); b != nil {
		t.Errorf("expected nil for empty map, got %q", b)
	}
	if b := marshalLog(nil); b != nil {
		t.Errorf("expected nil for nil map, got %q", b)
	}
}

func TestMarshalLog_RoundTrip(t *testing.T) {
	in := map[string]string{"reservationId": "res-1", "transferId": "tr-1"}
	b := marshalLog(in)
	if b == nil {
		t.Fatal("expected non-nil bytes for non-empty map")
	}
	out := unmarshalLog(b)
	if len(out) != len(in) {
		t.Fatalf("round trip size mismatch: got %d want %d", len(out), len(in))
	}
	for k, v := range in {
		if out[k] != v {
			t.Errorf("key %q = %q, want %q", k, out[k], v)
		}
	}
}

func TestUnmarshalLog_EmptyBytes(t *testing.T) {
	m := unmarshalLog(nil)
	if m == nil || len(m) != 0 {
		t.Errorf("expected empty non-nil map, got %v", m)
	}
	m = unmarshalLog([]byte{})
	if m == nil || len(m) != 0 {
		t.Errorf("expected empty non-nil map, got %v", m)
	}
}

func TestUnmarshalLog_InvalidJSON(t *testing.T) {
	m := unmarshalLog([]byte(`{not valid`))
	if m == nil {
		t.Fatal("expected non-nil map on invalid JSON")
	}
	if len(m) != 0 {
		t.Errorf("expected empty map on invalid JSON, got %v", m)
	}
}

// ---------------------------------------------------------------------------
// unmarshalSagaLog
// ---------------------------------------------------------------------------

func TestUnmarshalSagaLog_EmptyReturnsFreshLog(t *testing.T) {
	sl := unmarshalSagaLog(nil)
	if sl == nil {
		t.Fatal("expected non-nil SagaLog")
	}
	if sl.Refs == nil {
		t.Error("Refs map should be initialized")
	}
	if sl.CompCounts == nil {
		t.Error("CompCounts map should be initialized")
	}
}

func TestUnmarshalSagaLog_InvalidJSONReturnsFreshLog(t *testing.T) {
	sl := unmarshalSagaLog([]byte(`{bad json`))
	if sl == nil {
		t.Fatal("expected non-nil SagaLog on invalid JSON")
	}
	if sl.Refs == nil || sl.CompCounts == nil {
		t.Error("maps should be initialized on invalid JSON fallback")
	}
}

func TestUnmarshalSagaLog_ValidRoundTrip(t *testing.T) {
	orig := newSagaLog()
	orig.appendStep("F1", "ok", "")
	orig.appendStep("F2", "err", "boom")
	orig.Refs["reservationId"] = "res-1"
	orig.CompCounts["C1"] = 2

	b := orig.marshalBytes()
	sl := unmarshalSagaLog(b)

	if len(sl.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(sl.Steps))
	}
	if sl.Steps[0].Step != "F1" || sl.Steps[0].Outcome != "ok" {
		t.Errorf("step 0 = %+v", sl.Steps[0])
	}
	if sl.Steps[1].Error != "boom" {
		t.Errorf("step 1 error = %q, want boom", sl.Steps[1].Error)
	}
	if sl.Refs["reservationId"] != "res-1" {
		t.Errorf("ref reservationId = %q", sl.Refs["reservationId"])
	}
	if sl.CompCounts["C1"] != 2 {
		t.Errorf("compCount C1 = %d, want 2", sl.CompCounts["C1"])
	}
}

func TestUnmarshalSagaLog_NilMapsRehydrated(t *testing.T) {
	// JSON with no refs/compCounts must still yield initialized maps.
	sl := unmarshalSagaLog([]byte(`{"steps":[{"step":"F1","outcome":"ok"}]}`))
	if sl.Refs == nil {
		t.Error("Refs should be re-initialized when absent from JSON")
	}
	if sl.CompCounts == nil {
		t.Error("CompCounts should be re-initialized when absent from JSON")
	}
	sl.Refs["x"] = "y" // must not panic
	sl.CompCounts["z"] = 1
}
