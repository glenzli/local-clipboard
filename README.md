# Local Clipboard

Local Clipboard is a small LAN-only shuttle for text, images, and files. One
always-on machine—typically a NAS—runs the hub; every desktop runs a local
client. There is no cloud service and no account system.

```text
device A ── WebSocket ──▶ NAS hub ◀── WebSocket ── device B
                              │
                         file relay
                         24-hour TTL
```

- `Cmd+Option+C` on macOS copies the current selection and sends the fresh
  clipboard value.
- Text and images land directly in the receiving device's clipboard.
- Files, including a multi-selection in Finder, arrive as pending offers. The
  receiver gets a notification and accepts them from the local control panel.
- Each client exposes its control panel only on
  <http://127.0.0.1:9101>.

## Interaction model

Local Clipboard does **not** watch or synchronize every clipboard change.
Normal `Cmd+C` / `Ctrl+C` remains local.

| Action | Behavior |
|---|---|
| Built-in macOS/Windows hotkey | Copy the frontmost selection, wait for a new clipboard value, then send it |
| Linux desktop shortcut | Send the clipboard as it already exists; copy first |
| Tray menu or `send-clipboard` | Send the clipboard as it already exists; it never triggers Copy |
| Incoming text or image | Replace the local clipboard |
| Incoming file | Create a pending offer; download only after acceptance |

Clipboard detection order is **files → text → image**. Text precedes image
because rich-text applications often attach a rendered TIFF preview to copied
text.

## Requirements

- Go 1.27 or newer to build from source
- A trusted LAN or private VPN between devices
- A Linux amd64 hub for the supplied Dockerfile
- Apple Silicon macOS for the supplied macOS artifact and installer
- Windows PowerShell for Windows image clipboard support
- `xclip` on Linux X11, or `wl-clipboard` on Wayland

## Set up the NAS hub

The Docker image is `FROM scratch`; it packages a prebuilt Linux binary and
does not build Go source inside Docker.

```bash
git clone https://github.com/glenzli/local-clipboard.git
cd local-clipboard
cp .env.example .env
```

Set a random shared token in `.env`:

```dotenv
LOCALCLIP_TOKEN=replace-with-output-from-openssl-rand-hex-16
```

Build the hub binary, then start the container:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags "-s -w" -o local-clipboard-linux-amd64 .
docker compose up -d --build
```

The hub listens on TCP port `9100`. Its persistent instance ID and relayed
files live in the Compose-mounted `./data` directory.

## Install a client

### macOS

From a repository checkout on an Apple Silicon Mac:

```bash
LOCALCLIP_SERVER=nas-ip:9100 \
LOCALCLIP_TOKEN='<shared-token>' \
./deploy/macos/install.sh
```

The installer creates `~/Applications/LocalClipboard.app`, registers a user
LaunchAgent, and starts the client. On first use, grant the app both:

1. System Settings → Privacy & Security → Accessibility
2. System Settings → Privacy & Security → Input Monitoring

The hotkey registration retries every 15 seconds, so the app does not need to
be restarted after granting permission. A stable local code-signing identity
keeps these permissions across rebuilds; an ad-hoc signature may require them
again after installation.

Logs: `tail -f /tmp/local-clipboard.log`

### Linux

Place `local-clipboard-linux-amd64` beside `deploy/linux/install.sh`, then run:

```bash
./deploy/linux/install.sh nas-ip:9100 '<shared-token>'
```

The installer creates a systemd user service. Logs are available with:

```bash
journalctl --user -u local-clipboard -f
```

### Windows

Place `local-clipboard-windows-amd64.exe` beside
`deploy/windows/install.ps1`, then run PowerShell:

```powershell
$env:LOCALCLIP_SERVER = "nas-ip:9100"
$env:LOCALCLIP_TOKEN = "<shared-token>"
.\deploy\windows\install.ps1
```

The installer registers an on-logon scheduled task and starts the client.

## Automated rollout

`make rollout` is intended to run from an Apple Silicon Mac after the NAS
repository and `.env` have been bootstrapped.

```bash
cp deploy.env.example deploy.env
# Fill NAS_HOST, NAS_DIR, LOCALCLIP_SERVER, and LOCALCLIP_TOKEN.
make rollout
```

It builds all maintained platform artifacts, copies the Linux hub binary to
the NAS over SSH, rebuilds the container, and reinstalls the local macOS
client.

Other targets:

| Target | Purpose |
|---|---|
| `make build` | Build the current host into `dist/` |
| `make dist-all` | Build Linux amd64, macOS arm64, and Windows amd64 |
| `make deploy` | Upgrade only the NAS hub |
| `make install` | Reinstall only the local client |

## Hotkeys

| Platform | Default | Behavior |
|---|---|---|
| macOS | `Cmd+Option+C` | Copy selection, wait up to 1.5 seconds for clipboard change, then send |
| Windows | `Ctrl+Alt+C` | Copy selection, wait for clipboard change, then send |
| Linux static artifact | No built-in hotkey | Bind a desktop shortcut that runs `send-clipboard` after copying |

Change the built-in binding with, for example,
`run -server nas:9100 -hotkey ctrl+shift+d`; disable it with `-hotkey ""`.
An X11 build compiled locally with CGo can register `Ctrl+Alt+C`, but the
supplied static Linux artifact deliberately omits the in-process hotkey.

OS-native shortcuts can invoke `send-clipboard`, but that command only sends
the existing clipboard. It is not equivalent to the macOS/Windows built-in
copy-and-send gesture; copy first when using Shortcuts, Raycast, Alfred, skhd,
AutoHotkey, or a Linux desktop shortcut.

Example command for an installed macOS app:

```bash
$HOME/Applications/LocalClipboard.app/Contents/MacOS/local-clipboard send-clipboard
```

Example bindings are provided in `deploy/macos/skhdrc.example` and
`deploy/windows/hotkey.ahk.example`.

## Local control panel

Open <http://127.0.0.1:9101> on each client to inspect connection state,
pending files, progress, and recent transfer events. State-changing endpoints
accept only `POST` and reject browser requests from non-loopback origins.

## CLI

```text
run -server nas:9100 [-token s] [-name dev]
    Run the receiving daemon, tray, local UI, and built-in hotkey.

