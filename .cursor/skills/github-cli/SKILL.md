---
name: github-cli
description: Authenticate and use GitHub CLI (gh) for TwitchCaptain/auto-updater. Use when cloning, opening PRs, checking CI, or submitting Captain Updater fixes.
---

# GitHub CLI for Captain Updater

`gh` is the GitHub CLI. Prefer it over raw `curl` to `api.github.com`.

## Auth (check first)

```bash
gh auth status
```

Authenticated when any of these is true:

1. **`GH_TOKEN` or `GITHUB_TOKEN`** is set (classic PAT: `repo`, `read:org`, `workflow`; or a fine-grained PAT with read/write on this repo).
2. `gh auth login` already completed (`~/.config/gh/hosts.yml`).

Never print token values. Never commit tokens. Never put a PAT in git remotes in committed files.

After a token is available:

```bash
gh auth setup-git
gh auth status
```

## PRs and CI

```bash
git checkout -b cursor/short-description-51d4
git push -u origin HEAD
gh pr create --title "..." --body "..."
gh pr checks
```

If you cannot push to `TwitchCaptain/auto-updater`, fork and open a PR from the fork:

```bash
gh repo fork TwitchCaptain/auto-updater --remote-name fork --clone=false
git remote add fork "https://github.com/$(gh api user --jq .login)/auto-updater.git"
git push -u fork HEAD
gh pr create --repo TwitchCaptain/auto-updater --head "$(gh api user --jq .login):$(git branch --show-current)"
```

## Tests

Windows is the product target. Linux can unit-test non-GTK packages:

```bash
CGO_ENABLED=0 go test ./internal/...
```

Do not compile `package main` on Linux (Wails GTK).
