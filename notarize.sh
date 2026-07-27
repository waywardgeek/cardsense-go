#!/bin/bash
# Notarize cardsense-go.app for distribution

set -e  # Exit on error

# Configuration
APP_NAME="cardsense-go"
DEVELOPER_ID="Developer ID Application: Bill Cox (B2SUY7SU9A)"
KEYCHAIN_PROFILE="CardSense"
APPLE_ID="waywardgeek@gmail.com"
TEAM_ID="B2SUY7SU9A"

# Directories
PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
DIST_DIR="$PROJECT_ROOT/dist"
APP_PATH="$DIST_DIR/$APP_NAME.app"
ZIP_PATH="$DIST_DIR/$APP_NAME.zip"
DMG_PATH="$DIST_DIR/$APP_NAME.dmg"
ENTITLEMENTS="$PROJECT_ROOT/entitlements.plist"

echo "🔐 Notarizing $APP_NAME.app"

# Check that app exists
if [ ! -d "$APP_PATH" ]; then
    echo "❌ Error: $APP_PATH not found"
    echo "   Run ./build.sh first"
    exit 1
fi

# Check for entitlements file
if [ ! -f "$ENTITLEMENTS" ]; then
    echo "❌ Error: entitlements.plist not found"
    exit 1
fi

# Step 1: Code sign with hardened runtime
echo ""
echo "✍️  Step 1: Code signing..."
codesign --deep --force --verify --verbose \
    --options runtime \
    --entitlements "$ENTITLEMENTS" \
    --sign "$DEVELOPER_ID" \
    "$APP_PATH"

if [ $? -ne 0 ]; then
    echo "❌ Code signing failed"
    exit 1
fi

echo "✅ Code signing successful"

# Verify signature
echo ""
echo "🔍 Verifying signature..."
codesign --verify --verbose=4 "$APP_PATH"
spctl --assess --verbose=4 "$APP_PATH" || true

# Step 2: Create ZIP for notarization
echo ""
echo "📦 Step 2: Creating ZIP..."
if [ -f "$ZIP_PATH" ]; then
    rm "$ZIP_PATH"
fi

ditto -c -k --keepParent "$APP_PATH" "$ZIP_PATH"

ZIP_SIZE=$(ls -lh "$ZIP_PATH" | awk '{print $5}')
echo "✅ Created: $ZIP_PATH ($ZIP_SIZE)"

# Step 3: Submit for notarization
echo ""
echo "📤 Step 3: Submitting to Apple for notarization..."
echo "   This may take 5-15 minutes..."
echo ""

xcrun notarytool submit "$ZIP_PATH" \
    --keychain-profile "$KEYCHAIN_PROFILE" \
    --wait

if [ $? -ne 0 ]; then
    echo "❌ Notarization failed"
    echo ""
    echo "💡 To check the log:"
    echo "   xcrun notarytool log <submission-id> --keychain-profile $KEYCHAIN_PROFILE"
    exit 1
fi

echo "✅ Notarization successful!"

# Step 4: Staple the ticket
echo ""
echo "📎 Step 4: Stapling notarization ticket..."
xcrun stapler staple "$APP_PATH"

if [ $? -ne 0 ]; then
    echo "⚠️  Stapling failed (this is sometimes OK)"
else
    echo "✅ Stapling successful"
fi

# Verify stapling
xcrun stapler validate "$APP_PATH" || true

# Step 5: Create DMG
echo ""
echo "💿 Step 5: Creating DMG..."
if [ -f "$DMG_PATH" ]; then
    rm "$DMG_PATH"
fi

hdiutil create \
    -volname "CardSense" \
    -srcfolder "$APP_PATH" \
    -ov \
    -format UDZO \
    "$DMG_PATH"

if [ $? -ne 0 ]; then
    echo "❌ DMG creation failed"
    exit 1
fi

DMG_SIZE=$(ls -lh "$DMG_PATH" | awk '{print $5}')
echo "✅ Created: $DMG_PATH ($DMG_SIZE)"

# Final verification
echo ""
echo "✨ Done! Distribution files ready:"
echo "   📦 Signed .app: $APP_PATH"
echo "   💿 DMG: $DMG_PATH"
echo ""
echo "📤 Next steps:"
echo "   1. Test the DMG: open $DMG_PATH"
echo "   2. Upload to GitHub releases"
echo "   3. Users can download and install to /Applications"
echo ""
echo "🔐 Security status:"
spctl --assess --verbose=4 "$APP_PATH" 2>&1 | grep -i "accepted" && echo "   ✅ Gatekeeper approved" || echo "   ⚠️  Check Gatekeeper status"
