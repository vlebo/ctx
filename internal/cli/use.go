// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/vlebo/ctx/internal/cloud"
	"github.com/vlebo/ctx/internal/config"
)

var (
	confirmFlag bool
	exportFlag  bool
	replaceFlag bool
)

func newUseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use <name>",
		Short: "Switch to a context",
		Long: `Switch to a specified context. This will update environment variables,
switch cloud provider profiles, and configure orchestration tools.

For production environments, the --confirm flag is required, or you will be
prompted for confirmation.`,
		Args: cobra.ExactArgs(1),
		RunE: runUse,
	}

	cmd.Flags().BoolVar(&confirmFlag, "confirm", false, "Confirm switching to production environment")
	cmd.Flags().BoolVar(&exportFlag, "export", false, "Output environment variables for shell eval (used by shell hook)")
	cmd.Flags().BoolVar(&replaceFlag, "replace", false, "Deactivate previous context (disconnect VPN, stop tunnels) before switching")

	return cmd
}

func runUse(cmd *cobra.Command, args []string) error {
	contextName := args[0]

	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	// Deactivate previous context if --replace flag is set or auto_deactivate is enabled
	// This must run even with --export flag, because shell hook updates CTX_CURRENT after --export
	appConfig := mgr.GetAppConfig()
	shouldDeactivate := replaceFlag || (appConfig != nil && appConfig.AutoDeactivate)
	if shouldDeactivate {
		previousContext := os.Getenv("CTX_CURRENT")
		if previousContext != "" && previousContext != contextName {
			if err := deactivatePreviousContext(mgr, previousContext); err != nil {
				yellow := color.New(color.FgYellow)
				yellow.Fprintf(os.Stderr, "⚠ Failed to deactivate previous context: %v\n", err)
			}
		}
	}

	ctx, err := mgr.LoadContext(contextName)
	if err != nil {
		return fmt.Errorf("failed to load context: %w", err)
	}

	// Prevent using abstract/base contexts directly
	if ctx.Abstract {
		return fmt.Errorf("context '%s' is abstract (a base template) and cannot be used directly. Create a context that extends it", contextName)
	}

	// Validate context
	if err := config.ValidateContext(ctx); err != nil {
		return fmt.Errorf("invalid context configuration: %w", err)
	}

	// If --export flag, only output env vars (no side effects, no confirmation)
	// This must come BEFORE production confirmation so shell hook doesn't hang
	if exportFlag {
		envVars := mgr.GenerateEnvVars(ctx)
		for key, value := range envVars {
			fmt.Printf("export %s=%q\n", key, value)
		}
		return nil
	}

	// Check if production and require confirmation
	if ctx.IsProd() && !confirmFlag {
		if !confirmProductionSwitch(ctx) {
			return fmt.Errorf("aborted: production switch not confirmed")
		}
	}

	// Perform the context switch (with side effects like kubectl, gcloud, etc.)
	failures, err := switchContext(mgr, ctx)
	if err != nil {
		return err
	}

	// Print success message (skipping failures)
	printSwitchSuccess(ctx, failures)

	return nil
}

