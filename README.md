<img src="assets/icons/logo.png" width="96" alt="sing-box-tray icon">

# sing-box-tray

A minimal Windows system tray launcher for [sing-box](https://sing-box.sagernet.org/).

## Features

- **Three proxy modes** switchable from the tray menu
  - **Off** — sing-box runs without touching system settings
  - **System Proxy** — sets Windows HTTP proxy from the sing-box config inbound
  - **TUN** — injects a TUN inbound into a temp config, requires elevation
- Tray icon reflects state: grey = stopped, green = running, red = crashed
- **Config** tray submenu — switch which sing-box config file is active, picked from a folder that can hold several (also duplicated in Settings); the submenu refreshes live if files are added or removed from the folder
- Settings window to configure paths (sing-box, wintun.dll), the config folder/active config, autostart, update options, and language
- Log viewer with live updates
- Crash detection with desktop notification
- File watcher — prompts to restart when config files change
- Autostart via the registry `Run` key (Task Scheduler with `/RL HIGHEST` only for TUN mode, which needs silent elevation), toggleable from the tray menu (checkbox reflects actual state) or Settings
- Single instance enforced via named kernel mutex
- **Updates** tray submenu — auto-updates both the tray launcher itself and the `sing-box` binary from GitHub Releases, each with its own auto-update toggle (on by default); sing-box also has a stable/pre-release channel toggle
- First-run check — on startup, repoints `sing_box_path`/`wintun_dll_path`/the active config at a same-named file in the exe's folder if the configured path no longer exists (e.g. after moving the install); if still missing, offers to download `sing-box.exe`/`wintun.dll`, or warns if no config file can be found
- **Languages** tray submenu — switch the UI language live, no restart, in addition to auto-detecting it from the Windows locale
- UI in English, Russian, or Ukrainian
- **About** tray menu item — shows the tray launcher and sing-box versions plus a clickable link to the project repository

## Build requirements

Only the Go toolchain is needed, on either host platform — no C compiler, no Windows SDK. Nothing in this project uses cgo (`CGO_ENABLED=0` in the Makefile/scripts is mandatory, not just a preference: all Windows API calls go through `syscall`/`golang.org/x/sys/windows`, never cgo), so there's nothing to cross-compile besides Go itself.

- **Go** — matching or newer than the version pinned in `go.mod` (currently `1.25.9`). Since Go 1.21, the `go` directive in `go.mod` is an enforced minimum: an older toolchain on `PATH` will auto-fetch the pinned one via `GOTOOLCHAIN=auto` if network access is available.
- **On Linux/macOS/WSL**: `./scripts/build.sh` cross-compiles to `build/sing_box_tray_runner.exe`.
- **On native Windows**: `scripts\build.ps1` (PowerShell) builds the same output, no `make` required.
- **`make`** is optional — only needed if you use `make build` instead of the scripts above.
- Regenerating `rsrc.syso` (only if `app.manifest` changes) needs `github.com/akavel/rsrc` — a pure-Go tool, installable and runnable on Linux too (see [Building](#building)).

## Runtime requirements

- Windows 10/11 x64
- [sing-box](https://github.com/SagerNet/sing-box) binary — auto-downloadable on first run if missing
- [wintun.dll](https://www.wintun.net/) — required only for TUN mode; auto-downloadable on first run if missing, or manually from the same link if the automatic download fails
- Internet access to `api.github.com`/`github.com` — required for the **Updates** feature and the first-run sing-box download; `www.wintun.net` is required for the first-run wintun.dll download. Everything else works fully offline

## Installation

1. Place `sing_box_tray_runner.exe` anywhere on disk.
2. On first launch, `tray-config.json` is created next to the executable with default values.
3. Right-click the tray icon → **Settings...** and set the paths.

## Configuration

`tray-config.json` (created automatically on first run):

```json
{
  "sing_box_path": "sing-box.exe",
  "wintun_dll_path": "wintun.dll",
  "config_dir": ".",
  "selected_config": "config.json",
  "system_proxy_inbound": "",
  "autostart": false,
  "default_mode": "system_proxy",
  "start_on_launch": false,
  "log_lines": 200,
  "language": "auto",
  "system_proxy": {
    "tag": "mixed-in",
    "listen": "127.0.0.1",
    "listen_port": 2080
  },
  "update": {
    "channel": "alpha",
    "auto_update": true
  },
  "launcher_update": {
    "auto_update": true
  },
  "tun": {
    "interface_name": "singbox-tun",
    "address": ["172.19.0.1/30"],
    "mtu": 9000
  }
}
```

| Field | Description |
|---|---|
| `sing_box_path` | Path to `sing-box.exe`. Relative paths are resolved from the tray exe directory. Rewritten automatically after an auto-update. |
| `wintun_dll_path` | Path to `wintun.dll`. Copied next to `sing-box.exe` on TUN start if not already present. |
| `config_dir` | Folder scanned (non-recursively) for `*.json` sing-box configs; the tray's **Config** submenu and the Settings config dropdown both list what's found here. |
| `selected_config` | File name (inside `config_dir`) of the currently active sing-box config. This file is never modified. |
| `system_proxy_inbound` | Tag of the `http` or `mixed` inbound to read the proxy address from. Leave empty to use the first one found. |
| `autostart` | Kept in sync with the "Autostart" checkbox in the tray menu and in Settings (whether the registry `Run` value or Task Scheduler task exists); not meant to be hand-edited. |
| `default_mode` | Starting mode: `off`, `system_proxy`, or `tun`. |
| `start_on_launch` | If `true`, sing-box starts automatically when the tray app launches. |
| `log_lines` | Size of the in-memory log buffer shown in the log viewer. |
| `language` | UI language: `auto` (detect from Windows), `en`, `ru`, or `ua`. |
| `system_proxy.*` | Default `mixed` inbound injected when running in System Proxy mode and the base `config.json` has none. |
| `update.channel` | `stable` or `alpha` — which sing-box releases the updater considers. |
| `update.auto_update` | If `true`, sing-box updates install (and restart sing-box if running) automatically, no prompt. |
| `launcher_update.auto_update` | If `true`, tray launcher updates install and relaunch the app automatically, no prompt. |
| `tun.*` | TUN interface settings injected at runtime. The base `config.json` does not need a TUN section. |

### Config

`config_dir` can hold several sing-box config files side by side. The tray's **Config** submenu lists every `*.json` file found there and lets you pick which one is active; picking a different one restarts sing-box live if it's running. The same list is duplicated as a dropdown in Settings. The folder is polled every 2 seconds while the tray is running, so adding or removing files in `config_dir` refreshes the submenu automatically without needing to restart the tray.

### System Proxy mode

The tray reads `listen` and `listen_port` from the sing-box config's `http` or `mixed` inbound and sets the Windows system proxy in the registry. `system_proxy_inbound` is the inbound tag to use if there are multiple.

### TUN mode

The tray injects a `tun` inbound and a `route.auto_detect_interface: true` setting into a temporary copy of the config, then starts sing-box with that copy. The original config is untouched. Elevation is requested automatically via UAC.

The tray also prepends a route rule that sends sing-box's own process traffic via `direct`, preventing it from being re-captured by the TUN interface.

### Updates

The tray menu has an **Updates** submenu with two independent sections, both fetching releases straight from GitHub — no separate updater app:

- **sing-box-tray** (the tray launcher itself) — **Check for Updates** downloads the latest release of this project, swaps it into place, and relaunches. **Auto-update** (checkbox, also in Settings) skips the confirmation prompt on startup and does this silently — if sing-box happens to be running, it's stopped first (otherwise it would be orphaned once the old tray process exits) and not restarted automatically after the relaunch.
- **sing-box** — same idea as before: **Check for Updates**, plus **Auto-update** and **Use pre-release versions** checkboxes (also duplicated in Settings). Updates install into a tray-managed `sing-box/<version>/` folder next to the executable; `sing_box_path` is switched to point at the new version automatically, and older versions are removed. With auto-update on, a running sing-box is restarted automatically after an update; with it off, the tray only toasts that an update is available and installs on the next manual "Check for Updates" click.

Both sections check once on startup, then again every 6 hours for as long as the tray keeps running (toast-only unless auto-update is on), and on demand via their "Check for Updates" item.

### Localization

The UI (tray menu, dialogs, notifications, Settings/Log windows) is available in English, Russian, and Ukrainian. `language: "auto"` detects the language from the Windows UI locale. Switch it live — no restart — via the tray's **Languages** submenu or the language dropdown in Settings; both write the choice back to `tray-config.json`. Log output stays in English regardless of UI language.

## Building

```sh
make build          # any host with `make`
./scripts/build.sh   # Linux/macOS/WSL, no `make` required
scripts\build.ps1    # native Windows (PowerShell), no `make` required
```

The output is `build/sing_box_tray_runner.exe`. Set a `VERSION` env var (e.g. `VERSION=v1.2.3 ./scripts/build.sh`) to embed a version string the tray launcher's self-updater can compare against — this is how `release.yml` builds tagged releases; local dev builds leave it unset (`"dev"`).

The `rsrc.syso` file in the repo root embeds a Windows manifest (Common Controls v6) and the app icon (`assets/icons/working.ico`). It is linked automatically by the Go toolchain and enables proper visual styling for the Settings and Log windows. Regenerate it if the manifest or icon changes:

```sh
go install github.com/akavel/rsrc@latest
rsrc -manifest app.manifest -ico assets/icons/working.ico -o rsrc.syso
```

## CI/CD

- **CI** (`.github/workflows/ci.yml`) — on every push/PR: `golangci-lint` (with `GOOS=windows`) plus a build matrix on `ubuntu-latest` and `windows-latest` to verify both `scripts/build.sh` and `scripts/build.ps1` work.
- **Release** (`.github/workflows/release.yml`) — pushing a `v*` tag (e.g. `git tag v1.0.0 && git push --tags`) builds the exe (with the tag embedded as its version, so the launcher's own self-updater can detect it) and publishes it as a GitHub Release with auto-generated notes.
- **Dependabot** (`.github/dependabot.yml`) — weekly PRs for Go module and GitHub Actions updates.

## License

MIT — see [LICENSE](LICENSE).
