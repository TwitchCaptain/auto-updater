package locver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TwitchCaptain/auto-updater/internal/config"
)

func TestHTTPJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Token") != "secret" {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		_, _ = w.Write([]byte(`{"config":{"version":"1.82.1"}}`))
	}))
	t.Cleanup(srv.Close)

	v, err := Read(context.Background(), config.App{
		VersionHTTP:       srv.URL,
		VersionJSON:       "config.version",
		VersionHTTPSecret: "secret",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if v != "1.82.1" {
		t.Fatalf("got %s", v)
	}
}

func TestLastAppliedFallback(t *testing.T) {
	t.Parallel()

	v, err := Read(context.Background(), config.App{LastVersion: "0.9.0"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if v != "0.9.0" {
		t.Fatalf("got %s", v)
	}
}

func TestParseVersion(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"unpackerr v0.14.5-123\n  Branch: master": "0.14.5-123",
		"v1.82.1":                           "1.82.1",
		"version 2.0.0":                     "2.0.0",
		"unknown shorthand flag: 'v' in -v": "",
		"":                                  "",
	}
	for in, want := range cases {
		if got := parseVersion(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestFromCLI(t *testing.T) {
	orig := versionCmd
	t.Cleanup(func() { versionCmd = orig })

	versionCmd = func(_ context.Context, _ string, args []string) (string, error) {
		if len(args) > 0 && args[0] == "-v" {
			return "unpackerr v0.14.5\n", nil
		}

		return "nope", nil
	}

	v, err := Read(context.Background(), config.App{ExePath: `C:\Program Files\unpackerr\unpackerr.exe`}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if v != "0.14.5" {
		t.Fatalf("got %s", v)
	}
}
