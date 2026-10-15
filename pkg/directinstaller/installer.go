package directinstaller

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"debug/elf"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"omnipack/pkg/manifest"
	"omnipack/pkg/models"
)

type ProgressCallback func(update models.ProgressUpdate)

type Installer struct {
	store *manifest.Store
}

func NewInstaller(store *manifest.Store) *Installer {
	return &Installer{store: store}
}

// InstallFromURL downloads a package from URL and installs it
func (in *Installer) InstallFromURL(url string, customName string, onProgress ProgressCallback) (models.InstallResult, error) {
	if onProgress != nil {
		onProgress(models.ProgressUpdate{
			Step:    "downloading",
			Message: "Connecting to " + url,
			Percent: 0,
		})
	}

	// 1. Prepare download destination in cache
	home, err := os.UserHomeDir()
	if err != nil {
		return models.InstallResult{}, err
	}
	cacheDir := filepath.Join(home, ".cache", "lpm", "downloads")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return models.InstallResult{}, err
	}

	// Guess filename from URL
	urlPath := strings.Split(url, "?")[0]
	fileName := filepath.Base(urlPath)
	if fileName == "" || fileName == "/" || fileName == "." {
		fileName = "downloaded_package"
	}
	destPath := filepath.Join(cacheDir, fmt.Sprintf("%d_%s", time.Now().Unix(), fileName))

	// 2. Perform streaming HTTP download with progress
	resp, err := http.Get(url)
	if err != nil {
		return models.InstallResult{}, fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return models.InstallResult{}, fmt.Errorf("HTTP error %s when downloading %s", resp.Status, url)
	}

	outFile, err := os.Create(destPath)
	if err != nil {
		return models.InstallResult{}, err
	}
	defer outFile.Close()

	totalSize := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 64*1024)
	startTime := time.Now()

	for {
		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := outFile.Write(buf[:n]); wErr != nil {
				return models.InstallResult{}, wErr
			}
			downloaded += int64(n)

			if onProgress != nil {
				var percent float64
				if totalSize > 0 {
					percent = float64(downloaded) / float64(totalSize) * 100.0
				}
				elapsed := time.Since(startTime).Seconds()
				var speed string
				if elapsed > 0 {
					speedVal := float64(downloaded) / (1024 * 1024) / elapsed
					speed = fmt.Sprintf("%.2f MB/s", speedVal)
				}

				onProgress(models.ProgressUpdate{
					Step:       "downloading",
					Message:    fmt.Sprintf("Downloading %s (%.1f MB / %.1f MB)", fileName, float64(downloaded)/(1024*1024), float64(totalSize)/(1024*1024)),
					Percent:    percent,
					Downloaded: downloaded,
					Total:      totalSize,
					Speed:      speed,
				})
			}
		}
		if rErr != nil {
			if rErr == io.EOF {
				break
			}
			return models.InstallResult{}, rErr
		}
	}
	_ = outFile.Sync()
	outFile.Close()

	// 3. Install from downloaded local file
	res, err := in.InstallFromFile(destPath, customName, url, onProgress)
	// Cleanup download cache
	_ = os.Remove(destPath)
	return res, err
}

