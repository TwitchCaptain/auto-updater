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

	if strings.HasPrefix(s, "git@") {
		if i := strings.Index(s, ":"); i >= 0 {
			s = s[i+1:]
		}
	}

	lower := strings.ToLower(s)
	for _, prefix := range []string{
		"git+https://github.com/",
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

	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}

	s = strings.Trim(s, "/")
	s = strings.TrimSuffix(s, ".git")
	s = strings.Trim(s, "/")
	parts := strings.Split(s, "/")
	if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
		name := strings.TrimSuffix(parts[1], ".git")

		return parts[0] + "/" + name
	}

	return s
}

// ValidRepo is true for owner/name after NormalizeRepo.
func ValidRepo(s string) bool {
	s = NormalizeRepo(s)
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return false
	}

	return parts[0] != "" && parts[1] != "" && !strings.ContainsAny(s, " \t")
}
