// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fatih/color"
	"github.com/vlebo/ctx/internal/config"
)

// SecretsResult holds resolved secrets and metadata about what was loaded.
type SecretsResult struct {
	Secrets                map[string]string
	BitwardenCount         int
	OnePasswordCount       int
	VaultCount             int
	AWSSecretsManagerCount int
	AWSSSMCount            int
	GCPSecretManagerCount  int
}

// resolveAllSecrets fetches secrets from all configured providers.
// Provider configs control auth behavior. AWS/GCP configs are used for cloud secret managers.
// mgr and contextName are used for saving/loading credentials from keychain.
// vaultToken is the saved vault token for authentication.
// gcpConfigDir is the per-context gcloud config directory (for CLOUDSDK_CONFIG).
// awsCreds are optional cached credentials from aws-vault.
func resolveAllSecrets(cfg *config.SecretsConfig, mgr *config.Manager, contextName string, bitwardenCfg *config.BitwardenConfig, onePasswordCfg *config.OnePasswordConfig, vaultCfg *config.VaultConfig, vaultToken string, awsCfg *config.AWSConfig, awsCreds *config.AWSCredentials, gcpCfg *config.GCPConfig, gcpConfigDir string, browserCfg *config.BrowserConfig) (*SecretsResult, error) {
	if cfg == nil {
		return nil, nil
	}

	// Check if any secrets are configured
	hasSecrets := len(cfg.Bitwarden) > 0 || len(cfg.OnePassword) > 0 || len(cfg.Vault) > 0 ||
		len(cfg.AWSSecretsManager) > 0 || len(cfg.AWSSSM) > 0 || len(cfg.GCPSecretManager) > 0
	if !hasSecrets {
		return nil, nil
	}

	result := &SecretsResult{
		Secrets: make(map[string]string),
	}

	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)

	// Resolve Bitwarden secrets
	if len(cfg.Bitwarden) > 0 {
		yellow.Fprintf(os.Stderr, "• Fetching secrets from Bitwarden...\n")

		if err := checkBitwardenCLI(); err != nil {
			return nil, err
		}
		if err := ensureBitwardenUnlocked(bitwardenCfg, mgr, contextName, browserCfg); err != nil {
			return nil, err
		}

		for envVar, itemName := range cfg.Bitwarden {
			value, err := getBitwardenSecret(itemName)
			if err != nil {
				return nil, fmt.Errorf("bitwarden: failed to get '%s': %w", itemName, err)
			}
			result.Secrets[envVar] = value
			result.BitwardenCount++
		}
		green.Fprintf(os.Stderr, "✓ Loaded %d secret(s) from Bitwarden\n", result.BitwardenCount)
	}

	// Resolve 1Password secrets
	if len(cfg.OnePassword) > 0 {
		yellow.Fprintf(os.Stderr, "• Fetching secrets from 1Password...\n")

		if err := checkOnePasswordCLI(); err != nil {
			return nil, err
		}
		if err := ensureOnePasswordUnlocked(onePasswordCfg, mgr, contextName, browserCfg); err != nil {
			return nil, err
		}

		for envVar, itemName := range cfg.OnePassword {
			value, err := getOnePasswordSecret(itemName)
			if err != nil {
				return nil, fmt.Errorf("1password: failed to get '%s': %w", itemName, err)
			}
			result.Secrets[envVar] = value
			result.OnePasswordCount++
		}
		green.Fprintf(os.Stderr, "✓ Loaded %d secret(s) from 1Password\n", result.OnePasswordCount)
	}

	// Resolve Vault secrets
	if len(cfg.Vault) > 0 {
		if vaultCfg == nil {
			return nil, fmt.Errorf("secrets.vault configured but no vault: section found")
		}

		yellow.Fprintf(os.Stderr, "• Fetching secrets from Vault...\n")

		if err := checkVaultCLI(); err != nil {
			return nil, err
		}

		for envVar, pathSpec := range cfg.Vault {
			value, err := getVaultSecret(vaultCfg, vaultToken, pathSpec)
			if err != nil {
				return nil, fmt.Errorf("vault: failed to get '%s': %w", pathSpec, err)
			}
			result.Secrets[envVar] = value
			result.VaultCount++
		}
		green.Fprintf(os.Stderr, "✓ Loaded %d secret(s) from Vault\n", result.VaultCount)
	}

	// Resolve AWS Secrets Manager secrets
	if len(cfg.AWSSecretsManager) > 0 {
		yellow.Fprintf(os.Stderr, "• Fetching secrets from AWS Secrets Manager...\n")

		if err := checkAWSCLI(); err != nil {
			return nil, err
		}

		for envVar, secretSpec := range cfg.AWSSecretsManager {
			value, err := getAWSSecretsManagerSecret(awsCfg, awsCreds, secretSpec)
			if err != nil {
				return nil, fmt.Errorf("aws secrets manager: failed to get '%s': %w", secretSpec, err)
			}
			result.Secrets[envVar] = value
			result.AWSSecretsManagerCount++
		}
		green.Fprintf(os.Stderr, "✓ Loaded %d secret(s) from AWS Secrets Manager\n", result.AWSSecretsManagerCount)
	}

	// Resolve AWS SSM Parameter Store secrets
	if len(cfg.AWSSSM) > 0 {
		yellow.Fprintf(os.Stderr, "• Fetching secrets from AWS Parameter Store...\n")

		if err := checkAWSCLI(); err != nil {
			return nil, err
		}

		for envVar, paramPath := range cfg.AWSSSM {
			value, err := getAWSSSMParameter(awsCfg, awsCreds, paramPath)
			if err != nil {
				return nil, fmt.Errorf("aws ssm: failed to get '%s': %w", paramPath, err)
			}
			result.Secrets[envVar] = value
			result.AWSSSMCount++
		}
		green.Fprintf(os.Stderr, "✓ Loaded %d secret(s) from AWS Parameter Store\n", result.AWSSSMCount)
	}

	// Resolve GCP Secret Manager secrets
	if len(cfg.GCPSecretManager) > 0 {
		yellow.Fprintf(os.Stderr, "• Fetching secrets from GCP Secret Manager...\n")

		if err := checkGCloudCLI(); err != nil {
			return nil, err
		}

		for envVar, secretSpec := range cfg.GCPSecretManager {
			value, err := getGCPSecret(gcpCfg, gcpConfigDir, secretSpec)
			if err != nil {
				return nil, fmt.Errorf("gcp secret manager: failed to get '%s': %w", secretSpec, err)
			}
			result.Secrets[envVar] = value
			result.GCPSecretManagerCount++
		}
		green.Fprintf(os.Stderr, "✓ Loaded %d secret(s) from GCP Secret Manager\n", result.GCPSecretManagerCount)
	}

	return result, nil
}

