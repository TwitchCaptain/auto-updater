package protocol

import "testing"

func TestParseAppID(t *testing.T) {
	t.Parallel()

	id := "3f0e2d9a-1111-4b2c-8c3d-abcdef123456"
	cases := []struct {
		in, want string
	}{
		{"captainupdater://app/" + id, id},
		{`"` + "captainupdater://app/" + id + `"`, id},
		{"captainupdater://app/" + id + "/", id},
		{"CAPTAINUPDATER://APP/" + id, id},
		{"  captainupdater://app/" + id + "  ", id},
		{"captainupdater://app/" + id + "?focus=1", id},
		{"captainupdater://app/" + id + "#upgrade", id},
		{`'captainupdater://app/` + id + `'`, id},
		{`captainupdater://app/` + id + `/"`, id},
		{"not-a-url", ""},
		{"captainupdater://", ""},
		{"", ""},
	}

	for _, tc := range cases {
		if got := ParseAppID(tc.in); got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}

	if !IsURL("CaptainUpdater://app/x") {
		t.Fatal("expected protocol URL")
	}

	if IsURL("https://example") {
		t.Fatal("not a protocol URL")
	}
}
