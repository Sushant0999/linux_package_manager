package packagemanager

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"

	"omnipack/pkg/models"
)

type Manager struct {
	systemInfo models.SystemInfo
}

func NewManager(info models.SystemInfo) *Manager {
	return &Manager{systemInfo: info}
}

// SearchPackages searches across native package manager and flatpak
func (m *Manager) SearchPackages(query string) ([]models.PackageResult, error) {
	if query == "" {
		return []models.PackageResult{}, nil
	}

	results := []models.PackageResult{}

	// 1. Search native package manager
	switch m.systemInfo.NativeManager {
	case "pacman":
		if pkgs, err := searchPacman(query); err == nil {
			results = append(results, pkgs...)
		}
	case "apt":
		if pkgs, err := searchApt(query); err == nil {
			results = append(results, pkgs...)
		}
	case "dnf":
		if pkgs, err := searchDnf(query); err == nil {
			results = append(results, pkgs...)
		}
	}

	// 2. Search Flatpak if available
	if m.systemInfo.HasFlatpak {
		if pkgs, err := searchFlatpak(query); err == nil {
			results = append(results, pkgs...)
		}
	}

	return results, nil
}

// Search pacman
func searchPacman(query string) ([]models.PackageResult, error) {
	cmd := exec.Command("pacman", "-Ss", query)
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return nil, err
	}

	var results []models.PackageResult
	scanner := bufio.NewScanner(strings.NewReader(string(out)))

	var currentPkg *models.PackageResult
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			// Header line: repo/name version [installed]
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				namePart := parts[0]
				repoName := strings.Split(namePart, "/")
				pkgName := repoName[len(repoName)-1]
				version := parts[1]
				installed := strings.Contains(line, "[installed")

				currentPkg = &models.PackageResult{
					Name:      pkgName,
					Version:   version,
					Backend:   "pacman",
					Installed: installed,
				}
			}
		} else if currentPkg != nil {
			currentPkg.Description = strings.TrimSpace(line)
			results = append(results, *currentPkg)
			currentPkg = nil
			if len(results) >= 30 {
				break
			}
		}
	}

	return results, nil
}

// Search apt
func searchApt(query string) ([]models.PackageResult, error) {
	cmd := exec.Command("apt-cache", "search", query)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var results []models.PackageResult
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " - ", 2)
		if len(parts) == 2 {
			results = append(results, models.PackageResult{
				Name:        strings.TrimSpace(parts[0]),
				Description: strings.TrimSpace(parts[1]),
				Backend:     "apt",
			})
			if len(results) >= 30 {
				break
			}
		}
	}
	return results, nil
}

// Search dnf
func searchDnf(query string) ([]models.PackageResult, error) {
	cmd := exec.Command("dnf", "search", "-q", query)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var results []models.PackageResult
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " : ", 2)
		if len(parts) == 2 {
			results = append(results, models.PackageResult{
				Name:        strings.TrimSpace(parts[0]),
				Description: strings.TrimSpace(parts[1]),
				Backend:     "dnf",
			})
			if len(results) >= 30 {
				break
			}
		}
	}
	return results, nil
}

// Search flatpak
func searchFlatpak(query string) ([]models.PackageResult, error) {
	cmd := exec.Command("flatpak", "search", query)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var results []models.PackageResult
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	isFirstLine := true

	for scanner.Scan() {
		line := scanner.Text()
		if isFirstLine {
			isFirstLine = false
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) >= 3 {
			results = append(results, models.PackageResult{
				Name:        strings.TrimSpace(parts[1]), // App ID e.g. org.videolan.VLC
				Description: strings.TrimSpace(parts[0]) + " - " + strings.TrimSpace(parts[2]),
				Version:     strings.TrimSpace(parts[3]),
				Backend:     "flatpak",
			})
			if len(results) >= 30 {
				break
			}
		}
	}
	return results, nil
}

// ListFlatpakApps returns user and system installed Flatpak apps
func (m *Manager) ListFlatpakApps() ([]models.InstalledApp, error) {
	if !m.systemInfo.HasFlatpak {
		return []models.InstalledApp{}, nil
	}

	cmd := exec.Command("flatpak", "list", "--app", "--columns=name,application,version,size")
	out, err := cmd.Output()
	if err != nil {
		return []models.InstalledApp{}, nil
	}

	var apps []models.InstalledApp
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "\t")
		if len(parts) >= 2 {
			name := strings.TrimSpace(parts[0])
			appID := strings.TrimSpace(parts[1])
			version := ""
			if len(parts) >= 3 {
				version = strings.TrimSpace(parts[2])
			}

			apps = append(apps, models.InstalledApp{
				Name:        appID,
				DisplayName: name,
				Version:     version,
				Source:      "flatpak",
			})
		}
	}
	return apps, nil
}

// InstallPackage runs appropriate installer (using pkexec when needed)
func (m *Manager) InstallPackage(pkgName, backend string) (string, error) {
	var cmd *exec.Cmd

	switch backend {
	case "flatpak":
		cmd = exec.Command("flatpak", "install", "-y", "--user", pkgName)
	case "pacman":
		cmd = exec.Command("pkexec", "pacman", "-S", "--noconfirm", pkgName)
	case "apt":
		cmd = exec.Command("pkexec", "apt-get", "install", "-y", pkgName)
	case "dnf":
		cmd = exec.Command("pkexec", "dnf", "install", "-y", pkgName)
	default:
		return "", fmt.Errorf("unsupported package manager: %s", backend)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("install failed: %s (%w)", string(out), err)
	}
	return string(out), nil
}

// UninstallPackage runs uninstaller
func (m *Manager) UninstallPackage(pkgName, backend string) (string, error) {
	var cmd *exec.Cmd

	switch backend {
	case "flatpak":
		cmd = exec.Command("flatpak", "uninstall", "-y", pkgName)
	case "pacman":
		cmd = exec.Command("pkexec", "pacman", "-R", "--noconfirm", pkgName)
	case "apt":
		cmd = exec.Command("pkexec", "apt-get", "remove", "-y", pkgName)
	case "dnf":
		cmd = exec.Command("pkexec", "dnf", "remove", "-y", pkgName)
	default:
		return "", fmt.Errorf("unsupported package manager: %s", backend)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("uninstall failed: %s (%w)", string(out), err)
	}
	return string(out), nil
}
