package hash

import (
	"archive/zip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gocv.io/x/gocv"
)

// CardMeta holds metadata for a single card
type CardMeta struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	TypeLine    string `json:"type_line"`
	ManaCost    string `json:"mana_cost"`
	OracleText  string `json:"oracle_text"`
	OCRFallback bool   `json:"ocr_fallback,omitempty"`
	OCRText     string `json:"ocr_text,omitempty"`
}

// CardIndex holds the loaded pHash index + metadata
type CardIndex struct {
	Bits  [][]byte   // [N][64] byte hashes
	IDs   []string   // [N] card IDs
	Meta  []CardMeta // [N] card metadata
	Names []string   // [N] card names (cached for fast lookup)
}

// IdentifyResult holds the result of a card identification
type IdentifyResult struct {
	Meta   CardMeta
	Dist   int
	Margin int
}

// NumCards returns the number of cards in the index
func (idx *CardIndex) NumCards() int {
	return len(idx.Bits)
}

// DataDir returns the data directory path (bundle-aware for deployed apps)
func DataDir() string {
	// Check if running from .app bundle
	execPath, err := os.Executable()
	if err != nil {
		// Fallback to Python's data directory (development mode)
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, "projects", "cardsense", "hashindex", "data")
	}
	
	// Resolve symlinks (macOS may symlink executables)
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, "projects", "cardsense", "hashindex", "data")
	}
	
	// Check if we're in a .app bundle (path ends with .app/Contents/MacOS/cardsense-gui)
	if strings.Contains(execPath, ".app/Contents/MacOS/") {
		// Running from bundle: use bundled Resources/data/
		execDir := filepath.Dir(execPath)
		return filepath.Join(execDir, "..", "Resources", "data")
	}
	
	// Development mode: use Python's data directory
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "projects", "cardsense", "hashindex", "data")
}

// LoadCardIndex loads the pHash index from .npz and .json files
func LoadCardIndex(dataDir string) (*CardIndex, error) {
	if dataDir == "" {
		dataDir = DataDir()
	}

	// Load .npz file (it's a ZIP archive containing .npy files)
	npzPath := filepath.Join(dataDir, "phash_index.npz")
	bits, ids, err := loadNPZ(npzPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load %s: %w", npzPath, err)
	}

	// Load metadata JSON
	metaPath := filepath.Join(dataDir, "phash_meta.json")
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", metaPath, err)
	}

	var meta []CardMeta
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	if len(meta) != len(bits) {
		return nil, fmt.Errorf("mismatch: %d hashes but %d metadata entries", len(bits), len(meta))
	}

	// Cache names for fast lookup
	names := make([]string, len(meta))
	for i, m := range meta {
		names[i] = m.Name
	}

	return &CardIndex{
		Bits:  bits,
		IDs:   ids,
		Meta:  meta,
		Names: names,
	}, nil
}

// loadNPZ reads a .npz file (ZIP archive of .npy files) and extracts bits and ids
func loadNPZ(path string) ([][]byte, []string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open zip: %w", err)
	}
	defer r.Close()

	var bitsFlat []uint8
	var ids []string

	// Find and read "bits.npy" and "ids.npy" from the ZIP
	for _, f := range r.File {
		if f.Name == "bits.npy" {
			bitsFlat, err = readNPY(f)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to read bits.npy: %w", err)
			}
		} else if f.Name == "ids.npy" {
			// ids.npy contains strings - for now, skip proper string parsing
			// and assume it's UTF-8 encoded strings with length prefixes
			// TODO: Implement proper NPY string array parsing
			fmt.Println("[WARNING] ids.npy parsing not fully implemented, using placeholder")
		}
	}

	if bitsFlat == nil {
		return nil, nil, fmt.Errorf("bits.npy not found in archive")
	}

	// Reshape bits from flat array to [][]byte
	if len(bitsFlat)%DualBytes != 0 {
		return nil, nil, fmt.Errorf("bits array size %d not divisible by %d", len(bitsFlat), DualBytes)
	}
	n := len(bitsFlat) / DualBytes
	bits := make([][]byte, n)
	for i := 0; i < n; i++ {
		bits[i] = bitsFlat[i*DualBytes : (i+1)*DualBytes]
	}

	// Generate placeholder IDs if we couldn't parse them
	if len(ids) == 0 {
		ids = make([]string, n)
		for i := range ids {
			ids[i] = fmt.Sprintf("card_%d", i)
		}
	}

	return bits, ids, nil
}

