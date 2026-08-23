//go:build !cgo && !windows

package client

import (
	"errors"
	"log"
)

func RegisterHotkey(c *Client, spec string) error {
	log.Printf("[hotkey] disabled in this build (no cgo); bind the CLI command in your desktop environment instead")
	return errors.New("hotkey unsupported in this build")
}
