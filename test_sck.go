package main

import (
	"fmt"
	"github.com/waywardgeek/cardsense-go/pkg/capture"
	"gocv.io/x/gocv"
)

func main() {
	fmt.Println("Testing ScreenCaptureKit capture...")
	
	mat, err := capture.CaptureScreen(0)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	defer mat.Close()
	
	fmt.Printf("Captured: %dx%d\n", mat.Cols(), mat.Rows())
	_, maxVal, _, _ := gocv.MinMaxLoc(mat)
	fmt.Printf("Max pixel value: %.0f\n", maxVal)
	
	if maxVal < 10 {
		fmt.Println("All black - permission issue")
	} else {
		fmt.Println("SUCCESS!")
	}
}
