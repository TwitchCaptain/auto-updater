package extract

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestFromZipExtraFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	zpath := filepath.Join(dir, "rel.zip")
	if err := writeZip(zpath, map[string]string{
		"folder/autobrr.exe":    "primary",
		"folder/autobrrctl.exe": "ctl",
		"README.md":             "nope",
	}); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := FromZip(zpath, dest, "autobrr.exe", []string{"autobrrctl.exe"})
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 {
		t.Fatalf("wrote %v", got)
	}

	b, err := os.ReadFile(filepath.Join(dest, "autobrr.exe"))
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != "primary" {
		t.Fatalf("got %s", b)
	}
}

func TestFromZipGlob(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	zpath := filepath.Join(dir, "rel.zip")
	if err := writeZip(zpath, map[string]string{
		"app.exe":    "p",
		"appctl.exe": "c",
		"readme.txt": "r",
	}); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(dir, "out")
	got, err := FromZip(zpath, dest, "app.exe", []string{"*ctl.exe"})
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 {
		t.Fatalf("wrote %v", got)
	}
}

func TestFromZipUsesDestExeCasing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	zpath := filepath.Join(dir, "rel.zip")
	if err := writeZip(zpath, map[string]string{"APP.EXE": "body"}); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(dir, "out")
	got, err := FromZip(zpath, dest, filepath.Join(dest, "app.exe"), nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 || filepath.Base(got[0]) != "app.exe" {
		t.Fatalf("wrote %v", got)
	}

	if _, err := os.Stat(filepath.Join(dest, "app.exe")); err != nil {
		t.Fatal(err)
	}
}

func TestWouldWrite(t *testing.T) {
	t.Parallel()

	got := WouldWrite(`C:\Program Files\a\autobrr.exe`, []string{"autobrrctl.exe", "autobrr.exe"})
	if len(got) != 2 {
		t.Fatalf("%v", got)
	}
}

func writeZip(path string, files map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for name, body := range files {
		zw, err := w.Create(name)
		if err != nil {
			return err
		}

		if _, err := zw.Write([]byte(body)); err != nil {
			return err
		}
	}

	return w.Close()
}
