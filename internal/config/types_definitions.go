// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

// Package config contains shared configuration type definitions for the ctx CLI tool.
package config

import (
	"maps"

	"dario.cat/mergo"
)

// Environment represents the deployment environment type (can be any string).
type Environment string

// Common environment names (not enforced, just for convenience).
const (
	EnvProduction  Environment = "production"
	EnvStaging     Environment = "staging"
	EnvDevelopment Environment = "development"
)

// IsProd returns true if the environment is production.
func (e Environment) IsProd() bool {
	return e == EnvProduction || e == "prod"
}

// TunnelStatus represents the status of an SSH tunnel.
type TunnelStatus string

const (
	TunnelStatusConnected    TunnelStatus = "connected"
	TunnelStatusDisconnected TunnelStatus = "disconnected"
	TunnelStatusConnecting   TunnelStatus = "connecting"
	TunnelStatusError        TunnelStatus = "error"
)

// AWSConfig holds AWS-specific configuration.
type AWSConfig struct {
	Profile  string `yaml:"profile" mapstructure:"profile"`
	Region   string `yaml:"region" mapstructure:"region"`
	UseVault bool   `yaml:"use_vault" mapstructure:"use_vault"`
	SSOLogin bool   `yaml:"sso_login,omitempty" mapstructure:"sso_login"`
}

// GCPConfig holds GCP-specific configuration.
type GCPConfig struct {
	Project    string `yaml:"project" mapstructure:"project"`
	Region     string `yaml:"region" mapstructure:"region"`
	ConfigName string `yaml:"config_name" mapstructure:"config_name"`
	AutoLogin  bool   `yaml:"auto_login,omitempty" mapstructure:"auto_login"`
}

// AzureConfig holds Azure-specific configuration.
type AzureConfig struct {
	SubscriptionID string `yaml:"subscription_id" mapstructure:"subscription_id"`
	TenantID       string `yaml:"tenant_id" mapstructure:"tenant_id"`
	AutoLogin      bool   `yaml:"auto_login,omitempty" mapstructure:"auto_login"`
}

// AKSConfig holds Azure Kubernetes Service configuration for auto-fetching credentials.
type AKSConfig struct {
	Cluster       string `yaml:"cluster" mapstructure:"cluster"`
	ResourceGroup string `yaml:"resource_group" mapstructure:"resource_group"`
}

// EKSConfig holds AWS Elastic Kubernetes Service configuration for auto-fetching credentials.
type EKSConfig struct {
	Cluster string `yaml:"cluster" mapstructure:"cluster"`
	Region  string `yaml:"region,omitempty" mapstructure:"region"` // Optional, falls back to aws.region
}

// GKEConfig holds Google Kubernetes Engine configuration for auto-fetching credentials.
type GKEConfig struct {
	Cluster string `yaml:"cluster" mapstructure:"cluster"`
	Zone    string `yaml:"zone,omitempty" mapstructure:"zone"`       // Zonal cluster
	Region  string `yaml:"region,omitempty" mapstructure:"region"`   // Regional cluster
	Project string `yaml:"project,omitempty" mapstructure:"project"` // Optional, falls back to gcp.project
}

// KubernetesConfig holds Kubernetes-specific configuration.
type KubernetesConfig struct {
	Context    string     `yaml:"context,omitempty" mapstructure:"context"`
	Namespace  string     `yaml:"namespace,omitempty" mapstructure:"namespace"`
	Kubeconfig string     `yaml:"kubeconfig,omitempty" mapstructure:"kubeconfig"`
	AKS        *AKSConfig `yaml:"aks,omitempty" mapstructure:"aks"`
	EKS        *EKSConfig `yaml:"eks,omitempty" mapstructure:"eks"`
	GKE        *GKEConfig `yaml:"gke,omitempty" mapstructure:"gke"`
}

