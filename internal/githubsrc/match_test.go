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

	// Windows PE FileVersion / ProductVersion often has a fourth number.
	if Newer("v0.1.2", "0.1.2.1056") {
		t.Fatal("fourth PE number is a revision, not a newer release")
	}

	if Newer("0.1.2.0", "v0.1.2") {
		t.Fatal("padded Windows product version")
	}

	if !Newer("v0.1.3", "0.1.2.9") {
		t.Fatal("real bump still newer")
	}

	if Display("0.1.2.9") != "0.1.2" {
		t.Fatalf("display %s", Display("0.1.2.9"))
	}
}

func TestMatchSkipsInstallerWhenZipExists(t *testing.T) {
	t.Parallel()

	assets := []Asset{
		{Name: "captain-updater.amd64.installer.exe"},
		{Name: "captain-updater.amd64.exe.zip", URL: "https://example/zip"},
	}

	got, err := Match(assets, "amd64")
	if err != nil {
		t.Fatal(err)
	}

	if got.Name != "captain-updater.amd64.exe.zip" {
		t.Fatalf("got %s", got.Name)
	}
}

func TestMatchIgnoresSevenZ(t *testing.T) {
	t.Parallel()

	_, err := Match([]Asset{{Name: "app_windows_amd64.7z"}}, "amd64")
	if err == nil {
		t.Fatal("expected no match for 7z-only")
	}
}

func TestDownloadURL(t *testing.T) {
	t.Parallel()

	a := Asset{URL: "https://github.com/file.zip", APIURL: "https://api.github.com/repos/o/r/releases/assets/1"}
	if a.DownloadURL("") != a.URL {
		t.Fatal("public download")
	}

	if a.DownloadURL("tok") != a.APIURL {
		t.Fatal("private download uses API url")
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
