#!/bin/bash
set -e

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
BUILD_DIR="$PROJECT_DIR/build"
APP_NAME="Clippy"
APP_BUNDLE="$BUILD_DIR/$APP_NAME.app"

echo "🔨 Building Clippy v2..."

# Clean
rm -rf "$BUILD_DIR"
mkdir -p "$APP_BUNDLE/Contents/MacOS"
mkdir -p "$APP_BUNDLE/Contents/Resources"

# 1. Build Go backend
echo "📦 Building Go backend..."
cd "$PROJECT_DIR/go-backend"
CGO_ENABLED=1 go build -o "$APP_BUNDLE/Contents/MacOS/clippy-backend" .
echo "  ✅ Go backend built"

# 2. Build Swift frontend
echo "📦 Building Swift frontend..."
cd "$PROJECT_DIR/swift-frontend"
swift build -c release --arch arm64
echo "  ✅ Swift frontend built"

# 3. Copy Swift binary
cp "$(swift build -c release --arch arm64 --show-bin-path)/Clippy" "$APP_BUNDLE/Contents/MacOS/Clippy"

# 4. Copy resources - entire ui-prototype directory
cp -r "$PROJECT_DIR/ui-prototype" "$APP_BUNDLE/Contents/Resources/"

# 5. Create Info.plist
cat > "$APP_BUNDLE/Contents/Info.plist" << 'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>
    <string>Clippy</string>
    <key>CFBundleDisplayName</key>
    <string>Clippy</string>
    <key>CFBundleIdentifier</key>
    <string>com.iris.clippy-v2</string>
    <key>CFBundleVersion</key>
    <string>2.0.0</string>
    <key>CFBundleShortVersionString</key>
    <string>2.0.0</string>
    <key>CFBundleExecutable</key>
    <string>Clippy</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>LSUIElement</key>
    <true/>
    <key>LSMinimumSystemVersion</key>
    <string>13.0</string>
    <key>NSPrincipalClass</key>
    <string>NSApplication</string>
    <key>NSAppTransportSecurity</key>
    <dict>
        <key>NSAllowsLocalNetworking</key>
        <true/>
    </dict>
</dict>
</plist>
PLIST

echo ""
echo "✅ Build complete!"
echo "📂 App bundle: $APP_BUNDLE"
echo ""
echo "To run:"
echo "  open $APP_BUNDLE"
echo ""
echo "To install:"
echo "  cp -r $APP_BUNDLE /Applications/"
