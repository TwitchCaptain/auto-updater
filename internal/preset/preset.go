package preset

import "github.com/TwitchCaptain/auto-updater/internal/config"

// Info is a built-in template the user still has to point at a local exe.
type Info struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	OwnerRepo   string   `json:"ownerRepo"`
	PrimaryExe  string   `json:"primaryExe"`
	ExtraFiles  []string `json:"extraFiles"`
	VersionHTTP string   `json:"versionHttp,omitempty"`
	VersionJSON string   `json:"versionJson,omitempty"`
	Notes       string   `json:"notes"`
}

func All() []Info {
	return []Info{
		{
			ID:          "autobrr",
			Name:        "autobrr",
			OwnerRepo:   "autobrr/autobrr",
			PrimaryExe:  "autobrr.exe",
			ExtraFiles:  []string{"autobrrctl.exe"},
			VersionHTTP: "http://127.0.0.1:7474/api/config",
			VersionJSON: "version",
			Notes:       "One app. The zip also contains autobrrctl.exe, copied next to the primary exe.",
		},
		{
			ID:         "unpackerr",
			Name:       "unpackerr",
			OwnerRepo:  "Unpackerr/unpackerr",
			PrimaryExe: "unpackerr.exe",
			Notes:      "GitHub zip names include the CPU architecture (amd64, arm64, …). Version comes from unpackerr -v (no PE FileVersion).",
		},
		{
			ID:         "captain-updater",
			Name:       "Captain Updater",
			OwnerRepo:  "TwitchCaptain/auto-updater",
			PrimaryExe: "captain-updater.exe",
			Notes:      "Self-update. The running image is replaced after a staged copy.",
		},
	}
}

func Apply(id string) (config.App, error) {
	for _, p := range All() {
		if p.ID == id {
			return config.App{
				Name:        p.Name,
				Enabled:     true,
				Source:      config.SourceGitHub,
				OwnerRepo:   p.OwnerRepo,
				ExtraFiles:  append([]string{}, p.ExtraFiles...),
				VersionHTTP: p.VersionHTTP,
				VersionJSON: p.VersionJSON,
			}, nil
		}
	}

	return config.App{}, errUnknown(id)
}

type unknownError string

func (e unknownError) Error() string { return "unknown preset: " + string(e) }

func errUnknown(id string) error { return unknownError(id) }
