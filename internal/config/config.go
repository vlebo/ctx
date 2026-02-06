// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

// Package config handles configuration loading and management for the ctx CLI.
package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"
)

const (
	// keyringService is the service name used for keychain storage.
	keyringService = "ctx"
)

const (
	// DefaultConfigDir is the default configuration directory.
	DefaultConfigDir = ".config/ctx"
	// ConfigFileName is the name of the main configuration file.
	ConfigFileName = "config.yaml"
	// ContextsSubdir is the subdirectory for context files.
	ContextsSubdir = "contexts"
	// StateSubdir is the subdirectory for state files.
	StateSubdir = "state"
	// CurrentNameFile is the file that stores the current context name.
	CurrentNameFile = "current.name"
	// CurrentEnvFile is the file that stores the current environment variables.
	CurrentEnvFile = "current.env"
)

// Manager handles configuration operations.
type Manager struct {
	appConfig   *AppConfig
	configDir   string
	contextsDir string
	stateDir    string
}

// NewManager creates a new configuration manager.
func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, DefaultConfigDir)
	m := &Manager{
		configDir:   configDir,
		contextsDir: filepath.Join(configDir, ContextsSubdir),
		stateDir:    filepath.Join(configDir, StateSubdir),
	}

	return m, nil
}

// NewManagerWithDir creates a new configuration manager with a custom config directory.
func NewManagerWithDir(configDir string) *Manager {
	return &Manager{
		configDir:   configDir,
		contextsDir: filepath.Join(configDir, ContextsSubdir),
		stateDir:    filepath.Join(configDir, StateSubdir),
	}
}

// ConfigDir returns the configuration directory path.
func (m *Manager) ConfigDir() string {
	return m.configDir
}

// ContextsDir returns the contexts directory path.
func (m *Manager) ContextsDir() string {
	return m.contextsDir
}

// StateDir returns the state directory path.
func (m *Manager) StateDir() string {
	return m.stateDir
}

// EnsureDirs creates the configuration directories if they don't exist.
func (m *Manager) EnsureDirs() error {
	dirs := []string{m.configDir, m.contextsDir, m.stateDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// LoadAppConfig loads the main application configuration.
func (m *Manager) LoadAppConfig() (*AppConfig, error) {
	configPath := filepath.Join(m.configDir, ConfigFileName)

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Return default config if file doesn't exist
		m.appConfig = m.defaultAppConfig()
		return m.appConfig, nil
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := &AppConfig{}
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Apply defaults for empty fields
	if config.ContextsDir == "" {
		config.ContextsDir = m.contextsDir
	}
	if config.TunnelsDir == "" {
		config.TunnelsDir = filepath.Join(m.configDir, "tunnels")
	}
	if config.PromptFormat == "" {
		config.PromptFormat = "[ctx: {{.Name}}{{if .IsProd}} ⚠️{{end}}]"
	}

	m.appConfig = config
	return config, nil
}

// SaveAppConfig saves the main application configuration.
func (m *Manager) SaveAppConfig(config *AppConfig) error {
	if err := m.EnsureDirs(); err != nil {
		return err
	}

	configPath := filepath.Join(m.configDir, ConfigFileName)
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	m.appConfig = config
	return nil
}

// defaultAppConfig returns the default application configuration.
func (m *Manager) defaultAppConfig() *AppConfig {
	return &AppConfig{
		Version:          1,
		DefaultContext:   "",
		ShellIntegration: true,
		PromptFormat:     "[ctx: {{.Name}}{{if .IsProd}} ⚠️{{end}}]",
		TunnelsDir:       filepath.Join(m.configDir, "tunnels"),
		ContextsDir:      m.contextsDir,
	}
}

// GetAppConfig returns the cached app config, loading it if necessary.
func (m *Manager) GetAppConfig() *AppConfig {
	if m.appConfig == nil {
		m.LoadAppConfig()
	}
	return m.appConfig
}

// LoadContext loads a context configuration by name.
// If the context extends another context, it will be merged with the parent.
// After merging, ${VAR} references in string fields are expanded using the env: map.
func (m *Manager) LoadContext(name string) (*ContextConfig, error) {
	cfg, err := m.loadContextWithChain(name, nil)
	if err != nil {
		return nil, err
	}
	expandConfigVars(cfg)
	return cfg, nil
}

// loadContextWithChain loads a context and tracks the inheritance chain to detect cycles.
func (m *Manager) loadContextWithChain(name string, chain []string) (*ContextConfig, error) {
	// Check for circular dependency
	if slices.Contains(chain, name) {
		return nil, fmt.Errorf("circular inheritance detected: %s -> %s", strings.Join(append(chain, name), " -> "), name)
	}

	contextPath := filepath.Join(m.contextsDir, name+".yaml")

	data, err := os.ReadFile(contextPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("context '%s' not found", name)
		}
		return nil, fmt.Errorf("failed to read context file: %w", err)
	}

	config := &ContextConfig{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse context file: %w", err)
	}

	// Handle inheritance
	if config.Extends != "" {
		parent, err := m.loadContextWithChain(config.Extends, append(chain, name))
		if err != nil {
			return nil, fmt.Errorf("failed to load parent context '%s': %w", config.Extends, err)
		}
		config.MergeFrom(parent)
	}

	return config, nil
}

// SaveContext saves a context configuration.
func (m *Manager) SaveContext(config *ContextConfig) error {
	if err := m.EnsureDirs(); err != nil {
		return err
	}

	contextPath := filepath.Join(m.contextsDir, config.Name+".yaml")
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	if err := os.WriteFile(contextPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write context file: %w", err)
	}

	return nil
}

// DeleteContext deletes a context configuration.
func (m *Manager) DeleteContext(name string) error {
	contextPath := filepath.Join(m.contextsDir, name+".yaml")

	if _, err := os.Stat(contextPath); os.IsNotExist(err) {
		return fmt.Errorf("context '%s' not found", name)
	}

	if err := os.Remove(contextPath); err != nil {
		return fmt.Errorf("failed to delete context file: %w", err)
	}

	return nil
}

// ListContexts returns a list of all available context names.
func (m *Manager) ListContexts() ([]string, error) {
	if _, err := os.Stat(m.contextsDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(m.contextsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read contexts directory: %w", err)
	}

	var contexts []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) == ".yaml" || filepath.Ext(name) == ".yml" {
			contexts = append(contexts, name[:len(name)-len(filepath.Ext(name))])
		}
	}

	return contexts, nil
}

