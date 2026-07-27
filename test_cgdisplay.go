package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation
#include <CoreGraphics/CoreGraphics.h>
#include <stdlib.h>

typedef struct {
    unsigned char *data;
    int width;
    int height;
    int bytes_per_row;
} CaptureResult;

CaptureResult capture_screen() {
    CaptureResult result = {NULL, 0, 0, 0};
    
    CGDirectDisplayID displayID = CGMainDisplayID();
    CGImageRef image = CGDisplayCreateImage(displayID);
    if (!image) {
        return result;
    }
    
    result.width = (int)CGImageGetWidth(image);
    result.height = (int)CGImageGetHeight(image);
    result.bytes_per_row = (int)CGImageGetBytesPerRow(image);
    
    CFDataRef dataRef = CGDataProviderCopyData(CGImageGetDataProvider(image));
    if (dataRef) {
        size_t length = CFDataGetLength(dataRef);
        result.data = (unsigned char *)malloc(length);
        memcpy(result.data, CFDataGetBytePtr(dataRef), length);
        CFRelease(dataRef);
    }
    
    CGImageRelease(image);
    return result;
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

func main() {
	fmt.Println("Testing CGDisplay screen capture...")
	
	result := C.capture_screen()
	if result.data == nil {
		fmt.Println("Failed to capture screen")
		return
	}
	defer C.free(unsafe.Pointer(result.data))
	
	fmt.Printf("Captured: %dx%d (bytes_per_row=%d)\n", result.width, result.height, result.bytes_per_row)
	
	// Check pixel values
	dataLen := result.height * result.bytes_per_row
	data := unsafe.Slice(result.data, dataLen)
	
	maxVal := byte(0)
	for i := 0; i < len(data); i++ {
		if data[i] > maxVal {
			maxVal = data[i]
		}
	}
	
	fmt.Printf("Max pixel value: %d\n", maxVal)
	if maxVal < 10 {
		fmt.Println("ERROR: All black - permission denied")
	} else {
		fmt.Println("SUCCESS: Screen capture working!")
	}
}
