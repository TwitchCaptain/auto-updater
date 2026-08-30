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

func TestSamePathAndIsSelf(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := filepath.Join(dir, "a.exe")
	b := filepath.Join(dir, "b.exe")
	if err := os.WriteFile(a, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(b, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !samePath(a, a) {
		t.Fatal("same path")
	}

	if samePath(a, b) {
		t.Fatal("different files")
	}

	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	if !IsSelf(self) {
		t.Fatal("executable should be self")
	}

	if IsSelf(a) {
		t.Fatal("temp file is not self")
	}
}

func TestRunningCountMissing(t *testing.T) {
	t.Parallel()

	n, err := RunningCount(filepath.Join(t.TempDir(), "no-such-captain-updater-test.exe"))
	if err != nil {
		t.Fatal(err)
	}

	if n != 0 {
		t.Fatalf("count %d", n)
	}
}
