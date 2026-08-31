# Captain Updater

Windows tray app that keeps installed programs current from GitHub releases. Schedule an **Upgrade** or a sticky **Notify** toast. Click the tray icon to open the settings window.

Config and history live under `%APPDATA%\CaptainUpdater` so installing a new version does not wipe your apps list.

## Install

Download the installer for your CPU from [Releases](https://github.com/TwitchCaptain/auto-updater/releases):

- `captain-updater.amd64.installer.exe`
- `captain-updater.arm64.installer.exe`

Portable zips (`captain-updater.<arch>.exe.zip`) are also published. WebView2 is required (the installer can bootstrap it).

## Add an app

Each row is one GitHub repo, one downloaded asset, and up to five weekly schedules.

1. **Add**, or pick a template (autobrr, unpackerr, or Captain Updater itself).
2. Point at the **primary exe** (browse or drag-and-drop). Optional `.lnk` is parsed for Target / Args / Working Directory and used to start the app after an upgrade. Zip members like `unpackerr.amd64.exe` are renamed to that primary name.
3. List **extra files** to copy from the same zip (filenames or globs). autobrr is one app: primary `autobrr.exe`, extra `autobrrctl.exe`.
4. Save, then **Check now** / **Upgrade**, or schedule them.

Version is taken from an optional HTTP JSON field, then the PE ProductVersion of the primary exe (FileVersion's fourth number is a git revision, not the GitHub tag), then `exe -v` / `--version`, then the last version Captain Updater applied. autobrr uses the HTTP API (`/api/config` → `version`); `autobrr.exe` has no `-v` flag (`autobrrctl version` does).

**Notify** shows a sticky toast. Click it to focus this window on that app’s Upgrade button (`captainupdater://app/<id>`). If the tray app was quit over a scheduled minute, the most recent missed Upgrade or Notify runs once at the next start (skipped when history already recorded that occurrence).

Upgrade elevates with UAC only when the target directory is not writable. Self-update stages a new image and restarts.

## Settings

- Optional GitHub token (rate limits / private repos).
- Start with Windows.
- Optional password on `config.json` (Argon2id + AES-256-GCM). The activity log stays plaintext. A forgotten password means resetting config, not the log.

## Develop

Windows is the product target. Linux CI cross-compiles with Go (no CGO).

```text
wails3 generate bindings -ts
wails3 dev
go test ./...
```

Packaged Windows builds include WebView2 DevTools (tray **Inspect**, or **F12**). Runtime errors also show as a red banner in the window.

Release tags `v*` run `wails3 package` for amd64 and arm64, Authenticode-sign the exe and installer, then publish with GoReleaser Pro. The exe File version is `Major.Minor.Patch.REVISION` (`git rev-list --count --all`); Product version is the tag without `v`.
