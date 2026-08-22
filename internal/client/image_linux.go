//go:build linux

package client

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

func hasBin(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func wl() bool { return hasBin("wl-copy") && (os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("XDG_SESSION_TYPE") == "wayland") }

func ReadImage() ([]byte, error) {
	if wl() {
		out, err := exec.Command("wl-paste", "-t", "image/png").Output()
		if err != nil {
			return nil, fmt.Errorf("wl-paste: %v (no image in clipboard?)", err)
		}
		return out, nil
	}
	if !hasBin("xclip") {
		return nil, fmt.Errorf("install xclip (X11) or wl-clipboard (Wayland)")
	}
	out, err := exec.Command("xclip", "-selection", "clipboard", "-t", "image/png", "-o").Output()
	if err != nil {
		return nil, fmt.Errorf("xclip read image: %v (no image in clipboard?)", err)
	}
	return out, nil
}

func WriteImage(data []byte) error {
	var cmd *exec.Cmd
	if wl() {
		cmd = exec.Command("wl-copy", "-t", "image/png")
	} else {
		if !hasBin("xclip") {
			return fmt.Errorf("install xclip (X11) or wl-clipboard (Wayland)")
		}
		cmd = exec.Command("xclip", "-selection", "clipboard", "-t", "image/png")
	}
	cmd.Stdin = bytes.NewReader(data)
	return cmd.Run()
}
