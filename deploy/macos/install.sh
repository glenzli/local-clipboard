#!/bin/bash
set -e
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
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
  x86_64) echo "Intel Macs: run 'make build' on this machine to produce local-clipboard-darwin-amd64 first"; exit 1 ;;
  *) echo "unsupported arch"; exit 1 ;;
esac

APP="$HOME/Applications/LocalClipboard.app"
PLIST_FILE="$HOME/Library/LaunchAgents/com.glenzli.local-clipboard.plist"
launchctl unload "$PLIST_FILE" 2>/dev/null || true

mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
rm -f "$APP/Contents/MacOS/local-clipboard.cstemp"
cp "$SCRIPT_DIR/$BINFILE" "$APP/Contents/MacOS/local-clipboard"
cp "$SCRIPT_DIR/LocalClipboard.icns" "$APP/Contents/Resources/LocalClipboard.icns"
chmod +x "$APP/Contents/MacOS/local-clipboard"

cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleIdentifier</key>
    <string>com.glenzli.local-clipboard</string>
    <key>CFBundleName</key>
    <string>LocalClipboard</string>
    <key>CFBundleExecutable</key>
    <string>local-clipboard</string>
    <key>CFBundleIconFile</key>
    <string>LocalClipboard.icns</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>LSUIElement</key>
    <true/>
</dict>
</plist>
PLIST
# Sign with a stable identity if one exists, so TCC permissions (hotkey etc.)
# survive reinstalls. Ad-hoc (-) changes fingerprint every build.
SIGN_ID=$(security find-identity -v -p codesigning 2>/dev/null | grep -m1 '"[^"]*"' | sed 's/.*"\(.*\)".*/\1/')
if [ -n "$SIGN_ID" ] && codesign --force --timestamp=none --sign "$SIGN_ID" "$APP"; then
  echo "signed with: $SIGN_ID"
else
  if [ -n "$SIGN_ID" ]; then
    echo "WARNING: signing with '$SIGN_ID' failed; falling back to ad-hoc."
  fi
  codesign --force --sign - "$APP"
  echo "WARNING: ad-hoc signed — you will need to re-grant permissions after each reinstall."
  echo "To fix permanently: Keychain Access -> Certificate Assistant -> Create Certificate ->"
  echo "  name 'localclipboard-dev', type: Code Signing. Then re-run this installer."
fi
codesign --verify --deep --strict "$APP"

BIN="$APP/Contents/MacOS/local-clipboard"
sed -e "s|__BIN__|$BIN|g" -e "s|__SERVER__|$SERVER|g" -e "s|__TOKEN__|$TOKEN|g" -e "s|__NAME__|$NAME|g" \
  "$SCRIPT_DIR/com.glenzli.local-clipboard.plist.template" > "$PLIST_FILE"
launchctl load "$PLIST_FILE"
echo "installed as $APP and started."
echo ""
echo "TWO-TIME STEPS for the global hotkey (both needed):"
echo "  1. System Settings -> Privacy & Security -> Accessibility -> add/check: $APP"
echo "  2. System Settings -> Privacy & Security -> Input Monitoring -> add/check: $APP"
echo "  $APP"
echo "  (it retries automatically; no restart needed after granting)"
echo "logs: tail -f /tmp/local-clipboard.log"