// ListContextConfigs returns all context configurations.
func (m *Manager) ListContextConfigs() ([]*ContextConfig, error) {
	names, err := m.ListContexts()
	if err != nil {
		return nil, err
	}

	var configs []*ContextConfig
	for _, name := range names {
		config, err := m.LoadContext(name)
		if err != nil {
			// Log warning but continue with other contexts
			fmt.Fprintf(os.Stderr, "Warning: failed to load context '%s': %v\n", name, err)
			continue
		}
		configs = append(configs, config)
	}

	return configs, nil
}

// GetCurrentContextName returns the name of the currently active context.
func (m *Manager) GetCurrentContextName() (string, error) {
	namePath := filepath.Join(m.stateDir, CurrentNameFile)

	data, err := os.ReadFile(namePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read current context: %w", err)
	}

	return string(data), nil
}

// SetCurrentContext sets the current active context.
func (m *Manager) SetCurrentContext(name string) error {
	if err := m.EnsureDirs(); err != nil {
		return err
	}

	// Verify the context exists
	if _, err := m.LoadContext(name); err != nil {
		return err
	}

	namePath := filepath.Join(m.stateDir, CurrentNameFile)
	if err := os.WriteFile(namePath, []byte(name), 0o644); err != nil {
		return fmt.Errorf("failed to write current context: %w", err)
	}

	return nil
}

// GetCurrentContext returns the currently active context configuration.
func (m *Manager) GetCurrentContext() (*ContextConfig, error) {
	name, err := m.GetCurrentContextName()
	if err != nil {
		return nil, err
	}

	if name == "" {
		return nil, nil
	}

	return m.LoadContext(name)
}

// WriteEnvFile writes the environment variables for the current context to a file.
func (m *Manager) WriteEnvFile(ctx *ContextConfig) error {
	return m.WriteEnvFileWithSecrets(ctx, nil)
}

