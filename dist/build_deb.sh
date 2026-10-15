#!/usr/bin/env bash
# ==============================================================================
# Omnipack — Debian/Ubuntu .deb package builder
# Produces: omnipack_1.0.0_amd64.deb
# Usage: ./dist/build_deb.sh [version]
# ==============================================================================
set -euo pipefail

VERSION="${1:-1.0.0}"
PKGNAME="omnipack"
ARCH="amd64"
DEB_DIR="$(mktemp -d)"
PKG_ROOT="$DEB_DIR/${PKGNAME}_${VERSION}_${ARCH}"

echo "==> Building .deb package: ${PKGNAME}_${VERSION}_${ARCH}.deb"

# --- 1. Build the binary first ---
export PATH="$PATH:$(go env GOPATH)/bin"
if ! command -v wails &>/dev/null; then
    echo "==> Installing Wails CLI..."
    go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
fi
echo "==> Compiling Omnipack..."
wails build -tags webkit2_41 -o "$PKGNAME"

# --- 2. Create Debian package directory structure ---
mkdir -p "$PKG_ROOT/DEBIAN"
mkdir -p "$PKG_ROOT/usr/bin"
mkdir -p "$PKG_ROOT/usr/share/applications"
mkdir -p "$PKG_ROOT/usr/share/pixmaps"
mkdir -p "$PKG_ROOT/usr/share/icons/hicolor/256x256/apps"
mkdir -p "$PKG_ROOT/usr/share/icons/hicolor/512x512/apps"
mkdir -p "$PKG_ROOT/usr/share/doc/$PKGNAME"

# --- 3. Install files ---
install -m755 "build/bin/$PKGNAME" "$PKG_ROOT/usr/bin/$PKGNAME"
install -m644 "dist/$PKGNAME.desktop" "$PKG_ROOT/usr/share/applications/$PKGNAME.desktop"
install -m644 "dist/icons/$PKGNAME.png" "$PKG_ROOT/usr/share/pixmaps/$PKGNAME.png"
install -m644 "dist/icons/$PKGNAME.png" "$PKG_ROOT/usr/share/icons/hicolor/256x256/apps/$PKGNAME.png"
install -m644 "dist/icons/$PKGNAME.png" "$PKG_ROOT/usr/share/icons/hicolor/512x512/apps/$PKGNAME.png"

# --- 4. Write control file ---
INSTALLED_SIZE=$(du -sk "$PKG_ROOT/usr" | cut -f1)
cat > "$PKG_ROOT/DEBIAN/control" <<EOF
Package: $PKGNAME
Version: $VERSION
Section: utils
Priority: optional
Architecture: $ARCH
Installed-Size: $INSTALLED_SIZE
Depends: libwebkit2gtk-4.1-0 | libwebkit2gtk-4.0-37, libgtk-3-0, glib2.0-bin
Recommends: flatpak, xdg-utils
Maintainer: Sushant Raj <sushant0999@github.com>
Homepage: https://github.com/Sushant0999/linux_package_manager
Description: Universal Linux Application & Package Manager
 Omnipack is a modern GUI application that lets you install, manage,
 and remove software on any Linux distribution. It supports:
  - Direct installation from URLs, .tar.gz, .tar.xz, .zip, .AppImage
  - Native package manager integration (apt, pacman, dnf)
  - Flatpak and Snap support
  - 100% clean uninstallation via manifest tracking
EOF

# --- 5. Postinst / prerm scripts ---
cat > "$PKG_ROOT/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
if command -v gtk-update-icon-cache &>/dev/null; then
    gtk-update-icon-cache -f -t /usr/share/icons/hicolor || true
fi
if command -v update-desktop-database &>/dev/null; then
    update-desktop-database /usr/share/applications || true
fi
EOF
chmod 755 "$PKG_ROOT/DEBIAN/postinst"

cat > "$PKG_ROOT/DEBIAN/prerm" <<'EOF'
#!/bin/sh
set -e
EOF
chmod 755 "$PKG_ROOT/DEBIAN/prerm"

# --- 6. Build the .deb ---
OUT_DEB="${PKGNAME}_${VERSION}_${ARCH}.deb"
if command -v dpkg-deb &>/dev/null; then
    dpkg-deb --build --root-owner-group "$PKG_ROOT" "$OUT_DEB"
elif command -v fakeroot &>/dev/null; then
    fakeroot dpkg-deb --build "$PKG_ROOT" "$OUT_DEB"
else
    echo "ERROR: dpkg-deb not found. Install 'dpkg' first."
    exit 1
fi

rm -rf "$DEB_DIR"
echo ""
echo "==> SUCCESS: $OUT_DEB"
echo "    Install with: sudo dpkg -i $OUT_DEB"
