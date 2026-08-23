//go:build darwin

package client

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreGraphics -framework AppKit

#import <CoreGraphics/CoreGraphics.h>
#import <AppKit/AppKit.h>

static long long clipboardChangeCount() {
	return (long long)[[NSPasteboard generalPasteboard] changeCount];
}

static int sendCopyKeystroke() {
	CGEventSourceRef src = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
	if (!src) return 0;
	CGEventRef down = CGEventCreateKeyboardEvent(src, 8, true);
	CGEventRef up = CGEventCreateKeyboardEvent(src, 8, false);
	if (!down || !up) {
		if (down) CFRelease(down);
		if (up) CFRelease(up);
		CFRelease(src);
		return 0;
	}
	CGEventSetFlags(down, kCGEventFlagMaskCommand);
	CGEventSetFlags(up, kCGEventFlagMaskCommand);
	CGEventPost(kCGHIDEventTap, down);
	CFRelease(down);
	[NSThread sleepForTimeInterval:0.03];
	CGEventPost(kCGHIDEventTap, up);
	CFRelease(up);
	CFRelease(src);
	return 1;
}
*/
import "C"

import (
	"fmt"
	"time"
)

func RefreshClipboardForHotkey(timeout time.Duration) error {
	before := uint64(C.clipboardChangeCount())
	if C.sendCopyKeystroke() == 0 {
		return fmt.Errorf("could not create the macOS copy event")
	}
	return waitForClipboardChange(before, func() uint64 {
		return uint64(C.clipboardChangeCount())
	}, timeout)
}