// NomadConfig holds Nomad-specific configuration.
type NomadConfig struct {
	Address    string `yaml:"address" mapstructure:"address"`
	Namespace  string `yaml:"namespace" mapstructure:"namespace"`
	Token      string `yaml:"token" mapstructure:"token"`
	TokenEnv   string `yaml:"token_env" mapstructure:"token_env"`
	SkipVerify bool   `yaml:"skip_verify" mapstructure:"skip_verify"`
}

// ConsulConfig holds Consul-specific configuration.
type ConsulConfig struct {
	Address    string `yaml:"address" mapstructure:"address"`
	TokenEnv   string `yaml:"token_env" mapstructure:"token_env"`
	SkipVerify bool   `yaml:"skip_verify" mapstructure:"skip_verify"`
}

// BastionConfig holds SSH bastion configuration.
type BastionConfig struct {
	Host         string `yaml:"host" mapstructure:"host"`
	User         string `yaml:"user" mapstructure:"user"`
	IdentityFile string `yaml:"identity_file" mapstructure:"identity_file"`
	Port         int    `yaml:"port" mapstructure:"port"`
}

// SSHConfig holds SSH-specific configuration.
type SSHConfig struct {
	ControlMaster     string        `yaml:"control_master" mapstructure:"control_master"`
	ControlPersist    string        `yaml:"control_persist" mapstructure:"control_persist"`
	Bastion           BastionConfig `yaml:"bastion" mapstructure:"bastion"`
	KeepaliveInterval int           `yaml:"keepalive_interval" mapstructure:"keepalive_interval"`
	KeepaliveCountMax int           `yaml:"keepalive_count_max" mapstructure:"keepalive_count_max"`
	Persistent        bool          `yaml:"persistent" mapstructure:"persistent"`
}

// TunnelConfig holds configuration for a single tunnel.
type TunnelConfig struct {
	Name        string `yaml:"name" mapstructure:"name"`
	Description string `yaml:"description" mapstructure:"description"`
	RemoteHost  string `yaml:"remote_host" mapstructure:"remote_host"`
	RemotePort  int    `yaml:"remote_port" mapstructure:"remote_port"`
	LocalPort   int    `yaml:"local_port" mapstructure:"local_port"`
	AutoConnect bool   `yaml:"auto_connect,omitempty" mapstructure:"auto_connect"`
}

// VPNType represents the type of VPN connection.
type VPNType string

const (
	VPNTypeOpenVPN   VPNType = "openvpn"
	VPNTypeWireGuard VPNType = "wireguard"
	VPNTypeTailscale VPNType = "tailscale"
	VPNTypeCustom    VPNType = "custom"
)

// VPNConfig holds VPN connection configuration.
type VPNConfig struct {
	Type VPNType `yaml:"type" mapstructure:"type"`
	// OpenVPN
	ConfigFile   string `yaml:"config_file,omitempty" mapstructure:"config_file"`
	AuthUserPass string `yaml:"auth_user_pass,omitempty" mapstructure:"auth_user_pass"`
	// WireGuard
	Interface string `yaml:"interface,omitempty" mapstructure:"interface"`
	// Tailscale
	ExitNode string `yaml:"exit_node,omitempty" mapstructure:"exit_node"`
	// Custom commands
	ConnectCmd    string `yaml:"connect_cmd,omitempty" mapstructure:"connect_cmd"`
	DisconnectCmd string `yaml:"disconnect_cmd,omitempty" mapstructure:"disconnect_cmd"`
	StatusCmd     string `yaml:"status_cmd,omitempty" mapstructure:"status_cmd"`
	// Common
	AutoConnect    bool `yaml:"auto_connect,omitempty" mapstructure:"auto_connect"`
	AutoDisconnect bool `yaml:"auto_disconnect,omitempty" mapstructure:"auto_disconnect"`
}

// VaultAuthMethod represents HashiCorp Vault auth methods.
type VaultAuthMethod string

const (
	VaultAuthToken   VaultAuthMethod = "token"
	VaultAuthOIDC    VaultAuthMethod = "oidc"
	VaultAuthAWS     VaultAuthMethod = "aws"
	VaultAuthK8s     VaultAuthMethod = "kubernetes"
	VaultAuthAppRole VaultAuthMethod = "approle"
)

