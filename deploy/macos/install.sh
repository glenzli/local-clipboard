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
case $(uname -m) in
  arm64) BINFILE=local-clipboard-darwin-arm64 ;;
  x86_64) BINFILE=local-clipboard-darwin-amd64 ;;
  *) echo "unsupported arch"; exit 1 ;;
esac
BIN="$HOME/.local/bin/local-clipboard"
mkdir -p "$(dirname "$BIN")"
cp "$(dirname "$0")/$BINFILE" "$BIN"
chmod +x "$BIN"
PLIST="$HOME/Library/LaunchAgents/com.glenzli.local-clipboard.plist"
sed -e "s|__BIN__|$BIN|g" -e "s|__SERVER__|$SERVER|g" -e "s|__TOKEN__|$TOKEN|g" -e "s|__NAME__|$NAME|g" \
  "$(dirname "$0")/com.glenzli.local-clipboard.plist.template" > "$PLIST"
launchctl unload "$PLIST" 2>/dev/null || true
launchctl load "$PLIST"
echo "installed and started. logs: tail -f /tmp/local-clipboard.log"
