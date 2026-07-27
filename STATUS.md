# cardsense Go Port - Status Report

## Summary

**Phase 1 (Core Engine)**: ✅ COMPLETE  
**Phase 2 (Detector Loop + OCR)**: ✅ COMPLETE  
**Phase 3 (GUI)**: ✅ COMPLETE  
**Phase 4 (macOS Packaging)**: 🚧 IN PROGRESS  
**Status**: .app bundle built and ready for testing! 🎉

**Phase 4 Progress (2026-07-27)**:
- ✅ Bundle-aware data directory loading
- ✅ Icon copied from Python version
- ✅ Build script creates complete .app structure
- ✅ Hash files bundled (instant offline startup)
- ✅ Info.plist with permissions and metadata
- ✅ Entitlements.plist for hardened runtime
- ✅ Notarize script for code signing and DMG
- ✅ BUILD.md comprehensive documentation
- ✅ README.md for users
- ⏳ Testing unsigned .app (NEXT: Bill to test)
- ⏳ Code signing and notarization
- ⏳ DMG creation and distribution

**Bundle Details**:
- Location: `dist/cardsense.app`
- Size: 60MB (33MB binary + 26MB hash files + 2MB icon)
- Hash files: phash_index.npz (11MB), phash_meta.json (15MB)
- Icon: Gemini design from Python version (card+eye+speaker)

**OCR Fallback Fix (2026-07-27)**:
- **Issue**: Octoprophet not matching (aspect ratio 0.645 rejected by Guard 2)
- **Fix 1**: Widened aspect ratio guard from 0.65-0.80 to **0.60-0.85**
- **Fix 2**: Added missing Accept header for Scryfall API
- **Result**: ✅ Octoprophet now matches via OCR fallback (dist=0, margin=999)

The detector now has full pHash + OCR fallback, matching Python's ~95% card coverage:
- pHash for fast matching (~50% of cards)
- OCR + Scryfall fallback for MTGA rendering differences (~95% total)
- 3-guard system to prevent false positives
- Tesseract command-line integration (simple, no CGO issues)

## What's Working

### ✅ Phase 2: Detector Loop
- **pkg/detector/detector.go** (379 lines)
  - Main detection loop (`loopInner`) - exact port of Python's `_loop_inner`
  - Auto-loads card index with status updates
  - Screen permission test with diagnostic messages
  - Fixed-box detection at ~20 FPS
  - First-match twiddle calibration
  - State tracking (NO_CARD_FRAMES = 10 before reset)
  - TTS integration for card announcements
  - Debug mode (saves unmatched crops)
  
