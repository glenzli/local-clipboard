//go:build darwin

package client

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation

#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>

static char *readFileURLPaths() {
	NSPasteboard *pb = [NSPasteboard generalPasteboard];
	NSArray *urls = [pb readObjectsForClasses:@[[NSURL class]] options:nil];
	if (!urls || urls.count == 0) return NULL;
	NSMutableString *out = [NSMutableString string];
	for (NSURL *u in urls) {
		if (u.path) [out appendFormat:@"%@\n", u.path];
	}
	if (out.length == 0) return NULL;
	return strdup(out.UTF8String);
}
*/
import "C"
import (
	"fmt"
	"strings"
	"unsafe"
)

func ReadFileRefs() ([]string, error) {
	cstr := C.readFileURLPaths()
	if cstr == nil {
		return nil, nil
	}
	defer C.free(unsafe.Pointer(cstr))
	var refs []string
	for _, line := range strings.Split(strings.TrimSpace(C.GoString(cstr)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			refs = append(refs, line)
		}
	}
	if refs == nil {
		return nil, fmt.Errorf("clipboard holds no file references")
	}
	return refs, nil
}
