package capture

import (
	"fmt"
	"image"

	"github.com/waywardgeek/cardsense-go/pkg/hash"
	"gocv.io/x/gocv"
)

// Twiddle refines the crop box to minimize pHash distance.
// Uses hill-climbing with two passes: coarse (1%) and fine (0.1%).
// Returns (refined_box, best_distance, margin).
func Twiddle(shot gocv.Mat, box Box, idx *hash.CardIndex) (Box, int, int) {
	hMax, wMax := shot.Rows(), shot.Cols()

	// Score function: compute minimum pHash distance for a given box
	score := func(bx, by, bw, bh int) int {
		// Validate bounds
		if bx < 0 || by < 0 || bw < 20 || bh < 20 {
			return 9999
		}
		if bx+bw > wMax || by+bh > hMax {
			return 9999
		}

		// Extract crop and convert to grayscale
		rect := shot.Region(image.Rect(bx, by, bx+bw, by+bh))
		defer rect.Close()
		
		gray := gocv.NewMat()
		defer gray.Close()
		gocv.CvtColor(rect, &gray, gocv.ColorBGRToGray)

		// Compute pHash
		hashBytes := hash.DualPHash(gray, nil)

		// Scan index and find minimum distance
		distances := hash.HammingScan(hashBytes, idx.Bits)
		minDist := distances[0]
		for _, d := range distances[1:] {
			if d < minDist {
				minDist = d
			}
		}
		return minDist
	}

	x, y, w, h := box.X, box.Y, box.W, box.H
	bestScore := score(x, y, w, h)

	// Two-pass hill climbing: coarse (1%) then fine (0.1%)
	for _, pct := range []float64{0.08, 0.03, 0.01, 0.001} {
		improved := true
		rounds := 0

		for improved && rounds < 50 {
			improved = false
			rounds++

			// Try adjusting each dimension (x, y, w, h) in both directions
			for dim := 0; dim < 4; dim++ {
				for _, sign := range []int{1, -1} {
					trial := []int{x, y, w, h}
					step := max(1, int(float64(trial[dim])*pct))
					trial[dim] += sign * step

					s := score(trial[0], trial[1], trial[2], trial[3])
					if s <= bestScore {
						x, y, w, h = trial[0], trial[1], trial[2], trial[3]
						bestScore = s
						improved = true
					}
				}
			}
		}
	}

	fmt.Printf("[TWIDDLE] done box=(%d,%d,%d,%d) dist=%d\n", x, y, w, h, bestScore)

	// Get margin for the final box
	finalBox := Box{x, y, w, h}
	rect := shot.Region(image.Rect(x, y, x+w, y+h))
	defer rect.Close()
	
	gray := gocv.NewMat()
	defer gray.Close()
	gocv.CvtColor(rect, &gray, gocv.ColorBGRToGray)

	result := idx.Identify(gray, true, 9999, 0, nil, false)
	margin := 0
	if result != nil {
		margin = result.Margin
	}

	return finalBox, bestScore, margin
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
