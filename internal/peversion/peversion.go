package peversion

import "debug/pe"

// FileVersion reads ProductVersion (then FileVersion x.y.z) from a Windows PE.
// The PE FileVersion fourth number is a build/revision, not the GitHub tag.
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
