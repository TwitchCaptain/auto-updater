package peversion

import "debug/pe"

// FileVersion reads ProductVersion / FileVersion from a Windows PE if present.
// Works on any OS because it parses the binary.
func FileVersion(path string) (string, error) {
	f, err := pe.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	v, err := fromPE(f)
	if err != nil {
		return "", err
	}

	return v, nil
}