// WriteEnvFileWithSecrets writes env vars including resolved secrets to a file.
func (m *Manager) WriteEnvFileWithSecrets(ctx *ContextConfig, secrets map[string]string) error {
	if err := m.EnsureDirs(); err != nil {
		return err
	}

	envPath := filepath.Join(m.stateDir, CurrentEnvFile)
	envVars := m.GenerateEnvVars(ctx)

	// Merge secrets (secrets take precedence)
	maps.Copy(envVars, secrets)

	var content strings.Builder
	for key, value := range envVars {
		content.WriteString(fmt.Sprintf("export %s=%q\n", key, value))
	}

	if err := os.WriteFile(envPath, []byte(content.String()), 0o644); err != nil {
		return fmt.Errorf("failed to write env file: %w", err)
	}

	return nil
}

// GenerateEnvVars generates environment variables for a context.
func (m *Manager) GenerateEnvVars(ctx *ContextConfig) map[string]string {
	envVars := make(map[string]string)

	// AWS
	if ctx.AWS != nil {
		if ctx.AWS.Region != "" {
			envVars["AWS_REGION"] = ctx.AWS.Region
			envVars["AWS_DEFAULT_REGION"] = ctx.AWS.Region
		}

		// Check if we have temporary credentials from aws-vault
		if ctx.AWS.UseVault {
			if creds := m.LoadAWSCredentials(ctx.Name); creds != nil {
				// Use temporary credentials instead of profile
				envVars["AWS_ACCESS_KEY_ID"] = creds.AccessKeyID
				envVars["AWS_SECRET_ACCESS_KEY"] = creds.SecretAccessKey
				if creds.SessionToken != "" {
					envVars["AWS_SESSION_TOKEN"] = creds.SessionToken
				}
			} else if ctx.AWS.Profile != "" {
				// No cached credentials, still set profile for reference
				envVars["AWS_PROFILE"] = ctx.AWS.Profile
			}
		} else if ctx.AWS.Profile != "" {
			envVars["AWS_PROFILE"] = ctx.AWS.Profile
		}
	}

	// GCP
	if ctx.GCP != nil {
		// Set per-context config directory for isolation
		envVars["CLOUDSDK_CONFIG"] = m.GCPConfigDir(ctx.Name)
		if ctx.GCP.ConfigName != "" {
			envVars["CLOUDSDK_ACTIVE_CONFIG_NAME"] = ctx.GCP.ConfigName
		}
		if ctx.GCP.Project != "" {
			envVars["CLOUDSDK_CORE_PROJECT"] = ctx.GCP.Project
			envVars["GOOGLE_CLOUD_PROJECT"] = ctx.GCP.Project
		}
	}

	// Azure
	if ctx.Azure != nil {
		// Set per-context config directory for isolation
		envVars["AZURE_CONFIG_DIR"] = m.AzureConfigDir(ctx.Name)
		if ctx.Azure.SubscriptionID != "" {
			envVars["AZURE_SUBSCRIPTION_ID"] = ctx.Azure.SubscriptionID
		}
	}

	// Kubernetes
	if ctx.Kubernetes != nil {
		if ctx.Kubernetes.Kubeconfig != "" {
			envVars["KUBECONFIG"] = expandPath(ctx.Kubernetes.Kubeconfig)
		}
	}

	// Nomad
	if ctx.Nomad != nil {
		if ctx.Nomad.Address != "" {
			envVars["NOMAD_ADDR"] = ctx.Nomad.Address
		}
		if ctx.Nomad.Namespace != "" {
			envVars["NOMAD_NAMESPACE"] = ctx.Nomad.Namespace
		}
		if ctx.Nomad.SkipVerify {
			envVars["NOMAD_SKIP_VERIFY"] = "true"
		}
		// Note: Token is handled separately for security
	}

	// Consul
	if ctx.Consul != nil {
		if ctx.Consul.Address != "" {
			envVars["CONSUL_HTTP_ADDR"] = ctx.Consul.Address
		}
		if ctx.Consul.SkipVerify {
			envVars["CONSUL_HTTP_SSL_VERIFY"] = "false"
		}
	}

	// Vault
	if ctx.Vault != nil {
		if ctx.Vault.Address != "" {
			envVars["VAULT_ADDR"] = ctx.Vault.Address
		}
		if ctx.Vault.Namespace != "" {
			envVars["VAULT_NAMESPACE"] = ctx.Vault.Namespace
		}
		if ctx.Vault.SkipVerify {
			envVars["VAULT_SKIP_VERIFY"] = "true"
		}
		// Load saved token for this context
		if token := m.LoadVaultToken(ctx.Name); token != "" {
			envVars["VAULT_TOKEN"] = token
		}
	}

	// Git (via environment variables - works alongside git config)
	if ctx.Git != nil {
		if ctx.Git.UserName != "" {
			envVars["GIT_AUTHOR_NAME"] = ctx.Git.UserName
			envVars["GIT_COMMITTER_NAME"] = ctx.Git.UserName
		}
		if ctx.Git.UserEmail != "" {
			envVars["GIT_AUTHOR_EMAIL"] = ctx.Git.UserEmail
			envVars["GIT_COMMITTER_EMAIL"] = ctx.Git.UserEmail
		}
	}

	// Proxy
	if ctx.Proxy != nil {
		if ctx.Proxy.HTTP != "" {
			envVars["HTTP_PROXY"] = ctx.Proxy.HTTP
			envVars["http_proxy"] = ctx.Proxy.HTTP
		}
		if ctx.Proxy.HTTPS != "" {
			envVars["HTTPS_PROXY"] = ctx.Proxy.HTTPS
			envVars["https_proxy"] = ctx.Proxy.HTTPS
		}
		if ctx.Proxy.NoProxy != "" {
			envVars["NO_PROXY"] = ctx.Proxy.NoProxy
			envVars["no_proxy"] = ctx.Proxy.NoProxy
		}
	}

	// Databases - set common environment variables for DB tools
	if len(ctx.Databases) > 0 {
		// Set the first database as default
		db := ctx.Databases[0]
		switch db.Type {
		case DBTypePostgres:
			envVars["PGHOST"] = db.Host
			envVars["PGPORT"] = fmt.Sprintf("%d", db.Port)
			if db.Database != "" {
				envVars["PGDATABASE"] = db.Database
			}
			if db.Username != "" {
				envVars["PGUSER"] = db.Username
			}
			if db.SSLMode != "" {
				envVars["PGSSLMODE"] = db.SSLMode
			}
		case DBTypeMySQL:
			envVars["MYSQL_HOST"] = db.Host
			envVars["MYSQL_TCP_PORT"] = fmt.Sprintf("%d", db.Port)
			if db.Database != "" {
				envVars["MYSQL_DATABASE"] = db.Database
			}
			if db.Username != "" {
				envVars["MYSQL_USER"] = db.Username
			}
		case DBTypeRedis:
			envVars["REDIS_HOST"] = db.Host
			envVars["REDIS_PORT"] = fmt.Sprintf("%d", db.Port)
		case DBTypeMongoDB:
			envVars["MONGODB_HOST"] = db.Host
			envVars["MONGODB_PORT"] = fmt.Sprintf("%d", db.Port)
		}
	}

	// Custom environment variables
	maps.Copy(envVars, ctx.Env)

	// Context metadata
	envVars["CTX_CURRENT"] = ctx.Name
	envVars["CTX_ENVIRONMENT"] = string(ctx.Environment)

	return envVars
}

