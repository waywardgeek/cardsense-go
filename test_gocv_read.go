package main

import (
	"fmt"
	"gocv.io/x/gocv"
)

func main() {
	path := "/var/folders/p_/11vk6y114xq7k9g5r_pc3yv80000gn/T/debug-4024640524.png"
	
	fmt.Println("Testing different gocv read modes...")
	
	// Try different read modes
	modes := map[string]gocv.IMReadFlag{
		"IMReadColor":        gocv.IMReadColor,
		"IMReadGrayScale":    gocv.IMReadGrayScale,
		"IMReadUnchanged":    gocv.IMReadUnchanged,
		"IMReadAnyColor":     gocv.IMReadAnyColor,
		"IMReadAnyDepth":     gocv.IMReadAnyDepth,
	}
	
	for name, mode := range modes {
		mat := gocv.IMRead(path, mode)
		if mat.Empty() {
			fmt.Printf("%s: EMPTY\n", name)
			continue
		}
		
		_, maxVal, _, _ := gocv.MinMaxLoc(mat)
		fmt.Printf("%s: %dx%d type=%v channels=%d max=%.0f\n", 
			name, mat.Cols(), mat.Rows(), mat.Type(), mat.Channels(), maxVal)
		mat.Close()
	}
}
