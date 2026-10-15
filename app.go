package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"omnipack/pkg/detector"
	"omnipack/pkg/directinstaller"
	"omnipack/pkg/manifest"
	"omnipack/pkg/models"
	"omnipack/pkg/packagemanager"
)

// App struct
type App struct {
	ctx             context.Context
	systemInfo      models.SystemInfo
	manifestStore   *manifest.Store
	directInstaller *directinstaller.Installer
	packageManager  *packagemanager.Manager
}

// NewApp creates a new App application struct
func NewApp() *App {
	store, err := manifest.NewStore()
	if err != nil {
		fmt.Printf("Warning: failed to initialize manifest store: %v\n", err)
	}

	sysInfo := detector.DetectSystem()
	pkgMgr := packagemanager.NewManager(sysInfo)
	installer := directinstaller.NewInstaller(store)

	return &App{
		systemInfo:      sysInfo,
		manifestStore:   store,
		directInstaller: installer,
		packageManager:  pkgMgr,
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// GetSystemInfo returns detected OS and package manager details
func (a *App) GetSystemInfo() models.SystemInfo {
	return a.systemInfo
}

// LogMessage emits a log message to the frontend console
func (a *App) log(msg string) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "log:message", msg)
	}
}

// InstallFromURL downloads and installs an app from a web URL
func (a *App) InstallFromURL(url string, customName string) (models.InstallResult, error) {
	a.log(fmt.Sprintf("Starting installation from URL: %s", url))

	progressCb := func(update models.ProgressUpdate) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "install:progress", update)
		}
		a.log(fmt.Sprintf("[%s] %s (%.0f%%)", update.Step, update.Message, update.Percent))
	}

	result, err := a.directInstaller.InstallFromURL(url, customName, progressCb)
	if err != nil {
		a.log(fmt.Sprintf("Installation failed: %v", err))
		return models.InstallResult{}, err
	}

	a.log(fmt.Sprintf("Installed successfully! Binary: %s, Launcher: %s", result.Executable, result.DesktopFile))
	return result, nil
}

// InstallFromFile installs an app from a local archive or AppImage
func (a *App) InstallFromFile(filePath string, customName string) (models.InstallResult, error) {
	a.log(fmt.Sprintf("Starting installation from file: %s", filePath))

	progressCb := func(update models.ProgressUpdate) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "install:progress", update)
		}
		a.log(fmt.Sprintf("[%s] %s (%.0f%%)", update.Step, update.Message, update.Percent))
	}

	result, err := a.directInstaller.InstallFromFile(filePath, customName, "file://"+filePath, progressCb)
	if err != nil {
		a.log(fmt.Sprintf("Installation failed: %v", err))
		return models.InstallResult{}, err
	}

	a.log(fmt.Sprintf("Installed successfully! Binary: %s, Launcher: %s", result.Executable, result.DesktopFile))
	return result, nil
}

// SelectArchiveFile opens a native Linux file chooser dialog
func (a *App) SelectArchiveFile() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("context not ready")
	}

	file, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Package, Archive or AppImage",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "All Supported Packages (*.tar.gz, *.tar.xz, *.zip, *.AppImage, *.deb, *.rpm)",
				Pattern:     "*.tar.gz;*.tgz;*.tar.xz;*.tar.bz2;*.zip;*.AppImage;*.appimage;*.deb;*.rpm",
			},
			{
				DisplayName: "Archives (*.tar.gz, *.tar.xz, *.zip)",
				Pattern:     "*.tar.gz;*.tgz;*.tar.xz;*.tar.bz2;*.zip",
			},
			{
				DisplayName: "AppImages (*.AppImage)",
				Pattern:     "*.AppImage;*.appimage",
			},
			{
				DisplayName: "All Files (*.*)",
				Pattern:     "*.*",
			},
		},
	})

	return file, err
}

