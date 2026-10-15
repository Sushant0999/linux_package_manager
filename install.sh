#!/usr/bin/env bash
# ==============================================================================
# Omnipack — Universal One-Line Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/.../install.sh | bash
#        or: bash install.sh [--version v1.0.0] [--prefix /usr/local]
# ==============================================================================
set -euo pipefail

PKGNAME="omnipack"
REPO="yourusername/omnipack"
VERSION="${VERSION:-latest}"
PREFIX="${PREFIX:-$HOME/.local}"
BINDIR="$PREFIX/bin"
ICONDIR="$HOME/.local/share/icons/hicolor/256x256/apps"
DESKTOPDIR="$HOME/.local/share/applications"
TMPDIR="$(mktemp -d)"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --version) VERSION="$2"; shift 2 ;;
        --prefix)  PREFIX="$2"; BINDIR="$PREFIX/bin"; shift 2 ;;
        --system)  PREFIX="/usr/local"; BINDIR="$PREFIX/bin";
                   ICONDIR="/usr/share/icons/hicolor/256x256/apps";
                   DESKTOPDIR="/usr/share/applications"; shift ;;
        -h|--help)
            echo "Usage: install.sh [--version v1.0.0] [--prefix /usr/local] [--system]"
            exit 0 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

# Colours
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; BOLD='\033[1m'; RESET='\033[0m'

info()    { echo -e "${BLUE}==>${RESET} ${BOLD}$*${RESET}"; }
success() { echo -e "${GREEN}✓${RESET} $*"; }
warn()    { echo -e "${YELLOW}⚠${RESET} $*"; }
die()     { echo -e "${RED}✕ ERROR:${RESET} $*" >&2; exit 1; }

cleanup() { rm -rf "$TMPDIR"; }
trap cleanup EXIT

echo ""
echo -e "${BOLD}  ╔═══════════════════════════════════╗${RESET}"
echo -e "${BOLD}  ║      Omnipack Installer v1.0      ║${RESET}"
echo -e "${BOLD}  ╚═══════════════════════════════════╝${RESET}"
echo ""

# --- Detect OS and architecture ---
OS="$(uname -s)"
ARCH="$(uname -m)"
[[ "$OS" != "Linux" ]] && die "Omnipack only supports Linux."

case "$ARCH" in
    x86_64)  ARCH_SLUG="x86_64" ;;
    aarch64) ARCH_SLUG="aarch64" ;;
    *) die "Unsupported architecture: $ARCH" ;;
esac

info "Detected: Linux $ARCH_SLUG"

# --- Check system dependencies ---
info "Checking dependencies..."
MISSING=()
for dep in webkit2gtk gtk3; do
    if command -v pkg-config &>/dev/null; then
        if ! pkg-config --exists "webkit2gtk-4.1" 2>/dev/null && \
           ! pkg-config --exists "webkit2gtk-4.0" 2>/dev/null; then
            MISSING+=("webkit2gtk")
            break
        fi
    fi
done

if [[ ${#MISSING[@]} -gt 0 ]]; then
    warn "Missing system dependencies: ${MISSING[*]}"
    warn "Install them with your package manager:"
    echo "  Arch:   sudo pacman -S webkit2gtk-4.1"
    echo "  Debian: sudo apt install libwebkit2gtk-4.1-0"
    echo "  Fedora: sudo dnf install webkit2gtk4.1"
    echo ""
fi

# --- Resolve version ---
if [[ "$VERSION" == "latest" ]]; then
    info "Fetching latest release from GitHub..."
    if command -v curl &>/dev/null; then
        VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
            | grep '"tag_name"' | sed 's/.*"tag_name": *"\(.*\)".*/\1/')
    elif command -v wget &>/dev/null; then
        VERSION=$(wget -qO- "https://api.github.com/repos/$REPO/releases/latest" \
            | grep '"tag_name"' | sed 's/.*"tag_name": *"\(.*\)".*/\1/')
    else
        die "curl or wget is required."
    fi
    [[ -z "$VERSION" ]] && die "Could not determine latest version."
fi

VERSION_CLEAN="${VERSION#v}"
info "Installing $PKGNAME $VERSION..."

# --- Download binary ---
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/${PKGNAME}-${VERSION_CLEAN}-linux-${ARCH_SLUG}.tar.gz"
ARCHIVE="$TMPDIR/$PKGNAME.tar.gz"

info "Downloading from: $DOWNLOAD_URL"
if command -v curl &>/dev/null; then
    curl -fsSL --progress-bar "$DOWNLOAD_URL" -o "$ARCHIVE"
else
    wget -q --show-progress "$DOWNLOAD_URL" -o "$ARCHIVE"
fi

# --- Extract ---
info "Extracting..."
tar -xzf "$ARCHIVE" -C "$TMPDIR"

# --- Install binary ---
mkdir -p "$BINDIR"
BINARY=$(find "$TMPDIR" -type f -name "$PKGNAME" | head -1)
[[ -z "$BINARY" ]] && die "Binary not found in archive."
install -m755 "$BINARY" "$BINDIR/$PKGNAME"
success "Binary installed: $BINDIR/$PKGNAME"

# --- Install icon ---
mkdir -p "$ICONDIR"
ICON=$(find "$TMPDIR" -name "*.png" | head -1)
if [[ -n "$ICON" ]]; then
    install -m644 "$ICON" "$ICONDIR/$PKGNAME.png"
    success "Icon installed: $ICONDIR/$PKGNAME.png"
fi

# --- Install .desktop launcher ---
mkdir -p "$DESKTOPDIR"
DESKTOP=$(find "$TMPDIR" -name "*.desktop" | head -1)
if [[ -n "$DESKTOP" ]]; then
    # Update Exec path to point to the installed binary
    sed "s|^Exec=.*|Exec=$BINDIR/$PKGNAME|g" "$DESKTOP" > "$DESKTOPDIR/$PKGNAME.desktop"
    success "Launcher installed: $DESKTOPDIR/$PKGNAME.desktop"
fi

# Refresh desktop database
command -v update-desktop-database &>/dev/null && \
    update-desktop-database "$DESKTOPDIR" 2>/dev/null || true

# --- Check if BINDIR is on PATH ---
echo ""
if [[ ":$PATH:" != *":$BINDIR:"* ]]; then
    warn "$BINDIR is not in your PATH."
    echo ""
    echo "  Add it by appending to your shell config:"
    echo -e "  ${BOLD}echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.bashrc && source ~/.bashrc${RESET}"
    echo ""
fi

echo -e "${GREEN}${BOLD}✓ Omnipack $VERSION installed successfully!${RESET}"
echo ""
echo "  Launch from terminal: omnipack"
echo "  Or find 'Omnipack' in your application menu."
echo ""
