package githubsrc

import "testing"

func TestNormalizeRepo(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"autobrr/autobrr":                        "autobrr/autobrr",
		" https://github.com/autobrr/autobrr ":   "autobrr/autobrr",
		"https://github.com/autobrr/autobrr.git": "autobrr/autobrr",
		"https://github.com/autobrr/autobrr/":    "autobrr/autobrr",
		"github.com/TwitchCaptain/auto-updater":  "TwitchCaptain/auto-updater",
		"git@github.com:Unpackerr/unpackerr.git": "Unpackerr/unpackerr",
		`"https://github.com/foo/bar"`:           "foo/bar",
		"":                                       "",
		"not-a-repo":                             "not-a-repo",
	}

	for in, want := range cases {
		if got := NormalizeRepo(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}