// confirmProductionSwitch prompts the user to confirm switching to a production context.
func confirmProductionSwitch(ctx *config.ContextConfig) bool {
	warning := color.New(color.FgRed, color.Bold)
	warning.Printf("⚠️  Switching to PRODUCTION environment: %s\n", ctx.Name)
	fmt.Print("   Type 'yes' to confirm: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	return strings.TrimSpace(strings.ToLower(input)) == "yes"
}

// switchContext performs all the operations to switch to a new context.
func switchContext(mgr *config.Manager, ctx *config.ContextConfig) ([]string, error) {
	yellow := color.New(color.FgYellow)
	var failures []string

	// Connect VPN first (if configured with auto_connect)
	if ctx.VPN != nil && ctx.VPN.AutoConnect {
		// Skip if this VPN is already connected
		if checkVPNStatus(ctx.VPN) {
			green := color.New(color.FgGreen)
			green.Fprintf(os.Stderr, "✓ VPN already connected\n")
		} else if err := switchVPN(ctx.VPN); err != nil {
			yellow.Fprintf(os.Stderr, "⚠ VPN connection failed: %v\n", err)
			failures = append(failures, "VPN")
		}
	}

	// Switch cloud providers
	if ctx.AWS != nil {
		if err := switchAWS(ctx.AWS, ctx.Browser, mgr, ctx.Name); err != nil {
			yellow.Fprintf(os.Stderr, "⚠ AWS switch failed: %v\n", err)
			failures = append(failures, "AWS")
		}
	}

	if ctx.GCP != nil {
		if err := switchGCP(ctx.GCP, ctx.Browser, mgr, ctx.Name); err != nil {
			yellow.Fprintf(os.Stderr, "⚠ GCP switch failed: %v\n", err)
			failures = append(failures, "GCP")
		}
	}

	if ctx.Azure != nil {
		if err := switchAzure(ctx.Azure, ctx.Browser, mgr, ctx.Name); err != nil {
			yellow.Fprintf(os.Stderr, "⚠ Azure switch failed: %v\n", err)
			failures = append(failures, "Azure")
		}
	}

	// Resolve secret files (before orchestration, so ${KUBECONFIG} etc. can be used)
	var secretFilePaths map[string]string
	if ctx.Secrets != nil && len(ctx.Secrets.Files) > 0 {
		// Get GCP config dir for per-context credentials
		sfGCPConfigDir := ""
		if ctx.GCP != nil {
			sfGCPConfigDir = mgr.GCPConfigDir(ctx.Name)
		}

		// Load cached AWS credentials if using aws-vault
		var sfAWSCreds *config.AWSCredentials
		if ctx.AWS != nil && ctx.AWS.UseVault {
			sfAWSCreds = mgr.LoadAWSCredentials(ctx.Name)
		}

		// Load vault token from keychain
		sfVaultToken := ""
		if ctx.Vault != nil {
			sfVaultToken = mgr.LoadVaultToken(ctx.Name)
		}

		sfResult, err := resolveSecretFiles(ctx.Secrets, mgr, ctx.Name, ctx.Bitwarden, ctx.OnePassword,
			ctx.Vault, sfVaultToken, ctx.AWS, sfAWSCreds, ctx.GCP, sfGCPConfigDir, ctx.Browser)
		if err != nil {
			yellow.Fprintf(os.Stderr, "⚠ Secret files resolution failed: %v\n", err)
			failures = append(failures, "SecretFiles")
		} else if sfResult != nil {
			secretFilePaths = sfResult.EnvVars
			// Add file paths to ctx.Env so they can be referenced via ${VAR}
			if ctx.Env == nil {
				ctx.Env = make(map[string]string)
			}
			maps.Copy(ctx.Env, secretFilePaths)
			// Re-run variable interpolation so ${KUBECONFIG} etc. resolve to temp file paths
			config.ExpandConfigVars(ctx)
		}
	}

	// Switch orchestration tools
	if ctx.Kubernetes != nil {
		if err := switchKubernetes(ctx.Kubernetes, ctx, mgr); err != nil {
			yellow.Fprintf(os.Stderr, "⚠ Kubernetes switch failed: %v\n", err)
			failures = append(failures, "Kubernetes")
		}
	}

	// Configure Vault
	if ctx.Vault != nil {
		if err := switchVault(ctx.Vault, ctx.Browser, mgr, ctx.Name); err != nil {
			yellow.Fprintf(os.Stderr, "⚠ Vault configuration failed: %v\n", err)
			failures = append(failures, "Vault")
		}
	}

	// Configure Git identity
	if ctx.Git != nil {
		if err := switchGit(ctx.Git); err != nil {
			yellow.Fprintf(os.Stderr, "⚠ Git configuration failed: %v\n", err)
		}
	}

	// Configure Docker
	if ctx.Docker != nil {
		if err := switchDocker(ctx.Docker); err != nil {
			yellow.Fprintf(os.Stderr, "⚠ Docker configuration failed: %v\n", err)
		}
	}

	// Configure NPM
	if ctx.NPM != nil {
		if err := switchNPM(ctx.NPM); err != nil {
			yellow.Fprintf(os.Stderr, "⚠ NPM configuration failed: %v\n", err)
		}
	}

	// Start auto-connect tunnels
	var failedTunnels []string
	if len(ctx.Tunnels) > 0 && ctx.SSH != nil && ctx.SSH.Bastion.Host != "" {
		var err error
		_, failedTunnels, err = startAutoConnectTunnels(mgr, ctx)
		if err != nil {
			yellow.Fprintf(os.Stderr, "⚠ Tunnel auto-connect failed: %v\n", err)
			failures = append(failures, "Tunnels")
		} else if len(failedTunnels) > 0 {
			failures = append(failures, "Tunnels")
		}
	}

	// Resolve secrets from all configured providers
	var secrets map[string]string
	var secretsResult *SecretsResult
	if ctx.Secrets != nil {
		// Get GCP config dir for per-context credentials
		gcpConfigDir := ""
		if ctx.GCP != nil {
			gcpConfigDir = mgr.GCPConfigDir(ctx.Name)
		}

		// Load cached AWS credentials if using aws-vault
		var awsCreds *config.AWSCredentials
		if ctx.AWS != nil && ctx.AWS.UseVault {
			awsCreds = mgr.LoadAWSCredentials(ctx.Name)
		}

		// Load vault token from keychain
		vaultToken := ""
		if ctx.Vault != nil {
			vaultToken = mgr.LoadVaultToken(ctx.Name)
		}

		var err error
		secretsResult, err = resolveAllSecrets(ctx.Secrets, mgr, ctx.Name, ctx.Bitwarden, ctx.OnePassword, ctx.Vault, vaultToken, ctx.AWS, awsCreds, ctx.GCP, gcpConfigDir, ctx.Browser)
		if err != nil {
			yellow.Fprintf(os.Stderr, "⚠ Secrets resolution failed: %v\n", err)
			failures = append(failures, "Secrets")
		} else if secretsResult != nil {
			secrets = secretsResult.Secrets
		}
	}

	// If there were failures, ask user if they want to continue
	if len(failures) > 0 {
		if !confirmContinueWithFailures(failures) {
			// Send abort event to cloud
			sendAbortEvent(mgr, ctx, failures)
			return failures, fmt.Errorf("aborted: %s failed", strings.Join(failures, ", "))
		}
	}

	// Set current context
	if err := mgr.SetCurrentContext(ctx.Name); err != nil {
		return failures, fmt.Errorf("failed to set current context: %w", err)
	}

	// Merge secret file paths into secrets map so they're included in the env file
	if len(secretFilePaths) > 0 {
		if secrets == nil {
			secrets = make(map[string]string)
		}
		maps.Copy(secrets, secretFilePaths)
	}

	// Write environment file for shell hook (including resolved secrets)
	if err := mgr.WriteEnvFileWithSecrets(ctx, secrets); err != nil {
		return failures, fmt.Errorf("failed to write environment file: %w", err)
	}

	// Send cloud audit event and start heartbeat (non-blocking, errors are logged)
	sendCloudEvents(mgr, ctx, failures)

	return failures, nil
}

// sendCloudEvents sends audit event and starts heartbeat to ctx-cloud.
// This is non-blocking and errors are logged but do not fail the context switch.
func sendCloudEvents(mgr *config.Manager, ctx *config.ContextConfig, failures []string) {
	client := NewCloudClient(mgr)
	if client == nil {
		return // Cloud integration not configured
	}

	appConfig := mgr.GetAppConfig()
	if appConfig == nil || appConfig.Cloud == nil {
		return
	}

	// Determine VPN status and tunnels
	vpnConnected := false
	var activeTunnels []string

	if ctx.VPN != nil && !slices.Contains(failures, "VPN") {
		vpnConnected = checkVPNStatus(ctx.VPN)
	}

	for _, tunnel := range ctx.Tunnels {
		if tunnel.AutoConnect {
			activeTunnels = append(activeTunnels, tunnel.Name)
		}
	}

	// Build details for audit event
	details := make(map[string]any)
	if ctx.AWS != nil {
		details["aws_profile"] = ctx.AWS.Profile
		details["aws_region"] = ctx.AWS.Region
	}
	if ctx.GCP != nil {
		details["gcp_project"] = ctx.GCP.Project
	}
	if ctx.Kubernetes != nil {
		details["k8s_context"] = ctx.Kubernetes.Context
	}
	if vpnConnected {
		details["vpn_connected"] = true
		if ctx.VPN != nil {
			details["vpn_type"] = string(ctx.VPN.Type)
		}
	}
	if len(activeTunnels) > 0 {
		details["tunnels"] = activeTunnels
	}
	if len(failures) > 0 {
		details["partial_failures"] = failures
	}

	// Send switch audit event (async to not block)
	// VPN and tunnel auto-connects are included in the switch event details,
	// not as separate events. Manual `ctx vpn connect` and `ctx tunnel up`
	// commands send their own events.
	//
	// Success is true if context was loaded (even with partial failures).
	// Success is false only if the switch was aborted.
	if appConfig.Cloud.SendAuditEvents {
		go func() {
			event := &cloud.AuditEvent{
				Action:      "switch",
				ContextName: ctx.Name,
				Environment: string(ctx.Environment),
				Details:     details,
				Success:     true, // Context was loaded (partial failures are in details)
			}
			if err := client.SendAuditEvent(event); err != nil {
				yellow := color.New(color.FgYellow)
				yellow.Fprintf(os.Stderr, "⚠ Cloud audit event failed: %v\n", err)
			}
		}()
	}

	// Start heartbeat
	if appConfig.Cloud.SendHeartbeat {
		interval := time.Duration(appConfig.Cloud.HeartbeatInterval) * time.Second
		if interval == 0 {
			interval = 30 * time.Second
		}

		hbMgr := cloud.NewHeartbeatManager(mgr.StateDir())
		if err := hbMgr.StartHeartbeat(client, ctx.Name, string(ctx.Environment), vpnConnected, activeTunnels, interval); err != nil {
			yellow := color.New(color.FgYellow)
			yellow.Fprintf(os.Stderr, "⚠ Cloud heartbeat failed: %v\n", err)
		}
	}
}

// sendAbortEvent sends an audit event when the user aborts a context switch due to failures.
func sendAbortEvent(mgr *config.Manager, ctx *config.ContextConfig, failures []string) {
	client := NewCloudClient(mgr)
	if client == nil {
		return
	}

	appConfig := mgr.GetAppConfig()
	if appConfig == nil || appConfig.Cloud == nil || !appConfig.Cloud.SendAuditEvents {
		return
	}

	details := map[string]any{
		"partial_failures": failures,
		"aborted":          true,
	}

	event := &cloud.AuditEvent{
		Action:       "switch",
		ContextName:  ctx.Name,
		Environment:  string(ctx.Environment),
		Details:      details,
		Success:      false, // Aborted - context was NOT loaded
		ErrorMessage: fmt.Sprintf("Aborted: %s failed", strings.Join(failures, ", ")),
	}

	// Send synchronously since we're about to exit anyway
	if err := client.SendAuditEvent(event); err != nil {
		yellow := color.New(color.FgYellow)
		yellow.Fprintf(os.Stderr, "⚠ Cloud audit event failed: %v\n", err)
	}
}

// confirmContinueWithFailures asks the user if they want to continue loading the context despite failures.
func confirmContinueWithFailures(failures []string) bool {
	yellow := color.New(color.FgYellow, color.Bold)
	yellow.Printf("\n⚠ The following failed: %s\n", strings.Join(failures, ", "))
	fmt.Print("   Do you still want to load the context? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	answer := strings.TrimSpace(strings.ToLower(input))
	return answer == "y" || answer == "yes"
}

// switchAWS configures AWS environment.
func switchAWS(cfg *config.AWSConfig, browser *config.BrowserConfig, mgr *config.Manager, contextName string) error {
	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)

	// Warn if using standard profile without SSO or aws-vault
	if !cfg.UseVault && !cfg.SSOLogin && cfg.Profile != "" {
		yellow.Fprintf(os.Stderr, "⚠ AWS: using profile '%s' from ~/.aws/credentials (plaintext)\n", cfg.Profile)
		fmt.Fprintf(os.Stderr, "  Consider using 'sso_login: true' or 'use_vault: true' for better security\n")
	}

	// Handle aws-vault mode
	if cfg.UseVault {
		if _, err := exec.LookPath("aws-vault"); err != nil {
			return fmt.Errorf("aws-vault is required but not found in PATH")
		}

		// Check if we already have valid cached credentials
		if creds := mgr.LoadAWSCredentials(contextName); creds != nil {
			green.Printf("✓ AWS: using cached aws-vault credentials for '%s'\n", contextName)
			return nil
		}

		// Get new credentials from aws-vault
		yellow.Printf("• AWS: getting credentials from aws-vault for profile '%s'...\n", cfg.Profile)

		cmd := exec.Command("aws-vault", "exec", cfg.Profile, "--json")
		// Set region for STS calls if configured
		if cfg.Region != "" {
			cmd.Env = append(os.Environ(), "AWS_REGION="+cfg.Region, "AWS_DEFAULT_REGION="+cfg.Region)
		}
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("aws-vault exec failed: %w", err)
		}

		// Parse the JSON credentials
		var creds config.AWSCredentials
		if err := parseAWSVaultOutput(output, &creds); err != nil {
			return fmt.Errorf("failed to parse aws-vault output: %w", err)
		}

		// Save credentials for later use
		if err := mgr.SaveAWSCredentials(contextName, &creds); err != nil {
			yellow.Fprintf(os.Stderr, "⚠ Failed to cache AWS credentials: %v\n", err)
		} else {
			green.Printf("✓ AWS: credentials cached for '%s'\n", contextName)
		}

		return nil
	}

	// SSO login if configured
	if cfg.SSOLogin {
		if _, err := exec.LookPath("aws"); err != nil {
			return fmt.Errorf("aws CLI is required for SSO login but not found in PATH")
		}

		if browser != nil {
			yellow.Printf("• AWS SSO login - opening %s profile '%s'...\n", browser.Type, browser.Profile)
		} else {
			yellow.Println("• AWS SSO login - opening browser...")
		}

		args := []string{"sso", "login"}
		if cfg.Profile != "" {
			args = append(args, "--profile", cfg.Profile)
		}

		cmd := exec.Command("aws", args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Set BROWSER env var to use the configured browser profile
		if browser != nil {
			browserCmd := getBrowserCommand(browser)
			if browserCmd != "" {
				cmd.Env = append(os.Environ(), "BROWSER="+browserCmd)
			}
		}

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("aws sso login failed: %w", err)
		}
	}

	return nil
}

