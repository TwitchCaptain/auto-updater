package backup

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestZip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.json")
	hist := filepath.Join(dir, "history.jsonl")
	if err := os.WriteFile(cfg, []byte(`{"apps":[]}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(hist, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(dir, "out", "backup.zip")
	if err := Zip(dest, cfg, hist); err != nil {
		t.Fatal(err)
	}

	r, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	got := map[string]bool{}
	for _, f := range r.File {
		got[f.Name] = true
	}

	for _, name := range []string{"config.json", "history.jsonl", "README.txt"} {
		if !got[name] {
			t.Fatalf("missing %s in %v", name, got)
		}
	}
}
