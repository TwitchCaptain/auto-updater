package githubsrc

import "testing"

func TestNormalizeRepo(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in, want string
	}{
		{"autobrr/autobrr", "autobrr/autobrr"},
		{" https://github.com/autobrr/autobrr ", "autobrr/autobrr"},
		{"https://github.com/autobrr/autobrr.git", "autobrr/autobrr"},
		{"https://github.com/autobrr/autobrr.git/", "autobrr/autobrr"},
		{"https://github.com/autobrr/autobrr/", "autobrr/autobrr"},
		{"https://github.com/Unpackerr/unpackerr/releases", "Unpackerr/unpackerr"},
		{"https://github.com/Unpackerr/unpackerr/releases/tag/v0.16.1", "Unpackerr/unpackerr"},
		{"github.com/TwitchCaptain/auto-updater", "TwitchCaptain/auto-updater"},
		{"git@github.com:Unpackerr/unpackerr.git", "Unpackerr/unpackerr"},
		{`"https://github.com/foo/bar"`, "foo/bar"},
		{"", ""},
		{"not-a-repo", "not-a-repo"},
	}

	for _, tc := range cases {
		if got := NormalizeRepo(tc.in); got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}

	if !ValidRepo("foo/bar") {
		t.Fatal("expected valid")
	}

	if ValidRepo("not-a-repo") || ValidRepo("") {
		t.Fatal("expected invalid")
	}
}
