package manifest

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"omnipack/pkg/models"
)

type Store struct {
	mu           sync.RWMutex
	manifestPath string
	appsDir      string
}

func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	baseDir := filepath.Join(home, ".local", "share", "lpm")
	appsDir := filepath.Join(baseDir, "apps")
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		return nil, err
	}

	binDir := filepath.Join(home, ".local", "bin")
	_ = os.MkdirAll(binDir, 0755)

	desktopDir := filepath.Join(home, ".local", "share", "applications")
	_ = os.MkdirAll(desktopDir, 0755)

	manifestPath := filepath.Join(baseDir, "manifest.json")
	store := &Store{
		manifestPath: manifestPath,
		appsDir:      appsDir,
	}

	// Initialize file if not exists
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		_ = store.saveRaw(map[string]models.InstalledApp{})
	}

	return store, nil
}

func (s *Store) GetAppsDir() string {
	return s.appsDir
}

func (s *Store) loadRaw() (map[string]models.InstalledApp, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]models.InstalledApp), nil
		}
		return nil, err
	}

	var apps map[string]models.InstalledApp
	if err := json.Unmarshal(data, &apps); err != nil {
		return make(map[string]models.InstalledApp), nil
	}
	return apps, nil
}

func (s *Store) saveRaw(apps map[string]models.InstalledApp) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(apps, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.manifestPath, data, 0644)
}

// SaveApp adds or updates an application record
func (s *Store) SaveApp(app models.InstalledApp) error {
	apps, err := s.loadRaw()
	if err != nil {
		return err
	}

	if app.InstalledAt.IsZero() {
		app.InstalledAt = time.Now()
	}

	if app.InstallDir != "" {
		app.SizeMB = calculateDirSizeMB(app.InstallDir)
	}

	apps[app.Name] = app
	return s.saveRaw(apps)
}

// ListInstalled returns all sideloaded/direct apps
func (s *Store) ListInstalled() ([]models.InstalledApp, error) {
	apps, err := s.loadRaw()
	if err != nil {
		return nil, err
	}

	result := make([]models.InstalledApp, 0, len(apps))
	for _, app := range apps {
		if app.InstallDir != "" {
			app.SizeMB = calculateDirSizeMB(app.InstallDir)
		}
		result = append(result, app)
	}
	return result, nil
}

// UninstallApp performs a 100% clean uninstallation of a direct installed app
func (s *Store) UninstallApp(name string) error {
	apps, err := s.loadRaw()
	if err != nil {
		return err
	}

	app, exists := apps[name]
	if !exists {
		return fmt.Errorf("app %s not found in manifest", name)
	}

	// 1. Remove all created symlinks
	for _, symlink := range app.Symlinks {
		_ = os.Remove(symlink)
	}

	// 2. Remove desktop entry
	if app.DesktopEntry != "" {
		_ = os.Remove(app.DesktopEntry)
	}

	// 3. Remove application installation directory
	if app.InstallDir != "" && app.InstallDir != "/" {
		_ = os.RemoveAll(app.InstallDir)
	}

	// 4. Update desktop database if available
	home, _ := os.UserHomeDir()
	desktopDir := filepath.Join(home, ".local", "share", "applications")
	_ = exec.Command("update-desktop-database", desktopDir).Run()

	// 5. Remove from manifest
	delete(apps, name)
	return s.saveRaw(apps)
}

func calculateDirSizeMB(path string) float64 {
	var totalBytes int64
	_ = filepath.Walk(path, func(_ string, info fs.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if !info.IsDir() {
			totalBytes += info.Size()
		}
		return nil
	})
	return float64(totalBytes) / (1024 * 1024)
}