// GetInstalledApps lists all apps installed via Omnipack + Flatpak
func (a *App) GetInstalledApps() ([]models.InstalledApp, error) {
	allApps := []models.InstalledApp{}

	// Sideloaded / Direct apps
	if a.manifestStore != nil {
		if directApps, err := a.manifestStore.ListInstalled(); err == nil {
			allApps = append(allApps, directApps...)
		}
	}

	// Flatpak apps
	if a.packageManager != nil {
		if flatpakApps, err := a.packageManager.ListFlatpakApps(); err == nil {
			allApps = append(allApps, flatpakApps...)
		}
	}

	return allApps, nil
}

// UninstallApp cleanly removes an app
func (a *App) UninstallApp(appName string, source string) error {
	a.log(fmt.Sprintf("Uninstalling app '%s' (source: %s)...", appName, source))

	if source == "flatpak" {
		out, err := a.packageManager.UninstallPackage(appName, "flatpak")
		a.log(out)
		return err
	}

	if source == "pacman" || source == "apt" || source == "dnf" {
		out, err := a.packageManager.UninstallPackage(appName, source)
		a.log(out)
		return err
	}

	// Direct sideloaded app
	if a.manifestStore != nil {
		err := a.manifestStore.UninstallApp(appName)
		if err != nil {
			return err
		}
		a.log(fmt.Sprintf("Cleanly uninstalled %s (files, symlinks, desktop entry removed)", appName))
		return nil
	}

	return fmt.Errorf("unknown source or manifest store unavailable")
}

// SearchPackages searches across system repos and Flatpak
func (a *App) SearchPackages(query string) ([]models.PackageResult, error) {
	a.log(fmt.Sprintf("Searching for '%s'...", query))
	return a.packageManager.SearchPackages(query)
}

// InstallSystemPackage installs a repository package
func (a *App) InstallSystemPackage(pkgName string, backend string) (string, error) {
	a.log(fmt.Sprintf("Triggering installation for %s via %s...", pkgName, backend))
	out, err := a.packageManager.InstallPackage(pkgName, backend)
	if err != nil {
		a.log(fmt.Sprintf("Error: %v", err))
		return out, err
	}
	a.log(fmt.Sprintf("Success: %s installed!", pkgName))
	return out, nil
}

// LaunchApp launches the executable or desktop entry
func (a *App) LaunchApp(appName string) error {
	// Look up in manifest
	if a.manifestStore != nil {
		apps, err := a.manifestStore.ListInstalled()
		if err == nil {
			for _, app := range apps {
				if app.Name == appName {
					cmdPath := app.ExecutablePath
					if len(app.Symlinks) > 0 {
						cmdPath = app.Symlinks[0]
					}
					cmd := exec.Command(cmdPath)
					return cmd.Start()
				}
			}
		}
	}

	// Try running directly
	cmd := exec.Command(appName)
	return cmd.Start()
}

// OpenAppDirectory opens the application folder in the system file manager
func (a *App) OpenAppDirectory(appName string) error {
	if a.manifestStore != nil {
		apps, err := a.manifestStore.ListInstalled()
		if err == nil {
			for _, app := range apps {
				if app.Name == appName && app.InstallDir != "" {
					return exec.Command("xdg-open", app.InstallDir).Start()
				}
			}
		}
	}
	home, _ := os.UserHomeDir()
	return exec.Command("xdg-open", filepath.Join(home, ".local", "share", "lpm", "apps")).Start()
}

// CleanCache clears downloaded archive cache and returns stats
func (a *App) CleanCache() (map[string]interface{}, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	cacheDir := filepath.Join(home, ".cache", "lpm", "downloads")
	_ = os.RemoveAll(cacheDir)
	_ = os.MkdirAll(cacheDir, 0755)

	a.log("Cache directory cleared.")
	return map[string]interface{}{
		"success": true,
		"message": "Downloads cache cleaned successfully",
	}, nil
}
