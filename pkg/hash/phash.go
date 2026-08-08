package hash

import (
	"image"
	"math"

	"gocv.io/x/gocv"
)

const (
	// HashSize is the DCT coefficient grid size (16x16 -> 256 bits)
	HashSize = 16
	// ImgSize is the pre-DCT resize target (64x64)
	ImgSize = HashSize * 4
	// CW and CH are canonical card dimensions for region crops
	CW = 200
	CH = 280
	// DualBytes is the total hash size (32 full + 32 art)
	DualBytes = 64
)

// ArtBox defines the art region as fractions of card dimensions
// (y0, y1, x0, x1) = (0.11, 0.56, 0.06, 0.94)
// This matches the SCRYFALL printed-card layout, which is what the index is
// built from. Do NOT use it for live MTGA captures -- see QueryArtBox.
var ArtBox = struct{ Y0, Y1, X0, X1 float64 }{0.11, 0.56, 0.06, 0.94}

// QueryArtBox is the art region for LIVE MTGA CAPTURES.
//
// MTGA does not display the printed card scan; it re-renders the card with its
// own layout. The art window sits HIGHER and is TALLER than on the printed
// card, and there is no outer black border. Hashing a live capture with the
// Scryfall fractions therefore compares two DIFFERENT PHYSICAL REGIONS, which
// makes the art half of the dual hash pure noise.
//
// Measured on a live Badgermole Cub capture vs its (correct, dist=0) index art:
//
//	art region with ArtBox      fractions (0.11..0.56): 130/256  <- random
//	art region with QueryArtBox fractions (0.05..0.52):  36/256  <- real match
//
// Found by sweeping y0/y1 for the live crop against the Scryfall art region.
var QueryArtBox = struct{ Y0, Y1, X0, X1 float64 }{0.05, 0.52, 0.04, 0.96}

// precomputedPopCount contains bit counts for all byte values 0-255
var precomputedPopCount = makePopCountTable()

func makePopCountTable() [256]uint16 {
	var table [256]uint16
	for i := 0; i < 256; i++ {
		count := uint16(0)
		for j := 0; j < 8; j++ {
			if (i & (1 << j)) != 0 {
				count++
			}
		}
		table[i] = count
	}
	return table
}

// phash256 computes a 256-bit DCT pHash from a grayscale image.
// Returns 32 bytes (256 bits).
func phash256(gray gocv.Mat) []byte {
	// Resize to ImgSize x ImgSize
	small := gocv.NewMat()
	defer small.Close()
	gocv.Resize(gray, &small, image.Pt(ImgSize, ImgSize), 0, 0, gocv.InterpolationArea)

	// Convert to float32 for DCT
	smallFloat := gocv.NewMat()
	defer smallFloat.Close()
	small.ConvertTo(&smallFloat, gocv.MatTypeCV32F)

	// Compute DCT
	dct := gocv.NewMat()
	defer dct.Close()
	gocv.DCT(smallFloat, &dct, 0)

	// Extract top-left HashSize x HashSize coefficients (must copy, not just view)
	coeffsRegion := dct.Region(image.Rect(0, 0, HashSize, HashSize))
	coeffs := gocv.NewMat()
	coeffsRegion.CopyTo(&coeffs)
	coeffsRegion.Close()
	defer coeffs.Close()

	// Compute median
	coeffData, _ := coeffs.DataPtrFloat32()
	median := computeMedian(coeffData)

	// Pack bits: coefficient > median -> 1, else -> 0
	// NumPy's packbits packs MSB first: bit 0 goes to position 7, bit 7 goes to position 0
	bits := make([]byte, 32) // 256 bits / 8 = 32 bytes
	for i := 0; i < HashSize*HashSize; i++ {
		if coeffData[i] > median {
			byteIdx := i / 8
			bitIdx := uint(7 - (i % 8)) // MSB first (bit 0 -> position 7)
			bits[byteIdx] |= 1 << bitIdx
		}
	}

	return bits
}

// DualPHash computes the dual 512-bit pHash: full image (256 bits) + art box (256 bits).
// borderTrim is optional (left, top, right, bottom) pixels to remove before hashing.
// DualPHash computes the dual hash using the SCRYFALL art fractions (ArtBox).
// Use this for INDEX BUILDING and for hashing Scryfall reference images.
func DualPHash(gray gocv.Mat, borderTrim *[4]int) []byte {
	return dualPHashWith(gray, borderTrim, ArtBox.X0, ArtBox.Y0, ArtBox.X1, ArtBox.Y1)
}

