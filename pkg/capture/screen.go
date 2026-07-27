package capture

import (
	"fmt"
	"image"

	"github.com/kbinani/screenshot"
	"gocv.io/x/gocv"
)

// ListScreens returns all available displays
func ListScreens() []image.Rectangle {
	n := screenshot.NumActiveDisplays()
	screens := make([]image.Rectangle, n)
	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)
		screens[i] = bounds
	}
	return screens
}

// CaptureScreen captures the entire screen at the given index.
// Returns a BGR Mat (OpenCV format) matching Python's mss behavior.
func CaptureScreen(screenIndex int) (gocv.Mat, error) {
	n := screenshot.NumActiveDisplays()
	if screenIndex < 0 || screenIndex >= n {
		return gocv.NewMat(), fmt.Errorf("invalid screen index %d (have %d displays)", screenIndex, n)
	}

	bounds := screenshot.GetDisplayBounds(screenIndex)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return gocv.NewMat(), fmt.Errorf("screen capture failed: %w", err)
	}

	// Convert image.Image to gocv.Mat in BGR format (matching Python's mss/OpenCV)
	mat, err := gocv.ImageToMatRGB(img)
	if err != nil {
		return gocv.NewMat(), fmt.Errorf("image conversion failed: %w", err)
	}

	// Convert RGB to BGR (OpenCV uses BGR)
	bgr := gocv.NewMat()
	gocv.CvtColor(mat, &bgr, gocv.ColorRGBToBGR)
	mat.Close()

	return bgr, nil
}

// CaptureRegion captures a specific region of the screen.
// Region is specified in screen coordinates (x, y, w, h).
func CaptureRegion(screenIndex int, x, y, w, h int) (gocv.Mat, error) {
	// Capture full screen
	full, err := CaptureScreen(screenIndex)
	if err != nil {
		return gocv.NewMat(), err
	}
	defer full.Close()

	// Validate bounds
	if x < 0 || y < 0 || w <= 0 || h <= 0 {
		return gocv.NewMat(), fmt.Errorf("invalid region: x=%d y=%d w=%d h=%d", x, y, w, h)
	}
	if x+w > full.Cols() || y+h > full.Rows() {
		return gocv.NewMat(), fmt.Errorf("region out of bounds: (%d,%d,%d,%d) exceeds screen %dx%d",
			x, y, w, h, full.Cols(), full.Rows())
	}

	// Crop to region
	rect := image.Rect(x, y, x+w, y+h)
	cropped := full.Region(rect)
	result := gocv.NewMat()
	cropped.CopyTo(&result)
	cropped.Close()

	return result, nil
}

// TestScreenCapturePermission tests if screen capture is working.
// Returns true if permission granted, false otherwise.
// This detects the macOS behavior where blocked capture returns all-black pixels.
func TestScreenCapturePermission(screenIndex int) (bool, error) {
	mat, err := CaptureScreen(screenIndex)
	if err != nil {
		return false, err
	}
	defer mat.Close()

	// Check if image is all-black (permission denied on macOS)
	_, maxVal, _, _ := gocv.MinMaxLoc(mat)
	if maxVal < 10 {
		return false, nil
	}

	return true, nil
}

// GetPrimaryDisplayBounds returns the bounds of the primary display.
func GetPrimaryDisplayBounds() image.Rectangle {
	n := screenshot.NumActiveDisplays()
	if n > 0 {
		return screenshot.GetDisplayBounds(0)
	}
	// Fallback if no displays found
	return image.Rect(0, 0, 1920, 1080)
}
