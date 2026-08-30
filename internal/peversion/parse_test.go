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

func TestParseFixedDropsRevision(t *testing.T) {
	t.Parallel()

	raw := make([]byte, 32)
	binary.LittleEndian.PutUint32(raw[0:], ffiSig)
	binary.LittleEndian.PutUint32(raw[8:], (0<<16)|15)
	binary.LittleEndian.PutUint32(raw[12:], (3<<16)|1056)

	got, err := parseFixed(raw)
	if err != nil {
		t.Fatal(err)
	}

	if got != "0.15.3" {
		t.Fatalf("got %s", got)
	}
}

func TestParseFixedMissing(t *testing.T) {
	t.Parallel()

	if _, err := parseFixed([]byte{1, 2, 3, 4}); err == nil {
		t.Fatal("expected error")
	}
}

func TestProductVersionPrefersString(t *testing.T) {
	t.Parallel()

	raw := make([]byte, 32, 128)
	binary.LittleEndian.PutUint32(raw[0:], ffiSig)
	binary.LittleEndian.PutUint32(raw[8:], (0<<16)|15)
	binary.LittleEndian.PutUint32(raw[12:], (3<<16)|1056)
	raw = append(raw, utf16z("ProductVersion")...)
	raw = append(raw, utf16z("0.15.3")...)

	got, err := fromRaw(raw)
	if err != nil {
		t.Fatal(err)
	}

	if got != "0.15.3" {
		t.Fatalf("got %s", got)
	}
}

func fromRaw(raw []byte) (string, error) {
	if v := stringAfterUTF16Key(raw, "ProductVersion"); v != "" {
		return v, nil
	}

	return parseFixed(raw)
}