// parseAWSVaultOutput parses the JSON output from aws-vault exec --json.
func parseAWSVaultOutput(data []byte, creds *config.AWSCredentials) error {
	// aws-vault outputs JSON directly
	return json.Unmarshal(data, creds)
}

// switchGCP configures GCP environment.
func switchGCP(cfg *config.GCPConfig, browser *config.BrowserConfig, mgr *config.Manager, contextName string) error {
	// Check if gcloud is available
	if _, err := exec.LookPath("gcloud"); err != nil {
		// gcloud not available, just set env vars
		return nil
	}

	// Ensure per-context config directory exists
	if err := mgr.EnsureGCPConfigDir(contextName); err != nil {
		return fmt.Errorf("failed to create GCP config directory: %w", err)
	}

	gcpConfigDir := mgr.GCPConfigDir(contextName)
	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)

	// Check if already authenticated in this context's config
	if isGCPAuthenticated(gcpConfigDir) {
		green.Printf("✓ GCP: using saved credentials for '%s'\n", contextName)
	} else if cfg.AutoLogin {
		// Need to login
		if browser != nil {
			yellow.Printf("• GCP login - opening %s profile '%s'...\n", browser.Type, browser.Profile)
		} else {
			yellow.Println("• GCP login - opening browser...")
		}

		cmd := exec.Command("gcloud", "auth", "login")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Set per-context config directory
		cmd.Env = append(os.Environ(), "CLOUDSDK_CONFIG="+gcpConfigDir)

		// Set BROWSER env var to use the configured browser profile
		if browser != nil {
			browserCmd := getBrowserCommand(browser)
			if browserCmd != "" {
				cmd.Env = append(cmd.Env, "BROWSER="+browserCmd)
			}
		}

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("gcloud auth login failed: %w", err)
		}
		green.Printf("✓ GCP: credentials saved for '%s'\n", contextName)
	}

	// Optionally activate the config if it exists
	if cfg.ConfigName != "" {
		cmd := exec.Command("gcloud", "config", "configurations", "activate", cfg.ConfigName)
		cmd.Env = append(os.Environ(), "CLOUDSDK_CONFIG="+gcpConfigDir)
		if err := cmd.Run(); err != nil {
			// Config might not exist, print warning but continue
			yellow.Fprintf(os.Stderr, "⚠ gcloud config '%s' not found (will use env vars)\n", cfg.ConfigName)
		}
	}

	// Set project if specified
	if cfg.Project != "" {
		cmd := exec.Command("gcloud", "config", "set", "project", cfg.Project)
		cmd.Env = append(os.Environ(), "CLOUDSDK_CONFIG="+gcpConfigDir)
		cmd.Run() // Ignore error - project setting is also done via env vars
	}

	return nil
}

