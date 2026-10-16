//go:build windows

package client

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"unsafe"
)

func TestWindowsTrayIconLoadsWithWindowsAPI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tray.ico")
	if err := os.WriteFile(path, trayIcon(), 0o600); err != nil {
		t.Fatal(err)
	}
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	const imageIcon = 1
	const loadFromFile = 0x10
	const defaultSize = 0x40
	handle, _, err := user32.NewProc("LoadImageW").Call(0, uintptr(unsafe.Pointer(name)), imageIcon, 0, 0, loadFromFile|defaultSize)
	if handle == 0 {
		t.Fatalf("Windows could not load the tray icon: %v", err)
	}
	user32.NewProc("DestroyIcon").Call(handle)
}