// InstallFromFile installs from a local archive, AppImage, or binary
func (in *Installer) InstallFromFile(filePath string, customName string, sourceURL string, onProgress ProgressCallback) (models.InstallResult, error) {
	if _, err := os.Stat(filePath); err != nil {
		return models.InstallResult{}, fmt.Errorf("file does not exist: %w", err)
	}

	baseName := filepath.Base(filePath)
	cleanName := sanitizeAppName(customName, baseName)

	if onProgress != nil {
		onProgress(models.ProgressUpdate{
			Step:    "extracting",
			Message: fmt.Sprintf("Preparing installation for %s...", cleanName),
			Percent: 10,
		})
	}

	// 1. Create target app directory ~/.local/share/lpm/apps/<cleanName>
	targetAppDir := filepath.Join(in.store.GetAppsDir(), cleanName)
	_ = os.RemoveAll(targetAppDir) // clean old if reinstalling
	if err := os.MkdirAll(targetAppDir, 0755); err != nil {
		return models.InstallResult{}, fmt.Errorf("failed to create app dir: %w", err)
	}

	// 2. Identify file format and extract / copy
	isAppImage := strings.HasSuffix(strings.ToLower(baseName), ".appimage")
	isZip := strings.HasSuffix(strings.ToLower(baseName), ".zip")
	isTarGz := strings.HasSuffix(strings.ToLower(baseName), ".tar.gz") || strings.HasSuffix(strings.ToLower(baseName), ".tgz")
	isTarXz := strings.HasSuffix(strings.ToLower(baseName), ".tar.xz")
	isTarBz2 := strings.HasSuffix(strings.ToLower(baseName), ".tar.bz2")

	var appType string

	if isAppImage {
		appType = "appimage"
		destBinary := filepath.Join(targetAppDir, cleanName+".AppImage")
		if err := copyFile(filePath, destBinary); err != nil {
			return models.InstallResult{}, err
		}
		_ = os.Chmod(destBinary, 0755)
	} else if isZip {
		appType = "zip"
		if err := extractZip(filePath, targetAppDir); err != nil {
			return models.InstallResult{}, fmt.Errorf("failed to extract zip: %w", err)
		}
	} else if isTarGz {
		appType = "tar.gz"
		if err := extractTarGz(filePath, targetAppDir); err != nil {
			return models.InstallResult{}, fmt.Errorf("failed to extract tar.gz: %w", err)
		}
	} else if isTarBz2 {
		appType = "tar.bz2"
		if err := extractTarBz2(filePath, targetAppDir); err != nil {
			return models.InstallResult{}, fmt.Errorf("failed to extract tar.bz2: %w", err)
		}
	} else if isTarXz {
		appType = "tar.xz"
		if err := extractTarXz(filePath, targetAppDir); err != nil {
			return models.InstallResult{}, fmt.Errorf("failed to extract tar.xz: %w", err)
		}
	} else {
		// Standalone executable or script
		appType = "binary"
		destBinary := filepath.Join(targetAppDir, cleanName)
		if err := copyFile(filePath, destBinary); err != nil {
			return models.InstallResult{}, err
		}
		_ = os.Chmod(destBinary, 0755)
	}

	if onProgress != nil {
		onProgress(models.ProgressUpdate{
			Step:    "scanning",
			Message: "Scanning for Linux ELF executables and desktop files...",
			Percent: 60,
		})
	}

	// 3. Inspect target directory for main executable, icons, and desktop entries
	executablePath, desktopFile, iconPath := inspectAppDir(targetAppDir, cleanName)
	if executablePath == "" {
		return models.InstallResult{}, fmt.Errorf("no runnable executable found in archive")
	}

	// Ensure executable permission
	_ = os.Chmod(executablePath, 0755)

	if onProgress != nil {
		onProgress(models.ProgressUpdate{
			Step:    "integrating",
			Message: fmt.Sprintf("Creating system launcher and PATH symlinks for %s...", cleanName),
			Percent: 85,
		})
	}

	// 4. Symlink to ~/.local/bin/<cleanName>
	home, _ := os.UserHomeDir()
	binDir := filepath.Join(home, ".local", "bin")
	symlinkPath := filepath.Join(binDir, cleanName)
	_ = os.Remove(symlinkPath) // remove previous if exists
	if err := os.Symlink(executablePath, symlinkPath); err != nil {
		return models.InstallResult{}, fmt.Errorf("failed to symlink binary: %w", err)
	}

	// 5. Desktop Entry Integration
	appsLauncherDir := filepath.Join(home, ".local", "share", "applications")
	finalDesktopPath := filepath.Join(appsLauncherDir, cleanName+".desktop")

	if desktopFile != "" {
		// Update existing desktop file with correct paths
		_ = updateDesktopEntry(desktopFile, finalDesktopPath, symlinkPath, iconPath, cleanName)
	} else {
		// Auto-generate modern .desktop file
		_ = generateDesktopEntry(finalDesktopPath, cleanName, symlinkPath, iconPath)
	}

	// Refresh desktop database if available
	_ = exec.Command("update-desktop-database", appsLauncherDir).Run()

	// 6. Record in Manifest Store
	installedApp := models.InstalledApp{
		Name:           cleanName,
		DisplayName:    strings.Title(cleanName),
		Version:        extractVersionFromFileName(baseName),
		Source:         appType,
		SourceURL:      sourceURL,
		InstallDir:     targetAppDir,
		ExecutablePath: executablePath,
		DesktopEntry:   finalDesktopPath,
		IconPath:       iconPath,
		InstalledAt:    time.Now(),
		Symlinks:       []string{symlinkPath},
	}
	_ = in.store.SaveApp(installedApp)

	if onProgress != nil {
		onProgress(models.ProgressUpdate{
			Step:    "complete",
			Message: fmt.Sprintf("Successfully installed %s! Ready to launch.", cleanName),
			Percent: 100,
		})
	}

	return models.InstallResult{
		Success:      true,
		AppName:      cleanName,
		Version:      installedApp.Version,
		Executable:   symlinkPath,
		DesktopFile:  finalDesktopPath,
		Symlinks:     []string{symlinkPath},
		InstalledDir: targetAppDir,
		Message:      "Application installed and integrated successfully",
	}, nil
}

