# Local Clipboard

[中文](#中文) | [English](#english)

<a id="中文"></a>
## 中文

Local Clipboard 是一个通过 NAS Hub 在局域网内传输文本、图片和文件的工具。
桌面客户端负责读写本机剪贴板，Hub 负责设备连接、消息转发和临时文件中转；手机通过
Hub 提供的网页显式收发内容。

```text
桌面客户端 A ── WebSocket ──▶ NAS Hub ◀── WebSocket ── 桌面客户端 B
                                  │
                                  └── 手机网页
```

项目没有云服务和账号系统，也不会监视或自动同步每一次剪贴板变化。普通的
`Cmd+C` / `Ctrl+C` 仍然只在本机生效。

### 当前能力

- macOS 和 Windows 的内置快捷键会先执行复制，再发送新的剪贴板内容。
- 托盘菜单和 `send-clipboard` 发送剪贴板中已有的内容，不会执行复制。
- 收到文本或图片时，桌面客户端会写入本机剪贴板。
- 收到文件时，桌面客户端会创建待接收项目；确认后保存到
  `~/Downloads/local-clipboard`，并可从传输记录打开所在位置。
- 发送时可以填写目标设备名；留空时发送给其他在线设备。
- 每个桌面客户端提供只监听 `127.0.0.1:9101` 的本地控制台，用于发送内容、接收文件
  和查看近期记录。
- Hub 首页提供适合手机使用的收发页面。手机可以发送文本、把单张图片发送到桌面
  剪贴板，或把图片和其他内容作为文件发送；也可以复制或下载收到的内容。
- 手机浏览器可以选择“信任此设备”。Hub 保存派生凭证而不是共享令牌，之后通常不必
  再次输入令牌；清除浏览器数据、取消信任或修改共享令牌后需要重新验证。

### 当前边界

- Hub 使用明文 HTTP 和 WebSocket。共享令牌只用于授权，不提供传输加密。请只在可信
  局域网或私有 VPN 中使用，不要把 `9100` 端口直接暴露到公网。
- 第一次成功连接时，桌面客户端会记录 Hub 实例 ID。实例 ID 变化后会拒绝连接，直到
  用户删除错误信息中指出的本地记录并重新确认。
- 中转文件在 Hub 上保留 24 小时。消息历史只保存在内存中，上限为 200 条或 64 MiB，
  Hub 重启后会清空。
- 内联文本上限为 1 MiB，WebSocket 消息上限为 32 MiB，单个中转文件上限为 1 GiB。
- 当前 Docker/Hub 和 Linux 客户端构建目标是 Linux amd64；macOS 安装器面向 Apple
  Silicon；Windows 构建目标是 amd64。
- Linux X11 需要 `xclip`，Wayland 需要 `wl-clipboard`。静态 Linux 构建不包含进程内
  全局快捷键，需要先复制，再用桌面快捷方式调用 `send-clipboard`。
- 手机端是显式收发，不提供后台剪贴板监听、后台自动接收或推送通知。
- 手机页面包含 Web App Manifest，但安装方式和剪贴板 API 取决于浏览器和访问协议。
  普通 HTTP 下可能只能创建浏览器快捷方式，或需要使用系统的长按复制/粘贴；可信
  HTTPS 地址可以获得更完整的 PWA 和剪贴板能力。

### 部署 Hub

从源码构建需要 Go 1.27 或更新版本。Dockerfile 只打包已经构建好的 Linux amd64
二进制，不会在镜像内编译源码。

```bash
git clone https://github.com/glenzli/local-clipboard.git
cd local-clipboard
cp .env.example .env
```

生成一个随机共享令牌，并写入 `.env`：

```bash
openssl rand -hex 16
```

```dotenv
LOCALCLIP_TOKEN=<生成的令牌>
```

构建 Hub 并启动容器：

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags "-s -w" -o local-clipboard-linux-amd64 .
docker compose up -d --build
```

Hub 监听 TCP `9100`。Compose 将实例 ID 和中转文件保存在仓库下的 `./data`。

### 安装桌面客户端

安装脚本从自身所在目录读取对应平台的二进制。自行构建时，可以使用
`make dist-all` 生成 Linux amd64、macOS arm64 和 Windows amd64 产物。

#### macOS

仓库中包含 Apple Silicon 安装产物，可以直接运行：

```bash
LOCALCLIP_SERVER=nas-ip:9100 \
LOCALCLIP_TOKEN='<shared-token>' \
./deploy/macos/install.sh
```

安装位置是 `~/Applications/LocalClipboard.app`。首次使用内置快捷键时，需要在
“系统设置 → 隐私与安全性”中同时允许“辅助功能”和“输入监控”。日志位于
`/tmp/local-clipboard.log`。

#### Linux

把 `local-clipboard-linux-amd64` 放到 `deploy/linux/install.sh` 同一目录，然后运行：

```bash
./deploy/linux/install.sh nas-ip:9100 '<shared-token>'
```

安装器创建 systemd 用户服务。使用
`journalctl --user -u local-clipboard -f` 查看日志。

#### Windows

把 `local-clipboard-windows-amd64.exe` 放到 `deploy/windows/install.ps1` 同一目录，
然后在 PowerShell 中运行：

```powershell
$env:LOCALCLIP_SERVER = "nas-ip:9100"
$env:LOCALCLIP_TOKEN = "<shared-token>"
.\deploy\windows\install.ps1
```

安装器注册当前用户的登录启动项。共享令牌使用当前 Windows 用户的 DPAPI 加密后保存在
本地，不会写入启动命令。
登录时直接启动托盘客户端，不需要保持命令行或 PowerShell 窗口。再次运行安装器可以
保留已有服务器和令牌并升级客户端。日志位于
`%LOCALAPPDATA%\local-clipboard\local-clipboard.log`。

在 Windows 上自行构建无控制台窗口的版本：

```powershell
go build -ldflags "-s -w -H windowsgui" -o dist/local-clipboard-windows-amd64.exe .
```

图片读写仍使用系统 PowerShell，按需在后台短暂运行。PNG 格式保留透明度，同时提供
兼容旧应用的白底位图；复制截图和浏览器图片时都可以直接发送。

### 使用

| 平台 | 默认快捷键 | 行为 |
|---|---|---|
| macOS | `Cmd+Option+C` | 复制当前选择，等待剪贴板更新，然后发送 |
| Windows | `Ctrl+Alt+C` | 复制当前选择，等待剪贴板更新，然后发送 |
| Linux 静态构建 | 无 | 先复制，再通过桌面快捷方式运行 `send-clipboard` |

如果发送到的是旧剪贴板内容，请确认使用的是 macOS/Windows 客户端的内置快捷键。
托盘菜单、外部快捷方式和 `send-clipboard` 只发送已有剪贴板。

本地控制台：<http://127.0.0.1:9101>

手机页面：`http://nas-ip:9100/`

常用命令：

```text
run -server nas:9100 [-token s] [-name dev]
    运行桌面客户端、托盘、本地控制台和平台支持的内置快捷键。

send-clipboard [-to device]
    通过本地客户端发送已有剪贴板。

send-text -server nas:9100 [-token s] [-to device] "hello"
    发送指定文本。

send-file [-to device] file
    通过本地客户端发送文件。

detect
    显示当前剪贴板会被识别为什么类型，但不发送。

serve -addr :9100 [-token s]
    不使用 Docker，直接运行 Hub。
```

各命令的参数可运行 `local-clipboard <command> -h` 查看。

### 构建和更新

```bash
cp deploy.env.example deploy.env
# 填写 NAS_HOST、NAS_DIR、LOCALCLIP_SERVER 和 LOCALCLIP_TOKEN。
make rollout
```

`make rollout` 从 Apple Silicon Mac 构建各平台产物，通过 SSH 更新 NAS Hub，并重新安装
当前 Mac 客户端。也可以单独运行：

| 目标 | 行为 |
|---|---|
| `make build` | 构建当前平台到 `dist/` |
| `make dist-all` | 构建 Linux amd64、macOS arm64 和 Windows amd64 |
| `make deploy` | 更新 NAS Hub |
| `make install` | 重新安装当前 Mac/Linux 客户端 |

### 许可证

本项目采用 [MIT License](LICENSE)。

[返回顶部](#local-clipboard)

---

<a id="english"></a>
## English

Local Clipboard transfers text, images, and files across a LAN through a NAS
hub. Desktop clients read and write the local clipboard. The hub maintains
device connections, forwards messages, and temporarily stores relayed files.
Phones send and receive explicitly through the hub's web page.

```text
desktop A ── WebSocket ──▶ NAS hub ◀── WebSocket ── desktop B
                               │
                               └── mobile web page
```

The project has no cloud service or account system. It does not watch or
automatically synchronize every clipboard change; normal `Cmd+C` / `Ctrl+C`
remains local.

### Current behavior

- The built-in macOS and Windows hotkeys perform Copy before sending the new
  clipboard value.
- The tray menu and `send-clipboard` send the existing clipboard and do not
  perform Copy.
- Incoming text and images are written to the desktop clipboard.
- Incoming files become pending offers. Accepted files are saved under
  `~/Downloads/local-clipboard`, and their location can be opened from the
  transfer history.
- A sender can specify a device name. An empty target sends to the other online
  devices.
- Each desktop client provides a local Console on `127.0.0.1:9101` for sending
  content, accepting files, and viewing recent events.
- The hub home page provides a mobile interface. A phone can send text, send one
  image to a desktop clipboard, send images or other content as files, and copy
  or download received content.
- A mobile browser can trust the current device. The hub stores a derived
  credential rather than the shared token, so the token is normally entered
  only once. Clearing browser data, forgetting the device, or changing the
  shared token requires authentication again.

### Current boundaries

- The hub uses plain HTTP and WebSocket. The shared token authorizes requests
  but does not encrypt traffic. Use the service only on a trusted LAN or private
  VPN, and do not expose port `9100` directly to the public internet.
- On the first successful connection, a desktop client records the hub instance
  ID. It rejects a changed ID until the user removes the local record named in
  the error and confirms the new instance.
- Relayed files remain on the hub for 24 hours. Message history is memory-only,
  bounded to 200 entries or 64 MiB, and is cleared when the hub restarts.
- Inline text is limited to 1 MiB, WebSocket messages to 32 MiB, and each
  relayed file to 1 GiB.
- The current Docker/hub and Linux client targets are Linux amd64. The macOS
  installer targets Apple Silicon, and the Windows build targets amd64.
- Linux requires `xclip` on X11 or `wl-clipboard` on Wayland. The static Linux
  build has no in-process global hotkey; copy first, then bind
  `send-clipboard` to a desktop shortcut.
- Mobile transfer is explicit. There is no background clipboard monitoring,
  automatic background receive, or push notification support.
- The mobile page includes a Web App Manifest, but installation and clipboard
  APIs depend on the browser and access protocol. Plain HTTP may provide only a
  browser shortcut or require native long-press Copy/Paste. A trusted HTTPS URL
  enables more complete PWA and clipboard behavior.

### Run the hub

Building from source requires Go 1.27 or newer. The Dockerfile packages an
already-built Linux amd64 binary; it does not compile the source in the image.

```bash
git clone https://github.com/glenzli/local-clipboard.git
cd local-clipboard
cp .env.example .env
```

Generate a random shared token and place it in `.env`:

```bash
openssl rand -hex 16
```

```dotenv
LOCALCLIP_TOKEN=<generated-token>
```

Build the hub and start the container:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags "-s -w" -o local-clipboard-linux-amd64 .
docker compose up -d --build
```

The hub listens on TCP `9100`. Compose stores the instance ID and relayed files
in `./data` under the repository.

### Install desktop clients

Each installer reads its platform binary from the same directory. For local
builds, `make dist-all` produces Linux amd64, macOS arm64, and Windows amd64
artifacts.

#### macOS

The repository includes an Apple Silicon installation artifact:

```bash
LOCALCLIP_SERVER=nas-ip:9100 \
LOCALCLIP_TOKEN='<shared-token>' \
./deploy/macos/install.sh
```

The app is installed at `~/Applications/LocalClipboard.app`. Before using the
built-in hotkey for the first time, enable the app under both Accessibility and
Input Monitoring in System Settings → Privacy & Security. Logs are written to
`/tmp/local-clipboard.log`.

#### Linux

Place `local-clipboard-linux-amd64` beside `deploy/linux/install.sh`, then run:

```bash
./deploy/linux/install.sh nas-ip:9100 '<shared-token>'
```

The installer creates a systemd user service. View logs with
`journalctl --user -u local-clipboard -f`.

#### Windows

Place `local-clipboard-windows-amd64.exe` beside
`deploy/windows/install.ps1`, then run in PowerShell:

```powershell
$env:LOCALCLIP_SERVER = "nas-ip:9100"
$env:LOCALCLIP_TOKEN = "<shared-token>"
.\deploy\windows\install.ps1
```

The installer registers a current-user logon startup entry. The shared token is
stored locally using DPAPI for the current Windows user and is not written into
the startup command.
Logon starts the tray client directly, with no terminal or persistent PowerShell
process. Re-run the installer to upgrade using the saved server and token. Logs
are written to `%LOCALAPPDATA%\local-clipboard\local-clipboard.log`.

To build a Windows binary without a console window:

```powershell
go build -ldflags "-s -w -H windowsgui" -o dist/local-clipboard-windows-amd64.exe .
```

Image operations still use system PowerShell briefly in the background. PNG
preserves transparency, with a white-backed bitmap for older applications.
Screenshots and browser images can both be sent from the clipboard.

### Usage

| Platform | Default hotkey | Behavior |
|---|---|---|
| macOS | `Cmd+Option+C` | Copy the current selection, wait for the clipboard to change, then send |
| Windows | `Ctrl+Alt+C` | Copy the current selection, wait for the clipboard to change, then send |
| Static Linux build | None | Copy first, then run `send-clipboard` from a desktop shortcut |

If an old clipboard value is sent, confirm that the built-in macOS/Windows
hotkey was used. Tray actions, external shortcuts, and `send-clipboard` send
the clipboard as it already exists.

Local Console: <http://127.0.0.1:9101>

Mobile page: `http://nas-ip:9100/`

Common commands:

```text
run -server nas:9100 [-token s] [-name dev]
    Run the desktop client, tray, local Console, and supported built-in hotkey.

send-clipboard [-to device]
    Send the existing clipboard through the local client.

send-text -server nas:9100 [-token s] [-to device] "hello"
    Send explicit text.

send-file [-to device] file
    Send a file through the local client.

detect
    Show how the current clipboard is detected without sending it.

serve -addr :9100 [-token s]
    Run the hub without Docker.
```

Run `local-clipboard <command> -h` for that command's options.

### Build and update

```bash
cp deploy.env.example deploy.env
# Fill NAS_HOST, NAS_DIR, LOCALCLIP_SERVER, and LOCALCLIP_TOKEN.
make rollout
```

`make rollout` runs from an Apple Silicon Mac. It builds the platform artifacts,
updates the NAS hub over SSH, and reinstalls the current Mac client. Individual
targets are also available:

| Target | Behavior |
|---|---|
| `make build` | Build the current platform into `dist/` |
| `make dist-all` | Build Linux amd64, macOS arm64, and Windows amd64 |
| `make deploy` | Update the NAS hub |
| `make install` | Reinstall the current Mac/Linux client |

### License

This project is licensed under the [MIT License](LICENSE).

[Back to top](#local-clipboard)
