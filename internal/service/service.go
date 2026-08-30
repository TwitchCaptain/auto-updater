// Package service is the Wails backend: config, GitHub checks, upgrades, and history.
package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/TwitchCaptain/auto-updater/internal/config"
	"github.com/TwitchCaptain/auto-updater/internal/extract"
	"github.com/TwitchCaptain/auto-updater/internal/githubsrc"
	"github.com/TwitchCaptain/auto-updater/internal/history"
	"github.com/TwitchCaptain/auto-updater/internal/host"
	"github.com/TwitchCaptain/auto-updater/internal/locver"
	"github.com/TwitchCaptain/auto-updater/internal/peversion"
	"github.com/TwitchCaptain/auto-updater/internal/preset"
	"github.com/TwitchCaptain/auto-updater/internal/schedule"
	"github.com/TwitchCaptain/auto-updater/internal/scheduler"
	"github.com/TwitchCaptain/auto-updater/internal/toast"
	"github.com/TwitchCaptain/auto-updater/internal/upgrade"
)

const protocol = "captainupdater"

type Service struct {
	app   *application.App
	win   application.Window
	store *config.Store
	hist  *history.Log
	eng   *upgrade.Engine
	sched *scheduler.Runner
}

func New(store *config.Store, hist *history.Log) *Service {
	s := &Service{
		store: store,
		hist:  hist,
		eng:   &upgrade.Engine{GH: &githubsrc.Client{}},
	}
	s.sched = scheduler.New(store, s.onSlot)

	return s
}

//wails:ignore
func (s *Service) SetApp(app *application.App, win application.Window) {
	s.app = app
	s.win = win
	s.eng.GH.Token = s.store.Settings().GitHubToken
	s.sched.Start()
}

//wails:ignore
func (s *Service) Stop() {
	if s.sched != nil {
		s.sched.Stop()
	}
}

func (s *Service) NeedsUnlock() bool {
	return s.store.EncryptedOnDisk() && !s.store.Unlocked()
}

func (s *Service) Encrypted() bool {
	return s.store.EncryptedOnDisk()
}

func (s *Service) Unlock(password string) error {
	if err := s.store.Unlock(password); err != nil {
		return err
	}

	s.eng.GH.Token = s.store.Settings().GitHubToken

	return nil
}

func (s *Service) Lock() {
	s.store.Lock()
	s.hide()
}

func (s *Service) EnableEncryption(password string) error {
	return s.store.EnableEncryption(password)
}

func (s *Service) DisableEncryption(password string) error {
	return s.store.DisableEncryption(password)
}

func (s *Service) GetConfig() (config.Settings, error) {
	if err := s.requireOpen(); err != nil {
		return config.Settings{}, err
	}

	return s.store.Settings(), nil
}

func (s *Service) SaveConfig(in config.Settings) error {
	if err := s.requireOpen(); err != nil {
		return err
	}

	if err := s.store.Replace(in); err != nil {
		return err
	}

	s.eng.GH.Token = in.GitHubToken
	exe, _ := os.Executable()
	_ = host.SetStartup(in.StartWithWindows, exe)

	return nil
}

func (s *Service) SaveApp(app config.App) error {
	if err := s.requireOpen(); err != nil {
		return err
	}

	return s.store.PatchApp(app)
}

func (s *Service) DeleteApp(id string) error {
	if err := s.requireOpen(); err != nil {
		return err
	}

	set := s.store.Settings()
	out := set.Apps[:0]
	for _, a := range set.Apps {
		if a.ID != id {
			out = append(out, a)
		}
	}

	set.Apps = out

	return s.store.Replace(set)
}

func (s *Service) Presets() []preset.Info {
	return preset.All()
}

func (s *Service) ApplyPreset(id string) (config.App, error) {
	return preset.Apply(id)
}

func (s *Service) PreviewGitHub(ownerRepo string) (*githubsrc.Release, error) {
	if err := s.requireOpen(); err != nil {
		return nil, err
	}

	s.eng.GH.Token = s.store.Settings().GitHubToken

	return s.eng.GH.Latest(context.Background(), ownerRepo, runtime.GOARCH)
}

func (s *Service) WouldWrite(exePath string, extra []string) []string {
	return extract.WouldWrite(exePath, extra)
}

func (s *Service) LocalVersion(app config.App) (string, error) {
	return locver.Read(context.Background(), app, peversion.FileVersion)
}

type CheckResult struct {
	upgrade.Result
	AppID string `json:"appId"`
}

func (s *Service) CheckNow(id string) (*CheckResult, error) {
	if err := s.requireOpen(); err != nil {
		return nil, err
	}

	app, ok := s.store.App(id)
	if !ok {
		return nil, errors.New("unknown app")
	}

	res, err := s.eng.Check(context.Background(), app)
	if err != nil {
		_ = s.hist.Append(history.Event{AppID: app.ID, AppName: app.Name, Action: history.ActionError, Error: err.Error()})
		s.emitHistory()

		return nil, err
	}

	_ = s.hist.Append(history.Event{
		AppID: app.ID, AppName: app.Name, Action: history.ActionCheck,
		From: res.Local, To: res.Remote, Asset: res.AssetName, Result: fmt.Sprintf("newer=%v", res.Newer),
	})
	s.emitHistory()

	return &CheckResult{Result: *res, AppID: id}, nil
}

