package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TwitchCaptain/auto-updater/internal/crypt"
	"github.com/TwitchCaptain/auto-updater/internal/schedule"
)

func TestPlainRoundTrip(t *testing.T) {
	t.Parallel()

	p := filepath.Join(t.TempDir(), "config.json")
	s := New(p)
	if err := s.Detect(); err != nil {
		t.Fatal(err)
	}

	in := Settings{GitHubToken: "tok", Apps: []App{{Name: "x", OwnerRepo: "o/r", ExePath: "c:\\x.exe"}}}
	if err := s.Replace(in); err != nil {
		t.Fatal(err)
	}

	s2 := New(p)
	if err := s2.Detect(); err != nil {
		t.Fatal(err)
	}

	if s2.EncryptedOnDisk() {
		t.Fatal("should be plaintext")
	}

	if s2.Settings().GitHubToken != "tok" {
		t.Fatal(s2.Settings())
	}
}

func TestSanitizeOnSave(t *testing.T) {
	t.Parallel()

	s := New(filepath.Join(t.TempDir(), "config.json"))
	_ = s.Detect()

	got, err := s.PatchApp(App{
		Name:      "x",
		OwnerRepo: "https://github.com/foo/bar.git",
		ExePath:   `file:///C:/Program%20Files/app.exe`,
		Schedules: []schedule.Slot{{Days: []time.Weekday{1}, Time: "4:00:00", Action: schedule.ActionNotify}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got.OwnerRepo != "foo/bar" {
		t.Fatalf("repo %s", got.OwnerRepo)
	}

	if got.Schedules[0].Time != "04:00" {
		t.Fatalf("time %s", got.Schedules[0].Time)
	}

	if got.ExePath == "" || strings.Contains(strings.ToLower(got.ExePath), "file:") {
		t.Fatalf("exe %s", got.ExePath)
	}
}

func TestPatchAppSameIDTwice(t *testing.T) {
	t.Parallel()

	s := New(filepath.Join(t.TempDir(), "config.json"))
	_ = s.Detect()

	first, err := s.PatchApp(App{ID: "same", Name: "n", OwnerRepo: "o/r", ExePath: `c:\x.exe`})
	if err != nil {
		t.Fatal(err)
	}

	second, err := s.PatchApp(App{ID: first.ID, Name: "n2", OwnerRepo: "o/r", ExePath: `c:\x.exe`})
	if err != nil {
		t.Fatal(err)
	}

	if first.ID != second.ID {
		t.Fatalf("id changed %s -> %s", first.ID, second.ID)
	}

	if n := len(s.Settings().Apps); n != 1 {
		t.Fatalf("apps %d", n)
	}

	if s.Settings().Apps[0].Name != "n2" {
		t.Fatal(s.Settings().Apps[0])
	}
}

func TestDeleteApp(t *testing.T) {
	t.Parallel()

	s := New(filepath.Join(t.TempDir(), "config.json"))
	_ = s.Detect()

	keep, err := s.PatchApp(App{Name: "keep", OwnerRepo: "o/k", ExePath: `c:\k.exe`})
	if err != nil {
		t.Fatal(err)
	}

	gone, err := s.PatchApp(App{Name: "gone", OwnerRepo: "o/g", ExePath: `c:\g.exe`})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteApp(""); err == nil {
		t.Fatal("empty id")
	}

	if err := s.DeleteApp("missing"); err == nil {
		t.Fatal("unknown id")
	}

	if err := s.DeleteApp(gone.ID); err != nil {
		t.Fatal(err)
	}

	got := s.Settings().Apps
	if len(got) != 1 || got[0].ID != keep.ID {
		t.Fatal(got)
	}

	if err := s.DeleteApp(gone.ID); err == nil {
		t.Fatal("already deleted")
	}
}

func TestEncryptRefusesPlainWrite(t *testing.T) {
	t.Parallel()

	p := filepath.Join(t.TempDir(), "config.json")
	s := New(p)
	_ = s.Detect()
	if err := s.Replace(Settings{Apps: []App{{Name: "n"}}}); err != nil {
		t.Fatal(err)
	}

	if err := s.EnableEncryption("pw"); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}

	if !crypt.IsEncrypted(raw) {
		t.Fatal("expected encrypted file")
	}

	s.Lock()
	if err := s.Replace(Settings{Apps: []App{{Name: "n"}}}); err == nil {
		t.Fatal("locked store must not write")
	}

	s3 := New(p)
	_ = s3.Detect()
	if !s3.EncryptedOnDisk() {
		t.Fatal("detect encrypt")
	}

	if err := s3.Unlock("pw"); err != nil {
		t.Fatal(err)
	}

	if err := s3.Unlock("nope"); err == nil {
		t.Fatal("wrong password on open store")
	}

	s4 := New(p)
	_ = s4.Detect()
	if err := s4.Unlock("bad"); err == nil {
		t.Fatal("wrong password")
	}
}
