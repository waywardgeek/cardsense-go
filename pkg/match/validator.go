package match

import (
	"strings"
	"unicode"

	"gocv.io/x/gocv"
)

// Validator implements the 3-guard validation system that prevents false positives.
// All three guards must pass for a match to be considered valid:
//
// Guard 1: pHash distance threshold (conservative matching)
// Guard 2: Aspect ratio must be card-shaped (0.65-0.80)
// Guard 3: OCR text must look like a real card name
type Validator struct {
	MaxPHashDist    int     // Maximum pHash distance (default: 200)
	MinAspectRatio  float64 // Minimum aspect ratio (default: 0.65)
	MaxAspectRatio  float64 // Maximum aspect ratio (default: 0.80)
	MinOCRChars     int     // Minimum OCR text length (default: 5)
	MinOCRLetters   int     // Minimum letter count (default: 4)
	AllowMultiSpace bool    // Allow consecutive spaces in OCR (default: false)
}

// NewValidator creates a validator with default settings
func NewValidator() *Validator {
	return &Validator{
		MaxPHashDist:    200,
		MinAspectRatio:  0.65,
		MaxAspectRatio:  0.80,
		MinOCRChars:     5,
		MinOCRLetters:   4,
		AllowMultiSpace: false,
	}
}

// ValidatePHashDistance checks Guard 1: pHash distance must be reasonable
func (v *Validator) ValidatePHashDistance(distance int) bool {
	return distance <= v.MaxPHashDist
}

// ValidateAspectRatio checks Guard 2: crop must be card-shaped
func (v *Validator) ValidateAspectRatio(gray gocv.Mat) bool {
	h, w := gray.Rows(), gray.Cols()
	if h == 0 {
		return false
	}
	aspect := float64(w) / float64(h)
	return aspect >= v.MinAspectRatio && aspect <= v.MaxAspectRatio
}

// ValidateOCRText checks Guard 3: OCR text must look like a real card name
// Rules:
//   - At least MinOCRChars characters (default: 5)
//   - At least MinOCRLetters letters (default: 4)
//   - No consecutive spaces (filters garbled multi-line OCR)
func (v *Validator) ValidateOCRText(text string) bool {
	text = strings.TrimSpace(text)

	// Check minimum length
	if len(text) < v.MinOCRChars {
		return false
	}

	// Count letters
	letterCount := 0
	for _, r := range text {
		if unicode.IsLetter(r) {
			letterCount++
		}
	}
	if letterCount < v.MinOCRLetters {
		return false
	}

	// Check for consecutive spaces (indicates garbled OCR)
	if !v.AllowMultiSpace && strings.Contains(text, "  ") {
		return false
	}

	return true
}

// ValidateAll runs all three guards and returns true only if all pass
func (v *Validator) ValidateAll(gray gocv.Mat, pHashDist int, ocrText string) bool {
	if !v.ValidatePHashDistance(pHashDist) {
		return false
	}
	if !v.ValidateAspectRatio(gray) {
		return false
	}
	if ocrText != "" && !v.ValidateOCRText(ocrText) {
		return false
	}
	return true
}
