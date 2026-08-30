package githubsrc

import (
	"runtime"
	"strings"
)

// Asset is a GitHub release asset we might download.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

var archTokens = map[string][]string{
	"amd64": {"amd64", "x86_64", "x64", "win64", "windows64"},
	"386":   {"386", "i386", "i686", "win32", "x86"},
	"arm64": {"arm64", "aarch64"},
	"arm":   {"armv7", "armhf", "armv6"},
}

var otherOS = []string{"darwin", "macos", "osx", "linux", "freebsd", "android", "openbsd", "netbsd"}

// Match picks the Windows asset for hostArch (GOARCH). If hostArch is empty,
// runtime.GOARCH is used.
func Match(assets []Asset, hostArch string) (Asset, error) {
	if hostArch == "" {
		hostArch = runtime.GOARCH
	}

	cands := windowsCandidates(assets)
	if len(cands) == 0 {
		return Asset{}, errNoAsset
	}

	archHits := filterArch(cands, hostArch)
	switch {
	case len(archHits) == 1:
		return preferZip(archHits), nil
	case len(archHits) > 1:
		return preferZip(archHits), nil
	case len(cands) == 1:
		return cands[0], nil
	default:
		return preferZip(cands), nil
	}
}

func windowsCandidates(assets []Asset) []Asset {
	var out []Asset

	for _, a := range assets {
		name := strings.ToLower(a.Name)
		if !isArchiveOrExe(name) {
			continue
		}
		if mentionsOtherOS(name) && !strings.Contains(name, "windows") && !strings.Contains(name, "win32") && !strings.Contains(name, "win64") {
			continue
		}

		out = append(out, a)
	}

	return out
}

func isArchiveOrExe(name string) bool {
	return strings.HasSuffix(name, ".zip") ||
		strings.HasSuffix(name, ".exe") ||
		strings.HasSuffix(name, ".7z") ||
		strings.Contains(name, ".exe.")
}

func mentionsOtherOS(name string) bool {
	for _, osName := range otherOS {
		if strings.Contains(name, osName) {
			return true
		}
	}

	return false
}

func filterArch(assets []Asset, hostArch string) []Asset {
	tokens := archTokens[hostArch]
	if len(tokens) == 0 {
		tokens = []string{hostArch}
	}

	var out []Asset

	for _, a := range assets {
		lower := strings.ToLower(a.Name)
		if containsToken(lower, tokens) {
			out = append(out, a)
		}
	}

	return out
}

func containsToken(name string, tokens []string) bool {
	for _, tok := range tokens {
		if tok == "x86" {
			// avoid matching x86_64
			if strings.Contains(name, "x86_64") {
				continue
			}
		}
		if tok == "arm" {
			if strings.Contains(name, "arm64") || strings.Contains(name, "aarch64") {
				continue
			}
		}

		if strings.Contains(name, tok) {
			return true
		}
	}

	return false
}

func preferZip(assets []Asset) Asset {
	for _, a := range assets {
		if strings.HasSuffix(strings.ToLower(a.Name), ".zip") {
			return a
		}
	}

	return assets[0]
}
