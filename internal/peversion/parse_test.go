package peversion

import (
	"encoding/binary"
	"testing"
)

func TestParseFixed(t *testing.T) {
	t.Parallel()

	raw := make([]byte, 32)
	binary.LittleEndian.PutUint32(raw[0:], ffiSig)
	// FileVersion MS = 1.2, LS = 3.0 → 1.2.3
	binary.LittleEndian.PutUint32(raw[8:], (1<<16)|2)
	binary.LittleEndian.PutUint32(raw[12:], 3<<16)

	got, err := parseFixed(raw)
	if err != nil {
		t.Fatal(err)
	}

	if got != "1.2.3" {
		t.Fatalf("got %s", got)
	}
}

func TestParseFixedMissing(t *testing.T) {
	t.Parallel()

	if _, err := parseFixed([]byte{1, 2, 3, 4}); err == nil {
		t.Fatal("expected error")
	}
}