// DualPHashQuery computes the dual hash using the MTGA art fractions
// (QueryArtBox). Use this for LIVE SCREEN CAPTURES so that the art half of the
// hash covers the same physical artwork as the index entries.
func DualPHashQuery(gray gocv.Mat, borderTrim *[4]int) []byte {
	return dualPHashWith(gray, borderTrim, QueryArtBox.X0, QueryArtBox.Y0, QueryArtBox.X1, QueryArtBox.Y1)
}

func dualPHashWith(gray gocv.Mat, borderTrim *[4]int, ax0, ay0, ax1, ay1 float64) []byte {
	// Apply border trim if specified
	working := gray
	if borderTrim != nil {
		left, top, right, bottom := borderTrim[0], borderTrim[1], borderTrim[2], borderTrim[3]
		h, w := gray.Rows(), gray.Cols()
		rect := image.Rect(left, top, w-right, h-bottom)
		working = gray.Region(rect)
		defer working.Close()
	}

	// Resize to canonical card size (for art box extraction)
	canonical := gocv.NewMat()
	defer canonical.Close()
	gocv.Resize(working, &canonical, image.Pt(CW, CH), 0, 0, gocv.InterpolationArea)

	// Hash ORIGINAL full card (not the canonical resize)
	fullHash := phash256(working)

	// Extract art box from canonical
	y0 := int(ay0 * CH)
	y1 := int(ay1 * CH)
	x0 := int(ax0 * CW)
	x1 := int(ax1 * CW)
	artRect := image.Rect(x0, y0, x1, y1)
	artRegion := canonical.Region(artRect)
	artBox := gocv.NewMat()
	artRegion.CopyTo(&artBox)
	artRegion.Close()
	defer artBox.Close()

	// Hash art box
	artHash := phash256(artBox)

	// Concatenate: full ++ art
	result := make([]byte, DualBytes)
	copy(result[0:32], fullHash)
	copy(result[32:64], artHash)

	return result
}

// HammingScan computes Hamming distances between a query hash and an array of index hashes.
// query: 64-byte hash
// index: slice of 64-byte hashes
// Returns slice of Hamming distances (bit count differences)
func HammingScan(query []byte, index [][]byte) []int {
	distances := make([]int, len(index))
	for i, indexHash := range index {
		dist := 0
		for j := 0; j < DualBytes; j++ {
			xor := query[j] ^ indexHash[j]
			dist += int(precomputedPopCount[xor])
		}
		distances[i] = dist
	}
	return distances
}

// computeMedian computes the median of a float32 slice.
func computeMedian(data []float32) float32 {
	if len(data) == 0 {
		return 0
	}

	// Copy and sort
	sorted := make([]float32, len(data))
	copy(sorted, data)
	quickSort(sorted)

	// Return median
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

// quickSort sorts float32 slice in place
func quickSort(arr []float32) {
	if len(arr) < 2 {
		return
	}
	left, right := 0, len(arr)-1
	pivot := len(arr) / 2
	arr[pivot], arr[right] = arr[right], arr[pivot]
	for i := range arr {
		if arr[i] < arr[right] {
			arr[i], arr[left] = arr[left], arr[i]
			left++
		}
	}
	arr[left], arr[right] = arr[right], arr[left]
	quickSort(arr[:left])
	quickSort(arr[left+1:])
}

// AlignVariants generates crop variants to absorb border misalignment.
// Yields the original crop, then progressively inward-cropped variants.
func AlignVariants(gray gocv.Mat, sweep bool, borderTrim *[4]int) []gocv.Mat {
	variants := []gocv.Mat{}

	// First variant: with optional border trim
	if borderTrim != nil {
		left, top, right, bottom := borderTrim[0], borderTrim[1], borderTrim[2], borderTrim[3]
		h, w := gray.Rows(), gray.Cols()
		rect := image.Rect(left, top, w-right, h-bottom)
		trimmed := gray.Region(rect)
		variants = append(variants, trimmed)
	} else {
		variants = append(variants, gray)
	}

	if !sweep {
		return variants
	}

	// Additional variants: inward crops at 3% and 6%
	h, w := gray.Rows(), gray.Cols()
	for _, dz := range []float64{0.03, 0.06} {
		m := int(math.Min(float64(h), float64(w)) * dz)
		if m > 0 && h-2*m > 10 && w-2*m > 10 {
			rect := image.Rect(m, m, w-m, h-m)
			cropped := gray.Region(rect)

			// Apply border trim if specified
			if borderTrim != nil {
				left, top, right, bottom := borderTrim[0], borderTrim[1], borderTrim[2], borderTrim[3]
				hc, wc := cropped.Rows(), cropped.Cols()
				rect2 := image.Rect(left, top, wc-right, hc-bottom)
				croppedTrimmed := cropped.Region(rect2)
				cropped.Close()
				variants = append(variants, croppedTrimmed)
			} else {
				variants = append(variants, cropped)
			}
		}
	}

	return variants
}