// expandPath expands ~ to home directory in paths.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// expandConfigVars expands ${VAR} references in all string fields of a ContextConfig
// using the env: map as the variable source. This enables template-based configs
// where child contexts set variables that get expanded in inherited parent values.
func expandConfigVars(cfg *ContextConfig) {
	if len(cfg.Env) == 0 {
		return
	}
	expandStructVars(reflect.ValueOf(cfg), cfg.Env, "")
}

// skipFields are struct field names that should not be expanded.
var skipFields = map[string]bool{
	"Name":    true,
	"Extends": true,
	"Env":     true,
}

// expandStructVars recursively walks a struct and expands ${VAR} in string fields.
func expandStructVars(v reflect.Value, vars map[string]string, fieldName string) {
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return
		}
		expandStructVars(v.Elem(), vars, fieldName)

	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			name := t.Field(i).Name
			if skipFields[name] {
				continue
			}
			expandStructVars(v.Field(i), vars, name)
		}

	case reflect.String:
		if v.CanSet() {
			s := v.String()
			if strings.Contains(s, "$") {
				expanded := os.Expand(s, func(key string) string {
					if val, ok := vars[key]; ok {
						return val
					}
					// Leave undefined vars as-is
					return "${" + key + "}"
				})
				v.SetString(expanded)
			}
		}

	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			expandStructVars(v.Index(i), vars, fieldName)
		}

	case reflect.Map:
		if v.Type().Key().Kind() == reflect.String && v.Type().Elem().Kind() == reflect.String {
			for _, key := range v.MapKeys() {
				val := v.MapIndex(key).String()
				if strings.Contains(val, "$") {
					expanded := os.Expand(val, func(k string) string {
						if v, ok := vars[k]; ok {
							return v
						}
						return "${" + k + "}"
					})
					v.SetMapIndex(key, reflect.ValueOf(expanded))
				}
			}
		}
	}
}

