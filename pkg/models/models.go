package models

import "time"

// SystemInfo represents detected distribution and package managers
type SystemInfo struct {
	DistroID        string   `json:"distroId"`
	DistroName      string   `json:"distroName"`
	DistroVersion   string   `json:"distroVersion"`
	Kernel          string   `json:"kernel"`
	Arch            string   `json:"arch"`
	NativeManager   string   `json:"nativeManager"`
	HasFlatpak      bool     `json:"hasFlatpak"`
	HasSnap         bool     `json:"hasSnap"`
	DetectedManagers []string `json:"detectedManagers"`
}

// ProgressUpdate emitted during download/install
type ProgressUpdate struct {
	Step       string  `json:"step"`       // "downloading", "extracting", "scanning", "integrating", "complete", "failed"
	Message    string  `json:"message"`
	Percent    float64 `json:"percent"`    // 0 to 100
	Downloaded int64   `json:"downloaded"` // bytes
	Total      int64   `json:"total"`      // bytes
	Speed      string  `json:"speed"`
}

// InstalledApp represents an application managed by Omnipack
type InstalledApp struct {
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	Version        string    `json:"version"`
	Source         string    `json:"source"`         // "url", "archive", "appimage", "flatpak", "native"
	SourceURL      string    `json:"sourceUrl,omitempty"`
	InstallDir     string    `json:"installDir,omitempty"`
	ExecutablePath string    `json:"executablePath,omitempty"`
	DesktopEntry   string    `json:"desktopEntry,omitempty"`
	IconPath       string    `json:"iconPath,omitempty"`
	InstalledAt    time.Time `json:"installedAt"`
	SizeMB         float64   `json:"sizeMb"`
	Symlinks       []string  `json:"symlinks,omitempty"`
}

// PackageResult represents search results from repositories
type PackageResult struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Backend     string `json:"backend"` // "pacman", "apt", "dnf", "flatpak", "snap"
	Installed   bool   `json:"installed"`
}

// InstallResult returned when an app is installed
type InstallResult struct {
	Success        bool     `json:"success"`
	AppName        string   `json:"appName"`
	Version        string   `json:"version"`
	Executable     string   `json:"executable"`
	DesktopFile    string   `json:"desktopFile"`
	Symlinks       []string `json:"symlinks"`
	InstalledDir   string   `json:"installedDir"`
	Message        string   `json:"message"`
}
