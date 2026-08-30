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