// inspectAppDir searches for the best executable, icon, and desktop entry
func inspectAppDir(dir string, appName string) (string, string, string) {
	var candidates []string
	var desktopFile string
	var iconFile string

	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		lower := strings.ToLower(info.Name())

		// Detect desktop file
		if strings.HasSuffix(lower, ".desktop") && desktopFile == "" {
			desktopFile = p
		}

		// Detect icon file (.png or .svg)
		if (strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".svg")) && iconFile == "" {
			if strings.Contains(lower, "icon") || strings.Contains(lower, appName) || strings.Contains(p, "icons") {
				iconFile = p
			}
		}

		// Check if executable
		if isExecutable(p, info) {
			candidates = append(candidates, p)
		}
		return nil
	})

	if len(candidates) == 0 {
		return "", desktopFile, iconFile
	}

	// Prioritize:
	// 1. Matches appName exactly or appName.AppImage
	for _, c := range candidates {
		base := strings.ToLower(filepath.Base(c))
		if base == strings.ToLower(appName) || base == strings.ToLower(appName)+".appimage" {
			return c, desktopFile, iconFile
		}
	}

	// 2. Contains appName
	for _, c := range candidates {
		base := strings.ToLower(filepath.Base(c))
		if strings.Contains(base, strings.ToLower(appName)) {
			return c, desktopFile, iconFile
		}
	}

	// 3. In a 'bin' directory
	for _, c := range candidates {
		if strings.Contains(c, "/bin/") {
			return c, desktopFile, iconFile
		}
	}

	// Fallback to first executable candidate
	return candidates[0], desktopFile, iconFile
}

// isExecutable checks if file is an ELF binary or has executable bits/shebang
func isExecutable(path string, info os.FileInfo) bool {
	// Mode check
	if info.Mode()&0111 != 0 {
		// Test if it's an ELF file
		if file, err := os.Open(path); err == nil {
			defer file.Close()
			header := make([]byte, 4)
			if _, err := file.Read(header); err == nil {
				if bytes.Equal(header, []byte{0x7f, 'E', 'L', 'F'}) {
					// Verify ELF architecture
					if _, err := elf.Open(path); err == nil {
						return true
					}
				}
			}
		}

		// Or a script with shebang
		if file, err := os.Open(path); err == nil {
			defer file.Close()
			header := make([]byte, 2)
			if _, err := file.Read(header); err == nil {
				if string(header) == "#!" {
					return true
				}
			}
		}
	}
	return false
}