// isGCPAuthenticated checks if there are valid GCP credentials in the config directory.
func isGCPAuthenticated(configDir string) bool {
	// Check if credentials file exists
	credPath := filepath.Join(configDir, "credentials.db")
	if _, err := os.Stat(credPath); err == nil {
		// Verify by running a simple gcloud command
		cmd := exec.Command("gcloud", "auth", "list", "--format=value(account)")
		cmd.Env = append(os.Environ(), "CLOUDSDK_CONFIG="+configDir)
		output, err := cmd.Output()
		return err == nil && len(strings.TrimSpace(string(output))) > 0
	}
	return false
}

// switchAzure configures Azure environment.
func switchAzure(cfg *config.AzureConfig, browser *config.BrowserConfig, mgr *config.Manager, contextName string) error {
	// Check if az CLI is available
	if _, err := exec.LookPath("az"); err != nil {
		// az not available, just set env vars
		return nil
	}

	// Ensure per-context config directory exists
	if err := mgr.EnsureAzureConfigDir(contextName); err != nil {
		return fmt.Errorf("failed to create Azure config directory: %w", err)
	}

	azureConfigDir := mgr.AzureConfigDir(contextName)
	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)

	// Check if already authenticated in this context's config
	if isAzureAuthenticated(azureConfigDir) {
		green.Printf("✓ Azure: using saved credentials for '%s'\n", contextName)
	} else if cfg.AutoLogin {
		// Need to login
		if browser != nil {
			yellow.Printf("• Azure login - opening %s profile '%s'...\n", browser.Type, browser.Profile)
		} else {
			yellow.Println("• Azure login - opening browser...")
		}

		args := []string{"login", "--output", "none"}
		if cfg.TenantID != "" {
			args = append(args, "--tenant", cfg.TenantID)
		}

		cmd := exec.Command("az", args...)
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr

		// Set per-context config directory and disable interactive subscription picker
		cmd.Env = append(os.Environ(),
			"AZURE_CONFIG_DIR="+azureConfigDir,
			"AZURE_CORE_LOGIN_EXPERIENCE_V2=off", // Skip interactive subscription selection
		)

		// Set BROWSER env var to use the configured browser profile
		if browser != nil {
			browserCmd := getBrowserCommand(browser)
			if browserCmd != "" {
				cmd.Env = append(cmd.Env, "BROWSER="+browserCmd)
			}
		}

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("az login failed: %w", err)
		}
		green.Printf("✓ Azure: credentials saved for '%s'\n", contextName)
	}

	// Set the subscription
	if cfg.SubscriptionID != "" {
		cmd := exec.Command("az", "account", "set", "--subscription", cfg.SubscriptionID)
		cmd.Env = append(os.Environ(), "AZURE_CONFIG_DIR="+azureConfigDir)
		cmd.Stderr = os.Stderr
		// Ignore error - subscription might not be accessible
		cmd.Run()
	}

	return nil
}

