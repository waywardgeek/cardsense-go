package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"time"

	"github.com/waywardgeek/cardsense-go/pkg/capture"
	"github.com/waywardgeek/cardsense-go/pkg/hash"
	"gocv.io/x/gocv"
)

func main() {
	fmt.Println("Testing card identification with actual crop...")
	
	// Get screen dimensions
	bounds := capture.GetPrimaryDisplayBounds()
	W, H := bounds.Dx(), bounds.Dy()
	fmt.Printf("Screen: %d×%d\n", W, H)
	
	// Load calibration
	dataDir := filepath.Join(os.Getenv("HOME"), "projects", "cardsense", "hashindex", "data")
	box, calibrated := capture.LoadCalibration(dataDir, W, H)
	fmt.Printf("Detection box: (%d, %d, %d, %d) calibrated=%v\n", box.X, box.Y, box.W, box.H, calibrated)
	
	// Load card index
	fmt.Println("\nLoading card index...")
	idx, err := hash.LoadCardIndex(dataDir)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Loaded %d cards\n", idx.NumCards())
	
	// Give user time to switch to MTGA and right-click a card
	fmt.Println("\n⏱️  You have 5 seconds to switch to MTGA and right-click a card...")
	for i := 5; i > 0; i-- {
		fmt.Printf("%d... ", i)
		time.Sleep(1 * time.Second)
	}
	fmt.Println()
	
	// Capture screen
	fmt.Println("Capturing screen...")
	shot, err := capture.CaptureScreen(0)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer shot.Close()
	
	// Crop to detection box
	cardBox := image.Rect(box.X, box.Y, box.X+box.W, box.Y+box.H)
	region := shot.Region(cardBox)
	crop := gocv.NewMat()
	region.CopyTo(&crop)
	region.Close()
	
	// Convert to grayscale
	cropGray := gocv.NewMat()
	gocv.CvtColor(crop, &cropGray, gocv.ColorBGRToGray)
	
	// Save the crop
	cropPath := "/tmp/cardsense_test_crop.png"
	gocv.IMWrite(cropPath, cropGray)
	fmt.Printf("📸 Saved crop to: %s\n", cropPath)
	fmt.Printf("   Crop size: %dx%d\n", cropGray.Cols(), cropGray.Rows())
	
	// Try to identify
	fmt.Println("\n🔍 Attempting to identify card...")
	fmt.Println("   Parameters: sweep=true, maxDist=280, minMargin=20")
	hit := idx.Identify(cropGray, true, 280, 20, nil, false)
	
	if hit != nil {
		fmt.Printf("\n✅ SUCCESS!\n")
		fmt.Printf("   Card: %s\n", hit.Meta.Name)
		fmt.Printf("   Type: %s\n", hit.Meta.TypeLine)
		fmt.Printf("   Distance: %d\n", hit.Dist)
		fmt.Printf("   Margin: %d\n", hit.Margin)
	} else {
		fmt.Printf("\n❌ NO MATCH FOUND\n")
		fmt.Println("   This means pHash distance was too high or margin was too low")
		fmt.Println("   The crop might not contain a card, or the card isn't in the database")
	}
	
	crop.Close()
	cropGray.Close()
	
	fmt.Println("\n💡 Next: Check the saved crop to verify it contains a clear card image")
}
