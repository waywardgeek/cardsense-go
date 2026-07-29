# CardSense (Go Port)

Cross-platform accessibility tool for Magic: The Gathering Arena. Identifies cards on screen and reads them aloud for low-vision players.

**Platforms**: macOS, Windows (Linux planned)

## Features

- **Fast card identification**: pHash + OCR fallback (~95% accuracy)
- **Screen-reader friendly**: Speaks card name, type, and rules text at 550 WPM
- **Instant startup**: Bundled hash files, no download required
- **Native performance**: Go implementation, 10-20ms per frame
- **Simple GUI**: Start/stop, speed control, voice selection

## Installation

### macOS

**Download**: [cardsense.dmg](https://github.com/waywardgeek/cardsense-go/releases) (latest release)

1. Open the DMG
2. Drag CardSense to Applications
3. Launch CardSense
4. Grant Screen Recording permission when prompted
5. Open MTGA, right-click a card to hear it read aloud

**Requirements**: macOS 10.13+ (High Sierra or later)

### Windows

**Status**: Working! Automated builds via GitHub Actions.

**Download**: 
- [cardsense.exe](https://github.com/waywardgeek/cardsense-go/releases) (from GitHub Actions artifacts)
- Or build from source (see [BUILD.md](BUILD.md#windows-build))

**Installation**:
1. Download `cardsense.exe` from latest release or Actions artifacts
2. Place in a folder (e.g., `C:\CardSense\`)
3. Run `cardsense.exe`
4. Open MTGA, right-click a card to hear it read aloud

**Requirements**: 
- Windows 10/11
- OpenCV DLLs (see [BUILD.md](BUILD.md) for distribution notes)

**Known Limitations**:
- TTS uses PowerShell → SAPI (adds ~200-500ms latency)
- Future improvement: Replace with go-ole for faster speech

## Usage

1. **Launch CardSense** from Applications
2. **Click Start** to begin card detection
3. **Open MTGA** and right-click any card
4. CardSense will identify and speak the card immediately
5. **Adjust speed** (150-900 WPM) with the slider
6. **Change voice** (12 macOS voices available)

## How It Works

1. **Screen capture**: Monitors a fixed region at ~20 FPS
2. **Card detection**: Auto-calibrates detection box on first match
3. **Identification**: 
   - pHash (perceptual hash) for instant matching (~50% of cards)
   - OCR + Scryfall API fallback for MTGA rendering differences (~95% total)
4. **Text-to-speech**: Reads card name, type line, and rules text

## Development

See [BUILD.md](BUILD.md) for:
- Build instructions
- Code signing and notarization
- Bundle structure
- Distribution process

### Quick Build

```bash
# Install dependencies
brew install go opencv tesseract

# Build .app bundle
./build.sh

# Test (before signing)
open dist/cardsense.app

# Sign and notarize (requires Apple Developer ID)
./notarize.sh
```

### Project Structure

```
cardsense-go/
├── cmd/gui/           # GUI entry point
├── pkg/
│   ├── gui/          # Fyne GUI (Tkinter → Fyne port)
│   ├── detector/     # Detection loop
│   ├── hash/         # pHash + OCR + Scryfall
│   ├── capture/      # Screen capture + calibration
│   └── tts/          # Text-to-speech (NSSpeechSynthesizer)
├── build.sh          # Build .app bundle
├── notarize.sh       # Sign, notarize, create DMG
└── BUILD.md          # Detailed build instructions
```

## Technical Details

- **Language**: Go 1.25.4
- **GUI**: Fyne (cross-platform)
- **Vision**: OpenCV (gocv) for image processing
- **OCR**: Tesseract for text extraction
- **Hash Index**: 53,770 cards, 26MB (Scryfall bulk data)
- **Binary Size**: ~60MB bundled, ~35MB DMG

## License

Apache 2.0 (same as Python version)

## Credits

**Author**: Bill Cox (waywardgeek@gmail.com)  
**Development**: Built with CodeRhapsody AI coding agent  
**Python Version**: [cardsense](https://github.com/waywardgeek/cardsense)

**Purpose**: Accessibility tool for low-vision Magic: The Gathering Arena players. Bill Cox has macular dystrophy (vision ~20/180) and built this tool to play MTGA independently.

## Links

- **Python Version**: [github.com/waywardgeek/cardsense](https://github.com/waywardgeek/cardsense)
- **CodeRhapsody**: [coderhapsody.ai](https://coderhapsody.ai)
- **Scryfall API**: [scryfall.com/docs/api](https://scryfall.com/docs/api)

## Support

For bugs or feature requests, please open an issue on GitHub.

For questions about usage, see [BUILD.md](BUILD.md) troubleshooting section.