// Archive extractors
func extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	return untar(gzr, dest)
}

func extractTarBz2(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	bzr := bzip2.NewReader(f)
	return untar(bzr, dest)
}

func extractTarXz(src, dest string) error {
	// Use native tar -xJf to handle xz stream reliably
	cmd := exec.Command("tar", "-xJf", src, "-C", dest)
	return cmd.Run()
}

func untar(r io.Reader, dest string) error {
	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Zip-slip protection
		target := filepath.Join(dest, header.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(target, 0755)
		case tar.TypeReg, tar.TypeRegA:
			_ = os.MkdirAll(filepath.Dir(target), 0755)
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				continue
			}
			_, _ = io.Copy(outFile, tr)
			outFile.Close()
		case tar.TypeSymlink:
			_ = os.MkdirAll(filepath.Dir(target), 0755)
			_ = os.Remove(target)
			_ = os.Symlink(header.Linkname, target)
		}
	}
	return nil
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		target := filepath.Join(dest, f.Name)
		// Zip-slip protection
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)) {
			continue
		}

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0755)
			continue
		}

		_ = os.MkdirAll(filepath.Dir(target), 0755)
		rc, err := f.Open()
		if err != nil {
			continue
		}

		outFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			continue
		}

		_, _ = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func sanitizeAppName(customName, baseName string) string {
	if customName != "" {
		return strings.ToLower(strings.TrimSpace(customName))
	}
	name := strings.ToLower(baseName)
	for _, ext := range []string{".tar.gz", ".tar.xz", ".tar.bz2", ".tgz", ".zip", ".appimage", ".deb", ".rpm"} {
		name = strings.TrimSuffix(name, ext)
	}
	// Remove common architecture and OS tags
	for _, tag := range []string{"-linux-amd64", "-linux-x86_64", "_amd64", "_x86_64", "-linux64", "-linux", "_linux"} {
		name = strings.TrimSuffix(name, tag)
	}
	// Strip version numbers if at end e.g. "neovim-0.10.0" -> "neovim"
	parts := strings.Split(name, "-v")
	if len(parts) > 1 {
		return parts[0]
	}
	return name
}

func extractVersionFromFileName(fileName string) string {
	lower := strings.ToLower(fileName)
	for _, ext := range []string{".tar.gz", ".tar.xz", ".tar.bz2", ".tgz", ".zip", ".appimage"} {
		lower = strings.TrimSuffix(lower, ext)
	}
	parts := strings.Split(lower, "-")
	for _, p := range parts {
		if len(p) > 0 && (p[0] == 'v' || (p[0] >= '0' && p[0] <= '9')) {
			return p
		}
	}
	return "latest"
}

func generateDesktopEntry(destPath, appName, execPath, iconPath string) error {
	displayName := strings.Title(appName)
	icon := iconPath
	if icon == "" {
		icon = "application-x-executable"
	}

	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
GenericName=%s Application
Comment=Installed via Omnipack
Exec=%s
Icon=%s
Terminal=false
Categories=Utility;Application;
StartupNotify=true
`, displayName, displayName, execPath, icon)

	return os.WriteFile(destPath, []byte(content), 0644)
}

func updateDesktopEntry(sourcePath, destPath, execPath, iconPath, appName string) error {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return generateDesktopEntry(destPath, appName, execPath, iconPath)
	}

	lines := strings.Split(string(data), "\n")
	var newLines []string
	hasExec := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Exec=") {
			newLines = append(newLines, "Exec="+execPath)
			hasExec = true
		} else if strings.HasPrefix(trimmed, "Icon=") && iconPath != "" {
			newLines = append(newLines, "Icon="+iconPath)
		} else {
			newLines = append(newLines, line)
		}
	}

	if !hasExec {
		newLines = append(newLines, "Exec="+execPath)
	}

	return os.WriteFile(destPath, []byte(strings.Join(newLines, "\n")), 0644)
}
