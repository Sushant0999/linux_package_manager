package detector

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"omnipack/pkg/models"
)

// DetectSystem scans /etc/os-release and system PATH to build SystemInfo
func DetectSystem() models.SystemInfo {
	info := models.SystemInfo{
		Arch: runtime.GOARCH,
	}

	// 1. Read /etc/os-release or /usr/lib/os-release
	osReleasePaths := []string{"/etc/os-release", "/usr/lib/os-release"}
	for _, p := range osReleasePaths {
		if file, err := os.Open(p); err == nil {
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
					switch key {
					case "ID":
						info.DistroID = val
					case "PRETTY_NAME":
						info.DistroName = val
					case "VERSION_ID":
						info.DistroVersion = val
					}
				}
			}
			file.Close()
			break
		}
	}

	if info.DistroName == "" {
		info.DistroName = "Linux"
	}

	// 2. Kernel version
	if out, err := exec.Command("uname", "-r").Output(); err == nil {
		info.Kernel = strings.TrimSpace(string(out))
	}

	// 3. Detect Package Managers
	managers := []string{}
	checkBinary := func(name string) bool {
		_, err := exec.LookPath(name)
		return err == nil
	}

	// Native managers
	if checkBinary("pacman") {
		managers = append(managers, "pacman")
		if info.NativeManager == "" {
			info.NativeManager = "pacman"
		}
	}
	if checkBinary("apt") || checkBinary("apt-get") {
		managers = append(managers, "apt")
		if info.NativeManager == "" {
			info.NativeManager = "apt"
		}
	}
	if checkBinary("dnf") {
		managers = append(managers, "dnf")
		if info.NativeManager == "" {
			info.NativeManager = "dnf"
		}
	}
	if checkBinary("zypper") {
		managers = append(managers, "zypper")
		if info.NativeManager == "" {
			info.NativeManager = "zypper"
		}
	}
	if checkBinary("apk") {
		managers = append(managers, "apk")
		if info.NativeManager == "" {
			info.NativeManager = "apk"
		}
	}

	// Universal managers
	if checkBinary("flatpak") {
		managers = append(managers, "flatpak")
		info.HasFlatpak = true
	}
	if checkBinary("snap") {
		managers = append(managers, "snap")
		info.HasSnap = true
	}

	info.DetectedManagers = managers
	return info
}
