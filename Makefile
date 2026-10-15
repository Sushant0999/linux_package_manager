# ==============================================================================
# Omnipack — Makefile
# ==============================================================================

PKGNAME   := omnipack
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "1.0.0")
ARCH      := x86_64
WAILS     := $(shell go env GOPATH)/bin/wails
BUILD_TAG := webkit2_41

# Installation prefix
PREFIX    ?= $(HOME)/.local
BINDIR    := $(PREFIX)/bin
ICONDIR   := $(HOME)/.local/share/icons/hicolor/256x256/apps
DESKTOPDIR:= $(HOME)/.local/share/applications

.PHONY: all build dev install uninstall pkg-arch pkg-deb pkg-appimage \
        clean dist deps help

# ─── Default ──────────────────────────────────────────────────────────────────
all: build

help:
	@echo ""
	@echo "  Omnipack Build System"
	@echo "  ─────────────────────────────────────────────"
	@echo "  make build        Build production binary"
	@echo "  make dev          Start hot-reload dev mode"
	@echo "  make install      Install to ~/.local/bin"
	@echo "  make uninstall    Remove from ~/.local/bin"
	@echo "  make pkg-arch     Build local Arch package (.pkg.tar.zst)"
	@echo "  make pkg-deb      Build Debian .deb package"
	@echo "  make pkg-appimage Build portable .AppImage"
	@echo "  make dist         Build all distribution packages"
	@echo "  make clean        Remove build artifacts"
	@echo ""

# ─── Dependencies ─────────────────────────────────────────────────────────────
deps:
	@echo "==> Installing Wails CLI..."
	@export PATH="$$PATH:$(shell go env GOPATH)/bin"; \
	 go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
	@echo "==> Installing frontend dependencies..."
	@cd frontend && npm install

# ─── Build ────────────────────────────────────────────────────────────────────
build: deps
	@echo "==> Building $(PKGNAME) $(VERSION)..."
	@export PATH="$$PATH:$(shell go env GOPATH)/bin"; \
	 wails build -tags $(BUILD_TAG) -o $(PKGNAME)
	@echo "==> Binary: build/bin/$(PKGNAME)"
	@ls -lh build/bin/$(PKGNAME)

# ─── Dev ──────────────────────────────────────────────────────────────────────
dev:
	@export PATH="$$PATH:$(shell go env GOPATH)/bin"; \
	 wails dev -tags $(BUILD_TAG)

# ─── Install ──────────────────────────────────────────────────────────────────
install: build
	@echo "==> Installing to $(BINDIR)..."
	@mkdir -p $(BINDIR) $(ICONDIR) $(DESKTOPDIR)
	@install -m755 build/bin/$(PKGNAME) $(BINDIR)/$(PKGNAME)
	@install -m644 dist/icons/$(PKGNAME).png $(ICONDIR)/$(PKGNAME).png
	@sed "s|^Exec=.*|Exec=$(BINDIR)/$(PKGNAME)|g" dist/$(PKGNAME).desktop \
	    > $(DESKTOPDIR)/$(PKGNAME).desktop
	@update-desktop-database $(DESKTOPDIR) 2>/dev/null || true
	@echo "==> Installed! Run: $(PKGNAME)"

# ─── Uninstall ────────────────────────────────────────────────────────────────
uninstall:
	@rm -f $(BINDIR)/$(PKGNAME)
	@rm -f $(ICONDIR)/$(PKGNAME).png
	@rm -f $(DESKTOPDIR)/$(PKGNAME).desktop
	@update-desktop-database $(DESKTOPDIR) 2>/dev/null || true
	@echo "==> Uninstalled $(PKGNAME)"

# ─── Arch Package (.pkg.tar.zst via makepkg) ──────────────────────────────────
pkg-arch: build
	@echo "==> Building Arch package..."
	@mkdir -p dist/arch-build
	@cp dist/PKGBUILD dist/arch-build/PKGBUILD
	@cp -r . dist/arch-build/$(PKGNAME)-src
	@cd dist/arch-build && makepkg -si --noconfirm
	@echo "==> Package built in dist/arch-build/"

# ─── Debian Package (.deb) ────────────────────────────────────────────────────
pkg-deb: build
	@echo "==> Building .deb package..."
	@chmod +x dist/build_deb.sh
	@bash dist/build_deb.sh $(VERSION)

# ─── AppImage ─────────────────────────────────────────────────────────────────
pkg-appimage: build
	@echo "==> Building AppImage..."
	@chmod +x dist/build_appimage.sh
	@bash dist/build_appimage.sh $(VERSION)

# ─── Release archive (.tar.gz with binary + assets) ──────────────────────────
release-tar: build
	@echo "==> Creating release archive..."
	@mkdir -p dist/release/$(PKGNAME)-$(VERSION)-linux-$(ARCH)
	@cp build/bin/$(PKGNAME)          dist/release/$(PKGNAME)-$(VERSION)-linux-$(ARCH)/$(PKGNAME)
	@cp dist/$(PKGNAME).desktop       dist/release/$(PKGNAME)-$(VERSION)-linux-$(ARCH)/
	@cp dist/icons/$(PKGNAME).png     dist/release/$(PKGNAME)-$(VERSION)-linux-$(ARCH)/
	@cp install.sh                    dist/release/$(PKGNAME)-$(VERSION)-linux-$(ARCH)/
	@cp README.md                     dist/release/$(PKGNAME)-$(VERSION)-linux-$(ARCH)/
	@cd dist/release && tar -czf $(PKGNAME)-$(VERSION)-linux-$(ARCH).tar.gz \
	    $(PKGNAME)-$(VERSION)-linux-$(ARCH)/
	@sha256sum dist/release/$(PKGNAME)-$(VERSION)-linux-$(ARCH).tar.gz \
	    > dist/release/$(PKGNAME)-$(VERSION)-linux-$(ARCH).tar.gz.sha256
	@echo "==> dist/release/$(PKGNAME)-$(VERSION)-linux-$(ARCH).tar.gz"
	@echo "==> SHA256: $$(cat dist/release/$(PKGNAME)-$(VERSION)-linux-$(ARCH).tar.gz.sha256)"

# ─── All packages ─────────────────────────────────────────────────────────────
dist: release-tar pkg-appimage
	@echo ""
	@echo "==> Distribution packages:"
	@ls -lh dist/release/*.tar.gz 2>/dev/null || true
	@ls -lh *.AppImage 2>/dev/null || true
	@ls -lh *.deb 2>/dev/null || true

# ─── Clean ────────────────────────────────────────────────────────────────────
clean:
	@rm -rf build/bin/$(PKGNAME)
	@rm -rf dist/release
	@rm -rf dist/arch-build
	@rm -f *.AppImage *.deb *.pkg.tar.zst
	@echo "==> Clean."
