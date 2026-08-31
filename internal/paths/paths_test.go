package paths

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestCleanUserPath(t *testing.T) {
	t.Parallel()

	plain := `C:\Apps\app.exe`
	if got := CleanUserPath(`  ` + plain + `  `); got != plain {
		t.Fatalf("trim spaces: %q", got)
	}

	if got := CleanUserPath(`"` + plain + `"`); got != plain {
		t.Fatalf("trim quotes: %q", got)
	}

	got := CleanUserPath(`file:///C:/Program%20Files/app.exe`)
	want := filepath.FromSlash(`C:/Program Files/app.exe`)
	if runtime.GOOS != "windows" {
		// FromSlash is a no-op on Unix; the drive letter path is still usable.
		want = `C:/Program Files/app.exe`
	}

	if got != want {
		t.Fatalf("file URL: got %q want %q", got, want)
	}

	if CleanUserPath("") != "" {
		t.Fatal("empty")
	}

	if !IsShortcut(`C:\Users\Public\Desktop\Captain Updater.lnk`) {
		t.Fatal("public desktop shortcut")
	}

	if IsShortcut(`C:\Apps\app.exe`) {
		t.Fatal("exe is not a shortcut")
	}
}