func (s *Service) UpgradeNow(id string) (*CheckResult, error) {
	if err := s.requireOpen(); err != nil {
		return nil, err
	}

	app, ok := s.store.App(id)
	if !ok {
		return nil, errors.New("unknown app")
	}

	res, err := s.eng.Upgrade(context.Background(), app)
	if err != nil {
		_ = s.hist.Append(history.Event{
			AppID: app.ID, AppName: app.Name, Action: history.ActionError,
			From: resField(res, true), To: resField(res, false), Asset: assetName(res), Error: err.Error(),
		})
		s.emitHistory()

		return nil, err
	}

	_ = s.store.SetLastVersion(app.ID, res.Remote)
	_ = s.hist.Append(history.Event{
		AppID: app.ID, AppName: app.Name, Action: history.ActionUpgrade,
		From: res.Local, To: res.Remote, Asset: res.AssetName, Result: "ok",
	})
	s.emitHistory()

	if res.Restart {
		go func() {
			time.Sleep(400 * time.Millisecond)
			if s.app != nil {
				s.app.Quit()
			}
		}()
	}

	return &CheckResult{Result: *res, AppID: id}, nil
}

func (s *Service) History(limit int) ([]history.Event, error) {
	if limit <= 0 {
		limit = 200
	}

	return s.hist.Tail(limit)
}

func (s *Service) BrowseExe() (string, error) {
	if s.app == nil {
		return "", errors.New("app not ready")
	}

	dlg := s.app.Dialog.OpenFile().SetTitle("Select executable")
	if runtime.GOOS == "windows" {
		dlg = dlg.AddFilter("Programs", "*.exe")
	}

	return dlg.AddFilter("All files", "*.*").PromptForSingleSelection()
}

func (s *Service) BrowseShortcut() (string, error) {
	if s.app == nil {
		return "", errors.New("app not ready")
	}

	return s.app.Dialog.OpenFile().
		SetTitle("Select shortcut").
		AddFilter("Shortcuts", "*.lnk").
		AddFilter("All files", "*.*").
		PromptForSingleSelection()
}

func (s *Service) ParseShortcut(path string) (host.Shortcut, error) {
	return host.ParseShortcut(path)
}

func (s *Service) ShowWindow() {
	s.show("")
}

func (s *Service) ShowApp(id string) {
	s.show(id)
}

func (s *Service) HostArch() string {
	return runtime.GOARCH
}

func (s *Service) ProtocolURL(id string) string {
	return protocol + "://app/" + id
}

//wails:ignore
func (s *Service) HandleProtocol(raw string) {
	id := parseAppID(raw)
	s.show(id)
}

func (s *Service) onSlot(app config.App, slot schedule.Slot) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	switch slot.Action {
	case schedule.ActionNotify:
		res, err := s.eng.Check(ctx, app)
		if err != nil {
			_ = s.hist.Append(history.Event{AppID: app.ID, AppName: app.Name, Action: history.ActionError, Error: err.Error()})
			s.emitHistory()

			return
		}

		if !res.Newer {
			return
		}

		_ = s.hist.Append(history.Event{
			AppID: app.ID, AppName: app.Name, Action: history.ActionNotify,
			From: res.Local, To: res.Remote, Asset: res.AssetName,
		})
		s.emitHistory()
		_ = toast.Show(toast.Note{
			Title:   "Update available",
			Message: app.Name + " " + res.Remote,
			URL:     protocol + "://app/" + app.ID,
		})
	default:
		_, _ = s.UpgradeNow(app.ID)
	}
}

func (s *Service) requireOpen() error {
	if s.store.EncryptedOnDisk() && !s.store.Unlocked() {
		return errors.New("config is locked")
	}

	return nil
}

func (s *Service) show(appID string) {
	if s.win != nil {
		s.win.Show()
		s.win.Focus()
	}

	if s.app != nil && appID != "" {
		s.app.Event.Emit("open-app", appID)
	}
}

func (s *Service) hide() {
	if s.win != nil {
		s.win.Hide()
	}
}

func (s *Service) emitHistory() {
	if s.app != nil {
		s.app.Event.Emit("history-updated", "")
	}
}

//wails:ignore
func (s *Service) CheckAll() {
	set := s.store.Settings()
	for _, a := range set.Apps {
		if a.Enabled {
			_, _ = s.CheckNow(a.ID)
		}
	}
}

func parseAppID(raw string) string {
	raw = strings.TrimSpace(raw)
	if i := strings.Index(strings.ToLower(raw), "://app/"); i >= 0 {
		return strings.Trim(raw[i+7:], "/")
	}

	return ""
}

func resField(res *upgrade.Result, local bool) string {
	if res == nil {
		return ""
	}

	if local {
		return res.Local
	}

	return res.Remote
}

func assetName(res *upgrade.Result) string {
	if res == nil {
		return ""
	}

	return res.AssetName
}
