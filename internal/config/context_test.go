// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package config

import (
	"strings"
	"testing"
)

func TestGetContextSummary(t *testing.T) {
	ctx := &ContextConfig{
		Name:        "test-context",
		Environment: EnvProduction,
		AWS:         &AWSConfig{Profile: "test"},
		Kubernetes:  &KubernetesConfig{Context: "k8s-ctx"},
	}

	// Test when context is current
	summary := GetContextSummary(ctx, "test-context")
	if !summary.IsCurrent {
		t.Error("IsCurrent should be true")
	}
	if summary.Name != "test-context" {
		t.Errorf("Name = %v, want %v", summary.Name, "test-context")
	}
	if summary.Environment != EnvProduction {
		t.Errorf("Environment = %v, want %v", summary.Environment, EnvProduction)
	}
	if summary.CloudProvider != "aws" {
		t.Errorf("CloudProvider = %v, want %v", summary.CloudProvider, "aws")
	}
	if summary.Orchestration != "kubernetes" {
		t.Errorf("Orchestration = %v, want %v", summary.Orchestration, "kubernetes")
	}

	// Test when context is not current
	summary = GetContextSummary(ctx, "other-context")
	if summary.IsCurrent {
		t.Error("IsCurrent should be false")
	}
}

func TestFormatContextDetails(t *testing.T) {
	ctx := &ContextConfig{
		Name:        "test-context",
		Description: "Test Description",
		Environment: EnvProduction,
		Tags:        []string{"tag1", "tag2"},
		AWS: &AWSConfig{
			Profile:  "aws-profile",
			Region:   "us-west-2",
			UseVault: true,
		},
		GCP: &GCPConfig{
			Project:    "gcp-project",
			Region:     "us-central1",
			ConfigName: "gcp-config",
		},
		Kubernetes: &KubernetesConfig{
			Context:   "k8s-context",
			Namespace: "default",
		},
		Nomad: &NomadConfig{
			Address:   "http://nomad:4646",
			Namespace: "default",
		},
		Consul: &ConsulConfig{
			Address: "http://consul:8500",
		},
		SSH: &SSHConfig{
			Bastion: BastionConfig{
				Host:         "bastion.example.com",
				User:         "admin",
				Port:         22,
				IdentityFile: "~/.ssh/id_rsa",
			},
		},
		Tunnels: []TunnelConfig{
			{
				Name:        "postgres",
				Description: "PostgreSQL",
				RemoteHost:  "postgres.internal",
				RemotePort:  5432,
				LocalPort:   5432,
			},
		},
		Env: map[string]string{
			"CUSTOM_VAR": "custom_value",
		},
	}

	output := FormatContextDetails(ctx)

	// Check that all expected sections are present
	expectedStrings := []string{
		"Name: test-context",
		"Description: Test Description",
		"Environment: production",
		"Tags: tag1, tag2",
		"Cloud:",
		"AWS Profile: aws-profile",
		"AWS Region: us-west-2",
		"Using aws-vault: yes",
		"GCP Project: gcp-project",
		"GCP Region: us-central1",
		"Orchestration:",
		"Kubernetes Context: k8s-context",
		"Nomad Address: http://nomad:4646",
		"Consul Address: http://consul:8500",
		"SSH:",
		"Bastion: admin@bastion.example.com:22",
		"Tunnels:",
		"postgres: localhost:5432",
		"Custom Environment Variables:",
		"CUSTOM_VAR=custom_value",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Output missing expected string: %q", expected)
		}
	}
}

