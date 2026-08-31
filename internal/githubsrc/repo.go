package githubsrc

import "strings"

// NormalizeRepo turns a pasted GitHub URL or SSH remote into owner/name.
func NormalizeRepo(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'`)
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimRight(s, "/")

	if strings.HasPrefix(s, "git@") {
		if i := strings.Index(s, ":"); i >= 0 {
			s = s[i+1:]
		}
	}

	lower := strings.ToLower(s)
	for _, prefix := range []string{
		"https://github.com/",
		"http://github.com/",
		"https://www.github.com/",
		"http://www.github.com/",
		"github.com/",
		"www.github.com/",
	} {
		if strings.HasPrefix(lower, prefix) {
			s = s[len(prefix):]
			break
		}
	}

	s = strings.Trim(s, "/")
	parts := strings.Split(s, "/")
	if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
		return parts[0] + "/" + parts[1]
	}

	return s
}