send-clipboard [-to device]
    Ask the local daemon to send the existing clipboard.

send-clipboard -server nas:9100 [-token s] [-to device]
    Send the existing clipboard directly when no daemon is available.

send-text -server nas:9100 [-token s] [-to device] "hello"
    Send explicit text directly.

send-file [-to device] file
    Send one file through the local daemon.

send-file -server nas:9100 [-token s] [-to device] file...
    Upload one or more files directly.

detect
    Show how the current clipboard resolves without sending it.

accept -server nas:9100 [-token s] [-out directory] <file-id>
    Download a file offer directly.

serve -addr :9100 [-token s]
    Run the hub without Docker. LOCALCLIP_TOKEN is used when -token is absent.
```

An empty `-to` broadcasts to every other online device. A non-empty `-to`
must match the receiver's device name.

## Security and data lifecycle

- The shared token authorizes hub HTTP and WebSocket requests. Clients send it
  in the `X-Token` header rather than in URLs.
- The first successful connection pins the hub instance ID. Later identity
  changes are rejected until the local known-hosts record is deliberately
  removed. This is TOFU, not a certificate authority.
- Hub traffic uses plain HTTP and WebSocket. It is **not encrypted**; do not
  expose port `9100` to the public internet. Use a trusted LAN or private VPN.
- Clients deliberately ignore system proxy environment variables and connect
  directly to the configured hub.
- Relayed files are deleted from the hub after 24 hours.
- Hub message history is memory-only and bounded to 200 entries / 64 MiB.
- The local control panel binds only to `127.0.0.1`.

## Troubleshooting

### The hotkey is active but nothing happens on macOS

Confirm that `LocalClipboard.app` is enabled under both Accessibility and
Input Monitoring. Then inspect `/tmp/local-clipboard.log`.

### The old clipboard value is sent

Use the built-in macOS/Windows hotkey for copy-and-send. Tray actions,
`send-clipboard`, and external shortcut examples intentionally send the
clipboard as it already exists.

### A rebuilt hub is rejected

The hub instance ID changed. If the rebuild was intentional, remove that
server's entry from the known-hosts file reported in the error, then reconnect
and verify the new first-use pin.

### Files do not appear immediately on the receiver

Files are offers, not automatic downloads. The notification is informational;
open the local control panel to accept them. Accepted files are saved under
`~/Downloads/local-clipboard` by default.

### Linux clipboard access fails

Install `xclip` for X11 or `wl-clipboard` for Wayland, and make sure the systemd
user service inherits the correct display/session environment.

## Limits

- Inline text is limited to 1 MiB; larger text should be saved and sent as a
  file.
- WebSocket messages are limited to 32 MiB, including inline image encoding.
- Individual relayed files are limited to 1 GiB.
- The supplied Docker and Linux client artifacts target amd64.
- The supplied macOS installer targets Apple Silicon.
- iOS and iPadOS are not supported because background clipboard access is not
  available; a future mobile client would need a pull-style workflow.