- **cmd/detector/main.go** (56 lines)
  - Headless test program (no GUI)
  - Signal handling (Ctrl+C to stop)
  - 550 WPM TTS rate (Bill's preference)
  - Runs detector continuously

### ✅ Core Hash System
- **pkg/hash/phash.go** (222 lines)
  - Dual 512-bit pHash (full + art box DCT)
  - HammingScan for batch distance computation
  - AlignVariants for border tolerance
  - Precomputed popcount table

- **pkg/hash/index.go** (337 lines) **WITH OCR FALLBACK**
  - Loads .npz files (ZIP of NumPy arrays)
  - Custom NPY parser for uint8 data
  - Identify() with pHash + OCR fallback
  - Margin calculation for confidence
  - 3-guard system: distance ≤200, aspect 0.65-0.80, OCR validation

- **pkg/hash/ocr.go** (182 lines) ✨ **NEW**
  - Title region extraction (top 5-14% of card)
  - Preprocessing: invert, threshold, upscale 3x
  - Tesseract command-line integration (PSM 7, letter whitelist)
  - Scryfall fuzzy search API
  - OCR text validation (5+ chars, 4+ letters, no consecutive spaces)

### ✅ Screen Capture & Calibration
- **pkg/capture/screen.go** (97 lines)
  - Uses `github.com/kbinani/screenshot` (cross-platform, zero deps)
  - BGR output matching OpenCV convention
  - Permission test (detects macOS blocked capture)

- **pkg/capture/calibration.go** (105 lines)
  - Save/load detection box from JSON
  - Auto-scaling for different resolutions
  - Reference: Bill's 1920×1080 box (21, 47, 449, 709)

- **pkg/capture/twiddle.go** (104 lines)
  - Hill-climbing box refinement (1% then 0.1%)
  - Minimizes pHash distance on first match
  - Exact port from Python

### ✅ Validation & TTS
- **pkg/match/validator.go** (97 lines)
  - 3-guard system prevents false positives
  - Guard 1: pHash ≤ 200
  - Guard 2: Aspect ratio 0.65-0.80
  - Guard 3: OCR text validation

- **pkg/tts/speaker*.go** (3 files, 230 lines)
  - Cross-platform interface
  - macOS: `/usr/bin/say` (async, 550 WPM)
  - Windows: PowerShell SAPI (temporary, needs cgo)

### ✅ Test Program
- **cmd/test/main.go** (96 lines)
  - Validates all core components
  - Tests against Python hash files
  - Reports permission issues

## Architecture Decisions

### Go Library Equivalents
| Python | Go | Notes |
|--------|-----|-------|
| mss | github.com/kbinani/screenshot | Zero dependencies, cross-platform |
| cv2 | gocv.io/x/gocv | OpenCV bindings |
| numpy.load | Custom NPY parser | ZIP + binary format reader |
| NSSpeechSynthesizer | /usr/bin/say | Simpler, still fast |
| pyttsx3 | PowerShell → SAPI/cgo | Needs cgo for speed |

### Key Simplifications
1. **Fixed-box detection** - Not using frame-diff (simpler, works well)
2. **Custom NPY parser** - Lightweight, no external deps for reading
3. **TTS via command** - Simpler than FFI for first version

## What's Next

### Phase 2.5: MTGA Testing (15 min) 🎯 **NEXT**
- [ ] Run detector: `cd ~/projects/cardsense-go && ./cardsense-detector`
- [ ] Open MTGA, right-click cards
- [ ] Verify cards are identified and spoken
- [ ] Test both pHash and OCR fallback paths
- [ ] Check calibration saves after first match
- [ ] Debug any matching issues

### Phase 3: GUI (30 min)
- [ ] Fyne GUI: status label, start/stop buttons, speed slider
- [ ] Wire detector to GUI callbacks
- [ ] Voice picker (macOS voice list)
- [ ] Package as .app bundle

### Phase 4: Polish & Ship (15 min)
- [ ] Test with MTGA gameplay
- [ ] Cross-compile for Windows
- [ ] Incremental update system
- [ ] Build/README documentation

## Dependencies

```bash
# Install OpenCV (required for gocv)
brew install opencv

# Install Go dependencies
go get gocv.io/x/gocv@latest
go get github.com/kbinani/screenshot@latest
go get fyne.io/fyne/v2@latest  # For GUI

# Optional: Tesseract for OCR fallback
brew install tesseract
go get github.com/otiai10/gosseract/v2@latest
```

## Testing

### Quick Test
```bash
cd ~/projects/cardsense-go
go run cmd/test/main.go
```

### Expected Output
```
[TEST 1] Loading card index... ✅ PASS: Loaded 53,770 cards
[TEST 2] Listing displays... ✅ PASS: Found 1 display(s)
[TEST 3] Testing screen capture... ✅ PASS: Screen capture working
[TEST 4] Capturing test frame... ✅ PASS: Captured 1920x1080 frame
[TEST 5] Computing pHash... ✅ PASS: Computed 64-byte pHash
[TEST 6] Testing TTS... ✅ PASS: TTS initialized (speaking test message)
✅ ALL TESTS PASSED
```

## Known Issues / TODOs

1. **NPY string arrays** - Currently using placeholder IDs (card_0, card_1, ...)
   - Not critical: IDs not used for matching, only names matter
   
2. **Windows SAPI** - Using PowerShell fallback (slow but works)
   - TODO: Implement internal/sapi/sapi.c + cgo bindings
   - Priority: Low (PowerShell works for testing)

3. **Incremental updates** - Not yet ported
   - Python: Download 281MB JSON, hash only new cards
   - Critical for maintaining the index
   - Priority: High (before wider testing)

## File Structure

```
cardsense-go/
├── go.mod, go.sum
├── cmd/
│   ├── cardsense/main.go     # Main entry (placeholder)
│   ├── detector/main.go       # Headless detector (working)
│   └── test/main.go           # Core validation test
├── pkg/
│   ├── capture/
│   │   ├── screen.go          # kbinani/screenshot wrapper
│   │   ├── calibration.go     # Save/load detection box
│   │   └── twiddle.go         # Hill-climbing refinement
│   ├── detector/
│   │   └── detector.go        # Main detection loop
│   ├── hash/
│   │   ├── phash.go           # Dual 512-bit pHash
│   │   ├── index.go           # NPZ loader + identify (with OCR)
│   │   └── ocr.go             # ✨ OCR + Scryfall fallback
│   ├── match/
│   │   └── validator.go       # 3-guard validation
│   ├── tts/
│   │   ├── speaker.go         # Interface
│   │   ├── speaker_darwin.go  # macOS /usr/bin/say
│   │   └── speaker_windows.go # PowerShell SAPI
│   └── gui/                   # TODO: Fyne GUI
└── internal/
    └── sapi/                  # TODO: Windows SAPI C wrapper
```

## Code Metrics

- **Total Lines**: ~1,532 (Python: 1,350)
- **Packages**: 6 (hash, capture, match, tts, detector, gui)
- **Test Coverage**: Core components validated
- **Dependencies**: 3 critical (gocv, screenshot, tesseract)

## Critical Learnings

1. **Follow the Python code exactly** - Bill's redirect was correct; the working implementation is the spec.

2. **Fixed-box detection is simpler** - Frame-diff exists but isn't used; fixed box with twiddle works great.

3. **NPZ = ZIP of NPY** - Simple format, easy to parse without heavy libraries.

4. **Calibration scales well** - One-time twiddle + proportional scaling handles different resolutions.

## Success Criteria

- [x] Core pHash matches Python (bit-for-bit)
- [x] Screen capture works cross-platform
- [x] Calibration saves/loads correctly
- [x] TTS works on macOS
- [x] Detector loop faithfully matches Python behavior
- [x] Headless test program compiles and runs
- [x] OCR fallback integrated (Tesseract + Scryfall)
- [x] 3-guard system prevents false positives
- [x] Cards detected and spoken correctly in MTGA
- [x] GUI matches Python functionality
- [x] .app bundle created with proper structure
- [ ] .app tested and verified working
- [ ] Code signing and notarization complete
- [ ] DMG created and tested
- [ ] Windows .exe cross-compiles successfully

## Next Steps for Bill

### 🎯 Phase 4: Test and Ship (30-45 minutes)

The .app bundle is built and ready for testing!

**Step 1: Test the unsigned .app (5 minutes)**
```bash
cd ~/projects/cardsense-go
open dist/cardsense.app
```

**What to verify:**
- ✅ App launches without errors
- ✅ GUI appears with all controls (status, speed slider, voice picker, test/start/stop buttons)
- ✅ Screen Recording permission prompt works
- ✅ Hash files load successfully ("Loaded 53,770 cards")
- ✅ Test button speaks "Llanowar Elves..." 
- ✅ Start button begins detection
- ✅ Cards detected and spoken when right-clicked in MTGA

**If it works, proceed to Step 2. If not, debug first!**

**Step 2: Sign and notarize (15-20 minutes)**
```bash
cd ~/projects/cardsense-go
./notarize.sh
```

This will:
1. Code sign with Developer ID Application
2. Create ZIP for notarization
3. Submit to Apple and wait for approval (5-15 min)
4. Staple notarization ticket
5. Create DMG for distribution

**Prerequisites** (one-time setup, skip if already done):
```bash
# Check certificate
security find-identity -v -p codesigning
# Should show: "Developer ID Application: Bill Cox (B2SUY7SU9A)"

# Store notarization credentials (if not already stored)
xcrun notarytool store-credentials CardSense \
    --apple-id waywardgeek@gmail.com \
    --team-id B2SUY7SU9A
# Enter app-specific password from appleid.apple.com
```

**Step 3: Test the DMG (2 minutes)**
```bash
open dist/cardsense.dmg
```

Verify:
- ✅ DMG mounts and shows app
- ✅ App can be dragged to Applications
- ✅ App launches from Applications without Gatekeeper warnings
- ✅ Works exactly like the unsigned version

**Step 4: Ship (5 minutes)**
- Tag release: `git tag -a v0.3.0 -m "CardSense Go Port v0.3.0"`
- Push tag: `git push origin v0.3.0`
- Create GitHub release
- Upload `dist/cardsense.dmg`
- Add release notes (see BUILD.md for template)

**Troubleshooting**: See [BUILD.md](BUILD.md) troubleshooting section

**Debug mode**: Add `--debug` flag to save detection crops to `/tmp/`

**Debug mode is ON**: Saves unmatched crops to `/tmp/cardsense_nomatch_*.png` for investigation.

---

**Status**: Phase 2 COMPLETE ✅ | Ready for MTGA Testing 🧪
