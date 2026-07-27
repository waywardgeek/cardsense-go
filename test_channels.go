package main

import (
	"fmt"
	"gocv.io/x/gocv"
)

func main() {
	path := "/var/folders/p_/11vk6y114xq7k9g5r_pc3yv80000gn/T/debug-4024640524.png"
	
	mat := gocv.IMRead(path, gocv.IMReadColor)
	defer mat.Close()
	
	fmt.Printf("Image: %dx%d type=%v channels=%d\n", mat.Cols(), mat.Rows(), mat.Type(), mat.Channels())
	
	// Split into channels
	channels := gocv.Split(mat)
	defer func() {
		for i := range channels {
			channels[i].Close()
		}
	}()
	
	fmt.Printf("Number of channels: %d\n", len(channels))
	
	for i, ch := range channels {
		_, maxVal, _, _ := gocv.MinMaxLoc(ch)
		fmt.Printf("Channel %d: max=%.0f\n", i, maxVal)
	}
}
