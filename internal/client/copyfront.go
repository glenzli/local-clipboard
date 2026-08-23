package client

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	clipboardCopyTimeout  = 1500 * time.Millisecond
	clipboardPollInterval = 10 * time.Millisecond
)

var (
	errClipboardUnchanged = errors.New("frontmost app did not update the clipboard")
	hotkeySendMu          sync.Mutex
)

// SendClipboardFromHotkey owns the explicit remote-copy gesture. Platform
// code gets a fresh clipboard value first; the ordinary CLI and tray actions
// intentionally continue to send the clipboard as it already exists.
func SendClipboardFromHotkey(c *Client, to string, hooks SendEventHooks) error {
	hotkeySendMu.Lock()
	defer hotkeySendMu.Unlock()

	if err := RefreshClipboardForHotkey(clipboardCopyTimeout); err != nil {
		return fmt.Errorf("copy current selection: %w", err)
	}
	return SendCurrentClipboard(c, to, hooks)
}

func waitForClipboardChange(before uint64, current func() uint64, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if current() != before {
			return nil
		}
		if !time.Now().Before(deadline) {
			return errClipboardUnchanged
		}
		time.Sleep(clipboardPollInterval)
	}
}
