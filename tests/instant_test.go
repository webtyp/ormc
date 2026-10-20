//go:build !wasm

package tests

import (
	"testing"

	"webtyp.com/json"
	"webtyp.com/time"
)

// TestInstants_RoundTripAndKeepTheirUnit: a generated instant field survives a codec round trip, and the field's type
// is the unit (the assignments below would not compile with the other unit).
func TestInstants_RoundTripAndKeepTheirUnit(t *testing.T) {
	in := Session{Id: "s1", ExpiresAt: time.NowUnix() + 30*time.Day, UpdatedAt: time.NowNano()}
	var raw []byte
	if err := json.Encode(&in, &raw); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	var out Session
	if err := json.Decode(raw, &out); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if out != in {
		t.Errorf("round trip: got %+v, want %+v", out, in)
	}
	ptrs := in.Pointers()
	if p, ok := ptrs[1].(*int64); !ok || *p != int64(in.ExpiresAt) {
		t.Errorf("Pointers must expose the instant as *int64 for storage and codecs, got %T", ptrs[1])
	}
}
