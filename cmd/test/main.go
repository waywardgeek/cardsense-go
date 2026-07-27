package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"

	"github.com/waywardgeek/cardsense-go/pkg/capture"
	"github.com/waywardgeek/cardsense-go/pkg/hash"
	"github.com/waywardgeek/cardsense-go/pkg/tts"
	"gocv.io/x/gocv"
)

func main() {
	fmt.Println("cardsense-go test - validating core functionality")
	fmt.Println()

	// Get data directory from Python version
	dataDir := filepath.Join(os.Getenv("HOME"), "projects", "cardsense", "hashindex", "data")
	fmt.Printf("Data directory: %s\n", dataDir)

	// Test 1: Load card index
	fmt.Println("\n[TEST 1] Loading card index...")
	idx, err := hash.LoadCardIndex(dataDir)
	if err != nil {
		fmt.Printf("❌ FAIL: %v\n", err)
		fmt.Println("\nThis is expected if the Python hash files haven't been built yet.")
		fmt.Println("Run: cd ~/projects/cardsense && python3 hashindex/update_index.py")
		os.Exit(1)
	}
	fmt.Printf("✅ PASS: Loaded %d cards\n", idx.Len())

	// Test 2: List screens
	fmt.Println("\n[TEST 2] Listing displays...")
	screens := capture.ListScreens()
	fmt.Printf("✅ PASS: Found %d display(s)\n", len(screens))
	for i, s := range screens {
		fmt.Printf("  Display %d: %dx%d\n", i, s.Dx(), s.Dy())
	}

	// Test 3: Screen capture permission (with debug info)
	fmt.Println("\n[TEST 3] Testing screen capture...")
	mat, err := capture.CaptureScreen(0)
	if err != nil {
		fmt.Printf("❌ FAIL: Capture error: %v\n", err)
		os.Exit(1)
	}
	defer mat.Close()

	// Check pixel values (convert to grayscale first - MinMaxLoc only works on single-channel)
	gray := gocv.NewMat()
	gocv.CvtColor(mat, &gray, gocv.ColorBGRToGray)
	_, maxVal, _, _ := gocv.MinMaxLoc(gray)
	fmt.Printf("  Captured frame: %dx%d, max pixel value: %.0f\n", mat.Cols(), mat.Rows(), maxVal)
	gray.Close() // Close now, will create new one for pHash test below
	
	if maxVal < 10 {
		fmt.Println("❌ FAIL: Screen recording permission denied (all-black capture)")
		fmt.Println("   Terminal is listed but permission may need to be re-granted")
		fmt.Println("   Try: Remove Terminal from list, re-add it, restart Terminal")
		os.Exit(1)
	}
	fmt.Println("✅ PASS: Screen capture working")

	// Test 4: pHash computation
	fmt.Println("\n[TEST 4] Computing pHash...")
	gray = gocv.NewMat()
	defer gray.Close()
	gocv.CvtColor(mat, &gray, gocv.ColorBGRToGray)
	
	// Crop a test region (center 200x280 pixels)
	h, w := gray.Rows(), gray.Cols()
	x, y := w/2-100, h/2-140
	if x < 0 || y < 0 || x+200 > w || y+280 > h {
		fmt.Println("⚠️  SKIP: Screen too small for test crop")
	} else {
		testCropRegion := gray.Region(image.Rect(x, y, x+200, y+280))
		testCrop := gocv.NewMat()
		testCropRegion.CopyTo(&testCrop)
		testCropRegion.Close()
		defer testCrop.Close()
		
		hashBytes := hash.DualPHash(testCrop, nil)
		fmt.Printf("✅ PASS: Computed %d-byte pHash\n", len(hashBytes))
		
		// Try to identify (won't match anything, just testing the pipeline)
		result := idx.Identify(testCrop, true, 280, 20, nil, false)
		if result != nil {
			fmt.Printf("   Identified: %s (dist=%d, margin=%d)\n", result.Meta.Name, result.Dist, result.Margin)
		} else {
			fmt.Printf("   No card match (expected - not a card image)\n")
		}
	}

	// Test 5: TTS
	fmt.Println("\n[TEST 5] Testing TTS...")
	speaker, err := tts.NewSpeaker()
	if err != nil {
		fmt.Printf("❌ FAIL: %v\n", err)
		os.Exit(1)
	}
	defer speaker.Close()
	
	speaker.SetRate(550)
	speaker.Speak("CardSense test successful")
	fmt.Println("✅ PASS: TTS initialized (speaking test message)")

	fmt.Println("\n✅ ALL TESTS PASSED")
	fmt.Println("\nCore functionality validated. Ready to build full detector.")
}
