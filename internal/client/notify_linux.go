//go:build linux

package client

import "os/exec"

func Notify(text string) {
	if _, err := exec.LookPath("notify-send"); err == nil {
		exec.Command("notify-send", "-a", "local-clipboard", text).Run()
	}
}
