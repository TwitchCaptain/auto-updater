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

func TestPersistSelfImageSurvivesStagingCleanup(t *testing.T) {
	t.Parallel()

	staging := t.TempDir()
	src := filepath.Join(staging, "captain-updater.exe")
	if err := os.WriteFile(src, []byte("new-image"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := persistSelfImage(src, "captain-updater.exe")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = os.Remove(got) })

	if err := os.RemoveAll(staging); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != "new-image" {
		t.Fatalf("got %s", b)
	}
}
