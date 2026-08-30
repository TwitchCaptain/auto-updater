package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TwitchCaptain/auto-updater/internal/crypt"
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
