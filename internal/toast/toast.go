// Package toast shows a sticky Windows notification that opens captainupdater://.
package toast

// Note is one toast.
type Note struct {
	AppID   string
	Title   string
	Message string
	URL     string
}

// Show displays n. On non-Windows it is a no-op.
func Show(n Note) error {
	return show(n)
}