// isAzureAuthenticated checks if there are valid Azure credentials in the config directory.
func isAzureAuthenticated(configDir string) bool {
	// Check if accessTokens.json or msal_token_cache.json exists
	tokenPaths := []string{
		filepath.Join(configDir, "msal_token_cache.json"),
		filepath.Join(configDir, "accessTokens.json"),
	}

	for _, tokenPath := range tokenPaths {
		if _, err := os.Stat(tokenPath); err == nil {
			// Verify by running az account show
			cmd := exec.Command("az", "account", "show", "--output", "none")
			cmd.Env = append(os.Environ(), "AZURE_CONFIG_DIR="+configDir)
			if err := cmd.Run(); err == nil {
				return true
			}
		}
	}
	return false
}

// getBrowserCommand returns a path to a wrapper script for opening URLs in the specified browser profile.
// This is needed because BROWSER env var parsing varies across tools.
func getBrowserCommand(cfg *config.BrowserConfig) string {
	if cfg == nil {
		return ""
	}

	var browserCmd string
	var args string

	switch cfg.Type {
	case config.BrowserChrome:
		profileDir, err := findChromeProfileDir(cfg.Profile)
		if err != nil {
			return ""
		}
		browserCmd = getChromeCommand()
		args = fmt.Sprintf("--profile-directory=\"%s\"", profileDir)
	case config.BrowserFirefox:
		profile, err := findFirefoxProfileName(cfg.Profile)
		if err != nil {
			return ""
		}
		browserCmd = getFirefoxCommand()
		args = fmt.Sprintf("-P \"%s\"", profile)
	default:
		return ""
	}

	// Create a wrapper script that tools can use
	// This ensures proper handling of arguments and URLs
	script := fmt.Sprintf("#!/bin/sh\nexec \"%s\" %s \"$@\"\n", browserCmd, args)

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "ctx-browser-*.sh")
	if err != nil {
		return ""
	}
	tmpFile.WriteString(script)
	tmpFile.Chmod(0o755)
	tmpFile.Close()

	return tmpFile.Name()
}

