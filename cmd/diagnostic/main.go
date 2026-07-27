package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/waywardgeek/cardsense-go/pkg/hash"
	"gocv.io/x/gocv"
)

func main() {
	fmt.Println("Diagnostic: Compare pHash computation with Python...")
	
	// Load the crop
	cropPath := "/tmp/cardsense_test_crop.png"
	crop := gocv.IMRead(cropPath, gocv.IMReadGrayScale)
	if crop.Empty() {
		fmt.Printf("❌ Could not load %s\n", cropPath)
		os.Exit(1)
	}
	defer crop.Close()
	
	fmt.Printf("✅ Loaded crop: %dx%d\n", crop.Cols(), crop.Rows())
	
	// Compute pHash
	fmt.Println("\n🔢 Computing dual pHash...")
	hashes := hash.DualPHash(crop, nil)
	fmt.Printf("   Hash length: %d bytes\n", len(hashes))
	fmt.Printf("   First 16 bytes: %v\n", hashes[:16])
	
	// Load card index
	dataDir := filepath.Join(os.Getenv("HOME"), "projects", "cardsense", "hashindex", "data")
	fmt.Println("\n📚 Loading card index...")
	idx, err := hash.LoadCardIndex(dataDir)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Loaded %d cards\n", idx.NumCards())
	
	// Compute distances to all cards
	fmt.Println("\n🔍 Computing hamming distances to all cards...")
	distances := hash.HammingScan(hashes, idx.Bits)
	
	// Find top 10 matches
	type match struct {
		idx  int
		dist int
	}
	
	matches := make([]match, len(distances))
	for i, d := range distances {
		matches[i] = match{i, d}
	}
	
	// Sort by distance (bubble sort for simplicity)
	for i := 0; i < len(matches)-1; i++ {
		for j := 0; j < len(matches)-i-1; j++ {
			if matches[j].dist > matches[j+1].dist {
				matches[j], matches[j+1] = matches[j+1], matches[j]
			}
		}
	}
	
	fmt.Println("\n📊 Top 10 closest matches:")
	for i := 0; i < 10 && i < len(matches); i++ {
		m := matches[i]
		name := idx.Meta[m.idx].Name
		fmt.Printf("   %2d. %-30s  distance=%d\n", i+1, name, m.dist)
	}
	
	// Check what the margin is for Snakeskin Veil
	topName := idx.Meta[matches[0].idx].Name
	fmt.Printf("\n🎯 Best match: %s (distance=%d)\n", topName, matches[0].dist)
	
	// Find margin (distance to next different card)
	margin := -1
	for i := 1; i < len(matches); i++ {
		if idx.Meta[matches[i].idx].Name != topName {
			margin = matches[i].dist - matches[0].dist
			fmt.Printf("   Next different card: %s (distance=%d)\n", idx.Meta[matches[i].idx].Name, matches[i].dist)
			break
		}
	}
	
	if margin >= 0 {
		fmt.Printf("   Margin: %d\n", margin)
		
		if matches[0].dist <= 280 && margin >= 20 {
			fmt.Println("\n✅ Should PASS with maxDist=280, minMargin=20")
		} else {
			fmt.Println("\n❌ Should FAIL:")
			if matches[0].dist > 280 {
				fmt.Printf("   Distance %d > 280 threshold\n", matches[0].dist)
			}
			if margin < 20 {
				fmt.Printf("   Margin %d < 20 threshold\n", margin)
			}
		}
	}
	
	fmt.Println("\n💡 Now compare with Python version...")
}
