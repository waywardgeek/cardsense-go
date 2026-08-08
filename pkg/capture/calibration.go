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

// Reference calibration: measured directly from a verified-good crop on Bill's
// MacBook panel (1470×956), confirmed pixel-exact against the MTGA hover
// preview for "Tatyova, Benthic Druid" on 2026-08-07.
//
// The previous reference was (21,47,449,709) @ 1920×1080, aspect 0.633. That
// was ~80px too TALL: a real Magic card is 63×88mm = 0.716 aspect, and this box
// is 396/556 = 0.712. The stale height meant every crop included a strip of
// non-card below the frame, so pHash distance floored out around 188-198 with
// near-zero margin on EVERY display, not just laptops. Scaling this box up to
// 1920×1080 yields width 447 vs the old 449 — the old width was right, which is
// why the error hid for so long.
//
// The MTGA preview scales with window HEIGHT, which uniform (min) scaling in
// scaleBox reproduces correctly for both wider and narrower displays.
var (
	RefScreen = struct{ W, H int }{1470, 956}
	RefBox    = Box{13, 100, 396, 556}
)

// scaleBox maps a box from one screen resolution to another using a UNIFORM
// scale factor, preserving the box's aspect ratio.
//
// Using independent scaleX/scaleY (the old behavior) distorts the crop whenever
// the source and destination displays have different aspect ratios — e.g. a
// 16:9 4K external vs a 16:10 MacBook panel. A distorted crop can never match a
// card's pHash, and the twiddle refinement only nudges by 1% so it cannot
// recover the shape. Uniform scale keeps the card proportions correct; the
// box's position is scaled about the screen center so it tracks the MTGA
// window's relative placement.
func scaleBox(b Box, fromW, fromH, toW, toH int) Box {
	scale := float64(toW) / float64(fromW)
	if s := float64(toH) / float64(fromH); s < scale {
		scale = s
	}

	// Scale the box's center offset from the screen center, then re-derive the
	// origin from the scaled size. This keeps a centered box centered and a
	// left-anchored box proportionally left-anchored.
	cx := float64(b.X) + float64(b.W)/2
	cy := float64(b.Y) + float64(b.H)/2
	newCx := float64(toW)/2 + (cx-float64(fromW)/2)*scale
	newCy := float64(toH)/2 + (cy-float64(fromH)/2)*scale

	w := int(float64(b.W) * scale)
	h := int(float64(b.H) * scale)
	return Box{
		X: int(newCx - float64(w)/2),
		Y: int(newCy - float64(h)/2),
		W: w,
		H: h,
	}
}

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

			// Scale proportionally (uniform, aspect-preserving)
			scaled := scaleBox(cal.Box, cal.ScreenW, cal.ScreenH, screenW, screenH)
			fmt.Printf("[CALIBRATION] Loaded + scaled: %dx%d → %dx%d, box=(%d,%d,%d,%d)\n",
				cal.ScreenW, cal.ScreenH, screenW, screenH,
				scaled.X, scaled.Y, scaled.W, scaled.H)
			return scaled, true
		}
		fmt.Printf("[CALIBRATION] Load failed: %v, using default\n", err)
	}

	// Fall back to reference box, scaled to current screen (uniform)
	scaled := scaleBox(RefBox, RefScreen.W, RefScreen.H, screenW, screenH)
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
