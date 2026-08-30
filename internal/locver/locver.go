package locver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/TwitchCaptain/auto-updater/internal/config"
)

// Read current installed version: HTTP JSON, then PE FileVersion, then exe -v/--version, then last-applied.
func Read(ctx context.Context, app config.App, pe func(string) (string, error)) (string, error) {
	if app.VersionHTTP != "" {
		v, err := httpJSON(ctx, app)
		if err == nil && v != "" {
			return v, nil
		}
	}

	if pe != nil && app.ExePath != "" {
		v, err := pe(app.ExePath)
		if err == nil && v != "" {
			return v, nil
		}
	}

	if app.ExePath != "" {
		v, err := fromCLI(ctx, app.ExePath)
		if err == nil && v != "" {
			return v, nil
		}
	}

	if app.LastVersion != "" {
		return app.LastVersion, nil
	}

	return "", errors.New("could not determine installed version")
}

func httpJSON(ctx context.Context, app config.App) (string, error) {
	key := app.VersionJSON
	if key == "" {
		key = "version"
	}

	key = strings.TrimPrefix(key, "/")
	key = strings.ReplaceAll(key, "/", ".")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, app.VersionHTTP, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "CaptainUpdater")
	if app.VersionHTTPSecret != "" {
		header := app.VersionHTTPHeader
		if header == "" {
			header = "X-API-Token"
		}

		req.Header.Set(header, app.VersionHTTPSecret)
	}

	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("version http %s", resp.Status)
	}

	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", err
	}

	v, err := dig(raw, strings.Split(key, "."))
	if err != nil {
		return "", err
	}

	return strings.TrimPrefix(v, "v"), nil
}

func dig(v any, path []string) (string, error) {
	cur := v
	for _, p := range path {
		if p == "" {
			continue
		}

		m, ok := cur.(map[string]any)
		if !ok {
			return "", fmt.Errorf("json path %q", strings.Join(path, "."))
		}

		cur, ok = m[p]
		if !ok {
			return "", fmt.Errorf("json key %q missing", p)
		}
	}

	switch t := cur.(type) {
	case string:
		return t, nil
	default:
		return fmt.Sprint(t), nil
	}
}
