# SSHKing

SSHKing is a cross-platform SSH workspace for Windows and macOS. The backend is
written in Go and the optimized glass interface uses Wails with the operating
system webview.

## Current capabilities

- Searchable, persistent server library
- Pure-Go SSH transport (no local `ssh` child process)
- Configurable remote shell (`default`, `zsh`, `bash`, or `fish`)
- SSH agent and private-key authentication
- Bounded terminal scrollback
- Optional activity and command logs streamed to disk
- Frameless window with integrated minimize, maximize, and close controls

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
