//go:build darwin

package client

import (
	"os/exec"

	"fyne.io/systray"
)

func grantItem() *systray.MenuItem {
	return systray.AddMenuItem("Grant Accessibility…", "required for the global hotkey")
}

func openAccessibilitySettings() {
	exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility").Start()
}
