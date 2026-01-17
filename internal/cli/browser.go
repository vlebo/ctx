// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/vlebo/ctx/pkg/types"
	"gopkg.in/ini.v1"
)

// chromeProfile represents a Chrome profile with its directory and name.
type chromeProfile struct {
	Dir  string
	Name string
}

// firefoxProfile represents a Firefox profile with its directory and name.
type firefoxProfile struct {
	Dir     string
	Name    string
	Default bool
}

// wslDetected and windowsUser are cached results for WSL detection to avoid
// repeated filesystem checks. These are initialized lazily using sync.Once.
var (
	wslDetected     bool
	wslDetectedOnce sync.Once
	windowsUser     string
	windowsUserOnce sync.Once
)

// isWSL returns true if running inside Windows Subsystem for Linux.
func isWSL() bool {
	wslDetectedOnce.Do(func() {
		// Check for WSL interop file (works for both WSL1 and WSL2)
		if _, err := os.Stat("/proc/sys/fs/binfmt_misc/WSLInterop"); err == nil {
			wslDetected = true
			return
		}
		// Fallback: check /proc/version for Microsoft/WSL
		if data, err := os.ReadFile("/proc/version"); err == nil {
			content := strings.ToLower(string(data))
			wslDetected = strings.Contains(content, "microsoft") || strings.Contains(content, "wsl")
		}
	})
	return wslDetected
}

// getWindowsUsername returns the Windows username when running in WSL.
func getWindowsUsername() string {
	windowsUserOnce.Do(func() {
		// Try to get username from cmd.exe
		cmd := exec.Command("/mnt/c/Windows/System32/cmd.exe", "/c", "echo %USERNAME%")
		if output, err := cmd.Output(); err == nil {
			windowsUser = strings.TrimSpace(string(output))
			return
		}
		// Fallback: try to find user directory in /mnt/c/Users
		entries, err := os.ReadDir("/mnt/c/Users")
		if err != nil {
			return
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			// Skip common system directories
			if name == "Public" || name == "Default" || name == "Default User" || name == "All Users" {
				continue
			}
			// Check if this looks like a real user directory
			if _, err := os.Stat(filepath.Join("/mnt/c/Users", name, "AppData")); err == nil {
				windowsUser = name
				return
			}
		}
	})
	return windowsUser
}

