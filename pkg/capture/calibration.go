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
// scale factor anchored at the screen ORIGIN (top-left).
//
// Two things this gets right that the previous versions did not:
//
//  1. UNIFORM scale. Using independent scaleX/scaleY distorts the crop whenever
//     the source and destination aspect ratios differ (a 16:9 -> 16:10 move
//     squashed the box to aspect 0.537). A distorted crop can never match a
//     card's pHash. min() preserves card proportions, and also matches how MTGA
//     scales its UI: with window HEIGHT.
//
//  2. ORIGIN anchoring. The hover preview is pinned to the LEFT edge of the
//     screen, so its offset scales from the origin, not from the center.
//     Center-anchored scaling pushed x from 15 to 144 going 1470->1920 and
//     missed the card completely.
func scaleBox(b Box, fromW, fromH, toW, toH int) Box {
	scale := float64(toW) / float64(fromW)
	if s := float64(toH) / float64(fromH); s < scale {
		scale = s
	}

	return Box{
		X: int(float64(b.X) * scale),
		Y: int(float64(b.Y) * scale),
		W: int(float64(b.W) * scale),
		H: int(float64(b.H) * scale),
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

// DefaultBox returns the reference box scaled to the given screen, ignoring any
// saved calibration. Used by an explicit "Recalibrate" request so the search
// restarts from known-good geometry rather than from a possibly-bad saved box.
func DefaultBox(screenW, screenH int) Box {
	return scaleBox(RefBox, RefScreen.W, RefScreen.H, screenW, screenH)
}

// ClearCalibration deletes any saved calibration. Returns nil if none existed.
func ClearCalibration(dataDir string) error {
	calFile := filepath.Join(dataDir, "calibration.json")
	err := os.Remove(calFile)
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
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
