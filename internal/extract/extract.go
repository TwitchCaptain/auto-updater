package extract

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// FromZip copies the primary exe (matching destExe basename) and extra file
// names or globs from zpath into destDir. Extra globs match the zip entry
// basename (path.Match, case-insensitive).
func FromZip(zpath, destDir, destExe string, extra []string) ([]string, error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}

	members, err := selectMembers(zpath, destExe, extra)
	if err != nil {
		return nil, err
	}

	r, err := zip.OpenReader(zpath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var written []string

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		base := fileBase(f.Name)
		if !members[strings.ToLower(base)] {
			continue
		}

		outPath := filepath.Join(destDir, base)
		if err := copyZipFile(f, outPath); err != nil {
			return written, err
		}

		written = append(written, outPath)
		delete(members, strings.ToLower(base))
	}

	if len(members) > 0 {
		var missing []string
		for n := range members {
			missing = append(missing, n)
		}

		return written, fmt.Errorf("archive missing: %s", strings.Join(missing, ", "))
	}

	return written, nil
}

// Names lists zip entry basenames (files only).
func Names(zpath string) ([]string, error) {
	r, err := zip.OpenReader(zpath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var out []string
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		out = append(out, fileBase(f.Name))
	}

	return out, nil
}

// WouldWrite lists dest filenames that FromZip would create.
func WouldWrite(destExe string, extra []string) []string {
	seen := map[string]bool{}
	var out []string

	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}

		base := fileBase(name)
		key := strings.ToLower(base)
		if seen[key] || base == "." || base == "/" {
			return
		}

		seen[key] = true
		out = append(out, base)
	}

	add(destExe)
	for _, e := range extra {
		add(e)
	}

	return out
}

func selectMembers(zpath, destExe string, extra []string) (map[string]bool, error) {
	names, err := Names(zpath)
	if err != nil {
		return nil, err
	}

	want := map[string]bool{}
	patterns := append([]string{fileBase(destExe)}, extra...)

	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		matched := false
		for _, n := range names {
			if nameMatch(p, n) {
				want[strings.ToLower(n)] = true
				matched = true
			}
		}

		if !matched && !hasMeta(p) {
			want[strings.ToLower(fileBase(p))] = true
		} else if !matched {
			return nil, fmt.Errorf("archive missing: %s", p)
		}
	}

	return want, nil
}

func nameMatch(pattern, name string) bool {
	p := strings.ToLower(fileBase(pattern))
	n := strings.ToLower(fileBase(name))
	if p == n {
		return true
	}

	ok, err := path.Match(p, n)

	return err == nil && ok
}

func hasMeta(pattern string) bool {
	return strings.ContainsAny(fileBase(pattern), "*?[")
}

func fileBase(name string) string {
	name = strings.ReplaceAll(name, `\`, "/")

	return path.Base(name)
}

func copyZipFile(f *zip.File, dest string) error {
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	tmp := dest + ".new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, src); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)

		return err
	}

	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)

		return err
	}

	_ = os.Remove(dest)

	return os.Rename(tmp, dest)
}
