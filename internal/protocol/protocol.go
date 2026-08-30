// Package protocol parses captainupdater:// URLs (toast clicks, second instance).
package protocol

import "strings"

const scheme = "captainupdater://"

// IsURL reports whether s looks like a Captain Updater protocol URL.
func IsURL(s string) bool {
	return strings.Contains(strings.ToLower(s), scheme)
}

// ParseAppID extracts the app id from captainupdater://app/<id>.
// Windows protocol handlers sometimes wrap the URL in quotes and/or a trailing slash.
func ParseAppID(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, `"'`)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	lower := strings.ToLower(raw)
	const marker = "://app/"
	i := strings.Index(lower, marker)
	if i < 0 {
		return ""
	}

	id := raw[i+len(marker):]
	id = strings.TrimSpace(id)
	id = strings.Trim(id, `"'/`)
	if j := strings.IndexAny(id, "?#"); j >= 0 {
		id = id[:j]
	}

	return strings.Trim(strings.TrimSpace(id), `"'/`)
}
