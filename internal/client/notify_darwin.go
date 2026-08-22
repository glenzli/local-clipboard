//go:build darwin

package client

import "os/exec"

func Notify(text string) {
	exec.Command("osascript", "-e", `display notification "`+text+`" with title "Local Clipboard"`).Run()
}
