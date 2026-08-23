#!/bin/bash
set -e
SERVER=${1:-$LOCALCLIP_SERVER}
TOKEN=${2:-$LOCALCLIP_TOKEN}
NAME=$3
if [ -z "$NAME" ]; then NAME=$(hostname -s); fi
if [ -z "$TOKEN" ]; then
  printf "hub token: "; read -r TOKEN
fi
if [ -z "$SERVER" ]; then
  echo "usage: ./install.sh <nas:9100> [token] [name]"
  exit 1
fi
BIN="$HOME/.local/bin/local-clipboard"
mkdir -p "$(dirname "$BIN")"
cp "$(dirname "$0")/local-clipboard-linux-amd64" "$BIN"
chmod +x "$BIN"
mkdir -p "$HOME/.config/systemd/user"
sed -e "s|__BIN__|$BIN|g" -e "s|__SERVER__|$SERVER|g" -e "s|__TOKEN__|$TOKEN|g" -e "s|__NAME__|$NAME|g" \
  "$(dirname "$0")/local-clipboard.service.template" > "$HOME/.config/systemd/user/local-clipboard.service"
systemctl --user daemon-reload
systemctl --user enable --now local-clipboard
loginctl enable-linger "$USER" >/dev/null 2>&1 || true
echo "installed and started."
echo "logs: journalctl --user -u local-clipboard -f"
echo "note (X11/Wayland): requires xclip or wl-clipboard installed"
