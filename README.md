# Local Clipboard

[中文](#中文) | [English](#english)

<a id="中文"></a>
## 中文

Local Clipboard 是一个仅在局域网内使用的文本、图片和文件传输小工具。
一台常开的设备（通常是 NAS）作为 Hub，每台桌面设备运行本地客户端。
它不依赖云服务，也没有账号系统。

```text
设备 A ── WebSocket ──▶ NAS Hub ◀── WebSocket ── 设备 B
                            │
                         文件中转
                         保留 24 小时
                            │
                     手机浏览器显式收发
```

- macOS 上按 `Cmd+Option+C` 会先复制当前选择，再发送刚写入剪贴板的内容。
- 文本和图片会直接进入接收设备的剪贴板。
- 文件（包括 Finder 多选文件）会作为待接收项目出现；接收方收到通知后，
  在本地控制台中确认下载。
- 每个客户端的控制台仅监听 <http://127.0.0.1:9101>。
- 手机可以直接打开 NAS Hub 页面，显式发送或接收文本、图片和文件。

### 交互模型

Local Clipboard **不会**监视或同步每一次剪贴板变化。普通的 `Cmd+C` / `Ctrl+C`
仍然只在本机生效。

| 操作 | 行为 |
|---|---|
| macOS/Windows 内置快捷键 | 复制最前方应用中的当前选择，等待剪贴板更新，然后发送 |
| Linux 桌面快捷键 | 发送剪贴板已有内容；需要先自行复制 |
| 托盘菜单或 `send-clipboard` | 发送剪贴板已有内容，不会触发复制 |
| 收到文本或图片 | 替换本机剪贴板 |
| 收到文件 | 创建待接收项目，确认后才下载 |

剪贴板检测顺序是 **文件 → 文本 → 图片**。文本优先于图片，是因为富文本应用
经常会在复制文本时附带一份 TIFF 预览图。

### 运行要求

- 从源码构建需要 Go 1.27 或更新版本
- 设备之间需要可信局域网或私有 VPN
- 当前 Dockerfile 和预构建 Hub 面向 Linux amd64
- 当前 macOS 产物和安装器面向 Apple Silicon
- Windows 图片剪贴板需要 PowerShell
- Linux X11 需要 `xclip`，Wayland 需要 `wl-clipboard`

### 设置 NAS Hub

Docker 镜像基于 `FROM scratch`，只打包预构建的 Linux 二进制，
不会在 Docker 内编译 Go 源码。

```bash
git clone https://github.com/glenzli/local-clipboard.git
cd local-clipboard
cp .env.example .env
```

在 `.env` 中设置随机共享令牌：

```dotenv
LOCALCLIP_TOKEN=replace-with-output-from-openssl-rand-hex-16
```

构建 Hub 二进制并启动容器：

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags "-s -w" -o local-clipboard-linux-amd64 .
docker compose up -d --build
```

Hub 监听 TCP `9100` 端口。持久化实例 ID 和中转文件位于 Compose 挂载的
`./data` 目录中。

### 安装客户端

#### macOS

在 Apple Silicon Mac 的仓库检出目录中运行：

```bash
LOCALCLIP_SERVER=nas-ip:9100 \
LOCALCLIP_TOKEN='<shared-token>' \
./deploy/macos/install.sh
```

安装器会创建 `~/Applications/LocalClipboard.app`、注册用户 LaunchAgent，
并启动客户端。首次使用时，需要为 App 开启两项权限：

1. 系统设置 → 隐私与安全性 → 辅助功能
2. 系统设置 → 隐私与安全性 → 输入监控

全局快捷键每 15 秒重试一次，因此授权后通常不需要重启 App。稳定的本地代码签名
身份可以在重复安装时保留这些权限；ad-hoc 签名可能要求重新授权。

日志：`tail -f /tmp/local-clipboard.log`

#### Linux

将 `local-clipboard-linux-amd64` 放在 `deploy/linux/install.sh` 同一目录，
然后运行：

```bash
./deploy/linux/install.sh nas-ip:9100 '<shared-token>'
```

安装器会创建 systemd 用户服务。查看日志：

```bash
journalctl --user -u local-clipboard -f
```

#### Windows

将 `local-clipboard-windows-amd64.exe` 放在 `deploy/windows/install.ps1`
同一目录，然后运行 PowerShell：

```powershell
$env:LOCALCLIP_SERVER = "nas-ip:9100"
$env:LOCALCLIP_TOKEN = "<shared-token>"
.\deploy\windows\install.ps1
```

安装器会注册当前用户的登录启动项，并启动客户端。
共享令牌会使用当前 Windows 用户的 DPAPI 加密保存在本地，不会出现在计划任务
或客户端命令行中。

### 自动发布

`make rollout` 适合在完成 NAS 仓库和 `.env` 初始化后，从 Apple Silicon Mac 运行。

```bash
cp deploy.env.example deploy.env
# 填写 NAS_HOST、NAS_DIR、LOCALCLIP_SERVER 和 LOCALCLIP_TOKEN。
make rollout
```

它会构建所有维护中的平台产物，通过 SSH 将 Linux Hub 二进制复制到 NAS，
重建容器，并重新安装当前 Mac 上的客户端。

其他目标：

| 目标 | 用途 |
|---|---|
| `make build` | 将当前平台构建到 `dist/` |
| `make dist-all` | 构建 Linux amd64、macOS arm64 和 Windows amd64 |
| `make deploy` | 只升级 NAS Hub |
| `make install` | 只重新安装本机客户端 |

### 快捷键

| 平台 | 默认快捷键 | 行为 |
|---|---|---|
| macOS | `Cmd+Option+C` | 复制当前选择，最多等待 1.5 秒让剪贴板更新，然后发送 |
| Windows | `Ctrl+Alt+C` | 复制当前选择，等待剪贴板更新，然后发送 |
| Linux 静态产物 | 无内置快捷键 | 复制后，通过桌面快捷键调用 `send-clipboard` |

例如，可使用 `run -server nas:9100 -hotkey ctrl+shift+d` 修改内置快捷键；
使用 `-hotkey ""` 禁用。启用 CGo 后在本地构建的 X11 版本可以注册
`Ctrl+Alt+C`，但提供的 Linux 静态产物不会注册进程内快捷键。

系统原生快捷方式可以调用 `send-clipboard`，但该命令只发送已有剪贴板，
并不等同于 macOS/Windows 内置的“复制并发送”。使用 Shortcuts、Raycast、Alfred、
skhd、AutoHotkey 或 Linux 桌面快捷键时，需要先复制。

已安装 macOS App 的命令示例：

```bash
$HOME/Applications/LocalClipboard.app/Contents/MacOS/local-clipboard send-clipboard
```

示例配置位于 `deploy/macos/skhdrc.example` 和
`deploy/windows/hotkey.ahk.example`。

### 本地控制台

打开 <http://127.0.0.1:9101> 可以查看连接状态、待接收文件、传输进度和近期记录。
控制台支持中文和英文：首次打开时跟随系统首选语言，之后可以使用页面右上角的
`中文 / English` 切换，选择会保存在当前浏览器中。

所有修改状态的接口只接受 `POST`，并拒绝来自非回环地址的浏览器请求。

### 手机收发页面

手机与 NAS 位于同一可信网络时，打开 `http://nas-ip:9100/`。首次输入 Hub 的共享令牌并
选择“信任此设备”后，Hub 会为当前浏览器建立可跨 Hub 重启恢复的 HttpOnly 会话；共享
令牌不会写入 URL、cookie 或浏览器存储。只有清除浏览器数据、取消信任、凭证过期或
修改 Hub 共享令牌后，才需要重新输入。

- **发送**：选择在线目标设备，粘贴文本，或选择照片和文件上传。文本会进入桌面端
  剪贴板，文件会成为桌面端的待接收项目。
- **接收**：页面列出发给此手机设备名或广播的近期内容。文本需要点击复制，图片需要
  查看并保存，文件通过 Hub 下载。
- 手机设备名保存在当前浏览器中；桌面端可以将它填入目标设备名进行定向发送，留空
  则按现有规则广播。
- 收件箱复用 Hub 的有界内存历史，Hub 重启后会清空；中转文件仍按 24 小时规则删除。
- 页面包含可安装的 Web App Manifest。Android/桌面 Chrome 可以使用页面安装提示或浏览器
  菜单安装；iPhone 使用分享菜单中的“添加到主屏幕”。从主屏幕图标启动后会以独立窗口
  显示，不再显示普通浏览器地址栏。

页面支持中文和英文。移动浏览器在普通 HTTP 下可能不允许网页直接读写系统剪贴板；
此时页面会保留可选择文本，并提示使用系统的长按粘贴或复制。通过可信 HTTPS 反向代理
访问 Hub 可以获得更完整的剪贴板按钮体验，并让 Chrome 稳定识别和安装 PWA。普通
`http://nas-ip:9100` 下仍可从浏览器菜单尝试添加到主屏幕，但安装能力取决于平台。

### CLI

```text
run -server nas:9100 [-token s] [-name dev]
    运行接收守护进程、托盘、本地控制台和内置快捷键。

send-clipboard [-to device]
    请求本地守护进程发送已有剪贴板。

send-clipboard -server nas:9100 [-token s] [-to device]
    没有本地守护进程时，直接发送已有剪贴板。

send-text -server nas:9100 [-token s] [-to device] "hello"
    直接发送指定文本。

send-file [-to device] file
    通过本地守护进程发送一个文件。

send-file -server nas:9100 [-token s] [-to device] file...
    直接上传一个或多个文件。

detect
    检查当前剪贴板会被识别为什么类型，但不发送。

accept -server nas:9100 [-token s] [-out directory] <file-id>
    直接下载一个文件项目。

serve -addr :9100 [-token s]
    不使用 Docker，直接运行 Hub。未传入 -token 时读取 LOCALCLIP_TOKEN。
```

空的 `-to` 会发送给其他所有在线设备；非空的 `-to` 必须匹配接收端设备名。

### 安全与数据生命周期

- 共享令牌用于授权 Hub 的 HTTP 和 WebSocket 请求。客户端通过 `X-Token` 请求头
  发送令牌，不会把令牌放在 URL 中。
- 手机页面用共享令牌换取随机、HttpOnly、SameSite 会话 cookie；cookie 不包含共享
  令牌。选择“信任此设备”时，Hub 只把凭证的带密钥摘要、设备名和有效期写入
  `mobile_devices.json`，不会持久化原始 cookie 或共享令牌。可信设备最长保留 400 天，
  Hub 重启后仍然有效，最多记录 128 台设备；取消信任或修改共享令牌会使对应凭证失效。
- 第一次成功连接时，客户端会固定 Hub 实例 ID。后续身份变化会被拒绝，直到用户
  主动删除本地 known-hosts 记录。这是 TOFU，不是证书机构。
- Hub 使用明文 HTTP 和 WebSocket，**不提供加密**。不要把 `9100` 端口暴露到公网；
  请使用可信局域网或私有 VPN。
- 客户端会忽略系统代理环境变量，直接连接配置的 Hub。
- 中转文件会在 24 小时后从 Hub 删除。
- Hub 消息历史只保存在内存中，上限为 200 条 / 64 MiB。
- 本地控制台只绑定 `127.0.0.1`。

### 故障排查

#### macOS 快捷键显示有效，但没有反应

确认 `LocalClipboard.app` 已同时开启辅助功能和输入监控权限，
然后检查 `/tmp/local-clipboard.log`。

#### 发送的是旧剪贴板内容

使用 macOS/Windows 内置快捷键执行“复制并发送”。托盘操作、`send-clipboard`
和外部快捷键示例只会发送剪贴板已有内容。

#### 重建 Hub 后客户端拒绝连接

Hub 实例 ID 已变化。如果这是预期重建，请删除错误信息指出的 known-hosts 记录，
重新连接并核对新的首次固定身份。

#### 接收端没有立即出现文件

文件是待接收项目，不会自动下载。系统通知只用于提示；请打开本地控制台确认接收。
默认保存目录是 `~/Downloads/local-clipboard`。

#### Linux 无法读取剪贴板

X11 安装 `xclip`，Wayland 安装 `wl-clipboard`，并确认 systemd 用户服务继承了正确的
显示会话环境。

### 限制

- 内联文本上限为 1 MiB；更大的文本应保存为文件后发送。
- WebSocket 消息上限为 32 MiB，其中包括内联图片编码。
- 单个中转文件上限为 1 GiB。
- 当前 Docker 和 Linux 客户端产物面向 amd64。
- 当前 macOS 安装器面向 Apple Silicon。
- 手机页面采用显式收发，不监视后台剪贴板，也不提供后台自动接收或推送通知。

[返回顶部](#local-clipboard)

---

<a id="english"></a>
## English

Local Clipboard is a small LAN-only shuttle for text, images, and files. One
always-on machine—typically a NAS—runs the hub; every desktop runs a local
client. There is no cloud service and no account system.

```text
device A ── WebSocket ──▶ NAS hub ◀── WebSocket ── device B
                              │
                         file relay
                         24-hour TTL
                              │
                   explicit mobile web transfer
```

- `Cmd+Option+C` on macOS copies the current selection and sends the fresh
  clipboard value.
- Text and images land directly in the receiving device's clipboard.
- Files, including a multi-selection in Finder, arrive as pending offers. The
  receiver gets a notification and accepts them from the local Console.
- Each client exposes its Console only on <http://127.0.0.1:9101>.
- Phones can open the NAS hub page directly for explicit text, image, and file
  sending and receiving.

### Interaction model

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

### Requirements

- Go 1.27 or newer to build from source
- A trusted LAN or private VPN between devices
- A Linux amd64 hub for the supplied Dockerfile
- Apple Silicon macOS for the supplied macOS artifact and installer
- Windows PowerShell for Windows image clipboard support
- `xclip` on Linux X11, or `wl-clipboard` on Wayland

### Set up the NAS hub

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

### Install a client

#### macOS

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

#### Linux

Place `local-clipboard-linux-amd64` beside `deploy/linux/install.sh`, then run:

```bash
./deploy/linux/install.sh nas-ip:9100 '<shared-token>'
```

The installer creates a systemd user service. Logs are available with:

```bash
journalctl --user -u local-clipboard -f
```

#### Windows

Place `local-clipboard-windows-amd64.exe` beside
`deploy/windows/install.ps1`, then run PowerShell:

```powershell
$env:LOCALCLIP_SERVER = "nas-ip:9100"
$env:LOCALCLIP_TOKEN = "<shared-token>"
.\deploy\windows\install.ps1
```

The installer registers a current-user logon startup entry and starts the client.
The shared token is encrypted locally with DPAPI for the current Windows user;
it is not exposed in the scheduled task or client command line.

### Automated rollout

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

### Hotkeys

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

### Local Console

Open <http://127.0.0.1:9101> on each client to inspect connection state,
pending files, progress, and recent transfer events. The Console supports
Chinese and English: it follows the system's primary language on first use,
then saves an explicit `中文 / English` selection in the current browser.

State-changing endpoints accept only `POST` and reject browser requests from
non-loopback origins.

### Mobile transfer page

When the phone and NAS are on the same trusted network, open
`http://nas-ip:9100/`. Enter the hub's shared token and select **Trust this
device**. The hub creates an HttpOnly browser session that survives hub
restarts; the shared token is never stored in the URL, cookie, or browser
storage. You only need to enter it again after clearing browser data,
forgetting the device, credential expiry, or a hub shared-token change.

- **Send:** choose an online target, paste text, or select photos and files.
  Text lands in the desktop clipboard; files become pending offers.
- **Receive:** the page lists recent items addressed to this phone's device
  name or broadcast by another device. Copy text explicitly, view and save
  images, or download files from the hub.
- The phone's device name is stored in the browser. Desktop senders can enter
  that name for targeted delivery, or leave the target empty to broadcast.
- The inbox reuses the hub's bounded in-memory history and is cleared when the
  hub restarts. Relayed files still expire after 24 hours.
- The page includes an installable Web App Manifest. Android and desktop Chrome
  can use the page prompt or browser menu; on iPhone, use **Add to Home Screen**
  from the Share menu. Launching the home-screen icon uses a standalone window
  without the normal browser address bar.

The page supports Chinese and English. Mobile browsers may deny direct system
clipboard access over plain HTTP; the page then keeps the text selectable and
prompts for the native long-press Paste or Copy action. A trusted HTTPS reverse
proxy provides the complete clipboard-button experience and lets Chrome
reliably recognize and install the PWA. On plain `http://nas-ip:9100`, adding
the page from the browser menu may still work, but installation is platform-
dependent.

### CLI

```text
run -server nas:9100 [-token s] [-name dev]
    Run the receiving daemon, tray, local Console, and built-in hotkey.

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

### Security and data lifecycle

- The shared token authorizes hub HTTP and WebSocket requests. Clients send it
  in the `X-Token` header rather than in URLs.
- The mobile page exchanges the shared token for a random HttpOnly, SameSite
  session cookie. The cookie does not contain the shared token. With **Trust
  this device** enabled, the hub persists only a keyed credential digest,
  device name, and expiry in `mobile_devices.json`; neither the raw cookie nor
  shared token is written to disk. Trust lasts up to 400 days and survives hub
  restarts, with at most 128 remembered devices. Forgetting the device or
  changing the shared token invalidates it.
- The first successful connection pins the hub instance ID. Later identity
  changes are rejected until the local known-hosts record is deliberately
  removed. This is TOFU, not a certificate authority.
- Hub traffic uses plain HTTP and WebSocket. It is **not encrypted**; do not
  expose port `9100` to the public internet. Use a trusted LAN or private VPN.
- Clients deliberately ignore system proxy environment variables and connect
  directly to the configured hub.
- Relayed files are deleted from the hub after 24 hours.
- Hub message history is memory-only and bounded to 200 entries / 64 MiB.
- The local Console binds only to `127.0.0.1`.

### Troubleshooting

#### The hotkey is active but nothing happens on macOS

Confirm that `LocalClipboard.app` is enabled under both Accessibility and
Input Monitoring. Then inspect `/tmp/local-clipboard.log`.

#### The old clipboard value is sent

Use the built-in macOS/Windows hotkey for copy-and-send. Tray actions,
`send-clipboard`, and external shortcut examples intentionally send the
clipboard as it already exists.

#### A rebuilt hub is rejected

The hub instance ID changed. If the rebuild was intentional, remove that
server's entry from the known-hosts file reported in the error, then reconnect
and verify the new first-use pin.

#### Files do not appear immediately on the receiver

Files are offers, not automatic downloads. The notification is informational;
open the local Console to accept them. Accepted files are saved under
`~/Downloads/local-clipboard` by default.

#### Linux clipboard access fails

Install `xclip` for X11 or `wl-clipboard` for Wayland, and make sure the systemd
user service inherits the correct display/session environment.

### Limits

- Inline text is limited to 1 MiB; larger text should be saved and sent as a
  file.
- WebSocket messages are limited to 32 MiB, including inline image encoding.
- Individual relayed files are limited to 1 GiB.
- The supplied Docker and Linux client artifacts target amd64.
- The supplied macOS installer targets Apple Silicon.
- Mobile transfer is explicit: there is no background clipboard monitoring,
  automatic background receive, or push notification support.

[Back to top](#local-clipboard)