// checkBitwardenCLI verifies the Bitwarden CLI is installed.
func checkBitwardenCLI() error {
	if _, err := exec.LookPath("bw"); err != nil {
		return fmt.Errorf("bitwarden CLI (bw) not found. Install from: https://bitwarden.com/help/cli/")
	}
	return nil
}

// checkOnePasswordCLI verifies the 1Password CLI is installed.
func checkOnePasswordCLI() error {
	if _, err := exec.LookPath("op"); err != nil {
		return fmt.Errorf("1Password CLI (op) not found. Install from: https://developer.1password.com/docs/cli/")
	}
	return nil
}

// checkVaultCLI verifies the Vault CLI is installed.
func checkVaultCLI() error {
	if _, err := exec.LookPath("vault"); err != nil {
		return fmt.Errorf("vault CLI not found. Install from: https://developer.hashicorp.com/vault/downloads")
	}
	return nil
}

// ensureBitwardenUnlocked checks if Bitwarden is unlocked, auto-login if configured.
// If mgr and contextName are provided, it will try to use/save session from keychain.
func ensureBitwardenUnlocked(cfg *config.BitwardenConfig, mgr *config.Manager, contextName string, browserCfg *config.BrowserConfig) error {
	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)

	// Configure self-hosted server if specified
	if cfg != nil && cfg.Server != "" {
		// Check current server config
		cmd := exec.Command("bw", "config", "server")
		output, _ := cmd.Output()
		currentServer := strings.TrimSpace(string(output))

		// Only reconfigure if different
		if currentServer != cfg.Server {
			// Check if logged in - can't change server while logged in
			statusCmd := exec.Command("bw", "status")
			statusOutput, _ := statusCmd.Output()
			if !strings.Contains(string(statusOutput), `"status":"unauthenticated"`) {
				// Logged in to different server - need to logout first
				yellow.Fprintf(os.Stderr, "  Bitwarden: logging out to switch servers...\n")
				logoutCmd := exec.Command("bw", "logout")
				logoutCmd.Run() // Ignore errors - might already be logged out
			}

			yellow.Fprintf(os.Stderr, "  Configuring Bitwarden server: %s\n", cfg.Server)
			configCmd := exec.Command("bw", "config", "server", cfg.Server)
			if err := configCmd.Run(); err != nil {
				return fmt.Errorf("failed to configure bitwarden server: %w", err)
			}
		}
	}

	// Check if already unlocked via BW_SESSION env var
	if os.Getenv("BW_SESSION") != "" {
		cmd := exec.Command("bw", "status")
		output, err := cmd.Output()
		if err == nil && strings.Contains(string(output), `"status":"unlocked"`) {
			return nil
		}
	}

	// Try to load saved session from keychain
	if mgr != nil && contextName != "" {
		savedSession := mgr.LoadBitwardenSession(contextName)
		if savedSession != "" {
			// Verify session is still valid
			os.Setenv("BW_SESSION", savedSession)
			cmd := exec.Command("bw", "status")
			output, err := cmd.Output()
			if err == nil && strings.Contains(string(output), `"status":"unlocked"`) {
				green.Fprintf(os.Stderr, "  ✓ Bitwarden: using saved session for '%s'\n", contextName)
				return nil
			}
			// Session invalid, clear it
			os.Unsetenv("BW_SESSION")
			mgr.DeleteBitwardenSession(contextName)
			yellow.Fprintf(os.Stderr, "  Bitwarden: saved session expired, re-authenticating...\n")
		}
	}

	// Check login status
	cmd := exec.Command("bw", "status")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check bitwarden status: %w", err)
	}

	status := string(output)
	autoLogin := cfg != nil && cfg.AutoLogin
	useSSO := cfg != nil && cfg.SSO

	// Handle unauthenticated state
	if strings.Contains(status, `"status":"unauthenticated"`) {
		if !autoLogin {
			if useSSO {
				if cfg != nil && cfg.OrgIdentifier != "" {
					return fmt.Errorf("bitwarden not logged in. Run: bw login --sso %s", cfg.OrgIdentifier)
				}
				return fmt.Errorf("bitwarden not logged in. Run: bw login --sso (configure org_identifier in bitwarden config)")
			}
			return fmt.Errorf("bitwarden not logged in. Run: bw login")
		}

		// Auto-login
		var loginCmd *exec.Cmd
		email := ""
		if cfg != nil {
			email = cfg.Email
		}

		if useSSO {
			yellow.Fprintf(os.Stderr, "  Bitwarden SSO login - opening browser...\n")
			if cfg != nil && cfg.OrgIdentifier != "" {
				loginCmd = exec.Command("bw", "login", "--sso", cfg.OrgIdentifier)
			} else {
				loginCmd = exec.Command("bw", "login", "--sso")
			}
		} else if email != "" {
			yellow.Fprintf(os.Stderr, "  Bitwarden login (%s)...\n", email)
			loginCmd = exec.Command("bw", "login", email)
		} else {
			yellow.Fprintf(os.Stderr, "  Bitwarden login...\n")
			loginCmd = exec.Command("bw", "login")
		}

		loginCmd.Stdin = os.Stdin
		// Capture stdout to hide session token, but show stderr for prompts
		loginCmd.Stderr = os.Stderr

		// Set browser for SSO
		if useSSO && browserCfg != nil {
			browserCmd := getBrowserCommand(browserCfg)
			if browserCmd != "" {
				loginCmd.Env = append(os.Environ(), "BROWSER="+browserCmd)
			}
		}

		// Run and capture stdout (contains session token we don't want to display)
		if _, err := loginCmd.Output(); err != nil {
			return fmt.Errorf("bitwarden login failed: %w", err)
		}
		green.Fprintf(os.Stderr, "  ✓ Bitwarden login successful\n")

		// Re-check status after login
		cmd = exec.Command("bw", "status")
		output, err = cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to check bitwarden status after login: %w", err)
		}
		status = string(output)
	}

	// Handle locked state
	if strings.Contains(status, `"status":"locked"`) {
		if !autoLogin {
			yellow.Fprintln(os.Stderr, "  Bitwarden vault is locked. Please unlock:")
			fmt.Fprintln(os.Stderr, "  Run: export BW_SESSION=$(bw unlock --raw)")
			return fmt.Errorf("bitwarden vault is locked")
		}

		// Auto-unlock (interactive - requires master password)
		yellow.Fprintf(os.Stderr, "  Bitwarden vault is locked. Unlocking...\n")

		// We need to capture the session token
		unlockCmd := exec.Command("bw", "unlock", "--raw")
		unlockCmd.Stdin = os.Stdin
		unlockCmd.Stderr = os.Stderr

		sessionOutput, err := unlockCmd.Output()
		if err != nil {
			return fmt.Errorf("bitwarden unlock failed: %w", err)
		}

		session := strings.TrimSpace(string(sessionOutput))
		if session != "" {
			os.Setenv("BW_SESSION", session)
			green.Fprintf(os.Stderr, "  ✓ Bitwarden vault unlocked\n")

			// Save session to keychain for future use
			if mgr != nil && contextName != "" {
				if err := mgr.SaveBitwardenSession(contextName, session); err != nil {
					yellow.Fprintf(os.Stderr, "  ⚠ Failed to save Bitwarden session: %v\n", err)
				} else {
					green.Fprintf(os.Stderr, "  ✓ Bitwarden: session saved for '%s'\n", contextName)
				}
			}
		}
	}

	return nil
}