// VaultConfig holds HashiCorp Vault configuration.
type VaultConfig struct {
	Address    string          `yaml:"address" mapstructure:"address"`
	Namespace  string          `yaml:"namespace,omitempty" mapstructure:"namespace"`
	AuthMethod VaultAuthMethod `yaml:"auth_method,omitempty" mapstructure:"auth_method"`
	TokenEnv   string          `yaml:"token_env,omitempty" mapstructure:"token_env"`
	RoleID     string          `yaml:"role_id,omitempty" mapstructure:"role_id"`
	SecretID   string          `yaml:"secret_id_env,omitempty" mapstructure:"secret_id_env"`
	AutoLogin  bool            `yaml:"auto_login,omitempty" mapstructure:"auto_login"`
	SkipVerify bool            `yaml:"skip_verify,omitempty" mapstructure:"skip_verify"`
}

// BitwardenConfig holds Bitwarden authentication configuration.
type BitwardenConfig struct {
	Server        string `yaml:"server,omitempty" mapstructure:"server"`                 // Self-hosted Bitwarden server URL
	Email         string `yaml:"email,omitempty" mapstructure:"email"`                   // Email for login (pre-fills prompt)
	OrgIdentifier string `yaml:"org_identifier,omitempty" mapstructure:"org_identifier"` // Organization identifier for SSO login
	AutoLogin     bool   `yaml:"auto_login,omitempty" mapstructure:"auto_login"`         // Auto-run 'bw login' if not authenticated
	SSO           bool   `yaml:"sso,omitempty" mapstructure:"sso"`                       // Use SSO login instead of email/password
}

// OnePasswordConfig holds 1Password authentication configuration.
type OnePasswordConfig struct {
	Account   string `yaml:"account,omitempty" mapstructure:"account"`       // Account shorthand or URL (e.g., "my.1password.com")
	AutoLogin bool   `yaml:"auto_login,omitempty" mapstructure:"auto_login"` // Auto-run 'op signin' if not authenticated
	SSO       bool   `yaml:"sso,omitempty" mapstructure:"sso"`               // Use SSO login instead of email/password
}

// GitConfig holds Git identity configuration for per-client commits.
type GitConfig struct {
	UserName   string `yaml:"user_name,omitempty" mapstructure:"user_name"`
	UserEmail  string `yaml:"user_email,omitempty" mapstructure:"user_email"`
	SigningKey string `yaml:"signing_key,omitempty" mapstructure:"signing_key"`
	GPGSign    bool   `yaml:"gpg_sign,omitempty" mapstructure:"gpg_sign"`
}

// DockerRegistryConfig holds Docker registry configuration.
type DockerRegistryConfig struct {
	URL         string `yaml:"url" mapstructure:"url"`
	Username    string `yaml:"username,omitempty" mapstructure:"username"`
	PasswordEnv string `yaml:"password_env,omitempty" mapstructure:"password_env"`
	// Docker context to use
	Context string `yaml:"context,omitempty" mapstructure:"context"`
}

// NPMConfig holds NPM registry configuration.
type NPMConfig struct {
	Registry     string `yaml:"registry" mapstructure:"registry"`
	AuthTokenEnv string `yaml:"auth_token_env,omitempty" mapstructure:"auth_token_env"`
	Scope        string `yaml:"scope,omitempty" mapstructure:"scope"`
	AlwaysAuth   bool   `yaml:"always_auth,omitempty" mapstructure:"always_auth"`
}

// DatabaseType represents supported database types.
type DatabaseType string

const (
	DBTypePostgres DatabaseType = "postgres"
	DBTypeMySQL    DatabaseType = "mysql"
	DBTypeMongoDB  DatabaseType = "mongodb"
	DBTypeRedis    DatabaseType = "redis"
)

