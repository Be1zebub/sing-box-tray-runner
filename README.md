<img src="assets/icons/logo.png" width="96" alt="sing-box-tray icon">

# sing-box-tray

A minimal Windows system tray launcher for [sing-box](https://sing-box.sagernet.org/).

Personal fork of `soksanichenko/sing-box-tray-runner` (upstream).
Unlike upstream, this fork **has no updater**: it makes no network requests at all, and
`sing-box.exe`/`wintun.dll` are placed by hand (see [Runtime requirements](#runtime-requirements)).

## Features

- **Three proxy modes** switchable from the tray menu
  - **Off** — sing-box runs without touching system settings
  - **System Proxy** — sets Windows HTTP proxy from the sing-box config inbound
  - **TUN** — injects a TUN inbound into a temp config, requires elevation
- Tray icon reflects state: grey = stopped, green = running, red = crashed
- **Config** tray submenu — switch which sing-box config file is active, picked from a folder that can hold several (also duplicated in Settings); the submenu refreshes live if files are added or removed from the folder
- **Open config file** / **Open config folder** tray items — open the active config in your editor, or the config folder in Explorer
- Settings window to configure paths (sing-box, wintun.dll), the config folder/active config, autostart, and language
- Log viewer with live updates
- Crash detection with desktop notification
- File watcher — prompts to restart when config files change
- Autostart via the registry `Run` key (Task Scheduler with `/RL HIGHEST` only for TUN mode, which needs silent elevation), toggleable from the tray menu (checkbox reflects actual state) or Settings
- Single instance enforced via named kernel mutex
- First-run check — on startup, repoints `sing_box_path`/`wintun_dll_path`/the active config at a same-named file in the exe's folder if the configured path no longer exists (e.g. after moving the install); if a file is still missing, or no config file can be found, it says so and names the exact path the file must be placed at
- **Languages** tray submenu — switch the UI language live, no restart, in addition to auto-detecting it from the Windows locale
- UI in English, Russian, or Ukrainian
- **About** tray menu item — shows the tray launcher version plus a clickable link to the project repository

## Build requirements

Only the Go toolchain is needed, on either host platform — no C compiler, no Windows SDK. Nothing in this project uses cgo (`CGO_ENABLED=0` in the Makefile/scripts is mandatory, not just a preference: all Windows API calls go through `golang.org/x/sys/windows`, never cgo), so there's nothing to cross-compile besides Go itself.

- **Go** — matching or newer than the version pinned in `go.mod`. Since Go 1.21, the `go` directive in `go.mod` is an enforced minimum: an older toolchain on `PATH` will auto-fetch the pinned one via `GOTOOLCHAIN=auto` if network access is available.
- **On Linux/macOS/WSL**: `./scripts/build.sh` cross-compiles to `build/sing_box_tray_runner.exe`.
- **On native Windows**: `scripts\build.ps1` (PowerShell) builds the same output, no `make` required.
- **`make`** is optional — only needed if you use `make build` instead of the scripts above.
- Regenerating `rsrc.syso` (only if `app.manifest` changes) needs `github.com/akavel/rsrc` — a pure-Go tool, installable and runnable on Linux too (see [Building](#building)).

## Runtime requirements

- Windows 10/11 x64
- [sing-box](https://github.com/SagerNet/sing-box) binary — download a release manually and place `sing-box.exe` where `sing_box_path` points
- [wintun.dll](https://www.wintun.net/) — required only for TUN mode; download it manually and place it where `wintun_dll_path` points
- **No internet access is required by the tray itself.** It never contacts anything.

## Installation

1. Place `sing_box_tray_runner.exe` anywhere on disk.
2. On first launch, `tray-config.json` is created next to the executable with default values.
3. Right-click the tray icon → **Settings...** and set the paths.
4. Download `sing-box.exe` (and `wintun.dll` if you want TUN mode) yourself and put them where the paths point.

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
  "default_mode": "tun",
  "start_on_launch": false,
  "log_lines": 200,
  "language": "auto",
  "system_proxy": {
    "tag": "mixed-in",
    "listen": "127.0.0.1",
    "listen_port": 2080
  },
  "tun": {
    "interface_name": "singbox-tun",
    "address": ["172.19.0.1/30", "fdfe:dcba:9876::1/126"],
    "route_address": ["0.0.0.0/1", "128.0.0.0/1"],
    "route_exclude_address": [
      "127.0.0.0/8", "::1/128",
      "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16",
      "fc00::/7", "fe80::/10", "ff00::/8"
    ],
    "mtu": 9000
  }
}
```

| Field | Description |
|---|---|
| `sing_box_path` | Path to `sing-box.exe`. Relative paths are resolved from the tray exe directory. |
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
| `tun.*` | TUN settings injected at runtime, overriding the built-in defaults. The base `config.json` does not need a TUN section. See [TUN mode](#tun-mode). |

### Config

`config_dir` can hold several sing-box config files side by side. The tray's **Config** submenu lists every `*.json` file found there and lets you pick which one is active; picking a different one restarts sing-box live if it's running. The same list is duplicated as a dropdown in Settings. The folder is polled every 2 seconds while the tray is running, so adding or removing files in `config_dir` refreshes the submenu automatically without needing to restart the tray.

**Open config file** and **Open config folder** (tray menu) open the active config in its registered editor, or the folder in Explorer.

### System Proxy mode

The tray reads `listen` and `listen_port` from the sing-box config's `http` or `mixed` inbound and sets the Windows system proxy in the registry. `system_proxy_inbound` is the inbound tag to use if there are multiple.

### TUN mode

The tray injects a `tun` inbound plus route settings into a temporary copy of the config, then starts sing-box with that copy. The original config is untouched. Elevation is requested automatically via UAC.

The injected TUN inbound:

- always carries an **IPv6 address** in addition to the IPv4 one. With `strict_route` and no IPv6 address, sing-tun installs an unconditional IPv6 block filter that blackholes `::1` — which breaks anything resolving `localhost` to IPv6 (Node/Vite, Next, …). `tun.address` defaults to `["172.19.0.1/30", "fdfe:dcba:9876::1/126"]`.
- sets `route_address` to IPv4-only (`0.0.0.0/1`, `128.0.0.0/1`), so `auto_route` does not add an IPv6 default route that direct IPv6 traffic could loop back into.
- sets `route_exclude_address` to loopback, RFC1918 and link-local ranges, so LAN and loopback traffic never enters the tunnel.
- prepends two `route.rules` entries: an `ip_is_private → direct` rule first, then a `process_name: ["sing-box.exe"] → direct` rule that keeps sing-box's own connections from being re-captured by the TUN interface. Both use the explicit `action: "route"` form.

All three lists are overridable via `tun.address` / `tun.route_address` / `tun.route_exclude_address`.

### Localization

The UI (tray menu, dialogs, notifications, Settings/Log windows) is available in English, Russian, and Ukrainian. `language: "auto"` detects the language from the Windows UI locale. Switch it live — no restart — via the tray's **Languages** submenu or the language dropdown in Settings; both write the choice back to `tray-config.json`. Log output stays in English regardless of UI language.

## Building

```sh
make build          # any host with `make`
./scripts/build.sh   # Linux/macOS/WSL, no `make` required
scripts\build.ps1    # native Windows (PowerShell), no `make` required
```

The output is `build/sing_box_tray_runner.exe`. Set a `VERSION` env var (e.g. `VERSION=v1.2.3 ./scripts/build.sh`) to embed a version string — this is how `release.yml` builds tagged releases; local dev builds leave it unset (`"dev"`).

The `rsrc.syso` file in the repo root embeds a Windows manifest (Common Controls v6) and the app icon (`assets/icons/working.ico`). It is linked automatically by the Go toolchain and enables proper visual styling for the Settings and Log windows. Regenerate it if the manifest or icon changes:

```sh
go install github.com/akavel/rsrc@latest
rsrc -manifest app.manifest -ico assets/icons/working.ico -o rsrc.syso
```

## CI/CD

- **CI** (`.github/workflows/ci.yml`) — on every push/PR: `golangci-lint` (with `GOOS=windows`) plus a build matrix on `ubuntu-latest` and `windows-latest` to verify both `scripts/build.sh` and `scripts/build.ps1` work.
- **Release** (`.github/workflows/release.yml`) — pushing a `v*` tag (e.g. `git tag v1.0.0 && git push --tags`) builds the exe with the tag embedded as its version and publishes it as a GitHub Release with auto-generated notes.
- **Dependabot** (`.github/dependabot.yml`) — weekly PRs for Go module and GitHub Actions updates.

## License

MIT — see [LICENSE](LICENSE).
