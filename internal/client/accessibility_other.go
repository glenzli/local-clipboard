//go:build !darwin

package client

import "fyne.io/systray"

func grantItem() *systray.MenuItem { return nil }

func openAccessibilitySettings() {}
