// Package upgrade downloads a GitHub release asset and replaces the primary exe.
package upgrade

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/TwitchCaptain/auto-updater/internal/config"
	"github.com/TwitchCaptain/auto-updater/internal/extract"
	"github.com/TwitchCaptain/auto-updater/internal/githubsrc"
	"github.com/TwitchCaptain/auto-updater/internal/host"
	"github.com/TwitchCaptain/auto-updater/internal/locver"
	"github.com/TwitchCaptain/auto-updater/internal/peversion"
)

// Result of a check or upgrade.
type Result struct {
	Local      string   `json:"local"`
	Remote     string   `json:"remote"`
	AssetName  string   `json:"assetName"`
	AssetURL   string   `json:"assetURL"`
	AssetSize  int64    `json:"assetSize"`
	Newer      bool     `json:"newer"`
	HTMLURL    string   `json:"htmlUrl"`
	WouldWrite []string `json:"wouldWrite,omitempty"`
	Restart    bool     `json:"restart"`
}

// Engine checks GitHub and applies upgrades.
type Engine struct {
	GH *githubsrc.Client
}

func (e *Engine) Check(ctx context.Context, app config.App) (*Result, error) {
	rel, err := e.GH.Latest(ctx, app.OwnerRepo, "")
	if err != nil {
		return nil, err
	}

	local, _ := locver.Read(ctx, app, peversion.FileVersion)
	remote := strings.TrimPrefix(rel.TagName, "v")

	return &Result{
		Local:      local,
		Remote:     remote,
		AssetName:  rel.Asset.Name,
		AssetURL:   rel.Asset.URL,
		AssetSize:  rel.Asset.Size,
		Newer:      local == "" || githubsrc.Newer(rel.TagName, local),
		HTMLURL:    rel.HTMLURL,
		WouldWrite: extract.WouldWrite(app.ExePath, app.ExtraFiles),
	}, nil
}

func (e *Engine) Upgrade(ctx context.Context, app config.App) (*Result, error) {
	res, err := e.Check(ctx, app)
	if err != nil {
		return nil, err
	}

	if !res.Newer {
		return res, nil
	}

	tmp, err := os.MkdirTemp("", "captain-updater-*")
	if err != nil {
		return res, err
	}
	defer os.RemoveAll(tmp)

	dl := filepath.Join(tmp, sanitize(res.AssetName))
	if err := e.download(ctx, res.AssetURL, dl); err != nil {
		return res, err
	}

	destDir := filepath.Dir(app.ExePath)
	staging := filepath.Join(tmp, "out")
	self := host.IsSelf(app.ExePath)

	if strings.HasSuffix(strings.ToLower(res.AssetName), ".zip") {
		if _, err := extract.FromZip(dl, staging, app.ExePath, app.ExtraFiles); err != nil {
			return res, err
		}
	} else {
		if err := os.MkdirAll(staging, 0o755); err != nil {
			return res, err
		}

		if err := copyFile(dl, filepath.Join(staging, filepath.Base(app.ExePath))); err != nil {
			return res, err
		}
	}

	entries, err := os.ReadDir(staging)
	if err != nil {
		return res, err
	}

	svcName := ""
	if !self {
		svcName = host.FindService(app.ExePath)
		_ = host.Stop(app.ExePath)
		if svcName != "" {
			_ = host.StopService(svcName)
		}

		_ = host.WaitStopped(app.ExePath, 15*time.Second)
	}

	var written []string
	var selfNew string

	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}

		src := filepath.Join(staging, ent.Name())
		dest := filepath.Join(destDir, ent.Name())
		if self && strings.EqualFold(ent.Name(), filepath.Base(app.ExePath)) {
			selfNew = dest + ".new"
			if err := copyFile(src, selfNew); err != nil {
				return res, err
			}

			written = append(written, selfNew)

			continue
		}

		if err := host.Place(ctx, src, dest); err != nil {
			return res, fmt.Errorf("write %s: %w", dest, err)
		}

		written = append(written, dest)
	}

	res.WouldWrite = written

	if self {
		if selfNew == "" {
			return res, fmt.Errorf("archive did not contain %s", filepath.Base(app.ExePath))
		}

		if err := host.ScheduleReplace(selfNew, app.ExePath); err != nil {
			return res, err
		}

		res.Restart = true

		return res, nil
	}

	if svcName != "" {
		if err := host.StartService(svcName); err != nil {
			return res, fmt.Errorf("files copied; service start failed: %w", err)
		}

		return res, nil
	}

	if err := host.Start(ctx, app.ExePath, app.ShortcutPath); err != nil {
		return res, fmt.Errorf("files copied; start failed: %w", err)
	}

	return res, nil
}

func (e *Engine) download(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "CaptainUpdater")
	req.Header.Set("Accept", "application/octet-stream")
	if e.GH != nil && e.GH.Token != "" {
		req.Header.Set("Authorization", "Bearer "+e.GH.Token)
	}

	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s", resp.Status)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)

	return err
}

func copyFile(src, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	return os.WriteFile(dest, data, 0o755)
}

func sanitize(name string) string {
	name = strings.ReplaceAll(name, `\`, "/")
	name = path.Base(name)
	name = strings.ReplaceAll(name, "..", "")

	return name
}
