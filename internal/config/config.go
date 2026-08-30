package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/TwitchCaptain/auto-updater/internal/crypt"
	"github.com/TwitchCaptain/auto-updater/internal/schedule"
)

const SourceGitHub = "github"

// Settings is the on-disk config (minus encryption wrapper).
type Settings struct {
	GitHubToken      string `json:"githubToken,omitempty"`
	StartWithWindows bool   `json:"startWithWindows"`
	Apps             []App  `json:"apps"`
}

// App is one GitHub product to keep current.
type App struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Enabled           bool            `json:"enabled"`
	Source            string          `json:"source"`
	OwnerRepo         string          `json:"ownerRepo"`
	ExePath           string          `json:"exePath"`
	ShortcutPath      string          `json:"shortcutPath,omitempty"`
	ExtraFiles        []string        `json:"extraFiles,omitempty"`
	VersionHTTP       string          `json:"versionHttp,omitempty"`
	VersionJSON       string          `json:"versionJson,omitempty"`
	VersionHTTPHeader string          `json:"versionHttpHeader,omitempty"`
	VersionHTTPSecret string          `json:"versionHttpSecret,omitempty"`
	LastVersion       string          `json:"lastVersion,omitempty"`
	Schedules         []schedule.Slot `json:"schedules"`
}

// Store loads and saves config.json, optionally encrypted.
type Store struct {
	path     string
	mu       sync.Mutex
	password string
	enc      bool
	set      Settings
}

func New(path string) *Store {
	return &Store{path: path, set: Settings{Apps: []App{}}}
}

func (s *Store) EncryptedOnDisk() bool { return s.enc }

func (s *Store) Unlocked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.enc {
		return true
	}

	return s.password != ""
}

func (s *Store) Settings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()

	return clone(s.set)
}

func (s *Store) Detect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.enc = false
			s.set = Settings{Apps: []App{}}

			return nil
		}

		return err
	}

	s.enc = crypt.IsEncrypted(raw)
	if s.enc {
		return nil
	}

	return s.decode(raw)
}

func (s *Store) Unlock(password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}

	if !crypt.IsEncrypted(raw) {
		return errors.New("config is not encrypted")
	}

	plain, err := crypt.Open(raw, password)
	if err != nil {
		return err
	}

	if err := s.decode(plain); err != nil {
		return err
	}

	s.password = password
	s.enc = true

	return nil
}

func (s *Store) Lock() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.password = ""
	if s.enc {
		s.set = Settings{Apps: []App{}}
	}
}

func (s *Store) EnableEncryption(password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if password == "" {
		return errors.New("password required")
	}

	s.password = password
	s.enc = true

	return s.flush()
}

func (s *Store) DisableEncryption(password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.enc && s.password != password {
		return crypt.ErrWrongPassword
	}

	s.password = ""
	s.enc = false

	return s.flush()
}

func (s *Store) Replace(in Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validate(in); err != nil {
		return err
	}

	for i := range in.Apps {
		if in.Apps[i].ID == "" {
			in.Apps[i].ID = uuid.NewString()
		}

		if in.Apps[i].Source == "" {
			in.Apps[i].Source = SourceGitHub
		}
	}

	s.set = in

	return s.flush()
}

func (s *Store) PatchApp(app App) (App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	for i, a := range s.set.Apps {
		if a.ID != "" && a.ID == app.ID {
			s.set.Apps[i] = app
			found = true

			break
		}
	}

	if !found {
		if app.ID == "" {
			app.ID = uuid.NewString()
		}

		s.set.Apps = append(s.set.Apps, app)
	}

	if err := validate(s.set); err != nil {
		return App{}, err
	}

	if err := s.flush(); err != nil {
		return App{}, err
	}

	return app, nil
}

func (s *Store) DeleteApp(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id == "" {
		return errors.New("missing app id")
	}

	out := make([]App, 0, len(s.set.Apps))
	for _, a := range s.set.Apps {
		if a.ID != id {
			out = append(out, a)
		}
	}

	if len(out) == len(s.set.Apps) {
		return fmt.Errorf("unknown app %s", id)
	}

	s.set.Apps = out

	return s.flush()
}

func (s *Store) SetLastVersion(id, ver string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, a := range s.set.Apps {
		if a.ID == id {
			s.set.Apps[i].LastVersion = ver

			return s.flush()
		}
	}

	return fmt.Errorf("unknown app %s", id)
}

func (s *Store) App(id string) (App, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, a := range s.set.Apps {
		if a.ID == id {
			return a, true
		}
	}

	return App{}, false
}

func (s *Store) decode(raw []byte) error {
	var set Settings
	if err := json.Unmarshal(raw, &set); err != nil {
		return err
	}

	if set.Apps == nil {
		set.Apps = []App{}
	}

	s.set = set

	return nil
}

func (s *Store) flush() error {
	raw, err := json.MarshalIndent(s.set, "", "  ")
	if err != nil {
		return err
	}

	raw = append(raw, '\n')

	if s.enc {
		if s.password == "" {
			return errors.New("config is locked")
		}

		raw, err = crypt.Seal(raw, s.password)
		if err != nil {
			return err
		}
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}

	return os.Rename(tmp, s.path)
}

func validate(set Settings) error {
	for _, a := range set.Apps {
		if strings.TrimSpace(a.Name) == "" {
			return errors.New("app name required")
		}

		if a.Source != "" && a.Source != SourceGitHub {
			return fmt.Errorf("%s: only github source in v1", a.Name)
		}

		if err := schedule.Validate(a.Schedules); err != nil {
			return fmt.Errorf("%s: %w", a.Name, err)
		}
	}

	return nil
}

func clone(s Settings) Settings {
	apps := make([]App, len(s.Apps))
	for i, a := range s.Apps {
		apps[i] = a
		if a.ExtraFiles != nil {
			apps[i].ExtraFiles = append([]string{}, a.ExtraFiles...)
		}

		if a.Schedules != nil {
			apps[i].Schedules = make([]schedule.Slot, len(a.Schedules))
			for j, sl := range a.Schedules {
				apps[i].Schedules[j] = sl
				if sl.Days != nil {
					apps[i].Schedules[j].Days = append([]time.Weekday{}, sl.Days...)
				}
			}
		}
	}

	s.Apps = apps

	return s
}