// DatabaseConfig holds database connection configuration.
type DatabaseConfig struct {
	Name        string       `yaml:"name" mapstructure:"name"`
	Type        DatabaseType `yaml:"type" mapstructure:"type"`
	Host        string       `yaml:"host" mapstructure:"host"`
	Database    string       `yaml:"database,omitempty" mapstructure:"database"`
	Username    string       `yaml:"username,omitempty" mapstructure:"username"`
	PasswordEnv string       `yaml:"password_env,omitempty" mapstructure:"password_env"`
	SSLMode     string       `yaml:"ssl_mode,omitempty" mapstructure:"ssl_mode"`
	Port        int          `yaml:"port" mapstructure:"port"`
}

// ProxyConfig holds HTTP proxy configuration.
type ProxyConfig struct {
	HTTP    string `yaml:"http,omitempty" mapstructure:"http"`
	HTTPS   string `yaml:"https,omitempty" mapstructure:"https"`
	NoProxy string `yaml:"no_proxy,omitempty" mapstructure:"no_proxy"`
}

// BrowserType represents the type of browser.
type BrowserType string

const (
	BrowserChrome  BrowserType = "chrome"
	BrowserFirefox BrowserType = "firefox"
)

// SecretsConfig holds configuration for fetching secrets from various providers.
// Each provider has its own sub-section with ENV_VAR: "item-name" mappings.
type SecretsConfig struct {
	// Password Managers
	Bitwarden   map[string]string `yaml:"bitwarden,omitempty" mapstructure:"bitwarden"`     // ENV_VAR: "item-name"
	OnePassword map[string]string `yaml:"onepassword,omitempty" mapstructure:"onepassword"` // ENV_VAR: "item-name"
	Vault       map[string]string `yaml:"vault,omitempty" mapstructure:"vault"`             // ENV_VAR: "path#field"
	// Cloud Secret Managers (use existing cloud auth)
	AWSSecretsManager map[string]string `yaml:"aws_secrets_manager,omitempty" mapstructure:"aws_secrets_manager"` // ENV_VAR: "secret-name" or "secret-name#json-key"
	AWSSSM            map[string]string `yaml:"aws_ssm,omitempty" mapstructure:"aws_ssm"`                         // ENV_VAR: "/param/path"
	GCPSecretManager  map[string]string `yaml:"gcp_secret_manager,omitempty" mapstructure:"gcp_secret_manager"`   // ENV_VAR: "secret-name" or "projects/p/secrets/s/versions/v"
}

// BrowserConfig holds browser profile configuration.
type BrowserConfig struct {
	Type    BrowserType `yaml:"type" mapstructure:"type"`
	Profile string      `yaml:"profile" mapstructure:"profile"`
}

