package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

func main() {
	// Create temp file
	tmpfile, _ := os.CreateTemp("", "debug-*.png")
	tmpPath := tmpfile.Name()
	tmpfile.Close()
	
	fmt.Printf("Capturing to: %s\n", tmpPath)
	
	// Run screencapture
	cmd := exec.Command("screencapture", "-x", tmpPath)
	if err := cmd.Run(); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	
	// Wait a moment for file to be fully written
	time.Sleep(100 * time.Millisecond)
	
	// Check file exists and size
	info, err := os.Stat(tmpPath)
	if err != nil {
		fmt.Printf("ERROR: stat failed: %v\n", err)
		return
	}
	fmt.Printf("File size: %d bytes\n", info.Size())
	
	// Read first 16 bytes (PNG header should be: 89 50 4E 47 0D 0A 1A 0A)
	f, _ := os.Open(tmpPath)
	header := make([]byte, 16)
	n, _ := f.Read(header)
	f.Close()
	
	fmt.Printf("First %d bytes: %X\n", n, header)
	
	// Keep file for manual inspection
	fmt.Printf("\n✅ PNG saved at: %s\n", tmpPath)
	fmt.Println("Please open this file in Preview to verify it's not all-black")
}
