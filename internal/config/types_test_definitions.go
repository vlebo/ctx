// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package config

import (
	"testing"
)

func TestEnvironment_IsProd(t *testing.T) {
	tests := []struct {
		name string
		env  Environment
		want bool
	}{
		{"production", EnvProduction, true},
		{"staging", EnvStaging, false},
		{"development", EnvDevelopment, false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.env.IsProd(); got != tt.want {
				t.Errorf("Environment.IsProd() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContextConfig_GetCloudProviders(t *testing.T) {
	tests := []struct {
		name string
		ctx  *ContextConfig
		want []string
	}{
		{
			name: "all providers",
			ctx: &ContextConfig{
				AWS:   &AWSConfig{Profile: "test"},
				GCP:   &GCPConfig{Project: "test"},
				Azure: &AzureConfig{SubscriptionID: "test"},
			},
			want: []string{"aws", "gcp", "azure"},
		},
		{
			name: "aws only",
			ctx: &ContextConfig{
				AWS: &AWSConfig{Profile: "test"},
			},
			want: []string{"aws"},
		},
		{
			name: "none",
			ctx:  &ContextConfig{},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ctx.GetCloudProviders()
			if len(got) != len(tt.want) {
				t.Errorf("GetCloudProviders() = %v, want %v", got, tt.want)
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("GetCloudProviders()[%d] = %v, want %v", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestContextConfig_GetOrchestration(t *testing.T) {
	tests := []struct {
		name string
		ctx  *ContextConfig
		want []string
	}{
		{
			name: "all tools",
			ctx: &ContextConfig{
				Kubernetes: &KubernetesConfig{Context: "test"},
				Nomad:      &NomadConfig{Address: "http://localhost:4646"},
				Consul:     &ConsulConfig{Address: "http://localhost:8500"},
			},
			want: []string{"kubernetes", "nomad", "consul"},
		},
		{
			name: "kubernetes only",
			ctx: &ContextConfig{
				Kubernetes: &KubernetesConfig{Context: "test"},
			},
			want: []string{"kubernetes"},
		},
		{
			name: "none",
			ctx:  &ContextConfig{},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ctx.GetOrchestration()
			if len(got) != len(tt.want) {
				t.Errorf("GetOrchestration() = %v, want %v", got, tt.want)
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("GetOrchestration()[%d] = %v, want %v", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestContextConfig_IsProd(t *testing.T) {
	tests := []struct {
		name string
		ctx  *ContextConfig
		want bool
	}{
		{
			name: "production",
			ctx:  &ContextConfig{Environment: EnvProduction},
			want: true,
		},
		{
			name: "staging",
			ctx:  &ContextConfig{Environment: EnvStaging},
			want: false,
		},
		{
			name: "development",
			ctx:  &ContextConfig{Environment: EnvDevelopment},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ctx.IsProd(); got != tt.want {
				t.Errorf("ContextConfig.IsProd() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContextConfig_GetExtras(t *testing.T) {
	tests := []struct {
		name string
		ctx  *ContextConfig
		want []string
	}{
		{
			name: "all extras",
			ctx: &ContextConfig{
				VPN:       &VPNConfig{Type: VPNTypeWireGuard},
				Vault:     &VaultConfig{Address: "http://localhost:8200"},
				Git:       &GitConfig{UserName: "test"},
				Docker:    &DockerRegistryConfig{URL: "docker.io"},
				NPM:       &NPMConfig{Registry: "https://registry.npmjs.org"},
				Databases: []DatabaseConfig{{Name: "db", Type: DBTypePostgres}},
				Proxy:     &ProxyConfig{HTTP: "http://proxy:8080"},
			},
			want: []string{"vpn", "vault", "git", "docker", "npm", "databases", "proxy"},
		},
		{
			name: "vpn and vault only",
			ctx: &ContextConfig{
				VPN:   &VPNConfig{Type: VPNTypeOpenVPN},
				Vault: &VaultConfig{Address: "http://localhost:8200"},
			},
			want: []string{"vpn", "vault"},
		},
		{
			name: "none",
			ctx:  &ContextConfig{},
			want: nil,
		},
		{
			name: "databases only",
			ctx: &ContextConfig{
				Databases: []DatabaseConfig{
					{Name: "postgres", Type: DBTypePostgres},
					{Name: "redis", Type: DBTypeRedis},
				},
			},
			want: []string{"databases"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ctx.GetExtras()
			if len(got) != len(tt.want) {
				t.Errorf("GetExtras() = %v, want %v", got, tt.want)
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("GetExtras()[%d] = %v, want %v", i, v, tt.want[i])
				}
			}
		})
	}
}
