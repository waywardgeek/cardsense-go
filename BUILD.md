# Building and Distributing CardSense Go

This guide covers building, signing, and distributing CardSense for macOS and Windows.

## Quick Start

### macOS

```bash
# 1. Build .app bundle
./build.sh

# 2. Test the app (REQUIRED before signing)
open dist/cardsense.app

# 3. Sign and notarize (requires Apple Developer ID)
./notarize.sh

# 4. Test the DMG
open dist/cardsense.dmg
```

### Windows

```powershell
# 1. Install dependencies
choco install golang opencv tesseract mingw -y

# 2. Build executable
go build -o cardsense.exe cmd/gui/main.go

# 3. Test
.\cardsense.exe
```

## Prerequisites

### Development Dependencies

```bash
# Install Go (1.25.4 or later)
brew install go

# Install OpenCV (required for gocv)
brew install opencv

# Install Tesseract (for OCR fallback)
brew install tesseract

# Install Go dependencies
go get gocv.io/x/gocv@latest
go get github.com/kbinani/screenshot@latest
go get fyne.io/fyne/v2@latest
```

### Code Signing Prerequisites (one-time setup)

1. **Apple Developer ID Application certificate**
   - Must be installed in Keychain
   - Issued to: "Bill Cox (B2SUY7SU9A)"
   - Available at: https://developer.apple.com/account/resources/certificates

2. **App-specific password for notarization**
   ```bash
   # Generate at: https://appleid.apple.com
   # Sign in → Security → App-Specific Passwords → Generate
   
   # Store in Keychain (one-time):
   xcrun notarytool store-credentials CardSense \
       --apple-id waywardgeek@gmail.com \
       --team-id B2SUY7SU9A
   # Enter the app-specific password when prompted
   ```

3. **Verify certificate**
   ```bash
   security find-identity -v -p codesigning
   # Should show: "Developer ID Application: Bill Cox (B2SUY7SU9A)"
   ```

## Build Process

### 1. Build .app Bundle

The `build.sh` script creates a complete macOS .app bundle:

```bash
./build.sh
```

**What it does:**
- Compiles Go binary (`cmd/gui/main.go`)
- Creates .app directory structure:
  ```
  cardsense.app/
  ├── Contents/
  │   ├── Info.plist           # Bundle metadata
  │   ├── MacOS/
  │   │   └── cardsense-gui    # 33MB binary
  │   └── Resources/
  │       ├── AppIcon.icns     # 2MB icon
  │       └── data/            # 26MB hash files
  │           ├── phash_index.npz   (11MB)
  │           └── phash_meta.json   (15MB)
  ```
- Bundles hash files from Python version (`~/projects/cardsense/hashindex/data/`)
- Creates `entitlements.plist` for hardened runtime
- Sets executable permissions

**Output:**
- Location: `dist/cardsense.app`
- Size: ~60MB (33MB binary + 26MB hash files + 2MB icon)

### 2. Test Before Signing

**CRITICAL**: Always test the unsigned .app before signing!

```bash
open dist/cardsense.app
```

**What to verify:**
- ✅ App launches without errors
- ✅ GUI appears with controls
- ✅ Screen Recording permission prompt works
- ✅ Hash files load (53,770 cards)
- ✅ Test button speaks "Llanowar Elves..."
- ✅ Start/Stop buttons work
- ✅ Speed slider and voice picker work

**If the app doesn't work, DO NOT sign it.** Debug first!

### 3. Sign and Notarize

The `notarize.sh` script handles the entire code signing and notarization process:

```bash
./notarize.sh
```

**What it does:**

1. **Code signs** with Developer ID and hardened runtime
   - Uses `entitlements.plist` (allows loading bundled libraries like OpenCV)
   - Signature embedded in .app bundle
   
2. **Creates ZIP** for notarization (required by Apple)

3. **Submits to Apple** for notarization
   - Uploads to Apple's notarization service
   - Waits for approval (5-15 minutes typically)
   - Scans for malware and validates signature
   
4. **Staples ticket** to .app
   - Embeds notarization approval into .app
   - Allows offline verification by Gatekeeper
   
5. **Creates DMG** for distribution
   - Single-file installer
   - Users can drag to /Applications