// ContextConfig represents a complete context configuration.
type ContextConfig struct {
	// Cloud Providers
	AWS   *AWSConfig   `yaml:"aws,omitempty" mapstructure:"aws"`
	GCP   *GCPConfig   `yaml:"gcp,omitempty" mapstructure:"gcp"`
	Azure *AzureConfig `yaml:"azure,omitempty" mapstructure:"azure"`
	// Orchestration
	Kubernetes *KubernetesConfig `yaml:"kubernetes,omitempty" mapstructure:"kubernetes"`
	Nomad      *NomadConfig      `yaml:"nomad,omitempty" mapstructure:"nomad"`
	Consul     *ConsulConfig     `yaml:"consul,omitempty" mapstructure:"consul"`
	// SSH & Tunnels
	SSH *SSHConfig `yaml:"ssh,omitempty" mapstructure:"ssh"`
	// VPN
	VPN *VPNConfig `yaml:"vpn,omitempty" mapstructure:"vpn"`
	// Secrets & Identity
	Secrets     *SecretsConfig     `yaml:"secrets,omitempty" mapstructure:"secrets"`
	Bitwarden   *BitwardenConfig   `yaml:"bitwarden,omitempty" mapstructure:"bitwarden"`
	OnePassword *OnePasswordConfig `yaml:"onepassword,omitempty" mapstructure:"onepassword"`
	Vault       *VaultConfig       `yaml:"vault,omitempty" mapstructure:"vault"`
	Git         *GitConfig         `yaml:"git,omitempty" mapstructure:"git"`
	// Registries
	Docker *DockerRegistryConfig `yaml:"docker,omitempty" mapstructure:"docker"`
	NPM    *NPMConfig            `yaml:"npm,omitempty" mapstructure:"npm"`
	// Proxy
	Proxy *ProxyConfig `yaml:"proxy,omitempty" mapstructure:"proxy"`
	// Browser
	Browser *BrowserConfig `yaml:"browser,omitempty" mapstructure:"browser"`
	// Custom Environment Variables
	Env map[string]string `yaml:"env,omitempty" mapstructure:"env"`
	// URLs for quick access (ctx open)
	URLs map[string]string `yaml:"urls,omitempty" mapstructure:"urls"`
	// Deactivate behavior (overrides global config)
	Deactivate  *DeactivateConfig `yaml:"deactivate,omitempty" mapstructure:"deactivate"`
	Name        string            `yaml:"name" mapstructure:"name"`
	Extends     string            `yaml:"extends,omitempty" mapstructure:"extends"` // Parent context to inherit from
	Description string            `yaml:"description" mapstructure:"description"`
	Environment Environment       `yaml:"environment" mapstructure:"environment"`
	EnvColor    string            `yaml:"env_color,omitempty" mapstructure:"env_color"` // red, yellow, green, blue, cyan, magenta, white
	Cloud       string            `yaml:"cloud,omitempty" mapstructure:"cloud"`         // Custom cloud provider label (e.g., digitalocean, huawei)
	Tags        []string          `yaml:"tags" mapstructure:"tags"`
	Tunnels     []TunnelConfig    `yaml:"tunnels,omitempty" mapstructure:"tunnels"`
	// Databases
	Databases []DatabaseConfig `yaml:"databases,omitempty" mapstructure:"databases"`
	Abstract  bool             `yaml:"abstract,omitempty" mapstructure:"abstract"` // If true, context is a template and cannot be used directly
}

// GetCloudProviders returns a list of configured cloud providers.
func (c *ContextConfig) GetCloudProviders() []string {
	var providers []string
	// Add custom cloud label first if set
	if c.Cloud != "" {
		providers = append(providers, c.Cloud)
	}
	// Auto-detect configured providers
	if c.AWS != nil {
		providers = append(providers, "aws")
	}
	if c.GCP != nil {
		providers = append(providers, "gcp")
	}
	if c.Azure != nil {
		providers = append(providers, "azure")
	}
	return providers
}

// GetOrchestration returns a list of configured orchestration tools.
func (c *ContextConfig) GetOrchestration() []string {
	var tools []string
	if c.Kubernetes != nil {
		tools = append(tools, "kubernetes")
	}
	if c.Nomad != nil {
		tools = append(tools, "nomad")
	}
	if c.Consul != nil {
		tools = append(tools, "consul")
	}
	return tools
}

// GetExtras returns a list of additional configured tools/features.
func (c *ContextConfig) GetExtras() []string {
	var extras []string
	if c.VPN != nil {
		extras = append(extras, "vpn")
	}
	if c.Bitwarden != nil {
		extras = append(extras, "bitwarden")
	}
	if c.OnePassword != nil {
		extras = append(extras, "1password")
	}
	if c.Vault != nil {
		extras = append(extras, "vault")
	}
	if c.Git != nil {
		extras = append(extras, "git")
	}
	if c.Docker != nil {
		extras = append(extras, "docker")
	}
	if c.NPM != nil {
		extras = append(extras, "npm")
	}
	if len(c.Databases) > 0 {
		extras = append(extras, "databases")
	}
	if c.Proxy != nil {
		extras = append(extras, "proxy")
	}
	return extras
}

// IsProd returns true if the context is for a production environment.
func (c *ContextConfig) IsProd() bool {
	return c.Environment.IsProd()
}