// ClearCurrentContext clears the current context.
func (m *Manager) ClearCurrentContext() error {
	namePath := filepath.Join(m.stateDir, CurrentNameFile)
	envPath := filepath.Join(m.stateDir, CurrentEnvFile)

	// Remove name file
	if err := os.Remove(namePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove current context file: %w", err)
	}

	// Remove env file
	if err := os.Remove(envPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove env file: %w", err)
	}

	return nil
}

// ContextExists checks if a context with the given name exists.
func (m *Manager) ContextExists(name string) bool {
	contextPath := filepath.Join(m.contextsDir, name+".yaml")
	_, err := os.Stat(contextPath)
	return err == nil
}

// TokensDir returns the directory for storing per-context tokens.
func (m *Manager) TokensDir() string {
	return filepath.Join(m.stateDir, "tokens")
}

// EnsureTokensDir creates the tokens directory if it doesn't exist.
func (m *Manager) EnsureTokensDir() error {
	return os.MkdirAll(m.TokensDir(), 0o700) // Restrictive permissions for tokens
}

// vaultTokenKey returns the keyring key for a context's Vault token.
func vaultTokenKey(contextName string) string {
	return "vault-token-" + contextName
}

// SaveVaultToken saves a Vault token for a specific context.
// It tries to use the system keychain first, falling back to file storage.
func (m *Manager) SaveVaultToken(contextName, token string) error {
	key := vaultTokenKey(contextName)

	// Try keychain first
	err := keyring.Set(keyringService, key, token)
	if err == nil {
		// Successfully stored in keychain, remove any old file-based token
		tokenPath := filepath.Join(m.TokensDir(), contextName+".vault")
		os.Remove(tokenPath) // Ignore error - file might not exist
		return nil
	}

	// Keychain not available, fall back to file
	if err := m.EnsureTokensDir(); err != nil {
		return fmt.Errorf("failed to create tokens directory: %w", err)
	}

	tokenPath := filepath.Join(m.TokensDir(), contextName+".vault")
	// Use restrictive permissions - tokens are sensitive
	if err := os.WriteFile(tokenPath, []byte(token), 0o600); err != nil {
		return fmt.Errorf("failed to save vault token: %w", err)
	}

	return nil
}