func TestValidateContext(t *testing.T) {
	tests := []struct {
		name    string
		ctx     *ContextConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid context",
			ctx:     &ContextConfig{Name: "test", Environment: EnvDevelopment},
			wantErr: false,
		},
		{
			name:    "missing name",
			ctx:     &ContextConfig{Environment: EnvDevelopment},
			wantErr: true,
			errMsg:  "context name is required",
		},
		{
			name: "valid tunnel config",
			ctx: &ContextConfig{
				Name: "test",
				SSH: &SSHConfig{
					Bastion: BastionConfig{Host: "bastion.example.com"},
				},
				Tunnels: []TunnelConfig{
					{
						Name:       "postgres",
						RemoteHost: "postgres.internal",
						RemotePort: 5432,
						LocalPort:  5432,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "tunnel without name",
			ctx: &ContextConfig{
				Name: "test",
				SSH: &SSHConfig{
					Bastion: BastionConfig{Host: "bastion.example.com"},
				},
				Tunnels: []TunnelConfig{
					{
						RemoteHost: "postgres.internal",
						RemotePort: 5432,
						LocalPort:  5432,
					},
				},
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "tunnel without remote host",
			ctx: &ContextConfig{
				Name: "test",
				SSH: &SSHConfig{
					Bastion: BastionConfig{Host: "bastion.example.com"},
				},
				Tunnels: []TunnelConfig{
					{
						Name:       "postgres",
						RemotePort: 5432,
						LocalPort:  5432,
					},
				},
			},
			wantErr: true,
			errMsg:  "remote_host is required",
		},
		{
			name: "invalid remote port",
			ctx: &ContextConfig{
				Name: "test",
				SSH: &SSHConfig{
					Bastion: BastionConfig{Host: "bastion.example.com"},
				},
				Tunnels: []TunnelConfig{
					{
						Name:       "postgres",
						RemoteHost: "postgres.internal",
						RemotePort: 0,
						LocalPort:  5432,
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid remote_port",
		},
		{
			name: "invalid local port",
			ctx: &ContextConfig{
				Name: "test",
				SSH: &SSHConfig{
					Bastion: BastionConfig{Host: "bastion.example.com"},
				},
				Tunnels: []TunnelConfig{
					{
						Name:       "postgres",
						RemoteHost: "postgres.internal",
						RemotePort: 5432,
						LocalPort:  70000,
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid local_port",
		},
		{
			name: "tunnel without bastion",
			ctx: &ContextConfig{
				Name: "test",
				Tunnels: []TunnelConfig{
					{
						Name:       "postgres",
						RemoteHost: "postgres.internal",
						RemotePort: 5432,
						LocalPort:  5432,
					},
				},
			},
			wantErr: true,
			errMsg:  "SSH bastion must be configured",
		},
		{
			name: "valid AKS config",
			ctx: &ContextConfig{
				Name: "test",
				Kubernetes: &KubernetesConfig{
					AKS: &AKSConfig{
						Cluster:       "my-cluster",
						ResourceGroup: "my-rg",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AKS missing cluster",
			ctx: &ContextConfig{
				Name: "test",
				Kubernetes: &KubernetesConfig{
					AKS: &AKSConfig{
						ResourceGroup: "my-rg",
					},
				},
			},
			wantErr: true,
			errMsg:  "kubernetes.aks.cluster is required",
		},
		{
			name: "AKS missing resource group",
			ctx: &ContextConfig{
				Name: "test",
				Kubernetes: &KubernetesConfig{
					AKS: &AKSConfig{
						Cluster: "my-cluster",
					},
				},
			},
			wantErr: true,
			errMsg:  "kubernetes.aks.resource_group is required",
		},
		{
			name: "valid EKS config",
			ctx: &ContextConfig{
				Name: "test",
				Kubernetes: &KubernetesConfig{
					EKS: &EKSConfig{
						Cluster: "my-cluster",
						Region:  "us-east-1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "EKS missing cluster",
			ctx: &ContextConfig{
				Name: "test",
				Kubernetes: &KubernetesConfig{
					EKS: &EKSConfig{
						Region: "us-east-1",
					},
				},
			},
			wantErr: true,
			errMsg:  "kubernetes.eks.cluster is required",
		},
		{
			name: "valid GKE config with zone",
			ctx: &ContextConfig{
				Name: "test",
				Kubernetes: &KubernetesConfig{
					GKE: &GKEConfig{
						Cluster: "my-cluster",
						Zone:    "us-central1-a",
						Project: "my-project",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GKE missing cluster",
			ctx: &ContextConfig{
				Name: "test",
				Kubernetes: &KubernetesConfig{
					GKE: &GKEConfig{
						Zone:    "us-central1-a",
						Project: "my-project",
					},
				},
			},
			wantErr: true,
			errMsg:  "kubernetes.gke.cluster is required",
		},
		{
			name: "GKE missing zone and region",
			ctx: &ContextConfig{
				Name: "test",
				Kubernetes: &KubernetesConfig{
					GKE: &GKEConfig{
						Cluster: "my-cluster",
						Project: "my-project",
					},
				},
			},
			wantErr: true,
			errMsg:  "kubernetes.gke requires either zone or region",
		},
		{
			name: "multiple cloud k8s providers",
			ctx: &ContextConfig{
				Name: "test",
				Kubernetes: &KubernetesConfig{
					AKS: &AKSConfig{
						Cluster:       "aks-cluster",
						ResourceGroup: "my-rg",
					},
					EKS: &EKSConfig{
						Cluster: "eks-cluster",
					},
				},
			},
			wantErr: true,
			errMsg:  "only one of aks, eks, or gke",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContext(tt.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidateContext() error = %v, want error containing %q", err, tt.errMsg)
			}
		})
	}
}
