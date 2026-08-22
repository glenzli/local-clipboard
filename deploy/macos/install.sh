#!/bin/bash
set -e
if [ $# -lt 2 ]; then
  echo "usage: ./install.sh <nas:9100> <token> [name]"
  echo "place local-clipboard-darwin-arm64 (or -amd64) next to this script first"
  exit 1
fi
SERVER=$1; TOKEN=$2; NAME=${3:-$(hostname -s)}
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
