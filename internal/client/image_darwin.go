//go:build darwin

package client

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
)

func ReadImage() ([]byte, error) {
	cmd := exec.Command("osascript", "-")
	cmd.Stdin = strings.NewReader(`the clipboard as «class PNGf»`)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("osascript read image (no image in clipboard?): %v %s", err, strings.TrimSpace(stderr.String()))
	}
	s := strings.TrimSpace(string(out))
	const prefix, suffix = "«data PNGf", "»"
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return nil, fmt.Errorf("unexpected osascript output")
	}
	return hex.DecodeString(strings.TrimSuffix(strings.TrimPrefix(s, prefix), suffix))
}

func WriteImage(data []byte) error {
	script := "set the clipboard to («data PNGf" + hex.EncodeToString(data) + "»)"
	cmd := exec.Command("osascript", "-")
	cmd.Stdin = strings.NewReader(script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("osascript write image: %v %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
