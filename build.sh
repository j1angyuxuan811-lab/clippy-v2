#!/bin/bash
set -e

ROOT="$(cd "$(dirname "$0")" && pwd)"
APP="$ROOT/build/Clippy.app"
GO_BIN="$ROOT/go-backend"

echo "🔨 Building Go backend..."
cd "$GO_BIN"
go build -o clippy-server ./main.go
echo "✅ Go backend compiled"

echo "🔨 Building Swift frontend..."
cd "$ROOT/swift-frontend"
swift build 2>&1 | tail -3
echo "✅ Swift frontend compiled"

echo "📦 Assembling .app bundle..."
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS"
mkdir -p "$APP/Contents/Resources"

# Copy Swift binary
cp "$ROOT/swift-frontend/.build/debug/ClippyApp" "$APP/Contents/MacOS/Clippy"

# Copy Go backend binary
mkdir -p "$APP/Contents/Resources/go-backend"
cp "$GO_BIN/clippy-server" "$APP/Contents/Resources/go-backend/"

# Copy UI files
cp -r "$ROOT/ui-prototype" "$APP/Contents/Resources/"

echo "✅ Build complete: $APP"
echo ""
echo "Install:"
echo "  cp -r \"$APP\" /Applications/"
echo "  open /Applications/Clippy.app"
