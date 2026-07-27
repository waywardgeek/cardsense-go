package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/waywardgeek/cardsense-go/pkg/detector"
	"github.com/waywardgeek/cardsense-go/pkg/tts"
)

func main() {
	fmt.Println("cardsense detector test - headless mode")
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Get data directory from Python version
	dataDir := filepath.Join(os.Getenv("HOME"), "projects", "cardsense", "hashindex", "data")
	fmt.Printf("Data directory: %s\n", dataDir)

	// Initialize TTS speaker
	fmt.Println("\nInitializing TTS...")
	speaker, err := tts.NewSpeaker()
	if err != nil {
		fmt.Printf("❌ FAIL: %v\n", err)
		os.Exit(1)
	}
	defer speaker.Close()
	
	speaker.SetRate(550) // 550 WPM (Bill's preference)
	fmt.Println("✅ TTS ready")

	// Status callback for console output
	onStatus := func(msg string) {
		fmt.Printf("[STATUS] %s\n", msg)
	}

	// Create detector (debug mode enabled to save unmatched crops)
	det := detector.New(speaker, dataDir, onStatus, true)

	// Start detection loop
	fmt.Println("\nStarting detector...")
	det.Start()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")
	det.Stop()
	fmt.Println("✅ Detector stopped")
}
