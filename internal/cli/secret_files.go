// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/fatih/color"
	"github.com/vlebo/ctx/internal/config"
)

// SecretFilesResult holds the results of resolving secret files.
type SecretFilesResult struct {
	EnvVars   map[string]string     // ENV_VAR -> temp file path
	State     *config.SecretFilesState
	FileCount int
}

// resolveSecretFiles fetches secrets from providers and writes them to secure temp files.
// Returns a map of ENV_VAR -> temp file path for each resolved secret file.
func resolveSecretFiles(cfg *config.SecretsConfig, mgr *config.Manager, contextName string,
	bitwardenCfg *config.BitwardenConfig, onePasswordCfg *config.OnePasswordConfig,
	vaultCfg *config.VaultConfig, vaultToken string,
	awsCfg *config.AWSConfig, awsCreds *config.AWSCredentials,
	gcpCfg *config.GCPConfig, gcpConfigDir string,
	browserCfg *config.BrowserConfig) (*SecretFilesResult, error) {

	if cfg == nil || len(cfg.Files) == 0 {
		return nil, nil
	}

	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)

	yellow.Fprintf(os.Stderr, "• Resolving secret files...\n")

	tmpDir := getSecretTempDir()

	result := &SecretFilesResult{
		EnvVars: make(map[string]string),
		State: &config.SecretFilesState{
			ContextName: contextName,
			CreatedAt:   time.Now(),
			Files:       make(map[string]config.SecretFileEntry),
		},
	}

	for envVar, src := range cfg.Files {
		provider, itemSpec, err := getSecretFileProvider(src)
		if err != nil {
			return nil, fmt.Errorf("secret file %s: %w", envVar, err)
		}

		// Fetch the secret content using existing provider functions
		var content string
		switch provider {
		case "bitwarden":
			if err := checkBitwardenCLI(); err != nil {
				return nil, err
			}
			if err := ensureBitwardenUnlocked(bitwardenCfg, mgr, contextName, browserCfg); err != nil {
				return nil, err
			}
			content, err = getBitwardenSecret(itemSpec)
		case "onepassword":
			if err := checkOnePasswordCLI(); err != nil {
				return nil, err
			}
			if err := ensureOnePasswordUnlocked(onePasswordCfg, mgr, contextName, browserCfg); err != nil {
				return nil, err
			}
			content, err = getOnePasswordSecret(itemSpec)
		case "vault":
			if vaultCfg == nil {
				return nil, fmt.Errorf("secrets.files.%s uses vault but no vault: section configured", envVar)
			}
			if err := checkVaultCLI(); err != nil {
				return nil, err
			}
			content, err = getVaultSecret(vaultCfg, vaultToken, itemSpec)
		case "aws_secrets_manager":
			if err := checkAWSCLI(); err != nil {
				return nil, err
			}
			content, err = getAWSSecretsManagerSecret(awsCfg, awsCreds, itemSpec)
		case "aws_ssm":
			if err := checkAWSCLI(); err != nil {
				return nil, err
			}
			content, err = getAWSSSMParameter(awsCfg, awsCreds, itemSpec)
		case "gcp_secret_manager":
			if err := checkGCloudCLI(); err != nil {
				return nil, err
			}
			content, err = getGCPSecret(gcpCfg, gcpConfigDir, itemSpec)
		default:
			return nil, fmt.Errorf("secret file %s: unknown provider %q", envVar, provider)
		}

		if err != nil {
			return nil, fmt.Errorf("secret file %s (%s): %w", envVar, provider, err)
		}

		// Write content to a secure temp file
		filePath, err := writeSecretFile(tmpDir, contextName, envVar, content)
		if err != nil {
			return nil, fmt.Errorf("secret file %s: failed to write temp file: %w", envVar, err)
		}

		result.EnvVars[envVar] = filePath
		result.State.Files[envVar] = config.SecretFileEntry{
			Path:      filePath,
			EnvVar:    envVar,
			Provider:  provider,
			CreatedAt: time.Now(),
		}
		result.FileCount++
	}

	// Save state for cleanup on deactivate
	if err := mgr.SaveSecretFilesState(result.State); err != nil {
		yellow.Fprintf(os.Stderr, "⚠ Failed to save secret files state: %v\n", err)
	}

	green.Fprintf(os.Stderr, "✓ Wrote %d secret file(s) to %s\n", result.FileCount, tmpDir)

	return result, nil
}

// getSecretTempDir returns a secure temporary directory for secret files.
// Prefers /dev/shm (Linux tmpfs, never touches disk) when available.
func getSecretTempDir() string {
	if runtime.GOOS == "linux" {
		if info, err := os.Stat("/dev/shm"); err == nil && info.IsDir() {
			return "/dev/shm"
		}
	}
	return os.TempDir()
}

// writeSecretFile writes secret content to a temp file with restrictive permissions.
// Returns the path to the created file.
func writeSecretFile(dir, contextName, envVar, content string) (string, error) {
	// Some CLIs (notably 1Password's `op`) wrap multiline content in double quotes.
	// Strip surrounding quotes so the file contains the raw secret content.
	content = stripSurroundingQuotes(content)

	pattern := fmt.Sprintf("ctx-%s-%s-*", contextName, envVar)
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer f.Close()

	// Set restrictive permissions before writing content
	if err := f.Chmod(0o600); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("failed to set permissions: %w", err)
	}

	if _, err := f.WriteString(content); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("failed to write content: %w", err)
	}

	return f.Name(), nil
}

// stripSurroundingQuotes removes a matching pair of double quotes wrapping the content.
// This handles CLI tools that quote multiline output (e.g., 1Password's `op --fields`).
func stripSurroundingQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// secureDeleteFile zeros out a file's content before removing it.
func secureDeleteFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already gone
		}
		return err
	}

	// Zero-fill
	if size := info.Size(); size > 0 {
		zeros := make([]byte, size)
		if err := os.WriteFile(path, zeros, 0o600); err != nil {
			// Best effort - still try to remove
			os.Remove(path)
			return err
		}
	}

	return os.Remove(path)
}

// getSecretFileProvider determines which provider is configured in a SecretFileSource.
// Returns the provider name, item spec, and an error if zero or more than one provider is set.
func getSecretFileProvider(src config.SecretFileSource) (string, string, error) {
	var provider, itemSpec string
	count := 0

	if src.Bitwarden != "" {
		provider = "bitwarden"
		itemSpec = src.Bitwarden
		count++
	}
	if src.OnePassword != "" {
		provider = "onepassword"
		itemSpec = src.OnePassword
		count++
	}
	if src.Vault != "" {
		provider = "vault"
		itemSpec = src.Vault
		count++
	}
	if src.AWSSecretsManager != "" {
		provider = "aws_secrets_manager"
		itemSpec = src.AWSSecretsManager
		count++
	}
	if src.AWSSSM != "" {
		provider = "aws_ssm"
		itemSpec = src.AWSSSM
		count++
	}
	if src.GCPSecretManager != "" {
		provider = "gcp_secret_manager"
		itemSpec = src.GCPSecretManager
		count++
	}

	if count == 0 {
		return "", "", fmt.Errorf("no provider specified (set one of: bitwarden, onepassword, vault, aws_secrets_manager, aws_ssm, gcp_secret_manager)")
	}
	if count > 1 {
		return "", "", fmt.Errorf("exactly one provider must be specified, found %d", count)
	}

	return provider, itemSpec, nil
}