// LoadVaultToken loads a saved Vault token for a specific context.
// It tries the system keychain first, falling back to file storage.
// Returns empty string if no token is saved.
func (m *Manager) LoadVaultToken(contextName string) string {
	key := vaultTokenKey(contextName)

	// Try keychain first
	token, err := keyring.Get(keyringService, key)
	if err == nil && token != "" {
		return token
	}

	// Fall back to file
	tokenPath := filepath.Join(m.TokensDir(), contextName+".vault")
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// DeleteVaultToken removes a saved Vault token for a specific context.
// It removes from both keychain and file storage.
func (m *Manager) DeleteVaultToken(contextName string) error {
	key := vaultTokenKey(contextName)

	// Delete from keychain (ignore errors - might not exist)
	keyring.Delete(keyringService, key)

	// Delete from file
	tokenPath := filepath.Join(m.TokensDir(), contextName+".vault")
	if err := os.Remove(tokenPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete vault token: %w", err)
	}
	return nil
}

// bitwardenSessionKey returns the keyring key for a context's Bitwarden session.
func bitwardenSessionKey(contextName string) string {
	return "bitwarden-session-" + contextName
}

// SaveBitwardenSession saves a Bitwarden session for a specific context.
// It tries to use the system keychain first, falling back to file storage.
func (m *Manager) SaveBitwardenSession(contextName, session string) error {
	key := bitwardenSessionKey(contextName)

	// Try keychain first
	err := keyring.Set(keyringService, key, session)
	if err == nil {
		// Successfully stored in keychain, remove any old file-based session
		sessionPath := filepath.Join(m.TokensDir(), contextName+".bitwarden")
		os.Remove(sessionPath) // Ignore error - file might not exist
		return nil
	}

	// Keychain not available, fall back to file
	if err := m.EnsureTokensDir(); err != nil {
		return fmt.Errorf("failed to create tokens directory: %w", err)
	}

	sessionPath := filepath.Join(m.TokensDir(), contextName+".bitwarden")
	// Use restrictive permissions - sessions are sensitive
	if err := os.WriteFile(sessionPath, []byte(session), 0o600); err != nil {
		return fmt.Errorf("failed to save bitwarden session: %w", err)
	}

	return nil
}

// LoadBitwardenSession loads a saved Bitwarden session for a specific context.
// It tries the system keychain first, falling back to file storage.
// Returns empty string if no session is saved.
func (m *Manager) LoadBitwardenSession(contextName string) string {
	key := bitwardenSessionKey(contextName)

	// Try keychain first
	session, err := keyring.Get(keyringService, key)
	if err == nil && session != "" {
		return session
	}

	// Fall back to file
	sessionPath := filepath.Join(m.TokensDir(), contextName+".bitwarden")
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// DeleteBitwardenSession removes a saved Bitwarden session for a specific context.
// It removes from both keychain and file storage.
func (m *Manager) DeleteBitwardenSession(contextName string) error {
	key := bitwardenSessionKey(contextName)

	// Delete from keychain (ignore errors - might not exist)
	keyring.Delete(keyringService, key)

	// Delete from file
	sessionPath := filepath.Join(m.TokensDir(), contextName+".bitwarden")
	if err := os.Remove(sessionPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete bitwarden session: %w", err)
	}
	return nil
}

// onePasswordSessionKey returns the keyring key for a context's 1Password session.
func onePasswordSessionKey(contextName string) string {
	return "onepassword-session-" + contextName
}

// SaveOnePasswordSession saves a 1Password session for a specific context.
func (m *Manager) SaveOnePasswordSession(contextName, session string) error {
	key := onePasswordSessionKey(contextName)

	// Try keychain first
	err := keyring.Set(keyringService, key, session)
	if err == nil {
		sessionPath := filepath.Join(m.TokensDir(), contextName+".onepassword")
		os.Remove(sessionPath)
		return nil
	}

	// Keychain not available, fall back to file
	if err := m.EnsureTokensDir(); err != nil {
		return fmt.Errorf("failed to create tokens directory: %w", err)
	}

	sessionPath := filepath.Join(m.TokensDir(), contextName+".onepassword")
	if err := os.WriteFile(sessionPath, []byte(session), 0o600); err != nil {
		return fmt.Errorf("failed to save 1password session: %w", err)
	}

	return nil
}

// LoadOnePasswordSession loads a saved 1Password session for a specific context.
func (m *Manager) LoadOnePasswordSession(contextName string) string {
	key := onePasswordSessionKey(contextName)

	// Try keychain first
	session, err := keyring.Get(keyringService, key)
	if err == nil && session != "" {
		return session
	}

	// Fall back to file
	sessionPath := filepath.Join(m.TokensDir(), contextName+".onepassword")
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// DeleteOnePasswordSession removes a saved 1Password session for a specific context.
func (m *Manager) DeleteOnePasswordSession(contextName string) error {
	key := onePasswordSessionKey(contextName)

	// Delete from keychain (ignore errors - might not exist)
	keyring.Delete(keyringService, key)

	// Delete from file
	sessionPath := filepath.Join(m.TokensDir(), contextName+".onepassword")
	if err := os.Remove(sessionPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete 1password session: %w", err)
	}
	return nil
}

// CloudConfigDir returns the directory for storing per-context cloud provider configs.
func (m *Manager) CloudConfigDir() string {
	return filepath.Join(m.stateDir, "cloud")
}

// AzureConfigDir returns the Azure config directory for a specific context.
func (m *Manager) AzureConfigDir(contextName string) string {
	return filepath.Join(m.CloudConfigDir(), contextName, "azure")
}

// EnsureAzureConfigDir creates the Azure config directory for a context if it doesn't exist.
func (m *Manager) EnsureAzureConfigDir(contextName string) error {
	return os.MkdirAll(m.AzureConfigDir(contextName), 0o700)
}

// GCPConfigDir returns the GCP config directory for a specific context.
func (m *Manager) GCPConfigDir(contextName string) string {
	return filepath.Join(m.CloudConfigDir(), contextName, "gcloud")
}

// EnsureGCPConfigDir creates the GCP config directory for a context if it doesn't exist.
func (m *Manager) EnsureGCPConfigDir(contextName string) error {
	return os.MkdirAll(m.GCPConfigDir(contextName), 0o700)
}

// AWSCredentials holds temporary AWS credentials from aws-vault.
// The JSON field names match aws-vault's exec --json output format.
type AWSCredentials struct {
	AccessKeyID     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	SessionToken    string `json:"SessionToken"`
	Expiration      string `json:"Expiration"` // RFC3339 format
	Version         int    `json:"Version"`
}

// SaveAWSCredentials saves temporary AWS credentials for a context.
func (m *Manager) SaveAWSCredentials(contextName string, creds *AWSCredentials) error {
	if err := m.EnsureTokensDir(); err != nil {
		return fmt.Errorf("failed to create tokens directory: %w", err)
	}

	data, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	credPath := filepath.Join(m.TokensDir(), contextName+".aws")
	if err := os.WriteFile(credPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to save AWS credentials: %w", err)
	}
	return nil
}

// LoadAWSCredentials loads saved AWS credentials for a context.
// Returns nil if no credentials are saved or if they're expired.
func (m *Manager) LoadAWSCredentials(contextName string) *AWSCredentials {
	credPath := filepath.Join(m.TokensDir(), contextName+".aws")
	data, err := os.ReadFile(credPath)
	if err != nil {
		return nil
	}

	var creds AWSCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil
	}

	// Check if credentials are expired
	if creds.Expiration != "" {
		expTime, err := time.Parse(time.RFC3339, creds.Expiration)
		if err == nil && time.Now().After(expTime) {
			// Credentials expired, delete and return nil
			os.Remove(credPath)
			return nil
		}
	}

	return &creds
}

// DeleteAWSCredentials removes saved AWS credentials for a context.
func (m *Manager) DeleteAWSCredentials(contextName string) error {
	credPath := filepath.Join(m.TokensDir(), contextName+".aws")
	if err := os.Remove(credPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete AWS credentials: %w", err)
	}
	return nil
}

// cloudAPIKeyKey returns the keyring key for the cloud API key.
func cloudAPIKeyKey() string {
	return "cloud-api-key"
}

// SaveCloudAPIKey saves the ctx-cloud API key.
// It tries to use the system keychain first, falling back to file storage.
func (m *Manager) SaveCloudAPIKey(apiKey string) error {
	key := cloudAPIKeyKey()

	// Try keychain first
	err := keyring.Set(keyringService, key, apiKey)
	if err == nil {
		// Successfully stored in keychain, remove any old file-based key
		keyPath := filepath.Join(m.TokensDir(), "cloud.key")
		os.Remove(keyPath) // Ignore error - file might not exist
		return nil
	}

	// Keychain not available, fall back to file
	if err := m.EnsureTokensDir(); err != nil {
		return fmt.Errorf("failed to create tokens directory: %w", err)
	}

	keyPath := filepath.Join(m.TokensDir(), "cloud.key")
	// Use restrictive permissions - API keys are sensitive
	if err := os.WriteFile(keyPath, []byte(apiKey), 0o600); err != nil {
		return fmt.Errorf("failed to save cloud API key: %w", err)
	}

	return nil
}

// LoadCloudAPIKey loads the saved ctx-cloud API key.
// It tries the system keychain first, falling back to file storage.
// Returns empty string if no API key is saved.
func (m *Manager) LoadCloudAPIKey() string {
	key := cloudAPIKeyKey()

	// Try keychain first
	apiKey, err := keyring.Get(keyringService, key)
	if err == nil && apiKey != "" {
		return apiKey
	}

	// Fall back to file
	keyPath := filepath.Join(m.TokensDir(), "cloud.key")
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// DeleteCloudAPIKey removes the saved ctx-cloud API key.
// It removes from both keychain and file storage.
func (m *Manager) DeleteCloudAPIKey() error {
	key := cloudAPIKeyKey()

	// Delete from keychain (ignore errors - might not exist)
	keyring.Delete(keyringService, key)

	// Delete from file
	keyPath := filepath.Join(m.TokensDir(), "cloud.key")
	if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete cloud API key: %w", err)
	}
	return nil
}