// ensureOnePasswordUnlocked checks if 1Password is unlocked, auto-login if configured.
// If mgr and contextName are provided, it will try to use/save session from keychain.
func ensureOnePasswordUnlocked(cfg *config.OnePasswordConfig, mgr *config.Manager, contextName string, browserCfg *config.BrowserConfig) error {
	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)

	autoLogin := cfg != nil && cfg.AutoLogin
	useSSO := cfg != nil && cfg.SSO
	account := ""
	if cfg != nil {
		account = cfg.Account
	}

	// Try to load saved session from keychain
	if mgr != nil && contextName != "" {
		savedSession := mgr.LoadOnePasswordSession(contextName)
		if savedSession != "" {
			// Set session and verify it works
			os.Setenv("OP_SESSION", savedSession)
			cmd := exec.Command("op", "account", "list", "--format=json")
			output, err := cmd.Output()
			if err == nil && strings.TrimSpace(string(output)) != "[]" {
				green.Fprintf(os.Stderr, "  ✓ 1Password: using saved session for '%s'\n", contextName)
				return nil
			}
			// Session invalid, clear it
			os.Unsetenv("OP_SESSION")
			mgr.DeleteOnePasswordSession(contextName)
			yellow.Fprintf(os.Stderr, "  1Password: saved session expired, re-authenticating...\n")
		}
	}

	// 1Password CLI with biometric/system auth handles this automatically
	// Just verify we can access the vault
	cmd := exec.Command("op", "account", "list", "--format=json")
	output, err := cmd.Output()

	// Check if we need to sign in
	needsSignin := false
	if err != nil {
		needsSignin = true
	} else if strings.TrimSpace(string(output)) == "[]" {
		needsSignin = true
	}

	if needsSignin {
		if !autoLogin {
			if useSSO {
				return fmt.Errorf("1Password not signed in. Run: eval $(op signin --sso)")
			}
			if account != "" {
				return fmt.Errorf("1Password not signed in. Run: eval $(op signin --account %s)", account)
			}
			return fmt.Errorf("1Password not signed in. Run: eval $(op signin)")
		}

		// Auto-signin (without --raw to let op handle the session internally)
		var signinCmd *exec.Cmd
		if useSSO {
			yellow.Fprintf(os.Stderr, "  1Password SSO sign in - opening browser...\n")
			signinCmd = exec.Command("op", "signin", "--sso")
		} else if account != "" {
			yellow.Fprintf(os.Stderr, "  1Password sign in (%s)...\n", account)
			signinCmd = exec.Command("op", "signin", "--account", account)
		} else {
			yellow.Fprintf(os.Stderr, "  1Password sign in...\n")
			signinCmd = exec.Command("op", "signin")
		}

		signinCmd.Stdin = os.Stdin
		signinCmd.Stdout = os.Stderr
		signinCmd.Stderr = os.Stderr

		// Set browser for SSO
		if useSSO && browserCfg != nil {
			browserCmd := getBrowserCommand(browserCfg)
			if browserCmd != "" {
				signinCmd.Env = append(os.Environ(), "BROWSER="+browserCmd)
			}
		}

		if err := signinCmd.Run(); err != nil {
			return fmt.Errorf("1Password signin failed: %w", err)
		}

		green.Fprintf(os.Stderr, "  ✓ 1Password signed in\n")
	}

	return nil
}

