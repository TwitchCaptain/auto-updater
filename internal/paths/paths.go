package paths

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const DirName = "CaptainUpdater"

// Dir returns %APPDATA%/CaptainUpdater (or the platform equivalent).
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(base, DirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	return dir, nil
}

func ConfigFile() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "config.json"), nil
}

func HistoryFile() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "history.jsonl"), nil
}

func ActivateFile() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "activate.url"), nil
}

// CleanUserPath turns a browse/drop/typed path into a filesystem path.
// WebView2 on Windows sometimes yields file:///C:/... URLs or quoted strings.
func CleanUserPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, `"'`)
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}

	if strings.HasPrefix(strings.ToLower(p), "file:") {
		return fromFileURL(p)
	}

	return p
}

func fromFileURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return strings.TrimPrefix(raw, "file://")
	}

	path := u.Path
	if path == "" && u.Opaque != "" {
		path = u.Opaque
	}

	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}

	host := u.Hostname()
	if host != "" && !strings.EqualFold(host, "localhost") {
		return `\\` + host + filepath.FromSlash(path)
	}

	// file:///C:/foo and file://localhost/C:/foo
	if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}

	return filepath.FromSlash(path)
}

// IsShortcut reports whether p looks like a Windows .lnk.
func IsShortcut(p string) bool {
	return strings.EqualFold(filepath.Ext(p), ".lnk")
}
