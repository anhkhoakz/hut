#!/bin/bash

# Cross-compilation build script for hut
# Usage: ./build.sh [platforms...]
# If no platforms specified, builds for all supported platforms

set -e

# Default platforms if none specified
DEFAULT_PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "linux/386"
    "windows/amd64"
    "windows/386"
    "darwin/amd64"
    "darwin/arm64"
    "freebsd/amd64"
    "freebsd/arm64"
    "openbsd/amd64"
    "openbsd/arm64"
    "netbsd/amd64"
    "netbsd/arm64"
)

# Get platforms from command line or use defaults
if [ $# -eq 0 ]; then
    PLATFORMS=("${DEFAULT_PLATFORMS[@]}")
else
    PLATFORMS=("$@")
fi

# Create build directory
mkdir -p build

echo "Building hut for ${#PLATFORMS[@]} platform(s)..."

# Build for each platform
for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -r goos goarch <<< "$platform"

    # Determine file extension
    if [ "$goos" = "windows" ]; then
        ext=".exe"
    else
        ext=""
    fi

    # Build the binary
    echo "Building for $goos/$goarch..."
    GOOS="$goos" GOARCH="$goarch" go build -o "build/hut-$goos-$goarch$ext" .

    # Show file info
    if [ -f "build/hut-$goos-$goarch$ext" ]; then
        echo "  ✓ Created: build/hut-$goos-$goarch$ext"
        ls -lh "build/hut-$goos-$goarch$ext"
    else
        echo "  ✗ Failed to build for $goos/$goarch"
    fi
done

echo ""
echo "Build complete! Binaries are in the 'build/' directory:"
ls -la build/

# Create archives
echo ""
echo "Creating archives..."
cd build
for binary in hut-*; do
    platform=$(echo "$binary" | sed 's/hut-//')
    if [[ "$platform" == *"windows"* ]]; then
        echo "Creating $binary.zip..."
        zip "$binary.zip" "$binary" > /dev/null
    else
        echo "Creating $binary.tar.gz..."
        tar -czf "$binary.tar.gz" "$binary" > /dev/null
    fi
done

echo ""
echo "Archives created:"
ls -la *.zip *.tar.gz 2>/dev/null || echo "No archives created"
