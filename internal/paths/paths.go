package paths

import (
	"os"
	"path/filepath"
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
