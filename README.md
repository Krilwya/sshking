# SSHKing

SSHKing is a cross-platform SSH workspace for Windows and macOS. The backend is
written in Go and the optimized glass interface uses Wails with the operating
system webview.

## Current capabilities

- Searchable, persistent server library
- Foldable Personal Servers and Team Servers workspaces
- Local team creation, renaming, and server ownership assignment
- Google-backed cross-device server and terminal-tab synchronization
- Per-device SSH credential readiness indicators without uploading secrets
- Stable cloud tab IDs that reattach to the same remote tmux sessions
- Pure-Go SSH transport (no local `ssh` child process)
- Configurable remote shell (`default`, `zsh`, `bash`, or `fish`)
- SSH agent and private-key authentication
- Bounded terminal scrollback
- Optional activity and command logs streamed to disk
- Native Cocoa window chrome on macOS, including traffic lights and rounded corners
- Integrated Windows controls with DWM shadow and Windows 11 rounded corners
- Platform conventions such as macOS Preferences, Command shortcuts, and native About

## Development

```powershell
wails dev
```

Release build on Windows:

```powershell
wails build -clean
```

Release build on macOS:

```bash
wails build -clean
```

The web UI deliberately has no component framework, external font, animation
library, or image payload. Terminal output is bounded by the configured
scrollback limit and activity logs stream directly to disk.

Configuration is stored in the operating system user configuration directory
under `sshking/config.json`. Activity logs, when enabled, are stored beside it
in `logs/`.

## Cloud accounts and teams

SSHKing includes an optional self-hosted cloud API for Google/Apple sign-in,
cross-device server metadata, team membership, and tmux resume/share handles.
SSH credentials are never uploaded. Deployment files and OAuth callback details
are in [`deploy/cloud`](deploy/cloud/README.md).
