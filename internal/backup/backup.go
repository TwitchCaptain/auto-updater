package backup

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Zip writes config.json and history.jsonl (if present) into dest.
func Zip(dest, configPath, historyPath string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}

	zw := zip.NewWriter(f)
	add := func(name, src string) error {
		in, err := os.Open(src)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}

			return err
		}
		defer in.Close()

		st, err := in.Stat()
		if err != nil {
			return err
		}

		hdr, err := zip.FileInfoHeader(st)
		if err != nil {
			return err
		}

		hdr.Name = name
		hdr.Method = zip.Deflate
		hdr.Modified = st.ModTime()
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}

		_, err = io.Copy(w, in)

		return err
	}

	if err := add("config.json", configPath); err != nil {
		_ = zw.Close()
		_ = f.Close()
		_ = os.Remove(dest)

		return err
	}

	if err := add("history.jsonl", historyPath); err != nil {
		_ = zw.Close()
		_ = f.Close()
		_ = os.Remove(dest)

		return err
	}

	readme, err := zw.CreateHeader(&zip.FileHeader{
		Name:     "README.txt",
		Method:   zip.Deflate,
		Modified: time.Now(),
	})
	if err != nil {
		_ = zw.Close()
		_ = f.Close()
		_ = os.Remove(dest)

		return err
	}

	_, _ = io.WriteString(readme, "Captain Updater backup. Restore by copying config.json (and optionally history.jsonl) into the app data folder.\n")

	if err := zw.Close(); err != nil {
		_ = f.Close()
		_ = os.Remove(dest)

		return err
	}

	return f.Close()
}