**Output:**
- Signed .app: `dist/cardsense.app`
- DMG: `dist/cardsense.dmg` (~30-35MB compressed)

**If notarization fails:**
```bash
# Get the submission ID from the error message, then:
xcrun notarytool log <submission-id> --keychain-profile CardSense
```

Common failure reasons:
- Missing entitlements
- Unsigned nested binaries
- Insecure library loading
- Malware detected (false positive)

## Bundle-Aware Data Loading

The app detects whether it's running from a bundle or in development mode:

**Development mode** (running from `go run` or built binary in project):
- Uses Python's data directory: `~/projects/cardsense/hashindex/data/`
- Allows testing without rebuilding bundle

**Bundle mode** (running from .app):
- Uses bundled data: `.app/Contents/Resources/data/`
- No external dependencies, works offline immediately

This is implemented in `pkg/hash/index.go::DataDir()`:
```go
// Check if executable path contains ".app/Contents/MacOS/"
if strings.Contains(execPath, ".app/Contents/MacOS/") {
    // Use bundled Resources/data/
    return filepath.Join(execDir, "..", "Resources", "data")
}
// Otherwise use Python's data directory (dev mode)
```

## Distribution

### Testing the DMG

```bash
open dist/cardsense.dmg
```

**What users will see:**
1. DMG mounts as a volume
2. Finder window opens with cardsense.app
3. User drags to /Applications (or double-clicks to open directly)
4. First launch: Gatekeeper verifies notarization ✅
5. App requests Screen Recording permission
6. App loads and starts working

### GitHub Release

```bash
# Tag the release
git tag -a v0.3.0 -m "CardSense Go Port v0.3.0"
git push origin v0.3.0

# Upload to GitHub releases:
# - dist/cardsense.dmg (primary download)
# - SHA256 checksum
```

Example release notes:
```markdown
## CardSense v0.3.0 - Go Port

Native Go port of CardSense with improved performance and reliability.

### Download
- [cardsense.dmg](link) (35 MB) - macOS 10.13+

### Installation
1. Download cardsense.dmg
2. Open DMG and drag CardSense to Applications
3. Launch CardSense from Applications
4. Grant Screen Recording permission when prompted
5. Open MTGA and right-click cards

### Changes
- Native Go implementation (no Python runtime)
- Faster screen capture (10-20ms per frame)
- Bundled hash files (instant startup, no download)
- Smaller binary (~60MB vs ~86MB Python version)
- Same pHash + OCR accuracy (~95% card coverage)
```

## Windows Build

### Prerequisites

```powershell
# Using Chocolatey package manager (https://chocolatey.org/)
# Install Chocolatey first if not already installed

# Install Go (1.25.4 or later)
choco install golang -y

# Install OpenCV (required for gocv)
choco install opencv -y

# Install Tesseract (for OCR fallback)
choco install tesseract -y

# Install MinGW-w64 (for CGO compilation)
choco install mingw -y

# Refresh environment variables
refreshenv

# Install Go dependencies
go get gocv.io/x/gocv@latest
go get github.com/kbinani/screenshot@latest
go get fyne.io/fyne/v2@latest
```

### Environment Setup

OpenCV needs to be findable by CGO:

```powershell
# Set OpenCV environment variables (adjust paths if needed)
$env:CGO_CPPFLAGS="-IC:/tools/opencv/build/include"
$env:CGO_LDFLAGS="-LC:/tools/opencv/build/x64/vc16/lib"
$env:PATH="C:/tools/opencv/build/x64/vc16/bin;$env:PATH"

# Enable CGO
$env:CGO_ENABLED=1
```

**Note**: Path may vary depending on OpenCV version. Check your actual installation:
```powershell
dir C:\tools\opencv
```

### Build

```powershell
# Build GUI executable
go build -o cardsense.exe cmd/gui/main.go
```

**Output:**
- Location: `cardsense.exe`
- Size: ~35-40MB (depends on static vs dynamic linking)

### Testing

```powershell
# Run the executable
.\cardsense.exe

# Or run with debug output
.\cardsense.exe --debug
```

