package main

/*
#cgo LDFLAGS: -framework CoreGraphics -framework ImageIO
#include <CoreGraphics/CoreGraphics.h>
#include <ImageIO/ImageIO.h>

void save_screen_to_file(const char *path) {
    CGImageRef image = CGDisplayCreateImage(CGMainDisplayID());
    if (!image) return;
    
    CFURLRef url = CFURLCreateFromFileSystemRepresentation(NULL, (const UInt8*)path, strlen(path), false);
    CGImageDestinationRef dest = CGImageDestinationCreateWithURL(url, kUTTypePNG, 1, NULL);
    
    if (dest) {
        CGImageDestinationAddImage(dest, image, NULL);
        CGImageDestinationFinalize(dest);
        CFRelease(dest);
    }
    
    CFRelease(url);
    CGImageRelease(image);
}
*/
import "C"
import (
	"fmt"
	"gocv.io/x/gocv"
)

func main() {
	path := "/tmp/direct-capture.png"
	cpath := C.CString(path)
	
	fmt.Println("Capturing screen directly via CGDisplayCreateImage...")
	C.save_screen_to_file(cpath)
	
	mat := gocv.IMRead(path, gocv.IMReadColor)
	defer mat.Close()
	
	if mat.Empty() {
		fmt.Println("Failed to load image")
		return
	}
	
	fmt.Printf("Captured: %dx%d\n", mat.Cols(), mat.Rows())
	_, maxVal, _, _ := gocv.MinMaxLoc(mat)
	fmt.Printf("Max pixel: %.0f\n", maxVal)
	
	if maxVal < 10 {
		fmt.Println("All black - permission issue")
	} else {
		fmt.Println("SUCCESS!")
	}
}
