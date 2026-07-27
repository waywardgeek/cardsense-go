package main

import (
	"flag"

	"github.com/waywardgeek/cardsense-go/pkg/gui"
)

func main() {
	// Parse command-line flags
	debug := flag.Bool("debug", false, "Enable debug mode (save crops to /tmp)")
	flag.Parse()

	// DataDir will be auto-detected:
	// - Bundle mode: .app/Contents/Resources/data/
	// - Dev mode: ~/projects/cardsense/hashindex/data/
	// Pass empty string to use auto-detection
	dataDir := ""

	// Build and run GUI
	gui.BuildGUI(dataDir, *debug)
}
