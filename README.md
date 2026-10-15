# Omnipack — Universal Linux Application & Package Manager

<p align="center">
  <img src="dist/icons/omnipack.png" width="96" alt="Omnipack icon"/>
</p>

<p align="center">
  <strong>Install anything on any Linux distro — from URLs, archives, AppImages, and system repositories.</strong>
</p>

<p align="center">
  <a href="https://github.com/yourusername/omnipack/releases"><img src="https://img.shields.io/github/v/release/yourusername/omnipack?style=flat-square&color=6366f1" alt="Release"/></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="License"/></a>
  <img src="https://img.shields.io/badge/platform-Linux-informational?style=flat-square&logo=linux" alt="Platform"/>
  <img src="https://img.shields.io/badge/built_with-Go_%2B_Wails-00ADD8?style=flat-square&logo=go" alt="Go + Wails"/>
</p>

---

## Features

- **📥 Sideload Anything** — Install directly from URLs, `.tar.gz`, `.tar.xz`, `.zip`, `.AppImage`, `.deb`, `.rpm`, and standalone ELF binaries
- **🔍 Unified Search** — Search across `pacman`, `apt`, `dnf`, and `flatpak` from a single search bar
- **📦 App Manager** — View all installed apps (sideloaded + Flatpak + system) in one dashboard
- **🗑 Clean Uninstall** — Tracks every file, symlink, and desktop entry. Removes 100% on uninstall
- **🖥 Desktop Integration** — Auto-generates `.desktop` launchers and `$PATH` symlinks on install
- **🌐 Distro Agnostic** — Detects your distro and package managers automatically

## Install

### One-line installer (recommended)
```bash
curl -fsSL https://github.com/yourusername/omnipack/releases/latest/download/install.sh | bash
```

### Arch Linux (AUR)
```bash
yay -S omnipack
# or with paru:
paru -S omnipack
```

### Debian / Ubuntu
```bash
wget https://github.com/yourusername/omnipack/releases/latest/download/omnipack_1.0.0_amd64.deb
sudo dpkg -i omnipack_1.0.0_amd64.deb
```

### AppImage (any distro, portable)
```bash
wget https://github.com/yourusername/omnipack/releases/latest/download/omnipack-1.0.0-x86_64.AppImage
chmod +x omnipack-1.0.0-x86_64.AppImage
./omnipack-1.0.0-x86_64.AppImage
```

### Manual (tar.gz)
```bash
wget https://github.com/yourusername/omnipack/releases/latest/download/omnipack-1.0.0-linux-x86_64.tar.gz
tar -xzf omnipack-1.0.0-linux-x86_64.tar.gz
cd omnipack-1.0.0-linux-x86_64
bash install.sh
```

## System Requirements

| Component | Requirement |
|-----------|------------|
| OS | Linux (any distro) |
| Architecture | x86_64, aarch64 |
| Runtime | `webkit2gtk-4.1` or `webkit2gtk-4.0`, `gtk3` |
| Optional | `flatpak`, `snapd`, `xdg-utils` |

## Building from Source

### Prerequisites
- Go 1.22+
- Node.js 18+
- `gcc`, `pkg-config`
- `webkit2gtk-4.1` + `gtk3` dev headers

```bash
# Clone
git clone https://github.com/yourusername/omnipack.git
cd omnipack

# Install build tools & dependencies
make deps

# Build production binary
make build

# Install to ~/.local/bin (user-level, no sudo needed)
make install

# Development mode (hot-reload)
make dev
```

## Packaging

```bash
# Build all distribution packages at once
make dist

# Individual formats
make pkg-arch      # → Arch .pkg.tar.zst via makepkg
make pkg-deb       # → omnipack_1.0.0_amd64.deb
make pkg-appimage  # → omnipack-1.0.0-x86_64.AppImage
make release-tar   # → omnipack-1.0.0-linux-x86_64.tar.gz
```

## How Sideloading Works

```
URL / File
   │
   ▼
Download (streaming, with progress bar)
   │
   ▼
Safe Extraction (Zip-Slip protection for tar.gz, tar.xz, zip)
   │
   ▼
ELF Inspector (debug/elf scan — finds the Linux executable)
   │
   ▼
~/.local/bin/<name>          ← PATH symlink
~/.local/share/applications/ ← .desktop launcher (GNOME, KDE, rofi)
~/.local/share/lpm/apps/     ← App directory
manifest.json                ← Tracks all files for clean uninstall
```

## Project Structure

```
omnipack/
├── main.go                     # Wails window configuration
├── app.go                      # IPC bridge (all callable Go methods)
├── Makefile                    # Build, install, package targets
├── install.sh                  # Universal one-line installer
├── LICENSE
├── pkg/
│   ├── detector/               # OS and package manager detection
│   ├── directinstaller/        # URL/archive/AppImage install engine
│   ├── manifest/               # JSON manifest DB & clean uninstaller
│   ├── models/                 # Shared data types
│   └── packagemanager/         # pacman / apt / dnf / flatpak adapters
├── frontend/
│   ├── index.html
│   └── src/
│       ├── style.css           # Premium dark design system
│       └── main.js             # 4-panel SPA application
└── dist/
    ├── omnipack.desktop        # FreeDesktop launcher entry
    ├── icons/omnipack.png      # Application icon
    ├── PKGBUILD                # Arch AUR build script
    ├── omnipack.spec           # RPM spec for Fedora/RHEL
    ├── build_deb.sh            # Debian .deb builder
    └── build_appimage.sh       # AppImage builder
```

## License

MIT — see [LICENSE](LICENSE)
