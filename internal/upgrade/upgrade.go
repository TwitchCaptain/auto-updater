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

	token := ""
	if e.GH != nil {
		token = e.GH.Token
	}

	return &Result{
		Local:      local,
		Remote:     remote,
		AssetName:  rel.Asset.Name,
		AssetURL:   rel.Asset.DownloadURL(token),
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
	lowerAsset := strings.ToLower(res.AssetName)

	switch {
	case strings.HasSuffix(lowerAsset, ".zip"):
		if _, err := extract.FromZip(dl, staging, app.ExePath, app.ExtraFiles); err != nil {
			return res, err
		}
	case strings.HasSuffix(lowerAsset, ".exe"):
		if err := os.MkdirAll(staging, 0o755); err != nil {
			return res, err
		}

		if err := copyFile(dl, filepath.Join(staging, filepath.Base(app.ExePath))); err != nil {
			return res, err
		}
	default:
		return res, fmt.Errorf("unsupported asset %s (need a .zip or .exe)", res.AssetName)
	}

	entries, err := os.ReadDir(staging)
	if err != nil {
		return res, err
	}

	svcName := ""
	wasRunning := false
	if !self {
		n, _ := host.RunningCount(app.ExePath)
		wasRunning = n > 0
		svcName = host.FindService(app.ExePath)
		_ = host.Stop(app.ExePath)
		if svcName != "" {
			_ = host.StopService(svcName)
		}

		_ = host.WaitStopped(app.ExePath, 15*time.Second)
	}

	var copies []host.FileCopy
	var selfSrc string

	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}

		src := filepath.Join(staging, ent.Name())
		dest := filepath.Join(destDir, ent.Name())
		if self && strings.EqualFold(ent.Name(), filepath.Base(app.ExePath)) {
			selfSrc = src

			continue
		}

		copies = append(copies, host.FileCopy{Src: src, Dest: dest})
	}

	if self {
		if selfSrc == "" {
			return res, fmt.Errorf("archive did not contain %s", filepath.Base(app.ExePath))
		}

		if len(copies) > 0 {
			if err := host.PlaceMany(ctx, copies); err != nil {
				return res, err
			}
		}

		// Must live outside tmp: Upgrade's defer RemoveAll would delete a staged .new.
		selfNew, err := persistSelfImage(selfSrc, filepath.Base(app.ExePath))
		if err != nil {
			return res, err
		}

		if err := host.ScheduleReplace(selfNew, app.ExePath); err != nil {
			return res, err
		}

		res.WouldWrite = []string{app.ExePath}
		res.Restart = true

		return res, nil
	}

	if err := host.PlaceMany(ctx, copies); err != nil {
		return res, err
	}

	written := make([]string, 0, len(copies))
	for _, c := range copies {
		written = append(written, c.Dest)
	}

	res.WouldWrite = written

	if svcName != "" {
		if err := host.StartService(svcName); err != nil {
			return res, fmt.Errorf("files copied; service start failed: %w", err)
		}

		return res, nil
	}

	if !wasRunning {
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

func persistSelfImage(src, base string) (string, error) {
	dest := filepath.Join(os.TempDir(), "cu-self-"+sanitize(base))
	if err := copyFile(src, dest); err != nil {
		return "", err
	}

	return dest, nil
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
