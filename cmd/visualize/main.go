package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"time"

	"github.com/waywardgeek/cardsense-go/pkg/capture"
	"gocv.io/x/gocv"
)

func main() {
	fmt.Println("Visualizing detection box...")
	
	// Get screen dimensions
	bounds := capture.GetPrimaryDisplayBounds()
	W, H := bounds.Dx(), bounds.Dy()
	fmt.Printf("Screen: %d×%d\n", W, H)
	
	// Load calibration
	dataDir := filepath.Join(os.Getenv("HOME"), "projects", "cardsense", "hashindex", "data")
	box, calibrated := capture.LoadCalibration(dataDir, W, H)
	fmt.Printf("Detection box: (%d, %d, %d, %d) calibrated=%v\n", box.X, box.Y, box.W, box.H, calibrated)
	
	// Give user time to switch to MTGA and right-click a card
	fmt.Println("\n⏱️  You have 5 seconds to switch to MTGA and right-click a card...")
	fmt.Println("3... 2... 1...")
	time.Sleep(5 * time.Second)
	
	// Capture screen
	fmt.Println("Capturing screen...")
	shot, err := capture.CaptureScreen(0)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer shot.Close()
	
	// Draw rectangle around detection box
	rect := image.Rect(box.X, box.Y, box.X+box.W, box.Y+box.H)
	gocv.Rectangle(&shot, rect, color.RGBA{0, 255, 0, 255}, 5)
	
	// Save visualization
	outPath := "/tmp/cardsense_detection_box.png"
	gocv.IMWrite(outPath, shot)
	fmt.Printf("\n✅ Saved visualization to: %s\n", outPath)
	fmt.Println("The GREEN BOX shows where the detector is looking for cards.")
	fmt.Println("\nOpen this file to see if it matches where cards appear in MTGA when you right-click them.")
}
