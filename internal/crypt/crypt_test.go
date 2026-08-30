package crypt

import (
	"bytes"
	"errors"
	"testing"
)

func TestSealOpen(t *testing.T) {
	t.Parallel()

	plain := []byte(`{"githubToken":"secret"}`)
	blob, err := Seal(plain, "hunter2")
	if err != nil {
		t.Fatal(err)
	}

	if !IsEncrypted(blob) {
		t.Fatal("expected encrypted magic")
	}

	got, err := Open(blob, "hunter2")
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, plain) {
		t.Fatalf("got %s", got)
	}
}

func TestWrongPassword(t *testing.T) {
	t.Parallel()

	blob, err := Seal([]byte("hi"), "right")
	if err != nil {
		t.Fatal(err)
	}

	_, err = Open(blob, "wrong")
	if !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("got %v", err)
	}
}

func TestPlaintextNotEncrypted(t *testing.T) {
	t.Parallel()

	if IsEncrypted([]byte(`{"apps":[]}`)) {
		t.Fatal("plaintext should not look encrypted")
	}

	_, err := Open([]byte(`{"apps":[]}`), "x")
	if !errors.Is(err, ErrNotEncrypted) {
		t.Fatalf("got %v", err)
	}
}

func TestEmptyPassword(t *testing.T) {
	t.Parallel()

	if _, err := Seal([]byte("x"), ""); err == nil {
		t.Fatal("expected error")
	}
}