// MergeFrom merges another context config into this one.
// Values from 'other' (parent) fill in missing values in 'c' (child).
// Deep merge: child values take precedence, parent fills in gaps.
//
// Note: Due to Go's type system, boolean fields cannot be "unset" - they default to false.
// This means a parent's `true` boolean cannot be overridden to `false` by a child.
// If you need different boolean values, don't set them in the parent/base context.
func (c *ContextConfig) MergeFrom(other *ContextConfig) {
	// Save fields that should not be inherited
	name := c.Name
	extends := c.Extends
	abstract := c.Abstract
	childTags := c.Tags
	childEnv := c.Env
	childURLs := c.URLs

	// Deep merge parent into child (fills zero values from parent)
	mergo.Merge(c, other)

	// Restore non-inherited fields
	c.Name = name
	c.Extends = extends
	c.Abstract = abstract

	// Maps: merge with child values taking precedence
	if len(other.Env) > 0 || len(childEnv) > 0 {
		merged := make(map[string]string)
		maps.Copy(merged, other.Env)
		// child overrides
		maps.Copy(merged, childEnv)
		c.Env = merged
	}
	if len(other.URLs) > 0 || len(childURLs) > 0 {
		merged := make(map[string]string)
		maps.Copy(merged, other.URLs)
		// child overrides
		maps.Copy(merged, childURLs)
		c.URLs = merged
	}

	// Tags: merge and deduplicate
	if len(other.Tags) > 0 || len(childTags) > 0 {
		tagSet := make(map[string]bool)
		for _, t := range other.Tags {
			tagSet[t] = true
		}
		for _, t := range childTags {
			tagSet[t] = true
		}
		c.Tags = make([]string, 0, len(tagSet))
		for t := range tagSet {
			c.Tags = append(c.Tags, t)
		}
	}
}

// DeactivateConfig controls behavior when deactivating a context.
type DeactivateConfig struct {
	DisconnectVPN bool `yaml:"disconnect_vpn" mapstructure:"disconnect_vpn"`
	StopTunnels   bool `yaml:"stop_tunnels" mapstructure:"stop_tunnels"`
}

// NewDeactivateConfigDefaults returns default deactivate config (all true).
func NewDeactivateConfigDefaults() *DeactivateConfig {
	return &DeactivateConfig{
		DisconnectVPN: true,
		StopTunnels:   true,
	}
}

// AppConfig represents the main application configuration.
type AppConfig struct {
	Deactivate       *DeactivateConfig `yaml:"deactivate,omitempty" mapstructure:"deactivate"`
	Cloud            *CloudConfig      `yaml:"cloud,omitempty" mapstructure:"cloud"`
	DefaultContext   string            `yaml:"default_context" mapstructure:"default_context"`
	PromptFormat     string            `yaml:"prompt_format" mapstructure:"prompt_format"`
	TunnelsDir       string            `yaml:"tunnels_dir" mapstructure:"tunnels_dir"`
	ContextsDir      string            `yaml:"contexts_dir" mapstructure:"contexts_dir"`
	Version          int               `yaml:"version" mapstructure:"version"`
	ShellIntegration bool              `yaml:"shell_integration" mapstructure:"shell_integration"`
	AutoDeactivate   bool              `yaml:"auto_deactivate" mapstructure:"auto_deactivate"`
}

// CloudConfig holds ctx-cloud integration settings.
type CloudConfig struct {
	ServerURL         string `yaml:"server_url" mapstructure:"server_url"`                 // URL of the ctx-cloud server
	Enabled           bool   `yaml:"enabled" mapstructure:"enabled"`                       // Enable cloud integration
	SendAuditEvents   bool   `yaml:"send_audit_events" mapstructure:"send_audit_events"`   // Send audit events to cloud
	SendHeartbeat     bool   `yaml:"send_heartbeat" mapstructure:"send_heartbeat"`         // Send heartbeat to cloud
	HeartbeatInterval int    `yaml:"heartbeat_interval" mapstructure:"heartbeat_interval"` // Heartbeat interval in seconds (default: 30)
}
