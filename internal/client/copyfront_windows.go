//go:build windows

package client

import (
	"fmt"
	"syscall"
	"time"
)

var (
	user32                         = syscall.NewLazyDLL("user32.dll")
	procKeybdEvent                 = user32.NewProc("keybd_event")
	procGetClipboardSequenceNumber = user32.NewProc("GetClipboardSequenceNumber")
	procGetAsyncKeyState           = user32.NewProc("GetAsyncKeyState")
)

func RefreshClipboardForHotkey(timeout time.Duration) error {
	if !waitForWindowsModifiers(500 * time.Millisecond) {
		return fmt.Errorf("shortcut modifiers were not released")
	}
	before, _, _ := procGetClipboardSequenceNumber.Call()

	const vkControl = 0x11
	const vkC = 0x43
	const keyup = 0x0002
	procKeybdEvent.Call(vkControl, 0, 0, 0)
	procKeybdEvent.Call(vkC, 0, 0, 0)
	procKeybdEvent.Call(vkC, 0, keyup, 0)
	procKeybdEvent.Call(vkControl, 0, keyup, 0)

	return waitForClipboardChange(uint64(before), func() uint64 {
		sequence, _, _ := procGetClipboardSequenceNumber.Call()
		return uint64(sequence)
	}, timeout)
}

func waitForWindowsModifiers(timeout time.Duration) bool {
	const (
		vkShift   = 0x10
		vkControl = 0x11
		vkMenu    = 0x12
		vkLWin    = 0x5B
		vkRWin    = 0x5C
	)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pressed := false
		for _, key := range []uintptr{vkShift, vkControl, vkMenu, vkLWin, vkRWin} {
			state, _, _ := procGetAsyncKeyState.Call(key)
			if state&0x8000 != 0 {
				pressed = true
				break
			}
		}
		if !pressed {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}
