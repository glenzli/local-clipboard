//go:build darwin

package client

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework AppKit

#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>

static char *readClipboardImagePNG(int *outLen) {
	NSPasteboard *pb = [NSPasteboard generalPasteboard];
	NSData *png = [pb dataForType:NSPasteboardTypePNG];
	if (!png) {
		NSData *tiff = [pb dataForType:NSPasteboardTypeTIFF];
		if (!tiff) return NULL;
		NSBitmapImageRep *rep = [NSBitmapImageRep imageRepWithData:tiff];
		if (!rep) return NULL;
		png = [rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
	}
	if (!png.length) return NULL;
	char *buf = (char *)malloc(png.length);
	memcpy(buf, png.bytes, png.length);
	*outLen = (int)png.length;
	return buf;
}

static BOOL writeClipboardImage(const void *bytes, int len) {
	NSData *d = [NSData dataWithBytes:bytes length:len];
	NSPasteboard *pb = [NSPasteboard generalPasteboard];
	[pb clearContents];
	return [pb setData:d forType:NSPasteboardTypePNG];
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

func ReadImage() ([]byte, error) {
	var length C.int
	buf := C.readClipboardImagePNG(&length)
	if buf == nil || length == 0 {
		return nil, fmt.Errorf("no image in clipboard")
	}
	defer C.free(unsafe.Pointer(buf))
	return C.GoBytes(unsafe.Pointer(buf), length), nil
}

func WriteImage(img []byte) error {
	if len(img) == 0 {
		return fmt.Errorf("empty image")
	}
	ok := C.writeClipboardImage(unsafe.Pointer(&img[0]), C.int(len(img)))
	if !ok {
		return fmt.Errorf("failed to set clipboard image")
	}
	return nil
}
