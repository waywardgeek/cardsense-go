package main

import (
	"fmt"
	"os"
	"os/exec"
	"gocv.io/x/gocv"
)

func main() {
	// Create temp file
	tmpfile, _ := os.CreateTemp("", "test-*.png")
	tmpPath := tmpfile.Name()
	tmpfile.Close()
	
	fmt.Printf("Capturing to: %s\n", tmpPath)
	
	// Run screencapture
	cmd := exec.Command("screencapture", "-x", tmpPath)
	if err := cmd.Run(); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	
	// Check file exists and size
	info, _ := os.Stat(tmpPath)
	fmt.Printf("File size: %d bytes\n", info.Size())
	
	// Try to load with gocv
	mat := gocv.IMRead(tmpPath, gocv.IMReadColor)
	defer mat.Close()
	
	if mat.Empty() {
		fmt.Println("ERROR: Mat is empty")
		return
	}
	
	fmt.Printf("Mat size: %dx%d\n", mat.Cols(), mat.Rows())
	fmt.Printf("Mat type: %v\n", mat.Type())
	fmt.Printf("Mat channels: %d\n", mat.Channels())
	
	// Check pixel values
	_, maxVal, _, _ := gocv.MinMaxLoc(mat)
	fmt.Printf("Max pixel value: %.0f\n", maxVal)
	
	// Don't delete - keep for inspection
	fmt.Printf("\nKept file at: %s\n", tmpPath)
}