// getChromeConfigDir returns the Chrome config directory for the current OS.
func getChromeConfigDir() string {
	// Check for WSL first - use Windows Chrome config
	if isWSL() {
		winUser := getWindowsUsername()
		if winUser != "" {
			winChromeDir := filepath.Join("/mnt/c/Users", winUser, "AppData/Local/Google/Chrome/User Data")
			if _, err := os.Stat(winChromeDir); err == nil {
				return winChromeDir
			}
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "linux":
		// Try Chrome first, then Chromium
		chromeDir := filepath.Join(home, ".config", "google-chrome")
		if _, err := os.Stat(chromeDir); err == nil {
			return chromeDir
		}
		return filepath.Join(home, ".config", "chromium")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Google", "Chrome")
	}
	return ""
}

// getFirefoxConfigDir returns the Firefox config directory for the current OS.
// On Linux, checks snap location first, then regular location.
func getFirefoxConfigDir() string {
	// Check for WSL first - use Windows Firefox config
	if isWSL() {
		winUser := getWindowsUsername()
		if winUser != "" {
			winFirefoxDir := filepath.Join("/mnt/c/Users", winUser, "AppData/Roaming/Mozilla/Firefox")
			if _, err := os.Stat(filepath.Join(winFirefoxDir, "profiles.ini")); err == nil {
				return winFirefoxDir
			}
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "linux":
		// Check snap Firefox first (common on Ubuntu)
		snapDir := filepath.Join(home, "snap", "firefox", "common", ".mozilla", "firefox")
		if _, err := os.Stat(filepath.Join(snapDir, "profiles.ini")); err == nil {
			return snapDir
		}
		// Fall back to regular location
		return filepath.Join(home, ".mozilla", "firefox")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Firefox")
	}
	return ""
}

// listChromeProfiles returns all Chrome profiles with their friendly names.
func listChromeProfiles() ([]chromeProfile, error) {
	configDir := getChromeConfigDir()
	if configDir == "" {
		return nil, fmt.Errorf("chrome config directory not found")
	}

	entries, err := os.ReadDir(configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read chrome config: %w", err)
	}

	var profiles []chromeProfile
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Chrome profile directories are "Default", "Profile 1", "Profile 2", etc.
		if name != "Default" && !strings.HasPrefix(name, "Profile ") {
			continue
		}

		prefsPath := filepath.Join(configDir, name, "Preferences")
		data, err := os.ReadFile(prefsPath)
		if err != nil {
			continue
		}

		var prefs struct {
			Profile struct {
				Name string `json:"name"`
			} `json:"profile"`
		}
		if err := json.Unmarshal(data, &prefs); err != nil {
			continue
		}

		profiles = append(profiles, chromeProfile{
			Dir:  name,
			Name: prefs.Profile.Name,
		})
	}

	return profiles, nil
}

// listFirefoxProfiles returns all Firefox profiles from profiles.ini.
func listFirefoxProfiles() ([]firefoxProfile, error) {
	configDir := getFirefoxConfigDir()
	if configDir == "" {
		return nil, fmt.Errorf("firefox config directory not found")
	}

	profilesIni := filepath.Join(configDir, "profiles.ini")
	cfg, err := ini.Load(profilesIni)
	if err != nil {
		return nil, fmt.Errorf("failed to read profiles.ini: %w", err)
	}

	var profiles []firefoxProfile
	for _, section := range cfg.Sections() {
		if !strings.HasPrefix(section.Name(), "Profile") {
			continue
		}

		name := section.Key("Name").String()
		path := section.Key("Path").String()
		isRelative := section.Key("IsRelative").MustInt(1)
		isDefault := section.Key("Default").MustInt(0)

		if name == "" || path == "" {
			continue
		}

		dir := path
		if isRelative == 1 {
			dir = filepath.Join(configDir, path)
		}

		profiles = append(profiles, firefoxProfile{
			Dir:     dir,
			Name:    name,
			Default: isDefault == 1,
		})
	}

	return profiles, nil
}

// findChromeProfileDir finds the Chrome profile directory for a given profile name.
func findChromeProfileDir(profileName string) (string, error) {
	profiles, err := listChromeProfiles()
	if err != nil {
		return "", err
	}

	// First try exact match on friendly name
	for _, p := range profiles {
		if strings.EqualFold(p.Name, profileName) {
			return p.Dir, nil
		}
	}

	// Then try matching directory name directly
	for _, p := range profiles {
		if strings.EqualFold(p.Dir, profileName) {
			return p.Dir, nil
		}
	}

	// List available profiles in error message
	var available []string
	for _, p := range profiles {
		available = append(available, fmt.Sprintf("%s (%s)", p.Name, p.Dir))
	}
	return "", fmt.Errorf("chrome profile '%s' not found. Available: %s", profileName, strings.Join(available, ", "))
}

// findFirefoxProfileName finds the Firefox profile name for a given profile name.
func findFirefoxProfileName(profileName string) (string, error) {
	profiles, err := listFirefoxProfiles()
	if err != nil {
		return "", err
	}

	// First try exact match
	for _, p := range profiles {
		if strings.EqualFold(p.Name, profileName) {
			return p.Name, nil
		}
	}

	// List available profiles in error message
	var available []string
	for _, p := range profiles {
		available = append(available, p.Name)
	}
	return "", fmt.Errorf("firefox profile '%s' not found. Available: %s", profileName, strings.Join(available, ", "))
}

// getChromeCommand returns the Chrome executable for the current OS.
func getChromeCommand() string {
	// Check for WSL first - use Windows Chrome
	if isWSL() {
		// Try common Windows Chrome installation paths
		chromePaths := []string{
			"/mnt/c/Program Files/Google/Chrome/Application/chrome.exe",
			"/mnt/c/Program Files (x86)/Google/Chrome/Application/chrome.exe",
		}
		for _, path := range chromePaths {
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}

	switch runtime.GOOS {
	case "linux":
		// Try different Chrome executables
		for _, cmd := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
			if _, err := exec.LookPath(cmd); err == nil {
				return cmd
			}
		}
	case "darwin":
		return "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	}
	return "google-chrome"
}

// getFirefoxCommand returns the Firefox executable for the current OS.
func getFirefoxCommand() string {
	// Check for WSL first - use Windows Firefox
	if isWSL() {
		// Try common Windows Firefox installation paths
		firefoxPaths := []string{
			"/mnt/c/Program Files/Mozilla Firefox/firefox.exe",
			"/mnt/c/Program Files (x86)/Mozilla Firefox/firefox.exe",
		}
		for _, path := range firefoxPaths {
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}

	switch runtime.GOOS {
	case "linux":
		return "firefox"
	case "darwin":
		return "/Applications/Firefox.app/Contents/MacOS/firefox"
	}
	return "firefox"
}

// OpenURL opens a URL in the browser configured for the context.
// If no browser is configured, uses the system default browser.
func OpenURL(cfg *types.BrowserConfig, url string) error {
	if cfg == nil {
		// No browser config, use system default
		return openURLDefault(url)
	}

	switch cfg.Type {
	case types.BrowserChrome:
		return openURLChrome(cfg.Profile, url)
	case types.BrowserFirefox:
		return openURLFirefox(cfg.Profile, url)
	default:
		return openURLDefault(url)
	}
}

// openURLDefault opens a URL in the system default browser.
func openURLDefault(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	return cmd.Start()
}

// openURLChrome opens a URL in Chrome with the specified profile.
func openURLChrome(profileName string, url string) error {
	profileDir, err := findChromeProfileDir(profileName)
	if err != nil {
		return err
	}

	chromeCmd := getChromeCommand()
	args := []string{
		"--profile-directory=" + profileDir,
	}
	if url != "" {
		args = append(args, url)
	}

	cmd := exec.Command(chromeCmd, args...)
	return cmd.Start()
}

// openURLFirefox opens a URL in Firefox with the specified profile.
func openURLFirefox(profileName string, url string) error {
	profile, err := findFirefoxProfileName(profileName)
	if err != nil {
		return err
	}

	firefoxCmd := getFirefoxCommand()
	args := []string{"-P", profile}
	if url != "" {
		args = append(args, url)
	}

	cmd := exec.Command(firefoxCmd, args...)
	return cmd.Start()
}

// OpenBrowser opens the browser with the context's configured profile (no URL).
func OpenBrowser(cfg *types.BrowserConfig) error {
	if cfg == nil {
		return fmt.Errorf("no browser configured for this context")
	}

	switch cfg.Type {
	case types.BrowserChrome:
		return openURLChrome(cfg.Profile, "")
	case types.BrowserFirefox:
		return openURLFirefox(cfg.Profile, "")
	default:
		return fmt.Errorf("unsupported browser type: %s", cfg.Type)
	}
}
