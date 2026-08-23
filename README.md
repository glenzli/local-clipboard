# Local Clipboard

LAN-only clipboard & file shuttle for your own devices. No cloud, no accounts.

```
device A ──ws──▶ NAS hub ◀──ws── device B
                    │
                 file relay (24h TTL)
```

- Text & screenshots: push with a hotkey, lands in the receiver's clipboard automatically
- Files (even multi-select in Finder): same hotkey auto-detects, receiver confirms via notification or the local UI
- Local Web UI per machine: <http://127.0.0.1:9101> — pending files, live progress, transfer log

## Security model

| Layer | Mechanism |
|---|---|
| Who are you | shared token (`serve -token`, clients pass it too) |
| Is that really my hub | TOFU fingerprint — hub instance id pinned on first connect; mismatch refuses to send anything (defends against IP reuse / DNS hijack) |
| Network | connections forced direct; system proxy env (Clash etc.) deliberately ignored |

## Quick start

### 1. Hub on your always-on box (e.g. NAS with Docker)

```bash
git clone https://github.com/glenzli/local-clipboard.git
cd local-clipboard
cp .env.example .env          # put LOCALCLIP_TOKEN=<openssl rand -hex 16>
docker compose up -d --build
```

Note: Dockerfile is `FROM scratch`, so no base image pull is needed at build time — only the prebuilt linux binary must exist next to it (see below).

### 2. Configure once

```bash
cp deploy.env.example deploy.env   # then fill: SSH target/path + hub address/token
```

### 3. One-command rollout

```bash
make rollout    # builds all platforms -> upgrades NAS container -> installs local daemon (OS-aware)
```

Other targets: `make build` / `dist-all` / `deploy` (hub only) / `install` (local daemon only).

## Manual install on another computer

Grab the matching binary from `dist/` (or cross-build yourself):

```powershell
# Windows: copy exe + deploy/windows/ together
$env:LOCALCLIP_SERVER="nas-ip:9100"; $env:LOCALCLIP_TOKEN="<token>"; .\install.ps1
```

```bash
# Linux (needs xclip or wl-clipboard): copy binary + deploy/linux/ together
./install.sh nas-ip:9100            # token prompted if env unset
```

## CLI reference

```
run -server nas:9100 [-name dev]      daemon: receive -> clipboard / inbox notice (+ UI on :9101)
send-clipboard                        push current clipboard; auto-detects files > image > text
send-text "..."                       push raw text
send-file f1 [f2 ...]                 upload + offer file(s)
accept <file-id>                      download an offered file
serve -addr :9100 -token s            run the hub
```

All senders accept `-to <device>` to target one online device instead of broadcasting.
Without `-server` they route through the local daemon (progress shows in the UI); with `-server` they go direct.

## Hotkeys (bind send-clipboard to a combo that's free on your OS)

Recommended combo everywhere: `Ctrl+Alt+C` (macOS maps Option to Alt, same muscle memory).

**macOS — skhd**

```bash
brew install koekeishiya/formulae/skhd
echo "ctrl + alt - c : $HOME/.local/bin/local-clipboard send-clipboard" >> ~/.config/skhd/skhdrc
skhd --start-service
# grant Accessibility permission when prompted
```

Or use Raycast/Alfred: hotkey -> run command `/Users/you/.local/bin/local-clipboard send-clipboard`.
Or Shortcuts.app: new shortcut -> Run Shell Script -> assign hotkey (slower to fire).

**Windows — AutoHotkey v2**

```autohotkey
; %LOCALAPPDATA%\local-clipboard\hotkey.ahk
^!c::Run A_AppData "\local-clipboard\local-clipboard.exe send-clipboard"
```

**Linux — desktop environment custom shortcut**

Command: `$HOME/.local/bin/local-clipboard send-clipboard`
(GNOME: Settings > Keyboard > Custom Shortcuts; KDE: System Settings > Shortcuts)

## Platform notes

- macOS: nothing extra needed (uses pbcopy/pbpaste/osascript)
- Windows: PowerShell (STA) used for image clipboard; schtasks handles autostart
- Linux X11: install `xclip`; Wayland: install `wl-clipboard` (autostart via systemd user unit)

## Known limits

- Inline text sync caps at ~1MB (bigger: save as file)
- Image inline cap 32MB message size; giant screenshots fall back fine but stay inline
- iOS/iPadOS not covered yet (OS forbids background clipboard watch; would need a pull-style app)