// switchKubernetes fetches credentials (if cloud config present) and switches the kubectl context.
func switchKubernetes(cfg *config.KubernetesConfig, ctx *config.ContextConfig, mgr *config.Manager) error {
	// Check if kubectl is available
	if _, err := exec.LookPath("kubectl"); err != nil {
		// kubectl not available, skip context switch
		return nil
	}

	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)

	// Expand kubeconfig path if specified
	kubeconfigPath := cfg.Kubeconfig
	if kubeconfigPath != "" {
		kubeconfigPath = expandPath(kubeconfigPath)
	}

	// Fetch credentials from cloud provider if configured
	var fetchedContextName string
	var fetchErr error

	if cfg.AKS != nil {
		azureConfigDir := ""
		if ctx.Azure != nil {
			azureConfigDir = mgr.AzureConfigDir(ctx.Name)
		}
		fetchedContextName, fetchErr = fetchAKSCredentials(cfg.AKS, azureConfigDir, kubeconfigPath)
		if fetchErr != nil {
			yellow.Fprintf(os.Stderr, "⚠ AKS credential fetch failed: %v\n", fetchErr)
		} else {
			green.Fprintf(os.Stderr, "✓ AKS: fetched credentials for cluster '%s'\n", cfg.AKS.Cluster)
		}
	} else if cfg.EKS != nil {
		fetchedContextName, fetchErr = fetchEKSCredentials(cfg.EKS, ctx.AWS, kubeconfigPath)
		if fetchErr != nil {
			yellow.Fprintf(os.Stderr, "⚠ EKS credential fetch failed: %v\n", fetchErr)
		} else {
			green.Fprintf(os.Stderr, "✓ EKS: fetched credentials for cluster '%s'\n", cfg.EKS.Cluster)
		}
	} else if cfg.GKE != nil {
		gcpConfigDir := ""
		if ctx.GCP != nil {
			gcpConfigDir = mgr.GCPConfigDir(ctx.Name)
		}
		fetchedContextName, fetchErr = fetchGKECredentials(cfg.GKE, ctx.GCP, gcpConfigDir, kubeconfigPath)
		if fetchErr != nil {
			yellow.Fprintf(os.Stderr, "⚠ GKE credential fetch failed: %v\n", fetchErr)
		} else {
			green.Fprintf(os.Stderr, "✓ GKE: fetched credentials for cluster '%s'\n", cfg.GKE.Cluster)
		}
	}

	// Determine which context name to use
	contextName := cfg.Context
	if contextName == "" && fetchedContextName != "" {
		// No custom context name specified, use the auto-generated one
		contextName = fetchedContextName
	}

	if contextName == "" {
		// No context to switch to
		return nil
	}

	// Rename context if user specified a custom name different from auto-generated
	if cfg.Context != "" && fetchedContextName != "" && cfg.Context != fetchedContextName {
		if err := renameKubeContext(fetchedContextName, cfg.Context, kubeconfigPath); err != nil {
			// Rename failed, try to use the auto-generated name
			yellow.Fprintf(os.Stderr, "⚠ Failed to rename context to '%s', using '%s'\n", cfg.Context, fetchedContextName)
			contextName = fetchedContextName
		}
	}

	// Build kubectl command to switch context
	args := []string{"config", "use-context", contextName}
	if kubeconfigPath != "" {
		args = append([]string{"--kubeconfig", kubeconfigPath}, args...)
	}

	cmd := exec.Command("kubectl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Context might not exist in kubeconfig - print warning but continue
		yellow.Fprintf(os.Stderr, "⚠ kubectl context '%s' not found (will use env vars)\n", contextName)
		_ = output // Output already shown via stderr
		return nil
	}

	// Set namespace if specified
	if cfg.Namespace != "" {
		args := []string{"config", "set-context", "--current", "--namespace", cfg.Namespace}
		if kubeconfigPath != "" {
			args = append([]string{"--kubeconfig", kubeconfigPath}, args...)
		}

		cmd := exec.Command("kubectl", args...)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			yellow.Fprintf(os.Stderr, "⚠ failed to set namespace '%s'\n", cfg.Namespace)
		}
	}

	return nil
}

// fetchAKSCredentials runs az aks get-credentials and returns the generated context name.
func fetchAKSCredentials(aks *config.AKSConfig, azureConfigDir, kubeconfig string) (string, error) {
	if _, err := exec.LookPath("az"); err != nil {
		return "", fmt.Errorf("az CLI is required but not found in PATH")
	}

	if aks.Cluster == "" {
		return "", fmt.Errorf("aks.cluster is required")
	}
	if aks.ResourceGroup == "" {
		return "", fmt.Errorf("aks.resource_group is required")
	}

	args := []string{
		"aks", "get-credentials",
		"--resource-group", aks.ResourceGroup,
		"--name", aks.Cluster,
		"--overwrite-existing",
	}

	if kubeconfig != "" {
		args = append(args, "--file", kubeconfig)
	}

	cmd := exec.Command("az", args...)
	// Use per-context Azure config for authentication if available
	if azureConfigDir != "" {
		cmd.Env = append(os.Environ(), "AZURE_CONFIG_DIR="+azureConfigDir)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("az aks get-credentials failed: %w", err)
	}

	// AKS context name format: <cluster-name>
	return aks.Cluster, nil
}

