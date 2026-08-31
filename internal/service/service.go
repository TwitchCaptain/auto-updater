// Package service is the Wails backend: config, GitHub checks, upgrades, and history.
package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/TwitchCaptain/auto-updater/internal/config"
	"github.com/TwitchCaptain/auto-updater/internal/extract"
	"github.com/TwitchCaptain/auto-updater/internal/githubsrc"
	"github.com/TwitchCaptain/auto-updater/internal/history"
	"github.com/TwitchCaptain/auto-updater/internal/host"
	"github.com/TwitchCaptain/auto-updater/internal/locver"
	"github.com/TwitchCaptain/auto-updater/internal/paths"
	"github.com/TwitchCaptain/auto-updater/internal/peversion"
	"github.com/TwitchCaptain/auto-updater/internal/preset"
	"github.com/TwitchCaptain/auto-updater/internal/protocol"
	"github.com/TwitchCaptain/auto-updater/internal/schedule"
	"github.com/TwitchCaptain/auto-updater/internal/scheduler"
	"github.com/TwitchCaptain/auto-updater/internal/toast"
	"github.com/TwitchCaptain/auto-updater/internal/upgrade"
)

type Service struct {
	app     *application.App
	win     application.Window
	store   *config.Store
	hist    *history.Log
	eng     *upgrade.Engine
	sched   *scheduler.Runner
	pending muString
}

type muString struct {
	mu sync.Mutex
	s  string
}

func (m *muString) set(s string) {
	m.mu.Lock()
	m.s = s
	m.mu.Unlock()
}

func (m *muString) take() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	s := m.s
	m.s = ""

	return s
}

func New(store *config.Store, hist *history.Log) *Service {
	s := &Service{
		store: store,
		hist:  hist,
		eng:   &upgrade.Engine{GH: &githubsrc.Client{}},
	}
	s.sched = scheduler.New(store, s.onSlot)
	s.sched.Handled = s.slotHandled

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
	if s.app != nil {
		s.app.Event.Emit("config-locked", "")
	}
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

	set := s.store.Settings()
	// Installer "Start with Windows" writes a Startup .lnk without touching config.json.
	// Reflect the real shortcut so Save settings does not delete it.
	set.StartWithWindows = host.HasStartup()

	return set, nil
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

func (s *Service) SaveApp(app config.App) (config.App, error) {
	if err := s.requireOpen(); err != nil {
		return config.App{}, err
	}

	app = resolveAppPaths(app)

	return s.store.PatchApp(app)
}

func (s *Service) DeleteApp(id string) error {
	if err := s.requireOpen(); err != nil {
		return err
	}

	return s.store.DeleteApp(id)
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

// UpcomingItem is the next fire of one enabled app schedule.
type UpcomingItem struct {
	Time    time.Time `json:"time"`
	AppID   string    `json:"appId"`
	AppName string    `json:"appName"`
	Action  string    `json:"action"`
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

	_ = s.store.SetLastVersion(app.ID, githubsrc.Display(res.Remote))
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

	events, err := s.hist.Tail(limit)
	if events == nil {
		events = []history.Event{}
	}

	return events, err
}

func (s *Service) Upcoming() ([]UpcomingItem, error) {
	if err := s.requireOpen(); err != nil {
		return nil, err
	}

	set := s.store.Settings()
	now := time.Now()
	items := make([]UpcomingItem, 0)

	for _, app := range set.Apps {
		if !app.Enabled {
			continue
		}

		for _, p := range schedule.NextEach(app.Schedules, now) {
			items = append(items, UpcomingItem{
				Time:    p.Time,
				AppID:   app.ID,
				AppName: app.Name,
				Action:  p.Action,
			})
		}
	}

	slices.SortFunc(items, func(a, b UpcomingItem) int {
		if c := a.Time.Compare(b.Time); c != 0 {
			return c
		}

		return strings.Compare(a.AppName, b.AppName)
	})

	return items, nil
}

func (s *Service) BrowseExe() (string, error) {
	if s.app == nil {
		return "", errors.New("app not ready")
	}

	dlg := s.app.Dialog.OpenFile().SetTitle("Select executable")
	if runtime.GOOS == "windows" {
		dlg = dlg.AddFilter("Programs", "*.exe;*.lnk")
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
	return host.ParseShortcut(paths.CleanUserPath(path))
}

// ConsumePendingOpen returns (and clears) an app id from a protocol launch
// (toast click or second instance). The frontend calls this after it is mounted
// because the first-launch event is emitted before WebView2 is ready.
func (s *Service) ConsumePendingOpen() string {
	return s.pending.take()
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
	return "captainupdater://app/" + id
}

//wails:ignore
func (s *Service) HandleProtocol(raw string) {
	id := protocol.ParseAppID(raw)
	if id != "" {
		s.pending.set(id)
	}

	s.show(id)
}

func (s *Service) slotHandled(app config.App, slot schedule.Slot, dueAt time.Time) bool {
	events, err := s.hist.Tail(0)
	if err != nil {
		return false
	}

	want := history.ActionUpgrade
	if slot.Action == schedule.ActionNotify {
		want = history.ActionNotify
	}

	for _, ev := range events {
		if ev.AppID != app.ID || ev.Action != want {
			continue
		}

		if !ev.Time.Before(dueAt) {
			return true
		}
	}

	return false
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
			_ = s.hist.Append(history.Event{
				AppID: app.ID, AppName: app.Name, Action: history.ActionNotify,
				From: res.Local, To: res.Remote, Asset: res.AssetName, Result: "current",
			})
			s.emitHistory()

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
			URL:     "captainupdater://app/" + app.ID,
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
	if appID != "" {
		s.pending.set(appID)
	}

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

func resolveAppPaths(app config.App) config.App {
	app.ExePath = paths.CleanUserPath(app.ExePath)
	app.ShortcutPath = paths.CleanUserPath(app.ShortcutPath)

	if paths.IsShortcut(app.ExePath) {
		sc, err := host.ParseShortcut(app.ExePath)
		if err == nil && sc.Target != "" {
			if app.ShortcutPath == "" {
				app.ShortcutPath = app.ExePath
			}

			app.ExePath = paths.CleanUserPath(sc.Target)
		}
	}

	if app.ExePath == "" && paths.IsShortcut(app.ShortcutPath) {
		sc, err := host.ParseShortcut(app.ShortcutPath)
		if err == nil && sc.Target != "" {
			app.ExePath = paths.CleanUserPath(sc.Target)
		}
	}

	return app
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
