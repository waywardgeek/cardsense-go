package hash

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"gocv.io/x/gocv"
)

// ocrCardName extracts card name from title region via OCR.
// Returns card name string or empty string if OCR failed.
// Fast (~70ms) and works even when pHash fails due to MTGA vs Scryfall rendering.
func ocrCardName(grayCrop gocv.Mat) string {
	if grayCrop.Empty() {
		return ""
	}

	h := grayCrop.Rows()
	w := grayCrop.Cols()

	// Extract title region (top 5-14% of card, avoiding borders)
	titleY1 := int(float64(h) * 0.05)
	titleY2 := int(float64(h) * 0.14)
	titleX1 := int(float64(w) * 0.05)
	titleX2 := int(float64(w) * 0.95)

	// Validate bounds
	if titleY2 <= titleY1 || titleX2 <= titleX1 {
		return ""
	}

	title := grayCrop.Region(image.Rect(titleX1, titleY1, titleX2, titleY2))
	defer title.Close()

	// Preprocess for OCR: invert (white text on dark → black on white)
	inverted := gocv.NewMat()
	defer inverted.Close()
	gocv.BitwiseNot(title, &inverted)

	// Threshold to clean up (Otsu's method)
	binary := gocv.NewMat()
	defer binary.Close()
	gocv.Threshold(inverted, &binary, 0, 255, gocv.ThresholdBinary|gocv.ThresholdOtsu)

	// Upscale 3x for better OCR
	scaled := gocv.NewMat()
	defer scaled.Close()
	gocv.Resize(binary, &scaled, image.Point{}, 3.0, 3.0, gocv.InterpolationCubic)

	// Save to temp file for tesseract
	tmpfile, err := os.CreateTemp("", "cardsense-ocr-*.png")
	if err != nil {
		return ""
	}
	tmpPath := tmpfile.Name()
	defer os.Remove(tmpPath)
	tmpfile.Close()

	if !gocv.IMWrite(tmpPath, scaled) {
		return ""
	}

	// Run tesseract command
	// PSM 7 = single line of text
	// Whitelist = letters + space only
	cmd := exec.Command("tesseract", tmpPath, "stdout",
		"--psm", "7",
		"-c", "tessedit_char_whitelist=ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz ")

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = nil // Ignore errors

	if err := cmd.Run(); err != nil {
		return ""
	}

	// Clean and return
	cleaned := strings.TrimSpace(outBuf.String())
	return cleaned
}

// plausibleMatch reports whether Scryfall's fuzzy answer is close enough to the
// text OCR actually read to be trusted.
//
// Scryfall will happily map arbitrary noise onto a real card name. Requiring a
// character-level resemblance keeps genuine OCR near-misses (dropped/î
// substituted letters) while rejecting invented matches:
//
//	'Octoprophef'  -> 'Octoprophet'         ACCEPT (ratio ~0.9)
//	'Badgermole C' -> 'Badgermole Cub'      ACCEPT (prefix)
//	'Wg ZAM'       -> 'Merrow Grimeblotter' REJECT
//	'Wise'         -> "Voyage's End"        REJECT
func plausibleMatch(ocrText, cardName string) bool {
	norm := func(s string) string {
		var b strings.Builder
		for _, r := range strings.ToLower(s) {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				b.WriteRune(r)
			}
		}
		return b.String()
	}

	a, b := norm(ocrText), norm(cardName)
	if len(a) < 4 || len(b) == 0 {
		// Too little signal to verify. 'Wise' (4 chars) matching a long name is
		// exactly the false-positive shape, so demand real length.
		return false
	}

	// Accept a clean prefix relationship (OCR truncated the title).
	if strings.HasPrefix(b, a) || strings.HasPrefix(a, b) {
		return true
	}

	// Otherwise require a decent character-subsequence overlap: walk b looking
	// for a's characters in order. This tolerates substitutions and dropped
	// letters without accepting unrelated words.
	matched, j := 0, 0
	for i := 0; i < len(a) && j < len(b); i++ {
		for j < len(b) && b[j] != a[i] {
			j++
		}
		if j < len(b) {
			matched++
			j++
		}
	}

	longer := len(a)
	if len(b) > longer {
		longer = len(b)
	}
	return float64(matched)/float64(longer) >= 0.6
}

// queryScryfall queries Scryfall API for card by fuzzy name match.
// Returns CardMeta or nil if not found.
// Scryfall's fuzzy search is very forgiving of OCR errors.
func queryScryfall(cardName string) *CardMeta {
	if cardName == "" {
		return nil
	}

	// Build URL with fuzzy search
	baseURL := "https://api.scryfall.com/cards/named"
	params := url.Values{}
	params.Add("fuzzy", cardName)

	fullURL := baseURL + "?" + params.Encode()

	// Create request
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "cardsense/1.0")
	req.Header.Set("Accept", "application/json")

	// Execute request with timeout
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode != 200 {
		return nil
	}

	// Parse JSON response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil
	}

	// Extract fields
	meta := &CardMeta{
		OCRFallback: true,
	}

	if id, ok := result["id"].(string); ok {
		meta.ID = id
	}
	if name, ok := result["name"].(string); ok {
		// Scryfall's fuzzy search is EXTREMELY forgiving -- that is useful for
		// real OCR noise ('Octoprophef' -> 'Octoprophet') but dangerous for
		// garbage: 'Wg ZAM' resolved to 'Merrow Grimeblotter' and was spoken as
		// a confident answer. For an accessibility tool a wrong name is the
		// worst outcome, because the user cannot tell it is wrong.
		//
		// So verify the answer resembles what we actually read. This keeps the
		// forgiveness for near-misses while rejecting invented matches.
		if !plausibleMatch(cardName, name) {
			fmt.Printf("[DEBUG] OCR REJECTED: Scryfall returned %q for OCR text %q (too dissimilar)\n", name, cardName)
			return nil
		}
		meta.Name = name
	}
	if typeLine, ok := result["type_line"].(string); ok {
		meta.TypeLine = typeLine
	}
	if manaCost, ok := result["mana_cost"].(string); ok {
		meta.ManaCost = manaCost
	}
	if oracleText, ok := result["oracle_text"].(string); ok {
		meta.OracleText = oracleText
	}

	// Guard: Must have at least a name to be valid
	if meta.Name == "" {
		return nil
	}

	return meta
}

// validateOCRText checks if OCR text looks like a real card name.
// Guards against false positives from terminal text, UI elements, etc.
func validateOCRText(text string) bool {
	// Guard 3a: At least 5 characters (e.g., "Loki")
	if len(text) < 5 {
		return false
	}

	// Guard 3b: At least 4 letters (filters "d=0 m=999" style text)
	letterCount := 0
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			letterCount++
		}
	}
	if letterCount < 4 {
		return false
	}

	// Guard 3c: No more than 2 consecutive spaces (filters garbled multi-line OCR)
	if strings.Contains(text, "  ") {
		return false
	}

	return true
}