// fetchEKSCredentials runs aws eks update-kubeconfig and returns the generated context name.
func fetchEKSCredentials(eks *config.EKSConfig, aws *config.AWSConfig, kubeconfig string) (string, error) {
	if _, err := exec.LookPath("aws"); err != nil {
		return "", fmt.Errorf("aws CLI is required but not found in PATH")
	}

	if eks.Cluster == "" {
		return "", fmt.Errorf("eks.cluster is required")
	}

	// Determine region: use EKS-specific region, fall back to AWS config region
	region := eks.Region
	if region == "" && aws != nil {
		region = aws.Region
	}
	if region == "" {
		return "", fmt.Errorf("region is required for EKS (specify in kubernetes.eks.region or aws.region)")
	}

	args := []string{
		"eks", "update-kubeconfig",
		"--name", eks.Cluster,
		"--region", region,
	}

	if kubeconfig != "" {
		args = append(args, "--kubeconfig", kubeconfig)
	}

	// Use AWS profile if configured
	if aws != nil && aws.Profile != "" {
		args = append(args, "--profile", aws.Profile)
	}

	cmd := exec.Command("aws", args...)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("aws eks update-kubeconfig failed: %w", err)
	}

	// EKS context name format: arn:aws:eks:<region>:<account>:cluster/<cluster-name>
	// The actual name depends on the account, so we return an empty string
	// and let kubectl figure it out from the cluster name
	return "", nil
}

// fetchGKECredentials runs gcloud container clusters get-credentials and returns the generated context name.
func fetchGKECredentials(gke *config.GKEConfig, gcp *config.GCPConfig, gcpConfigDir, kubeconfig string) (string, error) {
	if _, err := exec.LookPath("gcloud"); err != nil {
		return "", fmt.Errorf("gcloud CLI is required but not found in PATH")
	}

	if gke.Cluster == "" {
		return "", fmt.Errorf("gke.cluster is required")
	}

	// Determine project: use GKE-specific project, fall back to GCP config project
	project := gke.Project
	if project == "" && gcp != nil {
		project = gcp.Project
	}
	if project == "" {
		return "", fmt.Errorf("project is required for GKE (specify in kubernetes.gke.project or gcp.project)")
	}

	args := []string{
		"container", "clusters", "get-credentials",
		gke.Cluster,
		"--project", project,
	}

	// Zone or region (mutually exclusive)
	location := ""
	if gke.Zone != "" {
		args = append(args, "--zone", gke.Zone)
		location = gke.Zone
	} else if gke.Region != "" {
		args = append(args, "--region", gke.Region)
		location = gke.Region
	} else {
		return "", fmt.Errorf("either gke.zone or gke.region is required")
	}

	cmd := exec.Command("gcloud", args...)
	// Use per-context GCP config for authentication if available
	env := os.Environ()
	if gcpConfigDir != "" {
		env = append(env, "CLOUDSDK_CONFIG="+gcpConfigDir)
	}
	if kubeconfig != "" {
		env = append(env, "KUBECONFIG="+kubeconfig)
	}
	cmd.Env = env
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gcloud container clusters get-credentials failed: %w", err)
	}

	// GKE context name format: gke_<project>_<zone-or-region>_<cluster>
	return fmt.Sprintf("gke_%s_%s_%s", project, location, gke.Cluster), nil
}

// renameKubeContext renames a kubectl context.
func renameKubeContext(oldName, newName, kubeconfig string) error {
	args := []string{"config", "rename-context", oldName, newName}
	if kubeconfig != "" {
		args = append([]string{"--kubeconfig", kubeconfig}, args...)
	}
	cmd := exec.Command("kubectl", args...)
	return cmd.Run()
}