// getBitwardenSecret fetches a secret from Bitwarden.
// itemSpec format: "item-name" or "item-name#field"
// If no field specified, tries: password, notes
// Supported fields: password, username, notes, or any custom field name
func getBitwardenSecret(itemSpec string) (string, error) {
	// Parse item name and optional field
	itemName := itemSpec
	field := ""

	if idx := strings.LastIndex(itemSpec, "#"); idx != -1 {
		itemName = itemSpec[:idx]
		field = itemSpec[idx+1:]
	}

	cmd := exec.Command("bw", "get", "item", itemName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("bw get item failed: %w", err)
	}

	// Parse JSON response
	var item struct {
		Login struct {
			Password string `json:"password"`
			Username string `json:"username"`
		} `json:"login"`
		Notes  string `json:"notes"`
		Fields []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"fields"`
	}

	if err := json.Unmarshal(output, &item); err != nil {
		return "", fmt.Errorf("failed to parse bitwarden response: %w", err)
	}

	// If specific field requested
	if field != "" {
		switch field {
		case "password":
			if item.Login.Password != "" {
				return item.Login.Password, nil
			}
			return "", fmt.Errorf("item '%s' has no password field", itemName)
		case "username":
			if item.Login.Username != "" {
				return item.Login.Username, nil
			}
			return "", fmt.Errorf("item '%s' has no username field", itemName)
		case "notes":
			if item.Notes != "" {
				return item.Notes, nil
			}
			return "", fmt.Errorf("item '%s' has no notes field", itemName)
		default:
			// Look in custom fields
			for _, f := range item.Fields {
				if f.Name == field {
					return f.Value, nil
				}
			}
			return "", fmt.Errorf("item '%s' has no field named '%s'", itemName, field)
		}
	}

	// Return password by default, fall back to notes
	if item.Login.Password != "" {
		return item.Login.Password, nil
	}
	if item.Notes != "" {
		return item.Notes, nil
	}

	return "", fmt.Errorf("item '%s' has no password or notes", itemName)
}

// getOnePasswordSecret fetches a secret from 1Password.
// itemSpec format: "item-name" or "item-name#field"
// If no field specified, tries: password, credential, notesPlain
func getOnePasswordSecret(itemSpec string) (string, error) {
	// Parse item name and optional field
	itemName := itemSpec
	field := ""

	if idx := strings.LastIndex(itemSpec, "#"); idx != -1 {
		itemName = itemSpec[:idx]
		field = itemSpec[idx+1:]
	}

	// If specific field requested, get just that field
	if field != "" {
		cmd := exec.Command("op", "item", "get", itemName, "--fields", field, "--reveal")
		output, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("field '%s' not found in item '%s'", field, itemName)
		}
		return strings.TrimSpace(string(output)), nil
	}

	// Try to get the password field by default
	cmd := exec.Command("op", "item", "get", itemName, "--fields", "password", "--reveal")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		return strings.TrimSpace(string(output)), nil
	}

	// Fall back to credential field (for API keys, etc.)
	cmd = exec.Command("op", "item", "get", itemName, "--fields", "credential", "--reveal")
	output, err = cmd.Output()
	if err == nil && len(output) > 0 {
		return strings.TrimSpace(string(output)), nil
	}

	// Fall back to notesPlain
	cmd = exec.Command("op", "item", "get", itemName, "--fields", "notesPlain", "--reveal")
	output, err = cmd.Output()
	if err == nil && len(output) > 0 {
		return strings.TrimSpace(string(output)), nil
	}

	return "", fmt.Errorf("item '%s' has no password, credential, or notes field", itemName)
}

// getVaultSecret fetches a secret from HashiCorp Vault.
// pathSpec format: "path/to/secret#field" or "path/to/secret" (defaults to "value" field)
// For KV v2, you can use either:
//   - CLI-style: "mount/path#field" (e.g., "operations/consul#http_user")
//   - API-style: "mount/data/path#field" (e.g., "operations/data/consul#http_user") - data/ is auto-stripped
//
// vaultToken is optional - if provided, it's used for authentication.
func getVaultSecret(cfg *config.VaultConfig, vaultToken, pathSpec string) (string, error) {
	// Parse path and field
	path := pathSpec
	field := "value" // default field for KV v1

	if idx := strings.LastIndex(pathSpec, "#"); idx != -1 {
		path = pathSpec[:idx]
		field = pathSpec[idx+1:]
	}

	// Handle API-style paths that include /data/ segment
	// vault kv get adds /data/ automatically for KV v2, so strip it if present
	// e.g., "secret/data/foo" -> "secret/foo"
	// e.g., "operations/data/consul" -> "operations/consul"
	parts := strings.SplitN(path, "/", 3)
	if len(parts) >= 3 && parts[1] == "data" {
		path = parts[0] + "/" + parts[2]
	}

	// Build vault command - try KV v2 first
	cmd := exec.Command("vault", "kv", "get", "-field="+field, path)

	// Set environment for vault command
	cmd.Env = append(os.Environ(), "VAULT_ADDR="+cfg.Address)
	if vaultToken != "" {
		cmd.Env = append(cmd.Env, "VAULT_TOKEN="+vaultToken)
	}
	if cfg.Namespace != "" {
		cmd.Env = append(cmd.Env, "VAULT_NAMESPACE="+cfg.Namespace)
	}
	if cfg.SkipVerify {
		cmd.Env = append(cmd.Env, "VAULT_SKIP_VERIFY=true")
	}

	output, err := cmd.Output()
	if err != nil {
		// Try legacy read command for KV v1
		cmd = exec.Command("vault", "read", "-field="+field, path)
		cmd.Env = append(os.Environ(), "VAULT_ADDR="+cfg.Address)
		if vaultToken != "" {
			cmd.Env = append(cmd.Env, "VAULT_TOKEN="+vaultToken)
		}
		if cfg.Namespace != "" {
			cmd.Env = append(cmd.Env, "VAULT_NAMESPACE="+cfg.Namespace)
		}
		if cfg.SkipVerify {
			cmd.Env = append(cmd.Env, "VAULT_SKIP_VERIFY=true")
		}

		output, err = cmd.Output()
		if err != nil {
			return "", fmt.Errorf("vault read failed: %w", err)
		}
	}

	return strings.TrimSpace(string(output)), nil
}

// checkAWSCLI verifies the AWS CLI is installed.
func checkAWSCLI() error {
	if _, err := exec.LookPath("aws"); err != nil {
		return fmt.Errorf("AWS CLI (aws) not found. Install from: https://aws.amazon.com/cli/")
	}
	return nil
}

// checkGCloudCLI verifies the gcloud CLI is installed.
func checkGCloudCLI() error {
	if _, err := exec.LookPath("gcloud"); err != nil {
		return fmt.Errorf("gcloud CLI not found. Install from: https://cloud.google.com/sdk/docs/install")
	}
	return nil
}

// getAWSSecretsManagerSecret fetches a secret from AWS Secrets Manager.
// secretSpec format: "secret-name" or "secret-name#json-key"
// awsCreds are optional temporary credentials from aws-vault.
func getAWSSecretsManagerSecret(cfg *config.AWSConfig, awsCreds *config.AWSCredentials, secretSpec string) (string, error) {
	// Parse secret name and optional JSON key
	secretName := secretSpec
	jsonKey := ""

	if idx := strings.LastIndex(secretSpec, "#"); idx != -1 {
		secretName = secretSpec[:idx]
		jsonKey = secretSpec[idx+1:]
	}

	// Build AWS CLI command
	args := []string{
		"secretsmanager", "get-secret-value",
		"--secret-id", secretName,
		"--query", "SecretString",
		"--output", "text",
	}

	cmd := exec.Command("aws", args...)

	// Set AWS environment
	cmd.Env = os.Environ()
	if awsCreds != nil {
		// Use temporary credentials from aws-vault
		cmd.Env = append(cmd.Env, "AWS_ACCESS_KEY_ID="+awsCreds.AccessKeyID)
		cmd.Env = append(cmd.Env, "AWS_SECRET_ACCESS_KEY="+awsCreds.SecretAccessKey)
		if awsCreds.SessionToken != "" {
			cmd.Env = append(cmd.Env, "AWS_SESSION_TOKEN="+awsCreds.SessionToken)
		}
	} else if cfg != nil && cfg.Profile != "" {
		cmd.Env = append(cmd.Env, "AWS_PROFILE="+cfg.Profile)
	}
	if cfg != nil && cfg.Region != "" {
		cmd.Env = append(cmd.Env, "AWS_REGION="+cfg.Region)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("aws secretsmanager get-secret-value failed: %w", err)
	}

	secretValue := strings.TrimSpace(string(output))

	// If a JSON key was specified, extract it
	if jsonKey != "" {
		var data map[string]any
		if err := json.Unmarshal([]byte(secretValue), &data); err != nil {
			return "", fmt.Errorf("failed to parse secret as JSON: %w", err)
		}
		if val, ok := data[jsonKey]; ok {
			return fmt.Sprintf("%v", val), nil
		}
		return "", fmt.Errorf("key '%s' not found in secret JSON", jsonKey)
	}

	return secretValue, nil
}

// getAWSSSMParameter fetches a parameter from AWS Systems Manager Parameter Store.
// awsCreds are optional temporary credentials from aws-vault.
func getAWSSSMParameter(cfg *config.AWSConfig, awsCreds *config.AWSCredentials, paramPath string) (string, error) {
	args := []string{
		"ssm", "get-parameter",
		"--name", paramPath,
		"--with-decryption",
		"--query", "Parameter.Value",
		"--output", "text",
	}

	cmd := exec.Command("aws", args...)

	// Set AWS environment
	cmd.Env = os.Environ()
	if awsCreds != nil {
		// Use temporary credentials from aws-vault
		cmd.Env = append(cmd.Env, "AWS_ACCESS_KEY_ID="+awsCreds.AccessKeyID)
		cmd.Env = append(cmd.Env, "AWS_SECRET_ACCESS_KEY="+awsCreds.SecretAccessKey)
		if awsCreds.SessionToken != "" {
			cmd.Env = append(cmd.Env, "AWS_SESSION_TOKEN="+awsCreds.SessionToken)
		}
	} else if cfg != nil && cfg.Profile != "" {
		cmd.Env = append(cmd.Env, "AWS_PROFILE="+cfg.Profile)
	}
	if cfg != nil && cfg.Region != "" {
		cmd.Env = append(cmd.Env, "AWS_REGION="+cfg.Region)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("aws ssm get-parameter failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// getGCPSecret fetches a secret from Google Cloud Secret Manager.
// secretSpec format: "secret-name" (uses latest version) or full resource name
// e.g., "my-secret" or "projects/my-project/secrets/my-secret/versions/latest"
// gcpConfigDir is the per-context gcloud config directory.
func getGCPSecret(cfg *config.GCPConfig, gcpConfigDir, secretSpec string) (string, error) {
	var secretPath string

	// Check if it's a full resource path or just a secret name
	if strings.HasPrefix(secretSpec, "projects/") {
		secretPath = secretSpec
	} else {
		// Need project ID to construct the path
		project := ""
		if cfg != nil && cfg.Project != "" {
			project = cfg.Project
		} else {
			// Try to get from gcloud config
			projectCmd := exec.Command("gcloud", "config", "get-value", "project")
			projectCmd.Env = os.Environ()
			if gcpConfigDir != "" {
				projectCmd.Env = append(projectCmd.Env, "CLOUDSDK_CONFIG="+gcpConfigDir)
			}
			projectOutput, err := projectCmd.Output()
			if err != nil || strings.TrimSpace(string(projectOutput)) == "" {
				return "", fmt.Errorf("GCP project not configured. Set gcp.project in context or run: gcloud config set project <PROJECT>")
			}
			project = strings.TrimSpace(string(projectOutput))
		}
		secretPath = fmt.Sprintf("projects/%s/secrets/%s/versions/latest", project, secretSpec)
	}

	args := []string{"secrets", "versions", "access", secretPath, "--format=value(payload.data)"}

	cmd := exec.Command("gcloud", args...)
	cmd.Env = os.Environ()
	if gcpConfigDir != "" {
		cmd.Env = append(cmd.Env, "CLOUDSDK_CONFIG="+gcpConfigDir)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gcloud secrets versions access failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}