// readNPY reads a .npy file from a ZIP entry and returns the data as []uint8
func readNPY(f *zip.File) ([]uint8, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	// NPY format: magic "\x93NUMPY" (6 bytes) + version (2 bytes) + header_len (2 or 4 bytes) + header (JSON) + data
	magic := make([]byte, 6)
	if _, err := io.ReadFull(rc, magic); err != nil {
		return nil, fmt.Errorf("failed to read magic: %w", err)
	}
	if string(magic) != "\x93NUMPY" {
		return nil, fmt.Errorf("invalid NPY magic: %v", magic)
	}

	// Read version (major, minor)
	version := make([]byte, 2)
	if _, err := io.ReadFull(rc, version); err != nil {
		return nil, fmt.Errorf("failed to read version: %w", err)
	}

	// Read header length (depends on version)
	var headerLen uint32
	if version[0] == 1 {
		var hl uint16
		if err := binary.Read(rc, binary.LittleEndian, &hl); err != nil {
			return nil, fmt.Errorf("failed to read header len: %w", err)
		}
		headerLen = uint32(hl)
	} else {
		if err := binary.Read(rc, binary.LittleEndian, &headerLen); err != nil {
			return nil, fmt.Errorf("failed to read header len: %w", err)
		}
	}

	// Skip header (we don't need to parse it for uint8 arrays)
	if _, err := io.CopyN(io.Discard, rc, int64(headerLen)); err != nil {
		return nil, fmt.Errorf("failed to skip header: %w", err)
	}

	// Read remaining data
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	return data, nil
}

// Len returns the number of cards in the index
func (idx *CardIndex) Len() int {
	return len(idx.Meta)
}

// Identify attempts to identify a card crop.
// Returns IdentifyResult on success, nil on failure.
//
// Strategy:
// 1. Try pHash first (fast, works for most cards)
// 2. If no match or low confidence, OCR fallback (not yet implemented in Go)
//
// Args:
//   gray: grayscale card image
//   sweep: enable alignment variants (default: true)
//   maxDist: maximum pHash distance for a match (default: 280)
//   minMargin: minimum margin over second-best match (default: 20)
//   borderTrim: optional (left, top, right, bottom) trim
//   ocrFallback: enable OCR fallback (TODO: implement)
func (idx *CardIndex) Identify(gray gocv.Mat, sweep bool, maxDist int, minMargin int, borderTrim *[4]int, ocrFallback bool) *IdentifyResult {
	// Default parameters
	if maxDist == 0 {
		maxDist = 280
	}
	if minMargin == 0 {
		minMargin = 20
	}

	// Try all alignment variants, keep best distances
	variants := AlignVariants(gray, sweep, borderTrim)
	defer func() {
		// Close all variants except the first (which is the original or a reference)
		for i := 1; i < len(variants); i++ {
			variants[i].Close()
		}
	}()

	var best []int
	for _, v := range variants {
		hashBytes := DualPHash(v, nil) // borderTrim already applied in AlignVariants
		distances := HammingScan(hashBytes, idx.Bits)
		if best == nil {
			best = distances
		} else {
			// Keep minimum distance per card
			for i := range best {
				if distances[i] < best[i] {
					best[i] = distances[i]
				}
			}
		}
	}

	// Find best match
	type match struct {
		idx  int
		dist int
	}
	matches := make([]match, len(best))
	for i, d := range best {
		matches[i] = match{i, d}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].dist < matches[j].dist
	})

	topIdx := matches[0].idx
	topName := idx.Names[topIdx]
	topDist := matches[0].dist

	// Find margin (distance to next different name)
	runnerDist := -1
	for i := 1; i < len(matches); i++ {
		if idx.Names[matches[i].idx] != topName {
			runnerDist = matches[i].dist
			break
		}
	}

	margin := 1_000_000_000
	if runnerDist >= 0 {
		margin = runnerDist - topDist
	}

	// Check if pHash gives confident match
	if topDist <= maxDist && margin >= minMargin {
		result := idx.Meta[topIdx]
		result.OCRFallback = false
		return &IdentifyResult{
			Meta:   result,
			Dist:   topDist,
			Margin: margin,
		}
	}

	// pHash failed or low confidence - try OCR fallback
	if !ocrFallback {
		return nil
	}

	// Guard 1: pHash distance must be reasonable (not random noise)
	// Tightened from 300 to 200 to reduce false positives
	if topDist > 200 {
		return nil
	}
	
	// Guard 1b: pHash margin must not be too low (prevents multi-card captures)
	// If pHash can't distinguish between cards (low margin), OCR is likely reading
	// random text from a board state rather than a single card
	if margin < 10 {
		return nil
	}

	// Guard 2: Aspect ratio must be card-like (0.60-0.85)
	// Widened from 0.65-0.80 to handle slight variations in screen capture
	h := gray.Rows()
	w := gray.Cols()
	aspect := float64(w) / float64(h)
	if aspect < 0.60 || aspect > 0.85 {
		return nil
	}

	// Try OCR extraction
	cardName := ocrCardName(gray)
	if cardName == "" {
		return nil
	}

	// Guard 3: OCR text must look like a real card name
	if !validateOCRText(cardName) {
		return nil
	}

	// Query Scryfall
	ocrMeta := queryScryfall(cardName)
	if ocrMeta == nil {
		return nil
	}

	// Success! Return with synthetic dist/margin to indicate OCR was used
	ocrMeta.OCRText = cardName
	return &IdentifyResult{
		Meta:   *ocrMeta,
		Dist:   0,   // Synthetic: indicates OCR path
		Margin: 999, // Synthetic: high confidence
	}

}

// Describe generates a human-readable description of a card
func Describe(meta CardMeta) string {
	parts := []string{meta.Name}
	if meta.TypeLine != "" {
		parts = append(parts, meta.TypeLine)
	}
	if meta.OracleText != "" {
		parts = append(parts, meta.OracleText)
	}
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ". "
		}
		result += p
	}
	return result
}