// printSwitchSuccess prints a success message after switching contexts.
// Items in the failures list are skipped (no checkmark shown).
func printSwitchSuccess(ctx *config.ContextConfig, failures []string) {
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	cyan := color.New(color.FgCyan)

	// Helper to check if an item failed
	failed := func(name string) bool {
		return slices.Contains(failures, name)
	}

	fmt.Println()

	// VPN
	if ctx.VPN != nil && !failed("VPN") {
		green.Print("✓ ")
		fmt.Printf("VPN: %s", ctx.VPN.Type)
		if ctx.VPN.Interface != "" {
			fmt.Printf(" (%s)", ctx.VPN.Interface)
		} else if ctx.VPN.ConfigFile != "" {
			fmt.Printf(" (%s)", filepath.Base(ctx.VPN.ConfigFile))
		}
		fmt.Println()
	}

	// Cloud providers
	if ctx.AWS != nil && !failed("AWS") {
		green.Print("✓ ")
		fmt.Printf("AWS profile: %s", ctx.AWS.Profile)
		if ctx.AWS.Region != "" {
			fmt.Printf(" (%s)", ctx.AWS.Region)
		}
		fmt.Println()
	}

	if ctx.GCP != nil && !failed("GCP") {
		green.Print("✓ ")
		fmt.Printf("GCP project: %s", ctx.GCP.Project)
		if ctx.GCP.Region != "" {
			fmt.Printf(" (%s)", ctx.GCP.Region)
		}
		fmt.Println()
	}

	if ctx.Azure != nil && !failed("Azure") {
		green.Print("✓ ")
		fmt.Printf("Azure subscription: %s\n", ctx.Azure.SubscriptionID)
	}

	// Orchestration
	if ctx.Kubernetes != nil && !failed("Kubernetes") {
		green.Print("✓ ")
		fmt.Printf("Kubernetes context: %s\n", ctx.Kubernetes.Context)
	}

	if ctx.Nomad != nil {
		// Show warning if Nomad uses localhost and tunnels failed
		if failed("Tunnels") && strings.Contains(ctx.Nomad.Address, "localhost") {
			yellow.Print("⚠ ")
			fmt.Printf("Nomad: %s (tunnel failed)\n", ctx.Nomad.Address)
		} else {
			green.Print("✓ ")
			fmt.Printf("Nomad: %s\n", ctx.Nomad.Address)
		}
	}

	if ctx.Consul != nil {
		// Show warning if Consul uses localhost and tunnels failed
		if failed("Tunnels") && strings.Contains(ctx.Consul.Address, "localhost") {
			yellow.Print("⚠ ")
			fmt.Printf("Consul: %s (tunnel failed)\n", ctx.Consul.Address)
		} else {
			green.Print("✓ ")
			fmt.Printf("Consul: %s\n", ctx.Consul.Address)
		}
	}

	// Vault
	if ctx.Vault != nil && !failed("Vault") {
		green.Print("✓ ")
		fmt.Printf("Vault: %s", ctx.Vault.Address)
		if ctx.Vault.Namespace != "" {
			fmt.Printf(" (namespace: %s)", ctx.Vault.Namespace)
		}
		fmt.Println()
	}

	// Git identity
	if ctx.Git != nil {
		green.Print("✓ ")
		fmt.Printf("Git identity: %s", ctx.Git.UserName)
		if ctx.Git.UserEmail != "" {
			fmt.Printf(" <%s>", ctx.Git.UserEmail)
		}
		fmt.Println()
	}

	// Docker
	if ctx.Docker != nil {
		green.Print("✓ ")
		if ctx.Docker.Context != "" {
			fmt.Printf("Docker context: %s\n", ctx.Docker.Context)
		} else if ctx.Docker.URL != "" {
			fmt.Printf("Docker registry: %s\n", ctx.Docker.URL)
		}
	}

	// NPM
	if ctx.NPM != nil {
		green.Print("✓ ")
		fmt.Printf("NPM registry: %s\n", ctx.NPM.Registry)
	}

	// Databases
	if len(ctx.Databases) > 0 {
		green.Print("✓ ")
		fmt.Printf("Databases: %d configured\n", len(ctx.Databases))
	}

	// Proxy
	if ctx.Proxy != nil {
		green.Print("✓ ")
		fmt.Println("Proxy settings configured")
	}

	// Secrets
	if ctx.Secrets != nil && !failed("Secrets") {
		totalSecrets := len(ctx.Secrets.Bitwarden) + len(ctx.Secrets.OnePassword) + len(ctx.Secrets.Vault) +
			len(ctx.Secrets.AWSSecretsManager) + len(ctx.Secrets.AWSSSM) + len(ctx.Secrets.GCPSecretManager)
		if totalSecrets > 0 {
			green.Print("✓ ")
			var providers []string
			if len(ctx.Secrets.Bitwarden) > 0 {
				providers = append(providers, fmt.Sprintf("Bitwarden:%d", len(ctx.Secrets.Bitwarden)))
			}
			if len(ctx.Secrets.OnePassword) > 0 {
				providers = append(providers, fmt.Sprintf("1Password:%d", len(ctx.Secrets.OnePassword)))
			}
			if len(ctx.Secrets.Vault) > 0 {
				providers = append(providers, fmt.Sprintf("Vault:%d", len(ctx.Secrets.Vault)))
			}
			if len(ctx.Secrets.AWSSecretsManager) > 0 {
				providers = append(providers, fmt.Sprintf("AWS-SM:%d", len(ctx.Secrets.AWSSecretsManager)))
			}
			if len(ctx.Secrets.AWSSSM) > 0 {
				providers = append(providers, fmt.Sprintf("AWS-SSM:%d", len(ctx.Secrets.AWSSSM)))
			}
			if len(ctx.Secrets.GCPSecretManager) > 0 {
				providers = append(providers, fmt.Sprintf("GCP-SM:%d", len(ctx.Secrets.GCPSecretManager)))
			}
			fmt.Printf("Secrets: %s\n", strings.Join(providers, ", "))
		}
	}

	// Secret files
	if ctx.Secrets != nil && len(ctx.Secrets.Files) > 0 && !failed("SecretFiles") {
		green.Print("✓ ")
		fmt.Printf("Secret files: %d file(s)\n", len(ctx.Secrets.Files))
	}

	if len(ctx.Env) > 0 {
		green.Print("✓ ")
		fmt.Println("Environment variables loaded")
	}

	fmt.Println()
	cyan.Printf("Context '%s' is now active.\n", ctx.Name)
}

// deactivatePreviousContext disconnects VPN and stops tunnels for the previous context.
func deactivatePreviousContext(mgr *config.Manager, contextName string) error {
	ctx, err := mgr.LoadContext(contextName)
	if err != nil {
		return err
	}

	yellow := color.New(color.FgYellow)

	// Disconnect VPN
	if ctx.VPN != nil {
		yellow.Fprintf(os.Stderr, "• Disconnecting VPN for '%s'...\n", contextName)
		if err := disconnectVPN(ctx.VPN); err != nil {
			return fmt.Errorf("VPN disconnect failed: %w", err)
		}
	}

	// Stop tunnels - tunnels are managed separately, just log for now
	if len(ctx.Tunnels) > 0 {
		yellow.Fprintf(os.Stderr, "• Note: Tunnels for '%s' may still be running. Use 'ctx tunnel down' to stop them.\n", contextName)
	}

	return nil
}
