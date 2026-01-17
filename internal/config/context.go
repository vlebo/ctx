// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"strings"

	"github.com/vlebo/ctx/pkg/types"
)

// ContextSummary provides a summary of a context for display purposes.
type ContextSummary struct {
	Name          string
	Environment   types.Environment
	CloudProvider string
	Orchestration string
	Extras        string
	IsCurrent     bool
}

// GetContextSummary returns a summary of a context for display in lists.
func GetContextSummary(ctx *types.ContextConfig, currentName string) ContextSummary {
	return ContextSummary{
		Name:          ctx.Name,
		Environment:   ctx.Environment,
		CloudProvider: strings.Join(ctx.GetCloudProviders(), ", "),
		Orchestration: strings.Join(ctx.GetOrchestration(), ", "),
		Extras:        strings.Join(ctx.GetExtras(), ", "),
		IsCurrent:     ctx.Name == currentName,
	}
}

// FormatContextDetails formats a context configuration for detailed display.
func FormatContextDetails(ctx *types.ContextConfig) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Name: %s\n", ctx.Name))
	if ctx.Abstract {
		sb.WriteString("Type: abstract (base template)\n")
	}
	if ctx.Extends != "" {
		sb.WriteString(fmt.Sprintf("Extends: %s\n", ctx.Extends))
	}
	if ctx.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", ctx.Description))
	}
	sb.WriteString(fmt.Sprintf("Environment: %s\n", ctx.Environment))

	if len(ctx.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(ctx.Tags, ", ")))
	}

	// Cloud Providers
	if ctx.AWS != nil || ctx.GCP != nil || ctx.Azure != nil {
		sb.WriteString("\nCloud:\n")
		if ctx.AWS != nil {
			sb.WriteString(fmt.Sprintf("  AWS Profile: %s\n", ctx.AWS.Profile))
			if ctx.AWS.Region != "" {
				sb.WriteString(fmt.Sprintf("  AWS Region: %s\n", ctx.AWS.Region))
			}
			if ctx.AWS.UseVault {
				sb.WriteString("  Using aws-vault: yes\n")
			}
		}
		if ctx.GCP != nil {
			sb.WriteString(fmt.Sprintf("  GCP Project: %s\n", ctx.GCP.Project))
			if ctx.GCP.Region != "" {
				sb.WriteString(fmt.Sprintf("  GCP Region: %s\n", ctx.GCP.Region))
			}
			if ctx.GCP.ConfigName != "" {
				sb.WriteString(fmt.Sprintf("  GCP Config: %s\n", ctx.GCP.ConfigName))
			}
		}
		if ctx.Azure != nil {
			sb.WriteString(fmt.Sprintf("  Azure Subscription: %s\n", ctx.Azure.SubscriptionID))
		}
	}

	// Orchestration
	if ctx.Kubernetes != nil || ctx.Nomad != nil || ctx.Consul != nil {
		sb.WriteString("\nOrchestration:\n")
		if ctx.Kubernetes != nil {
			sb.WriteString(fmt.Sprintf("  Kubernetes Context: %s\n", ctx.Kubernetes.Context))
			if ctx.Kubernetes.Namespace != "" {
				sb.WriteString(fmt.Sprintf("  Kubernetes Namespace: %s\n", ctx.Kubernetes.Namespace))
			}
			if ctx.Kubernetes.Kubeconfig != "" {
				sb.WriteString(fmt.Sprintf("  Kubeconfig: %s\n", ctx.Kubernetes.Kubeconfig))
			}
		}
		if ctx.Nomad != nil {
			sb.WriteString(fmt.Sprintf("  Nomad Address: %s\n", ctx.Nomad.Address))
			if ctx.Nomad.Namespace != "" {
				sb.WriteString(fmt.Sprintf("  Nomad Namespace: %s\n", ctx.Nomad.Namespace))
			}
		}
		if ctx.Consul != nil {
			sb.WriteString(fmt.Sprintf("  Consul Address: %s\n", ctx.Consul.Address))
		}
	}

	// SSH
	if ctx.SSH != nil && ctx.SSH.Bastion.Host != "" {
		sb.WriteString("\nSSH:\n")
		sb.WriteString(fmt.Sprintf("  Bastion: %s@%s:%d\n",
			ctx.SSH.Bastion.User,
			ctx.SSH.Bastion.Host,
			ctx.SSH.Bastion.Port))
		if ctx.SSH.Bastion.IdentityFile != "" {
			sb.WriteString(fmt.Sprintf("  Identity File: %s\n", ctx.SSH.Bastion.IdentityFile))
		}
	}

	// Tunnels
	if len(ctx.Tunnels) > 0 {
		sb.WriteString("\nTunnels:\n")
		for _, t := range ctx.Tunnels {
			sb.WriteString(fmt.Sprintf("  %s: localhost:%d → %s:%d\n",
				t.Name, t.LocalPort, t.RemoteHost, t.RemotePort))
			if t.Description != "" {
				sb.WriteString(fmt.Sprintf("    %s\n", t.Description))
			}
		}
	}

	// VPN
	if ctx.VPN != nil {
		sb.WriteString("\nVPN:\n")
		sb.WriteString(fmt.Sprintf("  Type: %s\n", ctx.VPN.Type))
		if ctx.VPN.ConfigFile != "" {
			sb.WriteString(fmt.Sprintf("  Config File: %s\n", ctx.VPN.ConfigFile))
		}
		if ctx.VPN.Interface != "" {
			sb.WriteString(fmt.Sprintf("  Interface: %s\n", ctx.VPN.Interface))
		}
		if ctx.VPN.ExitNode != "" {
			sb.WriteString(fmt.Sprintf("  Exit Node: %s\n", ctx.VPN.ExitNode))
		}
		if ctx.VPN.AutoConnect {
			sb.WriteString("  Auto Connect: yes\n")
		}
	}

	// Bitwarden
	if ctx.Bitwarden != nil {
		sb.WriteString("\nBitwarden:\n")
		if ctx.Bitwarden.AutoLogin {
			sb.WriteString("  Auto Login: yes\n")
		}
		if ctx.Bitwarden.SSO {
			sb.WriteString("  SSO: yes\n")
		}
	}

	// 1Password
	if ctx.OnePassword != nil {
		sb.WriteString("\n1Password:\n")
		if ctx.OnePassword.AutoLogin {
			sb.WriteString("  Auto Login: yes\n")
		}
		if ctx.OnePassword.SSO {
			sb.WriteString("  SSO: yes\n")
		}
		if ctx.OnePassword.Account != "" {
			sb.WriteString(fmt.Sprintf("  Account: %s\n", ctx.OnePassword.Account))
		}
	}

	// Vault
	if ctx.Vault != nil {
		sb.WriteString("\nVault:\n")
		sb.WriteString(fmt.Sprintf("  Address: %s\n", ctx.Vault.Address))
		if ctx.Vault.Namespace != "" {
			sb.WriteString(fmt.Sprintf("  Namespace: %s\n", ctx.Vault.Namespace))
		}
		if ctx.Vault.AuthMethod != "" {
			sb.WriteString(fmt.Sprintf("  Auth Method: %s\n", ctx.Vault.AuthMethod))
		}
		if ctx.Vault.AutoLogin {
			sb.WriteString("  Auto Login: yes\n")
		}
	}

	// Secrets
	if ctx.Secrets != nil {
		totalSecrets := len(ctx.Secrets.Bitwarden) + len(ctx.Secrets.OnePassword) + len(ctx.Secrets.Vault) +
			len(ctx.Secrets.AWSSecretsManager) + len(ctx.Secrets.AWSSSM) + len(ctx.Secrets.GCPSecretManager)
		if totalSecrets > 0 {
			sb.WriteString("\nSecrets:\n")
			if len(ctx.Secrets.Bitwarden) > 0 {
				sb.WriteString(fmt.Sprintf("  Bitwarden: %d item(s)\n", len(ctx.Secrets.Bitwarden)))
			}
			if len(ctx.Secrets.OnePassword) > 0 {
				sb.WriteString(fmt.Sprintf("  1Password: %d item(s)\n", len(ctx.Secrets.OnePassword)))
			}
			if len(ctx.Secrets.Vault) > 0 {
				sb.WriteString(fmt.Sprintf("  Vault: %d item(s)\n", len(ctx.Secrets.Vault)))
			}
			if len(ctx.Secrets.AWSSecretsManager) > 0 {
				sb.WriteString(fmt.Sprintf("  AWS Secrets Manager: %d item(s)\n", len(ctx.Secrets.AWSSecretsManager)))
			}
			if len(ctx.Secrets.AWSSSM) > 0 {
				sb.WriteString(fmt.Sprintf("  AWS Parameter Store: %d item(s)\n", len(ctx.Secrets.AWSSSM)))
			}
			if len(ctx.Secrets.GCPSecretManager) > 0 {
				sb.WriteString(fmt.Sprintf("  GCP Secret Manager: %d item(s)\n", len(ctx.Secrets.GCPSecretManager)))
			}
		}
	}

	// Git Identity
	if ctx.Git != nil {
		sb.WriteString("\nGit Identity:\n")
		if ctx.Git.UserName != "" {
			sb.WriteString(fmt.Sprintf("  Name: %s\n", ctx.Git.UserName))
		}
		if ctx.Git.UserEmail != "" {
			sb.WriteString(fmt.Sprintf("  Email: %s\n", ctx.Git.UserEmail))
		}
		if ctx.Git.SigningKey != "" {
			sb.WriteString(fmt.Sprintf("  Signing Key: %s\n", ctx.Git.SigningKey))
		}
		if ctx.Git.GPGSign {
			sb.WriteString("  GPG Signing: enabled\n")
		}
	}

	// Docker
	if ctx.Docker != nil {
		sb.WriteString("\nDocker:\n")
		if ctx.Docker.Context != "" {
			sb.WriteString(fmt.Sprintf("  Context: %s\n", ctx.Docker.Context))
		}
		if ctx.Docker.URL != "" {
			sb.WriteString(fmt.Sprintf("  Registry: %s\n", ctx.Docker.URL))
		}
		if ctx.Docker.Username != "" {
			sb.WriteString(fmt.Sprintf("  Username: %s\n", ctx.Docker.Username))
		}
	}

	// NPM
	if ctx.NPM != nil {
		sb.WriteString("\nNPM:\n")
		sb.WriteString(fmt.Sprintf("  Registry: %s\n", ctx.NPM.Registry))
		if ctx.NPM.Scope != "" {
			sb.WriteString(fmt.Sprintf("  Scope: %s\n", ctx.NPM.Scope))
		}
	}

	// Databases
	if len(ctx.Databases) > 0 {
		sb.WriteString("\nDatabases:\n")
		for _, db := range ctx.Databases {
			sb.WriteString(fmt.Sprintf("  %s (%s):\n", db.Name, db.Type))
			sb.WriteString(fmt.Sprintf("    Host: %s:%d\n", db.Host, db.Port))
			if db.Database != "" {
				sb.WriteString(fmt.Sprintf("    Database: %s\n", db.Database))
			}
			if db.Username != "" {
				sb.WriteString(fmt.Sprintf("    Username: %s\n", db.Username))
			}
		}
	}

	// Proxy
	if ctx.Proxy != nil {
		sb.WriteString("\nProxy:\n")
		if ctx.Proxy.HTTP != "" {
			sb.WriteString(fmt.Sprintf("  HTTP: %s\n", ctx.Proxy.HTTP))
		}
		if ctx.Proxy.HTTPS != "" {
			sb.WriteString(fmt.Sprintf("  HTTPS: %s\n", ctx.Proxy.HTTPS))
		}
		if ctx.Proxy.NoProxy != "" {
			sb.WriteString(fmt.Sprintf("  No Proxy: %s\n", ctx.Proxy.NoProxy))
		}
	}

	// Custom Environment Variables
	if len(ctx.Env) > 0 {
		sb.WriteString("\nCustom Environment Variables:\n")
		for k, v := range ctx.Env {
			sb.WriteString(fmt.Sprintf("  %s=%s\n", k, v))
		}
	}

	return sb.String()
}

// ValidateContext validates a context configuration.
func ValidateContext(ctx *types.ContextConfig) error {
	if ctx.Name == "" {
		return fmt.Errorf("context name is required")
	}

	// Validate tunnels
	for i, t := range ctx.Tunnels {
		if t.Name == "" {
			return fmt.Errorf("tunnel %d: name is required", i)
		}
		if t.RemoteHost == "" {
			return fmt.Errorf("tunnel %s: remote_host is required", t.Name)
		}
		if t.RemotePort <= 0 || t.RemotePort > 65535 {
			return fmt.Errorf("tunnel %s: invalid remote_port %d", t.Name, t.RemotePort)
		}
		if t.LocalPort <= 0 || t.LocalPort > 65535 {
			return fmt.Errorf("tunnel %s: invalid local_port %d", t.Name, t.LocalPort)
		}
	}

	// If tunnels are defined, SSH bastion should be configured
	if len(ctx.Tunnels) > 0 {
		if ctx.SSH == nil || ctx.SSH.Bastion.Host == "" {
			return fmt.Errorf("SSH bastion must be configured when tunnels are defined")
		}
	}

	return nil
}
