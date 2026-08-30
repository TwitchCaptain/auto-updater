package githubsrc

import (
	"testing"
)

func TestMatchAutobrr(t *testing.T) {
	t.Parallel()

	assets := []Asset{
		{Name: "autobrr_1.82.1_darwin_arm64.tar.gz"},
		{Name: "autobrr_1.82.1_linux_x86_64.tar.gz"},
		{Name: "autobrr_1.82.1_windows_x86_64.zip", URL: "https://example/win.zip"},
		{Name: "checksums.txt"},
	}

	got, err := Match(assets, "amd64")
	if err != nil {
		t.Fatal(err)
	}

	if got.Name != "autobrr_1.82.1_windows_x86_64.zip" {
		t.Fatalf("got %s", got.Name)
	}
}

func TestMatchUnpackerrArchInName(t *testing.T) {
	t.Parallel()

	assets := []Asset{
		{Name: "unpackerr_1.0.0_linux_amd64.tar.gz"},
		{Name: "unpackerr.amd64.exe.zip", URL: "https://example/u.zip"},
		{Name: "unpackerr.arm64.exe.zip"},
	}

	got, err := Match(assets, "amd64")
	if err != nil {
		t.Fatal(err)
	}

	if got.Name != "unpackerr.amd64.exe.zip" {
		t.Fatalf("got %s", got.Name)
	}

	got, err = Match(assets, "arm64")
	if err != nil {
		t.Fatal(err)
	}

	if got.Name != "unpackerr.arm64.exe.zip" {
		t.Fatalf("got %s", got.Name)
	}
}

func TestMatchSingleCandidate(t *testing.T) {
	t.Parallel()

	assets := []Asset{
		{Name: "notes.md"},
		{Name: "MyApp.zip", URL: "https://example/one.zip"},
	}

	got, err := Match(assets, "amd64")
	if err != nil {
		t.Fatal(err)
	}

	if got.Name != "MyApp.zip" {
		t.Fatalf("got %s", got.Name)
	}
}

func TestMatchPrefersZip(t *testing.T) {
	t.Parallel()

	assets := []Asset{
		{Name: "app_windows_amd64.exe"},
		{Name: "app_windows_amd64.zip"},
	}

	got, err := Match(assets, "amd64")
	if err != nil {
		t.Fatal(err)
	}

	if got.Name != "app_windows_amd64.zip" {
		t.Fatalf("got %s", got.Name)
	}
}

func TestNewer(t *testing.T) {
	t.Parallel()

	if !Newer("v1.2.0", "1.1.9") {
		t.Fatal("expected newer")
	}

	if Newer("1.0.0", "v1.0.0") {
		t.Fatal("same version")
	}

	if Newer("1.0.0", "1.1.0") {
		t.Fatal("older")
	}

	if !Newer("v1.0.0", "") {
		t.Fatal("empty local is older than a release")
	}
}

func TestMatchCaptainUpdater(t *testing.T) {
	t.Parallel()

	assets := []Asset{
		{Name: "captain-updater.amd64.exe.zip", URL: "https://example/amd.zip"},
		{Name: "captain-updater.arm64.exe.zip"},
		{Name: "captain-updater.amd64.installer.exe"},
		{Name: "checksums.txt"},
	}

	got, err := Match(assets, "amd64")
	if err != nil {
		t.Fatal(err)
	}

	if got.Name != "captain-updater.amd64.exe.zip" {
		t.Fatalf("got %s", got.Name)
	}
}

func TestMatchNoAsset(t *testing.T) {
	t.Parallel()

	_, err := Match([]Asset{{Name: "README.md"}}, "amd64")
	if err == nil {
		t.Fatal("expected error")
	}
}
