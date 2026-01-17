// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/vlebo/ctx/internal/config"
	"github.com/vlebo/ctx/pkg/types"
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
func confirmProductionSwitch(ctx *types.ContextConfig) bool {
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
func switchContext(mgr *config.Manager, ctx *types.ContextConfig) ([]string, error) {
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

	// Switch orchestration tools
	if ctx.Kubernetes != nil {
		if err := switchKubernetes(ctx.Kubernetes); err != nil {
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
	if len(ctx.Tunnels) > 0 && ctx.SSH != nil && ctx.SSH.Bastion.Host != "" {
		if err := startAutoConnectTunnels(mgr, ctx); err != nil {
			yellow.Fprintf(os.Stderr, "⚠ Tunnel auto-connect failed: %v\n", err)
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
			return failures, fmt.Errorf("aborted: %s failed", strings.Join(failures, ", "))
		}
	}

	// Set current context
	if err := mgr.SetCurrentContext(ctx.Name); err != nil {
		return failures, fmt.Errorf("failed to set current context: %w", err)
	}

	// Write environment file for shell hook (including resolved secrets)
	if err := mgr.WriteEnvFileWithSecrets(ctx, secrets); err != nil {
		return failures, fmt.Errorf("failed to write environment file: %w", err)
	}

	return failures, nil
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
func switchAWS(cfg *types.AWSConfig, browser *types.BrowserConfig, mgr *config.Manager, contextName string) error {
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
func switchGCP(cfg *types.GCPConfig, browser *types.BrowserConfig, mgr *config.Manager, contextName string) error {
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
func switchAzure(cfg *types.AzureConfig, browser *types.BrowserConfig, mgr *config.Manager, contextName string) error {
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

		args := []string{"login"}
		if cfg.TenantID != "" {
			args = append(args, "--tenant", cfg.TenantID)
		}

		cmd := exec.Command("az", args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Set per-context config directory
		cmd.Env = append(os.Environ(), "AZURE_CONFIG_DIR="+azureConfigDir)

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
func getBrowserCommand(cfg *types.BrowserConfig) string {
	if cfg == nil {
		return ""
	}

	var browserCmd string
	var args string

	switch cfg.Type {
	case types.BrowserChrome:
		profileDir, err := findChromeProfileDir(cfg.Profile)
		if err != nil {
			return ""
		}
		browserCmd = getChromeCommand()
		args = fmt.Sprintf("--profile-directory=\"%s\"", profileDir)
	case types.BrowserFirefox:
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
	tmpFile.Chmod(0755)
	tmpFile.Close()

	return tmpFile.Name()
}

// switchKubernetes switches the kubectl context.
func switchKubernetes(cfg *types.KubernetesConfig) error {
	// Check if kubectl is available
	if _, err := exec.LookPath("kubectl"); err != nil {
		// kubectl not available, skip context switch
		return nil
	}

	// Build kubectl command
	args := []string{"config", "use-context", cfg.Context}
	if cfg.Kubeconfig != "" {
		args = append([]string{"--kubeconfig", cfg.Kubeconfig}, args...)
	}

	cmd := exec.Command("kubectl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Context might not exist in kubeconfig - print warning but continue
		yellow := color.New(color.FgYellow)
		yellow.Fprintf(os.Stderr, "⚠ kubectl context '%s' not found (will use env vars)\n", cfg.Context)
		_ = output // Output already shown via stderr
		return nil
	}

	// Set namespace if specified
	if cfg.Namespace != "" {
		args := []string{"config", "set-context", "--current", "--namespace", cfg.Namespace}
		if cfg.Kubeconfig != "" {
			args = append([]string{"--kubeconfig", cfg.Kubeconfig}, args...)
		}

		cmd := exec.Command("kubectl", args...)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			yellow := color.New(color.FgYellow)
			yellow.Fprintf(os.Stderr, "⚠ failed to set namespace '%s'\n", cfg.Namespace)
		}
	}

	return nil
}

// printSwitchSuccess prints a success message after switching contexts.
// Items in the failures list are skipped (no checkmark shown).
func printSwitchSuccess(ctx *types.ContextConfig, failures []string) {
	green := color.New(color.FgGreen)
	cyan := color.New(color.FgCyan)

	// Helper to check if an item failed
	failed := func(name string) bool {
		for _, f := range failures {
			if f == name {
				return true
			}
		}
		return false
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
		green.Print("✓ ")
		fmt.Printf("Nomad: %s\n", ctx.Nomad.Address)
	}

	if ctx.Consul != nil {
		green.Print("✓ ")
		fmt.Printf("Consul: %s\n", ctx.Consul.Address)
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
