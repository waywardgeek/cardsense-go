# Build Windows Installer for CardSense
# Collects all required DLLs and files, then builds Inno Setup installer

param(
    [string]$OpenCVPath = "C:\opencv\build\install\x64\mingw\bin",
    [string]$TesseractPath = "C:\Program Files\Tesseract-OCR",
    [string]$HashDataPath = "..\..\..\cardsense\hashindex\data"
)

Write-Host "=== CardSense Windows Installer Build ===" -ForegroundColor Cyan

# Ensure we're in the installer directory
Set-Location $PSScriptRoot

# Create dist directory structure
Write-Host "`nCreating distribution directory structure..." -ForegroundColor Yellow
$distDir = "..\dist\windows-libs"
New-Item -ItemType Directory -Force -Path "$distDir\opencv" | Out-Null
New-Item -ItemType Directory -Force -Path "$distDir\tesseract" | Out-Null

# Copy OpenCV DLLs
Write-Host "Copying OpenCV DLLs from: $OpenCVPath" -ForegroundColor Yellow
if (Test-Path $OpenCVPath) {
    Copy-Item "$OpenCVPath\*.dll" -Destination "$distDir\opencv\" -Force
    $dllCount = (Get-ChildItem "$distDir\opencv\*.dll").Count
    Write-Host "  Copied $dllCount OpenCV DLLs" -ForegroundColor Green
} else {
    Write-Host "  WARNING: OpenCV path not found: $OpenCVPath" -ForegroundColor Red
    Write-Host "  Install OpenCV or run from CI environment" -ForegroundColor Red
}

# Copy Tesseract
Write-Host "Copying Tesseract from: $TesseractPath" -ForegroundColor Yellow
if (Test-Path $TesseractPath) {
    Copy-Item "$TesseractPath\tesseract.exe" -Destination "$distDir\tesseract\" -Force
    
    # Copy tessdata (OCR language data)
    if (Test-Path "$TesseractPath\tessdata") {
        Copy-Item "$TesseractPath\tessdata" -Destination "$distDir\tesseract\tessdata" -Recurse -Force
        Write-Host "  Copied Tesseract executable and data" -ForegroundColor Green
    } else {
        Write-Host "  WARNING: tessdata not found in Tesseract directory" -ForegroundColor Red
    }
} else {
    Write-Host "  WARNING: Tesseract not found: $TesseractPath" -ForegroundColor Red
    Write-Host "  Install with: choco install tesseract" -ForegroundColor Red
}

# Check for hash files
Write-Host "Checking hash files..." -ForegroundColor Yellow
$hashIndexPath = "..\hashindex\data\phash_index.npz"
$hashMetaPath = "..\hashindex\data\phash_meta.json"

if (Test-Path $hashIndexPath) {
    $hashSize = (Get-Item $hashIndexPath).Length / 1MB
    Write-Host "  Found phash_index.npz ($([math]::Round($hashSize, 1)) MB)" -ForegroundColor Green
} else {
    Write-Host "  WARNING: phash_index.npz not found" -ForegroundColor Red
    
    # Try Python version's data if available
    if (Test-Path $HashDataPath) {
        Write-Host "  Attempting to use Python version's hash files..." -ForegroundColor Yellow
        Copy-Item "$HashDataPath\*" -Destination "..\hashindex\data\" -Force
    } else {
        Write-Host "  Run Python version's cardsense once to generate hash files" -ForegroundColor Red
        Write-Host "  Or download from GitHub releases" -ForegroundColor Red
    }
}

# Check if executable exists
Write-Host "`nChecking for cardsense-gui.exe..." -ForegroundColor Yellow
if (Test-Path "..\cardsense-gui.exe") {
    $exeSize = (Get-Item "..\cardsense-gui.exe").Length / 1MB
    Write-Host "  Found cardsense-gui.exe ($([math]::Round($exeSize, 1)) MB)" -ForegroundColor Green
} else {
    Write-Host "  ERROR: cardsense-gui.exe not found!" -ForegroundColor Red
    Write-Host "  Build first with: go build -tags customenv -o cardsense-gui.exe cmd/gui/main.go" -ForegroundColor Red
    exit 1
}

# Check for Inno Setup
Write-Host "`nChecking for Inno Setup..." -ForegroundColor Yellow
$isccPath = "C:\Program Files (x86)\Inno Setup 6\ISCC.exe"
if (Test-Path $isccPath) {
    Write-Host "  Found Inno Setup" -ForegroundColor Green
    
    # Build installer
    Write-Host "`nBuilding installer..." -ForegroundColor Yellow
    & $isccPath "cardsense-setup.iss"
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "`n=== Installer built successfully! ===" -ForegroundColor Green
        Write-Host "Output: ..\dist\cardsense-setup-0.3.0.exe" -ForegroundColor Green
        
        $installerSize = (Get-Item "..\dist\cardsense-setup-0.3.0.exe").Length / 1MB
        Write-Host "Size: $([math]::Round($installerSize, 1)) MB" -ForegroundColor Green
    } else {
        Write-Host "`nInstaller build FAILED!" -ForegroundColor Red
        exit 1
    }
} else {
    Write-Host "  Inno Setup not found: $isccPath" -ForegroundColor Red
    Write-Host "  Download from: https://jrsoftware.org/isdl.php" -ForegroundColor Red
    Write-Host "`nFiles prepared in: $distDir" -ForegroundColor Yellow
    Write-Host "You can manually build with Inno Setup later" -ForegroundColor Yellow
}

Write-Host "`n=== Done ===" -ForegroundColor Cyan
