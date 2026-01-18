// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/vlebo/ctx/internal/config"
	"github.com/vlebo/ctx/pkg/types"
)

// switchVPN connects to VPN based on configuration.
func switchVPN(cfg *types.VPNConfig) error {
	if cfg == nil {
		return nil
	}

	switch cfg.Type {
	case types.VPNTypeOpenVPN:
		return switchOpenVPN(cfg)
	case types.VPNTypeWireGuard:
		return switchWireGuard(cfg)
	case types.VPNTypeTailscale:
		return switchTailscale(cfg)
	case types.VPNTypeCustom:
		return switchCustomVPN(cfg)
	default:
		return fmt.Errorf("unsupported VPN type: %s", cfg.Type)
	}
}

func switchOpenVPN(cfg *types.VPNConfig) error {
	if cfg.ConfigFile == "" {
		return fmt.Errorf("openvpn config_file is required")
	}

	// Expand ~ in path
	configPath := expandPath(cfg.ConfigFile)

	// Check if openvpn is available
	if _, err := exec.LookPath("openvpn"); err != nil {
		// Try nmcli (NetworkManager) as alternative
		if _, err := exec.LookPath("nmcli"); err != nil {
			return fmt.Errorf("neither openvpn nor nmcli found in PATH")
		}
		// Use NetworkManager to connect
		// Extract connection name from config file name
		connName := strings.TrimSuffix(filepath.Base(configPath), filepath.Ext(configPath))
		cmd := exec.Command("nmcli", "connection", "up", connName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Use openvpn directly (requires sudo)
	args := []string{
		"--config", configPath,
		"--daemon",
	}
	if cfg.AuthUserPass != "" {
		args = append(args, "--auth-user-pass", expandPath(cfg.AuthUserPass))
	}

	yellow := color.New(color.FgYellow)
	yellow.Println("⚠ OpenVPN requires sudo. You may be prompted for your password.")

	cmd := exec.Command("sudo", append([]string{"openvpn"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func switchWireGuard(cfg *types.VPNConfig) error {
	if cfg.Interface == "" {
		return fmt.Errorf("wireguard interface is required")
	}

	// Try wg-quick first
	if _, err := exec.LookPath("wg-quick"); err == nil {
		cmd := exec.Command("sudo", "wg-quick", "up", cfg.Interface)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			// Interface might already be up
			yellow := color.New(color.FgYellow)
			yellow.Printf("⚠ WireGuard interface '%s' may already be up\n", cfg.Interface)
		}
		return nil
	}

	// Try nmcli as alternative
	if _, err := exec.LookPath("nmcli"); err == nil {
		cmd := exec.Command("nmcli", "connection", "up", cfg.Interface)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	return fmt.Errorf("neither wg-quick nor nmcli found in PATH")
}

func switchTailscale(cfg *types.VPNConfig) error {
	if _, err := exec.LookPath("tailscale"); err != nil {
		return fmt.Errorf("tailscale command not found in PATH")
	}

	// Connect to Tailscale
	args := []string{"up"}
	if cfg.ExitNode != "" {
		args = append(args, "--exit-node", cfg.ExitNode)
	}

	// Try without sudo first
	cmd := exec.Command("tailscale", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Try with sudo
		cmd = exec.Command("sudo", append([]string{"tailscale"}, args...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return nil
}

func switchCustomVPN(cfg *types.VPNConfig) error {
	if cfg.ConnectCmd == "" {
		return fmt.Errorf("custom VPN connect_cmd is required")
	}

	cmd := exec.Command("sh", "-c", cfg.ConnectCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// disconnectVPN disconnects from VPN.
func disconnectVPN(cfg *types.VPNConfig) error {
	if cfg == nil {
		return nil
	}

	switch cfg.Type {
	case types.VPNTypeOpenVPN:
		// Find and kill only the openvpn process for this specific config
		cmd := exec.Command("pgrep", "-a", "openvpn")
		output, _ := cmd.Output()
		if len(output) == 0 {
			return nil // No openvpn running
		}

		configName := filepath.Base(expandPath(cfg.ConfigFile))
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		killed := false
		for _, line := range lines {
			// Check if this process is using our config file
			if strings.Contains(line, configName) {
				fields := strings.Fields(line)
				if len(fields) > 0 {
					pid := fields[0]
					killCmd := exec.Command("sudo", "kill", pid)
					if err := killCmd.Run(); err == nil {
						killed = true
					}
				}
			}
		}
		if !killed {
			// Config not found in running processes - maybe already disconnected
			return nil
		}
		return nil
	case types.VPNTypeWireGuard:
		if _, err := exec.LookPath("wg-quick"); err == nil {
			return exec.Command("sudo", "wg-quick", "down", cfg.Interface).Run()
		}
		return exec.Command("nmcli", "connection", "down", cfg.Interface).Run()
	case types.VPNTypeTailscale:
		// Try without sudo first, then with sudo
		cmd := exec.Command("tailscale", "down")
		if err := cmd.Run(); err != nil {
			// Try with sudo
			cmd = exec.Command("sudo", "tailscale", "down")
			cmd.Run() // Ignore errors - might already be disconnected
		}
		return nil
	case types.VPNTypeCustom:
		if cfg.DisconnectCmd != "" {
			return exec.Command("sh", "-c", cfg.DisconnectCmd).Run()
		}
		return nil
	}
	return nil
}

// switchVault configures HashiCorp Vault environment.
func switchVault(cfg *types.VaultConfig, browser *types.BrowserConfig, mgr *config.Manager, contextName string) error {
	if cfg == nil {
		return nil
	}

	if _, err := exec.LookPath("vault"); err != nil {
		// vault CLI not available, just use env vars
		return nil
	}

	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)

	// Check if we have a saved token for this context
	savedToken := mgr.LoadVaultToken(contextName)
	if savedToken != "" {
		// Verify the token is still valid
		if verifyVaultToken(cfg, savedToken) {
			green.Printf("✓ Vault: using saved token for '%s'\n", contextName)
			return nil
		}
		// Token is invalid, delete it and proceed with login
		yellow.Printf("• Vault: saved token expired, re-authenticating...\n")
		mgr.DeleteVaultToken(contextName)
	}

	// If OIDC auth is specified and auto_login is enabled, trigger a login
	if cfg.AuthMethod == types.VaultAuthOIDC && cfg.AutoLogin {
		if browser != nil {
			yellow.Printf("• Vault OIDC login - opening %s profile '%s'...\n", browser.Type, browser.Profile)
		} else {
			yellow.Println("• Vault OIDC login - opening browser...")
		}

		cmd := exec.Command("vault", "login", "-method=oidc", "-token-only")
		// Set VAULT_ADDR for this command since env vars aren't exported yet
		cmd.Env = append(os.Environ(), "VAULT_ADDR="+cfg.Address)
		if cfg.Namespace != "" {
			cmd.Env = append(cmd.Env, "VAULT_NAMESPACE="+cfg.Namespace)
		}
		// Set BROWSER env var to use the configured browser profile
		if browser != nil {
			browserCmd := getBrowserCommand(browser)
			if browserCmd != "" {
				cmd.Env = append(cmd.Env, "BROWSER="+browserCmd)
			}
		}
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr

		// Capture token from stdout
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("vault login failed: %w", err)
		}

		token := strings.TrimSpace(string(output))
		if token != "" {
			// Save token for this context
			if err := mgr.SaveVaultToken(contextName, token); err != nil {
				yellow.Printf("⚠ Failed to save vault token: %v\n", err)
			} else {
				green.Printf("✓ Vault: token saved for '%s'\n", contextName)
			}
		}
	}

	return nil
}

// verifyVaultToken checks if a Vault token is still valid.
func verifyVaultToken(cfg *types.VaultConfig, token string) bool {
	cmd := exec.Command("vault", "token", "lookup", "-format=json")
	cmd.Env = append(os.Environ(),
		"VAULT_ADDR="+cfg.Address,
		"VAULT_TOKEN="+token,
	)
	if cfg.Namespace != "" {
		cmd.Env = append(cmd.Env, "VAULT_NAMESPACE="+cfg.Namespace)
	}

	err := cmd.Run()
	return err == nil
}

// switchGit configures Git identity for the session.
func switchGit(cfg *types.GitConfig) error {
	if cfg == nil {
		return nil
	}

	if _, err := exec.LookPath("git"); err != nil {
		return nil // Git not available
	}

	// Set git config globally for this session via environment variables
	// This is handled in GenerateEnvVars, but we can also set them directly

	if cfg.UserName != "" {
		cmd := exec.Command("git", "config", "--global", "user.name", cfg.UserName)
		if err := cmd.Run(); err != nil {
			yellow := color.New(color.FgYellow)
			yellow.Printf("⚠ Failed to set git user.name: %v\n", err)
		}
	}

	if cfg.UserEmail != "" {
		cmd := exec.Command("git", "config", "--global", "user.email", cfg.UserEmail)
		if err := cmd.Run(); err != nil {
			yellow := color.New(color.FgYellow)
			yellow.Printf("⚠ Failed to set git user.email: %v\n", err)
		}
	}

	if cfg.SigningKey != "" {
		cmd := exec.Command("git", "config", "--global", "user.signingkey", cfg.SigningKey)
		cmd.Run()
	}

	if cfg.GPGSign {
		cmd := exec.Command("git", "config", "--global", "commit.gpgsign", "true")
		cmd.Run()
	}

	return nil
}

// switchDocker configures Docker registry and context.
func switchDocker(cfg *types.DockerRegistryConfig) error {
	if cfg == nil {
		return nil
	}

	if _, err := exec.LookPath("docker"); err != nil {
		return nil // Docker not available
	}

	// Switch Docker context if specified
	if cfg.Context != "" {
		cmd := exec.Command("docker", "context", "use", cfg.Context)
		if err := cmd.Run(); err != nil {
			yellow := color.New(color.FgYellow)
			yellow.Printf("⚠ Docker context '%s' not found\n", cfg.Context)
		}
	}

	// Login to registry if credentials are available
	if cfg.URL != "" && cfg.Username != "" && cfg.PasswordEnv != "" {
		password := os.Getenv(cfg.PasswordEnv)
		if password != "" {
			cmd := exec.Command("docker", "login", cfg.URL, "-u", cfg.Username, "--password-stdin")
			cmd.Stdin = strings.NewReader(password)
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				yellow := color.New(color.FgYellow)
				yellow.Printf("⚠ Docker login to %s failed\n", cfg.URL)
			}
		}
	}

	return nil
}

// switchNPM configures NPM registry.
func switchNPM(cfg *types.NPMConfig) error {
	if cfg == nil {
		return nil
	}

	if _, err := exec.LookPath("npm"); err != nil {
		return nil // npm not available
	}

	// Set registry
	if cfg.Registry != "" {
		var args []string
		if cfg.Scope != "" {
			args = []string{"config", "set", fmt.Sprintf("%s:registry", cfg.Scope), cfg.Registry}
		} else {
			args = []string{"config", "set", "registry", cfg.Registry}
		}
		cmd := exec.Command("npm", args...)
		if err := cmd.Run(); err != nil {
			yellow := color.New(color.FgYellow)
			yellow.Printf("⚠ Failed to set npm registry: %v\n", err)
		}
	}

	// Set auth token if available
	if cfg.AuthTokenEnv != "" {
		token := os.Getenv(cfg.AuthTokenEnv)
		if token != "" {
			// Get registry host for auth
			registryHost := cfg.Registry
			if registryHost == "" {
				registryHost = "registry.npmjs.org"
			}
			// Remove protocol
			registryHost = strings.TrimPrefix(registryHost, "https://")
			registryHost = strings.TrimPrefix(registryHost, "http://")

			cmd := exec.Command("npm", "config", "set", fmt.Sprintf("//%s/:_authToken", registryHost), token)
			cmd.Run()
		}
	}

	if cfg.AlwaysAuth {
		cmd := exec.Command("npm", "config", "set", "always-auth", "true")
		cmd.Run()
	}

	return nil
}

// expandPath expands ~ to home directory in a path.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// checkAnyVPNRunning checks if any VPN (other than the expected one) is running.
// Returns a description of what's running, or empty string if nothing.
func checkAnyVPNRunning(expected *types.VPNConfig) string {
	// Check for openvpn processes
	cmd := exec.Command("pgrep", "-a", "openvpn")
	if output, err := cmd.Output(); err == nil {
		lines := strings.TrimSpace(string(output))
		if lines != "" {
			// Extract config file name from the command line
			for line := range strings.SplitSeq(lines, "\n") {
				if strings.Contains(line, ".ovpn") || strings.Contains(line, ".conf") {
					parts := strings.FieldsSeq(line)
					for part := range parts {
						if strings.HasSuffix(part, ".ovpn") || strings.HasSuffix(part, ".conf") {
							return fmt.Sprintf("openvpn (%s)", filepath.Base(part))
						}
					}
				}
			}
			return "openvpn (unknown config)"
		}
	}

	// Check for wireguard interfaces
	cmd = exec.Command("wg", "show", "interfaces")
	if output, err := cmd.Output(); err == nil {
		ifaces := strings.TrimSpace(string(output))
		if ifaces != "" {
			return fmt.Sprintf("wireguard (%s)", ifaces)
		}
	}

	// Check tailscale
	cmd = exec.Command("tailscale", "status", "--json")
	if output, err := cmd.Output(); err == nil {
		if strings.Contains(string(output), `"BackendState":"Running"`) {
			return "tailscale"
		}
	}

	return ""
}

// checkVPNStatus checks if the VPN is currently connected.
func checkVPNStatus(cfg *types.VPNConfig) bool {
	if cfg == nil {
		return false
	}

	switch cfg.Type {
	case types.VPNTypeWireGuard:
		// Check if wireguard interface is up
		if cfg.Interface != "" {
			cmd := exec.Command("ip", "link", "show", cfg.Interface)
			if err := cmd.Run(); err == nil {
				return true
			}
		}
	case types.VPNTypeTailscale:
		// Check tailscale status
		cmd := exec.Command("tailscale", "status", "--json")
		output, err := cmd.Output()
		if err == nil && strings.Contains(string(output), `"BackendState":"Running"`) {
			return true
		}
	case types.VPNTypeOpenVPN:
		// Check if openvpn process is running with this specific config
		cmd := exec.Command("pgrep", "-a", "openvpn")
		output, err := cmd.Output()
		if err == nil {
			// Also verify a tun interface exists (process might be zombie)
			tunCheck := exec.Command("sh", "-c", "ip link show type tun 2>/dev/null | grep -q tun")
			if tunCheck.Run() != nil {
				return false // No tun interface, VPN not really connected
			}
			// If config file specified, check if this specific VPN is running
			if cfg.ConfigFile != "" {
				// Extract just the filename for comparison
				configName := filepath.Base(expandPath(cfg.ConfigFile))
				// Returns true if this config is running, false if different VPN is running
				return strings.Contains(string(output), configName)
			}
			// No specific config file, any openvpn counts as connected
			return true
		}
	case types.VPNTypeCustom:
		if cfg.StatusCmd != "" {
			cmd := exec.Command("sh", "-c", cfg.StatusCmd)
			if err := cmd.Run(); err == nil {
				return true
			}
		}
	}

	return false
}
