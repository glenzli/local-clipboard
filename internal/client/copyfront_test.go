package client

import (
	"errors"
	"testing"
	"time"
)

func TestWaitForClipboardChange(t *testing.T) {
	calls := 0
	err := waitForClipboardChange(7, func() uint64 {
		calls++
		if calls < 2 {
			return 7
		}
		return 8
	}, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("waitForClipboardChange: %v", err)
	}
}

func TestWaitForClipboardChangeTimesOut(t *testing.T) {
	err := waitForClipboardChange(7, func() uint64 { return 7 }, 0)
	if !errors.Is(err, errClipboardUnchanged) {
		t.Fatalf("got %v, want errClipboardUnchanged", err)
	}
}
