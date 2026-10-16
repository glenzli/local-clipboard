//go:build windows

package client

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"image/png"
)

// Use the color app icon so the tray remains visible on light and dark taskbars.
//
//go:embed web/app-icon.png
var windowsTrayIconData []byte

// Windows LoadImage(IMAGE_ICON) needs an ICO directory around the PNG payload.
func trayIcon() []byte {
	cfg, err := png.DecodeConfig(bytes.NewReader(windowsTrayIconData))
	if err != nil || cfg.Width > 256 || cfg.Height > 256 {
		return nil
	}
	icon := make([]byte, 22+len(windowsTrayIconData))
	binary.LittleEndian.PutUint16(icon[2:], 1) // icon type
	binary.LittleEndian.PutUint16(icon[4:], 1) // one image
	icon[6], icon[7] = byte(cfg.Width%256), byte(cfg.Height%256)
	binary.LittleEndian.PutUint16(icon[10:], 1) // planes
	binary.LittleEndian.PutUint16(icon[12:], 32)
	binary.LittleEndian.PutUint32(icon[14:], uint32(len(windowsTrayIconData)))
	binary.LittleEndian.PutUint32(icon[18:], 22)
	copy(icon[22:], windowsTrayIconData)
	return icon
}
