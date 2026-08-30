package host

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if !CanWrite(dir) {
		t.Fatal("temp dir should be writable")
	}

	sub := filepath.Join(dir, "nested")
	if !CanWrite(sub) {
		t.Fatal("should create nested dir")
	}

	if _, err := os.Stat(sub); err != nil {
		t.Fatal(err)
	}
}

func TestCopyOverwrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dest := filepath.Join(dir, "dest.bin")
	if err := os.WriteFile(src, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyOverwrite(src, dest); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != "abc" {
		t.Fatalf("got %s", b)
	}
}
