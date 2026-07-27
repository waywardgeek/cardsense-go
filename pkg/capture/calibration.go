package capture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Calibration holds the detection box position for a specific screen resolution
type Calibration struct {
	ScreenW int   `json:"screen_w"`
	ScreenH int   `json:"screen_h"`
	Box     Box   `json:"box"` // (x, y, w, h)
}

// Box represents a screen region (x, y, width, height)
type Box struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// Reference calibration: Bill's 1920×1080 setup
var (
	RefScreen = struct{ W, H int }{1920, 1080}
	RefBox    = Box{21, 47, 449, 709}
)

// LoadCalibration loads saved calibration, scaled to current screen resolution.
// Returns (box, calibrated) where calibrated=true if loaded from file.
// Falls back to RefBox scaled to current screen if no saved calibration.
func LoadCalibration(dataDir string, screenW, screenH int) (Box, bool) {
	calFile := filepath.Join(dataDir, "calibration.json")

	// Try to load saved calibration
	if data, err := os.ReadFile(calFile); err == nil {
		var cal Calibration
		if err := json.Unmarshal(data, &cal); err == nil {
			// Scale to current screen if resolution differs
			if screenW == cal.ScreenW && screenH == cal.ScreenH {
				fmt.Printf("[CALIBRATION] Loaded saved box: (%d,%d,%d,%d) (same resolution)\n",
					cal.Box.X, cal.Box.Y, cal.Box.W, cal.Box.H)
				return cal.Box, true
			}

			// Scale proportionally
			scaleX := float64(screenW) / float64(cal.ScreenW)
			scaleY := float64(screenH) / float64(cal.ScreenH)
			scaled := Box{
				X: int(float64(cal.Box.X) * scaleX),
				Y: int(float64(cal.Box.Y) * scaleY),
				W: int(float64(cal.Box.W) * scaleX),
				H: int(float64(cal.Box.H) * scaleY),
			}
			fmt.Printf("[CALIBRATION] Loaded + scaled: %dx%d → %dx%d, box=(%d,%d,%d,%d)\n",
				cal.ScreenW, cal.ScreenH, screenW, screenH,
				scaled.X, scaled.Y, scaled.W, scaled.H)
			return scaled, true
		}
		fmt.Printf("[CALIBRATION] Load failed: %v, using default\n", err)
	}

	// Fall back to reference box, scaled to current screen
	scaleX := float64(screenW) / float64(RefScreen.W)
	scaleY := float64(screenH) / float64(RefScreen.H)
	scaled := Box{
		X: int(float64(RefBox.X) * scaleX),
		Y: int(float64(RefBox.Y) * scaleY),
		W: int(float64(RefBox.W) * scaleX),
		H: int(float64(RefBox.H) * scaleY),
	}
	fmt.Printf("[CALIBRATION] Using default (scaled to %dx%d): (%d,%d,%d,%d)\n",
		screenW, screenH, scaled.X, scaled.Y, scaled.W, scaled.H)
	return scaled, false
}

// SaveCalibration saves calibrated box for future sessions
func SaveCalibration(dataDir string, screenW, screenH int, box Box) error {
	cal := Calibration{
		ScreenW: screenW,
		ScreenH: screenH,
		Box:     box,
	}

	data, err := json.MarshalIndent(cal, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("mkdir failed: %w", err)
	}

	calFile := filepath.Join(dataDir, "calibration.json")
	if err := os.WriteFile(calFile, data, 0644); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	fmt.Printf("[CALIBRATION] Saved box=(%d,%d,%d,%d) for %dx%d\n",
		box.X, box.Y, box.W, box.H, screenW, screenH)
	return nil
}
