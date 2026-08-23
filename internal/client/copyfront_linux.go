//go:build linux

package client

import "time"

// Linux has no portable way to inject Copy across both X11 and Wayland. Keep
// its established behavior: the hotkey sends the clipboard as it already is.
func RefreshClipboardForHotkey(time.Duration) error { return nil }
