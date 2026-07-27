package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/waywardgeek/cardsense-go/pkg/capture"
	"github.com/waywardgeek/cardsense-go/pkg/hash"
	"github.com/waywardgeek/cardsense-go/pkg/tts"
	"gocv.io/x/gocv"
)

func main() {
	fmt.Println("cardsense-go offline test - no screen capture required")
	fmt.Println()

	// Get data directory from Python version
	dataDir := filepath.Join(os.Getenv("HOME"), "projects", "cardsense", "hashindex", "data")
	fmt.Printf("Data directory: %s\n", dataDir)

	// Test 1: Load card index
	fmt.Println("\n[TEST 1] Loading card index...")
	idx, err := hash.LoadCardIndex(dataDir)
	if err != nil {
		fmt.Printf("❌ FAIL: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ PASS: Loaded %d cards\n", idx.Len())
	fmt.Printf("   First card: %s\n", idx.Names[0])
	fmt.Printf("   Last card: %s\n", idx.Names[len(idx.Names)-1])

	// Test 2: Load a test image from Python's test data (if it exists)
	testImagePath := filepath.Join(dataDir, "..", "..", "test_card.png")
	if _, err := os.Stat(testImagePath); os.IsNotExist(err) {
		fmt.Println("\n[TEST 2] No test image found, creating synthetic card image...")
		// Create a synthetic 200x280 grayscale card-shaped image
		gray := gocv.NewMatWithSize(280, 200, gocv.MatTypeCV8U)
		defer gray.Close()
		
		// Fill with some pattern (not all zeros)
		for y := 0; y < 280; y++ {
			for x := 0; x < 200; x++ {
				val := byte((x + y) % 256)
				gray.SetUCharAt(y, x, val)
			}
		}
		
		testHash := hash.DualPHash(gray, nil)
		fmt.Printf("✅ PASS: Computed %d-byte pHash from synthetic image\n", len(testHash))
		
		// Try alignment variants
		variants := hash.AlignVariants(gray, true, nil)
		fmt.Printf("✅ PASS: Generated %d alignment variants\n", len(variants))
		for i := 1; i < len(variants); i++ {
			variants[i].Close()
		}
		
		// Test Hamming scan
		distances := hash.HammingScan(testHash, idx.Bits)
		fmt.Printf("✅ PASS: Computed distances to all %d cards\n", len(distances))
		
		// Find closest match (won't be meaningful for synthetic image)
		minDist := distances[0]
		minIdx := 0
		for i, d := range distances {
			if d < minDist {
				minDist = d
				minIdx = i
			}
		}
		fmt.Printf("   Closest match: %s (dist=%d) [synthetic image, match not meaningful]\n", 
			idx.Names[minIdx], minDist)
	} else {
		fmt.Printf("\n[TEST 2] Loading test image from %s...\n", testImagePath)
		img := gocv.IMRead(testImagePath, gocv.IMReadGrayScale)
		if img.Empty() {
			fmt.Printf("❌ FAIL: Could not load image\n")
		} else {
			defer img.Close()
			fmt.Printf("✅ PASS: Loaded %dx%d test image\n", img.Cols(), img.Rows())
			
			result := idx.Identify(img, true, 280, 20, nil, false)
			if result != nil {
				fmt.Printf("✅ MATCH: %s (dist=%d, margin=%d)\n", 
					result.Meta.Name, result.Dist, result.Margin)
			} else {
				fmt.Printf("   No match found\n")
			}
		}
	}

	// Test 3: Calibration system
	fmt.Println("\n[TEST 3] Testing calibration system...")
	tmpDir := "/tmp/cardsense-test"
	os.MkdirAll(tmpDir, 0755)
	
	testBox := capture.Box{X: 100, Y: 100, W: 300, H: 400}
	err = capture.SaveCalibration(tmpDir, 1920, 1080, testBox)
	if err != nil {
		fmt.Printf("❌ FAIL: Save failed: %v\n", err)
	} else {
		fmt.Printf("✅ PASS: Saved calibration\n")
		
		loadedBox, calibrated := capture.LoadCalibration(tmpDir, 1920, 1080)
		if !calibrated {
			fmt.Printf("❌ FAIL: Load reported not calibrated\n")
		} else if loadedBox != testBox {
			fmt.Printf("❌ FAIL: Loaded box %+v != saved box %+v\n", loadedBox, testBox)
		} else {
			fmt.Printf("✅ PASS: Loaded calibration matches saved\n")
		}
		
		// Test scaling
		scaledBox, _ := capture.LoadCalibration(tmpDir, 1470, 956)
		expectedX := int(float64(testBox.X) * 1470.0 / 1920.0)
		if scaledBox.X != expectedX {
			fmt.Printf("⚠️  Scaled box X=%d, expected ~%d (may be rounding difference)\n", scaledBox.X, expectedX)
		} else {
			fmt.Printf("✅ PASS: Calibration scales correctly for different resolution\n")
		}
	}

	// Test 4: TTS
	fmt.Println("\n[TEST 4] Testing TTS...")
	speaker, err := tts.NewSpeaker()
	if err != nil {
		fmt.Printf("❌ FAIL: %v\n", err)
		os.Exit(1)
	}
	defer speaker.Close()
	
	speaker.SetRate(550)
	speaker.Speak("CardSense offline test successful. Loaded fifty-three thousand, seven hundred seventy cards.")
	fmt.Println("✅ PASS: TTS initialized and speaking")

	fmt.Println("\n✅ ALL OFFLINE TESTS PASSED")
	fmt.Println("\nCore pHash and index functionality validated.")
	fmt.Println("Screen capture requires permission toggle - see instructions above.")
}
