package extract

import (
	"archive/zip"
	"os"
	"path/filepath"
	"runtime"
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

func TestFromZipArchSuffixedPrimary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	zpath := filepath.Join(dir, "unpackerr.amd64.exe.zip")
	if err := writeZip(zpath, map[string]string{
		"unpackerr.amd64.exe":    "payload",
		"unpackerr.conf.example": "conf",
		"README.html":            "doc",
	}); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(dir, "out")
	got, err := FromZip(zpath, dest, filepath.Join(dest, "unpackerr.exe"), nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 || filepath.Base(got[0]) != "unpackerr.exe" {
		t.Fatalf("wrote %v", got)
	}

	b, err := os.ReadFile(filepath.Join(dest, "unpackerr.exe"))
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != "payload" {
		t.Fatalf("got %s", b)
	}
}

func TestFromZipPrefersHostArch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	zpath := filepath.Join(dir, "both.zip")
	if err := writeZip(zpath, map[string]string{
		"app.amd64.exe": "amd",
		"app.arm64.exe": "arm",
	}); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(dir, "out")
	got, err := FromZip(zpath, dest, "app.exe", nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 || filepath.Base(got[0]) != "app.exe" {
		t.Fatalf("wrote %v", got)
	}

	b, err := os.ReadFile(filepath.Join(dest, "app.exe"))
	if err != nil {
		t.Fatal(err)
	}

	want := "amd"
	if runtime.GOARCH == "arm64" {
		want = "arm"
	}

	if string(b) != want {
		t.Fatalf("GOARCH=%s got %s want %s", runtime.GOARCH, b, want)
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
