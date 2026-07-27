#!/bin/bash
# Build cardsense.app bundle for macOS

set -e  # Exit on error

# Configuration
APP_NAME="cardsense"
BUNDLE_ID="com.coderhapsody.cardsense"
VERSION="0.3.0"
DEVELOPER_ID="Developer ID Application: Bill Cox (B2SUY7SU9A)"

# Directories
PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
DIST_DIR="$PROJECT_ROOT/dist"
APP_DIR="$DIST_DIR/$APP_NAME.app"
CONTENTS_DIR="$APP_DIR/Contents"
MACOS_DIR="$CONTENTS_DIR/MacOS"
RESOURCES_DIR="$CONTENTS_DIR/Resources"
DATA_DIR="$RESOURCES_DIR/data"

# Python's data directory (for hash files)
PYTHON_DATA_DIR="$HOME/projects/cardsense/hashindex/data"

echo "🚀 Building $APP_NAME.app v$VERSION"
echo "   Project: $PROJECT_ROOT"

# Clean previous build
if [ -d "$APP_DIR" ]; then
    echo "🧹 Cleaning previous build..."
    rm -rf "$APP_DIR"
fi

# Create .app structure
echo "📁 Creating .app bundle structure..."
mkdir -p "$MACOS_DIR"
mkdir -p "$RESOURCES_DIR"
mkdir -p "$DATA_DIR"

# Build Go binary
echo "🔨 Building Go binary..."
cd "$PROJECT_ROOT"
go build -o "$MACOS_DIR/$APP_NAME-gui" cmd/gui/main.go

if [ ! -f "$MACOS_DIR/$APP_NAME-gui" ]; then
    echo "❌ Build failed: binary not found"
    exit 1
fi

echo "✅ Built binary: $MACOS_DIR/$APP_NAME-gui"

# Copy icon
echo "🎨 Copying icon..."
if [ -f "$PROJECT_ROOT/icon.icns" ]; then
    cp "$PROJECT_ROOT/icon.icns" "$RESOURCES_DIR/AppIcon.icns"
else
    echo "⚠️  icon.icns not found, skipping"
fi

# Copy hash files from Python version
echo "📦 Copying hash files..."
if [ -d "$PYTHON_DATA_DIR" ]; then
    cp "$PYTHON_DATA_DIR/phash_index.npz" "$DATA_DIR/"
    cp "$PYTHON_DATA_DIR/phash_meta.json" "$DATA_DIR/"
    
    HASH_SIZE=$(du -sh "$DATA_DIR" | awk '{print $1}')
    echo "✅ Bundled hash files ($HASH_SIZE)"
else
    echo "⚠️  Hash files not found at $PYTHON_DATA_DIR"
    echo "   App will need to download on first run"
fi

# Create Info.plist
echo "📝 Creating Info.plist..."
cat > "$CONTENTS_DIR/Info.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>
    <string>CardSense</string>
    
    <key>CFBundleDisplayName</key>
    <string>CardSense</string>
    
    <key>CFBundleIdentifier</key>
    <string>$BUNDLE_ID</string>
    
    <key>CFBundleVersion</key>
    <string>$VERSION</string>
    
    <key>CFBundleShortVersionString</key>
    <string>$VERSION</string>
    
    <key>CFBundleExecutable</key>
    <string>$APP_NAME-gui</string>
    
    <key>CFBundleIconFile</key>
    <string>AppIcon</string>
    
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    
    <key>CFBundleSignature</key>
    <string>????</string>
    
    <key>LSMinimumSystemVersion</key>
    <string>10.13</string>
    
    <key>NSHighResolutionCapable</key>
    <true/>
    
    <key>NSScreenCaptureUsageDescription</key>
    <string>CardSense needs screen recording permission to identify Magic: The Gathering Arena cards and read them aloud for accessibility.</string>
    
    <key>NSMicrophoneUsageDescription</key>
    <string>CardSense does not use the microphone. This permission request is a side effect of requesting screen recording access.</string>
</dict>
</plist>
EOF

# Create entitlements.plist for code signing
echo "📝 Creating entitlements.plist..."
cat > "$PROJECT_ROOT/entitlements.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <!-- Hardened Runtime entitlements (required for notarization) -->
    
    <!-- Disable library validation (allows loading bundled libraries like OpenCV) -->
    <key>com.apple.security.cs.disable-library-validation</key>
    <true/>
</dict>
</plist>
EOF

# Set executable permissions
chmod +x "$MACOS_DIR/$APP_NAME-gui"

# Get bundle size
BUNDLE_SIZE=$(du -sh "$APP_DIR" | awk '{print $1}')
echo ""
echo "✅ Bundle created successfully!"
echo "   Location: $APP_DIR"
echo "   Size: $BUNDLE_SIZE"
echo ""
echo "📋 Next steps:"
echo "   1. Test: open $APP_DIR"
echo "   2. Sign: codesign --deep --force --verify --verbose --options runtime --entitlements entitlements.plist --sign \"$DEVELOPER_ID\" \"$APP_DIR\""
echo "   3. Notarize: ./notarize.sh"
echo ""
