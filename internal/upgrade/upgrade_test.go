package upgrade

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitize(t *testing.T) {
	t.Parallel()

	if got := sanitize(`..\evil.exe`); got != "evil.exe" {
		t.Fatalf("got %s", got)
	}
}

func TestCopyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "a.exe")
	dest := filepath.Join(dir, "b", "a.exe")
	if err := os.WriteFile(src, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(src, dest); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != "hi" {
		t.Fatalf("got %s", b)
	}
}
