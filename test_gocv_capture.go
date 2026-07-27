package main

import (
	"fmt"
	"gocv.io/x/gocv"
)

func main() {
	// Try using gocv's VideoCapture with desktop
	fmt.Println("Testing gocv desktop capture...")
	
	// Try capturing from "desktop" or screen
	webcam, err := gocv.OpenVideoCapture(0)
	if err != nil {
		fmt.Printf("Failed to open video capture: %v\n", err)
		return
	}
	defer webcam.Close()
	
	img := gocv.NewMat()
	defer img.Close()
	
	if ok := webcam.Read(&img); !ok {
		fmt.Println("Cannot read from video capture")
		return
	}
	
	if img.Empty() {
		fmt.Println("Empty image")
		return
	}
	
	fmt.Printf("Captured: %dx%d\n", img.Cols(), img.Rows())
	_, maxVal, _, _ := gocv.MinMaxLoc(img)
	fmt.Printf("Max pixel value: %.0f\n", maxVal)
}