**What to verify:**
- ✅ GUI launches without errors
- ✅ Hash files load (checks Python version's data or downloads)
- ✅ Screen capture works (no UAC issues)
- ✅ TTS works (SAPI via PowerShell)
- ✅ Card detection works with MTGA

### Distribution

#### Manual Distribution

Simplest approach - ZIP the executable with required DLLs:

```powershell
# Create distribution directory
mkdir dist
Copy-Item cardsense.exe dist/

# Copy OpenCV DLLs (required for runtime)
Copy-Item "C:\tools\opencv\build\x64\vc16\bin\opencv_world*.dll" dist/

# Copy Tesseract (if not in PATH)
Copy-Item "C:\Program Files\Tesseract-OCR\tesseract.exe" dist/

# Create ZIP
Compress-Archive -Path dist\* -DestinationPath cardsense-windows.zip
```

Users extract and run `cardsense.exe`.

#### Installer (Optional - Future)

For a more polished distribution, create an installer with [Inno Setup](https://jrsoftware.org/isinfo.php):

```iss
[Setup]
AppName=CardSense
AppVersion=0.3.0
DefaultDirName={pf}\CardSense
DefaultGroupName=CardSense
OutputDir=dist
OutputBaseFilename=cardsense-setup

[Files]
Source: "cardsense.exe"; DestDir: "{app}"
Source: "opencv_world*.dll"; DestDir: "{app}"
Source: "tesseract.exe"; DestDir: "{app}"

[Icons]
Name: "{group}\CardSense"; Filename: "{app}\cardsense.exe"
```

Compile with:
```powershell
iscc installer.iss
```

### Platform Differences

**TTS Implementation:**
- macOS: NSSpeechSynthesizer via CGO (fast, ~550 WPM)
- Windows: SAPI via PowerShell (adds ~200-500ms latency per call)
  - TODO: Replace with go-ole or CGO for better performance

**Screen Capture:**
- macOS: Requires Screen Recording permission (System Settings)
- Windows: Works immediately, no special permissions needed

**Hash Files:**
- macOS: Bundled in .app (instant startup)
- Windows: Currently uses Python version's data or downloads on first run
  - TODO: Bundle in installer

**Voice Selection:**
- macOS: Uses system voices (Reed default)
- Windows: Uses SAPI voices (default system voice)

### CI/CD with GitHub Actions

The repository includes automated Windows builds via GitHub Actions (`.github/workflows/build.yml`).

On every push to `main`:
- Builds on `windows-latest` runner
- Installs OpenCV and Tesseract via Chocolatey
- Compiles `cardsense.exe`
- Runs tests
- Uploads artifact

Download artifacts from Actions tab: https://github.com/waywardgeek/cardsense-go/actions

On release:
- Automatically attaches `cardsense.exe` to GitHub release
- Available alongside macOS DMG

## Troubleshooting

### macOS Build Issues

**Problem**: `gocv` import fails
```bash
# Reinstall OpenCV
brew reinstall opencv
go clean -cache
go get gocv.io/x/gocv@latest
```

**Problem**: Binary too large (>100MB)
```bash
# Check what's included
du -sh dist/cardsense.app/*
# Should be: ~33MB binary, ~26MB data, ~2MB icon
```

### Code Signing Issues

**Problem**: "Developer ID Application: Bill Cox not found"
```bash
# Check certificate
security find-identity -v -p codesigning
# Install from developer.apple.com if missing
```

**Problem**: "No keychain profile found for CardSense"
```bash
# Store credentials again
xcrun notarytool store-credentials CardSense \
    --apple-id waywardgeek@gmail.com \
    --team-id B2SUY7SU9A
```

**Problem**: Notarization rejected
```bash
# Check rejection reason
xcrun notarytool log <submission-id> --keychain-profile CardSense

# Common fixes:
# 1. Update entitlements.plist
# 2. Sign nested binaries first
# 3. Use --deep flag on codesign
```

### Runtime Issues

**Problem**: Hash files not found
```bash
# Check bundle structure
ls -la dist/cardsense.app/Contents/Resources/data/
# Should contain: phash_index.npz, phash_meta.json

# Test data loading
./dist/cardsense.app/Contents/MacOS/cardsense-gui --debug
# Should print: "Loaded 53,770 cards"
```

**Problem**: Screen Recording permission denied
```bash
# Reset permissions
tccutil reset ScreenCapture com.coderhapsody.cardsense
# Relaunch app, grant permission again
```

### Windows Build Issues

**Problem**: `gocv` import fails or build errors
```powershell
# Verify OpenCV installation
dir C:\tools\opencv\build\include

# Reinstall if missing
choco uninstall opencv -y
choco install opencv -y

# Set environment variables (adjust paths if needed)
$env:CGO_CPPFLAGS="-IC:/tools/opencv/build/include"
$env:CGO_LDFLAGS="-LC:/tools/opencv/build/x64/vc16/lib"
$env:PATH="C:/tools/opencv/build/x64/vc16/bin;$env:PATH"
$env:CGO_ENABLED=1

# Clean and rebuild
go clean -cache
go build -o cardsense.exe cmd/gui/main.go
```

**Problem**: Missing DLL errors at runtime
```powershell
# Common missing DLLs:
# - opencv_world*.dll
# - msvcp*.dll (Visual C++ Runtime)

# Copy OpenCV DLLs to same directory as .exe
Copy-Item "C:\tools\opencv\build\x64\vc16\bin\opencv_world*.dll" .

# Install Visual C++ Redistributable if needed
choco install vcredist-all -y
```

**Problem**: Tesseract not found
```powershell
# Check Tesseract installation
tesseract --version

# Add to PATH if not found
$env:PATH="C:\Program Files\Tesseract-OCR;$env:PATH"

# Or reinstall
choco install tesseract -y
```

**Problem**: TTS not working
```powershell
# Test SAPI directly
powershell -Command "Add-Type -AssemblyName System.Speech; $synth = New-Object System.Speech.Synthesis.SpeechSynthesizer; $synth.Speak('Testing')"

# If that fails, Windows Speech Platform may need repair:
# Settings → Time & Language → Speech
```

**Problem**: Screen capture returns black screen
```powershell
# Try running as Administrator (UAC may block screen capture)
# Right-click cardsense.exe → Run as Administrator

# Or add Windows Defender exclusion
# Windows Security → Virus & threat protection → Exclusions
```

### Windows Runtime Issues

**Problem**: App crashes on launch
```powershell
# Run with debug output to see error
.\cardsense.exe --debug

# Check for missing dependencies
# Use Dependency Walker: https://www.dependencywalker.com/
```

**Problem**: Slow TTS (>500ms per card)
- Expected: PowerShell adds 200-500ms overhead
- This is a known limitation
- Future improvement: Replace with go-ole or CGO SAPI

**Problem**: Cards not detected
```powershell
# Test OCR directly
tesseract test.png stdout

# Check if hash files exist
dir hashindex\data\
# Should contain: phash_index.npz (11MB), phash_meta.json (15MB)
```

## File Structure

```
cardsense-go/
├── build.sh              # Build .app bundle
├── notarize.sh           # Sign, notarize, create DMG
├── entitlements.plist    # Hardened runtime entitlements
├── icon.icns             # App icon (2MB, from Python version)
├── dist/
│   ├── cardsense.app     # macOS app bundle (60MB)
│   ├── cardsense.zip     # For notarization (temp)
│   └── cardsense.dmg     # Distribution DMG (35MB)
├── cmd/
│   └── gui/main.go       # GUI entry point
└── pkg/
    ├── gui/              # Fyne GUI
    ├── detector/         # Card detection loop
    ├── hash/             # pHash + OCR
    ├── capture/          # Screen capture
    └── tts/              # Text-to-speech
```

## Version History

### v0.3.0 (2026-07-27)
- Initial Go port release
- Native Go implementation
- Fyne GUI
- pHash + OCR fallback
- Bundled hash files
- Notarized DMG distribution

## Next Steps

- [ ] Windows build with create-dmg alternative
- [ ] Auto-update check on launch
- [ ] Incremental hash file updates
- [ ] AVSpeechSynthesizer (replace deprecated NSSpeechSynthesizer)
- [ ] Reduce binary size (strip debug symbols)
- [ ] Linux support (.AppImage or .deb)
