#!/usr/bin/env bash
# ==============================================================================
# Omnipack — AppImage Builder
# Produces a self-contained AppImage that runs on any Linux distro (glibc ≥ 2.31)
# Usage: ./dist/build_appimage.sh [version]
# ==============================================================================
set -euo pipefail

VERSION="${1:-1.0.0}"
PKGNAME="omnipack"
ARCH="x86_64"

echo "==> Building AppImage for $PKGNAME v$VERSION"

# --- 1. Build the binary (if not already built) ---
export PATH="$PATH:$(go env GOPATH)/bin"
export APPIMAGE_EXTRACT_AND_RUN="${APPIMAGE_EXTRACT_AND_RUN:-1}"
if [ ! -f "build/bin/$PKGNAME" ]; then
    if ! command -v wails &>/dev/null; then
        echo "==> Installing Wails CLI..."
        go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
    fi
    wails build -tags webkit2_41 -o "$PKGNAME"
fi

# --- 2. Download appimagetool if not present ---
APPIMAGETOOL="./appimagetool-x86_64.AppImage"
if [ ! -f "$APPIMAGETOOL" ]; then
    echo "==> Downloading appimagetool..."
    curl -fsSL "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage" \
        -o "$APPIMAGETOOL"
    chmod +x "$APPIMAGETOOL"
fi

# --- 3. Create AppDir structure ---
APPDIR="$(mktemp -d)"
mkdir -p "$APPDIR/usr/bin"
mkdir -p "$APPDIR/usr/share/applications"
mkdir -p "$APPDIR/usr/share/icons/hicolor/256x256/apps"

# --- 4. Install files into AppDir ---
cp "build/bin/$PKGNAME" "$APPDIR/usr/bin/$PKGNAME"
cp "dist/icons/$PKGNAME.png" "$APPDIR/usr/share/icons/hicolor/256x256/apps/$PKGNAME.png"
cp "dist/$PKGNAME.desktop" "$APPDIR/usr/share/applications/$PKGNAME.desktop"

# AppImage also needs these at the root of AppDir
cp "dist/icons/$PKGNAME.png" "$APPDIR/$PKGNAME.png"
cp "dist/$PKGNAME.desktop" "$APPDIR/$PKGNAME.desktop"

# AppRun entrypoint
cat > "$APPDIR/AppRun" <<'EOF'
#!/bin/bash
SELF=$(readlink -f "$0")
HERE="${SELF%/*}"
export PATH="$HERE/usr/bin:$PATH"
export LD_LIBRARY_PATH="$HERE/usr/lib:$LD_LIBRARY_PATH"
exec "$HERE/usr/bin/omnipack" "$@"
EOF
chmod +x "$APPDIR/AppRun"

# --- 5. Build the AppImage ---
OUT_APPIMAGE="${PKGNAME}-${VERSION}-${ARCH}.AppImage"
ARCH=$ARCH "$APPIMAGETOOL" "$APPDIR" "$OUT_APPIMAGE"
rm -rf "$APPDIR"

chmod +x "$OUT_APPIMAGE"
echo ""
echo "==> SUCCESS: $OUT_APPIMAGE"
echo "    Run with: ./$OUT_APPIMAGE"
echo "    Or install system-wide with omnipack itself:"
echo "      omnipack install ./$OUT_APPIMAGE"
