// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManagerWithDir(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	if m.ConfigDir() != tmpDir {
		t.Errorf("ConfigDir() = %v, want %v", m.ConfigDir(), tmpDir)
	}

	expectedContextsDir := filepath.Join(tmpDir, ContextsSubdir)
	if m.ContextsDir() != expectedContextsDir {
		t.Errorf("ContextsDir() = %v, want %v", m.ContextsDir(), expectedContextsDir)
	}

	expectedStateDir := filepath.Join(tmpDir, StateSubdir)
	if m.StateDir() != expectedStateDir {
		t.Errorf("StateDir() = %v, want %v", m.StateDir(), expectedStateDir)
	}
}

func TestManager_EnsureDirs(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	err := m.EnsureDirs()
	if err != nil {
		t.Fatalf("EnsureDirs() error = %v", err)
	}

	// Check directories exist
	dirs := []string{
		m.ConfigDir(),
		m.ContextsDir(),
		m.StateDir(),
	}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Directory %s was not created", dir)
		}
	}
}

func TestManager_LoadAppConfig_DefaultWhenMissing(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	config, err := m.LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}

	if config.Version != 1 {
		t.Errorf("Version = %v, want %v", config.Version, 1)
	}

	if config.ShellIntegration != true {
		t.Errorf("ShellIntegration = %v, want %v", config.ShellIntegration, true)
	}
}

func TestManager_SaveAndLoadAppConfig(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	config := &AppConfig{
		Version:          1,
		DefaultContext:   "test-context",
		ShellIntegration: true,
		PromptFormat:     "[ctx: {{.Name}}]",
		ContextsDir:      m.ContextsDir(),
		TunnelsDir:       filepath.Join(tmpDir, "tunnels"),
	}

	err := m.SaveAppConfig(config)
	if err != nil {
		t.Fatalf("SaveAppConfig() error = %v", err)
	}

	loaded, err := m.LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}

	if loaded.DefaultContext != config.DefaultContext {
		t.Errorf("DefaultContext = %v, want %v", loaded.DefaultContext, config.DefaultContext)
	}
}

func TestManager_SaveAndLoadContext(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	ctx := &ContextConfig{
		Name:        "test-context",
		Description: "Test Description",
		Environment: EnvDevelopment,
		AWS: &AWSConfig{
			Profile: "test-profile",
			Region:  "us-east-1",
		},
		Kubernetes: &KubernetesConfig{
			Context:   "test-k8s-context",
			Namespace: "default",
		},
		Env: map[string]string{
			"TEST_VAR": "test_value",
		},
	}

	err := m.SaveContext(ctx)
	if err != nil {
		t.Fatalf("SaveContext() error = %v", err)
	}

	loaded, err := m.LoadContext("test-context")
	if err != nil {
		t.Fatalf("LoadContext() error = %v", err)
	}

	if loaded.Name != ctx.Name {
		t.Errorf("Name = %v, want %v", loaded.Name, ctx.Name)
	}
	if loaded.Description != ctx.Description {
		t.Errorf("Description = %v, want %v", loaded.Description, ctx.Description)
	}
	if loaded.Environment != ctx.Environment {
		t.Errorf("Environment = %v, want %v", loaded.Environment, ctx.Environment)
	}
	if loaded.AWS == nil || loaded.AWS.Profile != ctx.AWS.Profile {
		t.Errorf("AWS.Profile mismatch")
	}
	if loaded.Kubernetes == nil || loaded.Kubernetes.Context != ctx.Kubernetes.Context {
		t.Errorf("Kubernetes.Context mismatch")
	}
}

func TestManager_LoadContext_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)
	m.EnsureDirs()

	_, err := m.LoadContext("nonexistent")
	if err == nil {
		t.Error("LoadContext() expected error for nonexistent context")
	}
}

func TestManager_ListContexts(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	// Save some test contexts
	contexts := []*ContextConfig{
		{Name: "context-a", Environment: EnvDevelopment},
		{Name: "context-b", Environment: EnvStaging},
		{Name: "context-c", Environment: EnvProduction},
	}

	for _, ctx := range contexts {
		if err := m.SaveContext(ctx); err != nil {
			t.Fatalf("SaveContext() error = %v", err)
		}
	}

	names, err := m.ListContexts()
	if err != nil {
		t.Fatalf("ListContexts() error = %v", err)
	}

	if len(names) != len(contexts) {
		t.Errorf("ListContexts() returned %d contexts, want %d", len(names), len(contexts))
	}
}

func TestManager_DeleteContext(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	ctx := &ContextConfig{
		Name:        "to-delete",
		Environment: EnvDevelopment,
	}

	err := m.SaveContext(ctx)
	if err != nil {
		t.Fatalf("SaveContext() error = %v", err)
	}

	// Verify it exists
	if !m.ContextExists("to-delete") {
		t.Fatal("Context should exist before deletion")
	}

	err = m.DeleteContext("to-delete")
	if err != nil {
		t.Fatalf("DeleteContext() error = %v", err)
	}

	// Verify it's gone
	if m.ContextExists("to-delete") {
		t.Error("Context should not exist after deletion")
	}
}

func TestManager_CurrentContext(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	// Create a context first
	ctx := &ContextConfig{
		Name:        "current-test",
		Environment: EnvDevelopment,
	}
	m.SaveContext(ctx)

	// Initially no current context
	name, err := m.GetCurrentContextName()
	if err != nil {
		t.Fatalf("GetCurrentContextName() error = %v", err)
	}
	if name != "" {
		t.Errorf("GetCurrentContextName() = %v, want empty string", name)
	}

	// Set current context
	err = m.SetCurrentContext("current-test")
	if err != nil {
		t.Fatalf("SetCurrentContext() error = %v", err)
	}

	// Verify current context
	name, err = m.GetCurrentContextName()
	if err != nil {
		t.Fatalf("GetCurrentContextName() error = %v", err)
	}
	if name != "current-test" {
		t.Errorf("GetCurrentContextName() = %v, want %v", name, "current-test")
	}

	// Get full current context
	current, err := m.GetCurrentContext()
	if err != nil {
		t.Fatalf("GetCurrentContext() error = %v", err)
	}
	if current == nil || current.Name != "current-test" {
		t.Errorf("GetCurrentContext() returned wrong context")
	}

	// Clear current context
	err = m.ClearCurrentContext()
	if err != nil {
		t.Fatalf("ClearCurrentContext() error = %v", err)
	}

	name, _ = m.GetCurrentContextName()
	if name != "" {
		t.Errorf("After clear, GetCurrentContextName() = %v, want empty", name)
	}
}

func TestManager_GenerateEnvVars(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	ctx := &ContextConfig{
		Name:        "env-test",
		Environment: EnvProduction,
		AWS: &AWSConfig{
			Profile: "aws-profile",
			Region:  "us-west-2",
		},
		GCP: &GCPConfig{
			Project:    "gcp-project",
			ConfigName: "gcp-config",
		},
		Nomad: &NomadConfig{
			Address:   "http://nomad:4646",
			Namespace: "default",
		},
		Consul: &ConsulConfig{
			Address: "http://consul:8500",
		},
		Env: map[string]string{
			"CUSTOM_VAR": "custom_value",
		},
	}

	envVars := m.GenerateEnvVars(ctx)

	expected := map[string]string{
		"AWS_PROFILE":                 "aws-profile",
		"AWS_REGION":                  "us-west-2",
		"AWS_DEFAULT_REGION":          "us-west-2",
		"CLOUDSDK_ACTIVE_CONFIG_NAME": "gcp-config",
		"CLOUDSDK_CORE_PROJECT":       "gcp-project",
		"GOOGLE_CLOUD_PROJECT":        "gcp-project",
		"NOMAD_ADDR":                  "http://nomad:4646",
		"NOMAD_NAMESPACE":             "default",
		"CONSUL_HTTP_ADDR":            "http://consul:8500",
		"CUSTOM_VAR":                  "custom_value",
		"CTX_CURRENT":                 "env-test",
		"CTX_ENVIRONMENT":             "production",
	}

	for k, v := range expected {
		if envVars[k] != v {
			t.Errorf("envVars[%s] = %v, want %v", k, envVars[k], v)
		}
	}
}

func TestExpandConfigVars(t *testing.T) {
	t.Run("basic expansion in nested structs", func(t *testing.T) {
		cfg := &ContextConfig{
			Name:        "alpha",
			Environment: EnvProduction,
			Env: map[string]string{
				"CLUSTER_NAME": "alpha",
			},
			Kubernetes: &KubernetesConfig{
				Context:    "acme-cloud-${CLUSTER_NAME}-cluster",
				Namespace:  "default",
				Kubeconfig: "~/clusters/acme-cloud/${CLUSTER_NAME}/config",
			},
		}

		ExpandConfigVars(cfg)

		if cfg.Kubernetes.Context != "acme-cloud-alpha-cluster" {
			t.Errorf("Kubernetes.Context = %v, want acme-cloud-alpha-cluster", cfg.Kubernetes.Context)
		}
		if cfg.Kubernetes.Kubeconfig != "~/clusters/acme-cloud/alpha/config" {
			t.Errorf("Kubernetes.Kubeconfig = %v, want ~/clusters/acme-cloud/alpha/config", cfg.Kubernetes.Kubeconfig)
		}
		if cfg.Kubernetes.Namespace != "default" {
			t.Errorf("Kubernetes.Namespace = %v, want default", cfg.Kubernetes.Namespace)
		}
	})

	t.Run("expansion in tunnels slice", func(t *testing.T) {
		cfg := &ContextConfig{
			Name: "test",
			Env: map[string]string{
				"CLUSTER_NAME": "alpha",
			},
			Tunnels: []TunnelConfig{
				{
					Name:       "bastion",
					RemoteHost: "bastion.${CLUSTER_NAME}.example.com",
					RemotePort: 8080,
					LocalPort:  8080,
				},
			},
		}

		ExpandConfigVars(cfg)

		if cfg.Tunnels[0].RemoteHost != "bastion.alpha.example.com" {
			t.Errorf("Tunnels[0].RemoteHost = %v, want bastion.alpha.example.com", cfg.Tunnels[0].RemoteHost)
		}
	})

	t.Run("expansion in map values (urls)", func(t *testing.T) {
		cfg := &ContextConfig{
			Name: "test",
			Env: map[string]string{
				"CLUSTER_NAME": "alpha",
			},
			URLs: map[string]string{
				"dashboard": "https://${CLUSTER_NAME}.example.com/dashboard",
				"argocd":    "https://argocd.${CLUSTER_NAME}.example.com",
			},
		}

		ExpandConfigVars(cfg)

		if cfg.URLs["dashboard"] != "https://alpha.example.com/dashboard" {
			t.Errorf("URLs[dashboard] = %v, want https://alpha.example.com/dashboard", cfg.URLs["dashboard"])
		}
		if cfg.URLs["argocd"] != "https://argocd.alpha.example.com" {
			t.Errorf("URLs[argocd] = %v, want https://argocd.alpha.example.com", cfg.URLs["argocd"])
		}
	})

	t.Run("undefined vars left as-is", func(t *testing.T) {
		cfg := &ContextConfig{
			Name: "test",
			Env: map[string]string{
				"CLUSTER_NAME": "alpha",
			},
			Kubernetes: &KubernetesConfig{
				Context: "acme-cloud-${CLUSTER_NAME}-${UNDEFINED_VAR}",
			},
		}

		ExpandConfigVars(cfg)

		if cfg.Kubernetes.Context != "acme-cloud-alpha-${UNDEFINED_VAR}" {
			t.Errorf("Kubernetes.Context = %v, want acme-cloud-alpha-${UNDEFINED_VAR}", cfg.Kubernetes.Context)
		}
	})

	t.Run("name and extends not expanded", func(t *testing.T) {
		cfg := &ContextConfig{
			Name:    "${CLUSTER_NAME}",
			Extends: "${PARENT}",
			Env: map[string]string{
				"CLUSTER_NAME": "alpha",
				"PARENT":       "acme-cloud",
			},
		}

		ExpandConfigVars(cfg)

		if cfg.Name != "${CLUSTER_NAME}" {
			t.Errorf("Name was expanded to %v, should not be expanded", cfg.Name)
		}
		if cfg.Extends != "${PARENT}" {
			t.Errorf("Extends was expanded to %v, should not be expanded", cfg.Extends)
		}
	})

	t.Run("env values not expanded", func(t *testing.T) {
		cfg := &ContextConfig{
			Name: "test",
			Env: map[string]string{
				"CLUSTER_NAME": "alpha",
				"CLUSTER_FQDN": "${CLUSTER_NAME}.example.com",
			},
		}

		ExpandConfigVars(cfg)

		// Env values should NOT be expanded (they're the source, not targets)
		if cfg.Env["CLUSTER_FQDN"] != "${CLUSTER_NAME}.example.com" {
			t.Errorf("Env[CLUSTER_FQDN] = %v, should not be expanded", cfg.Env["CLUSTER_FQDN"])
		}
	})

	t.Run("empty env map no-op", func(t *testing.T) {
		cfg := &ContextConfig{
			Name: "test",
			Kubernetes: &KubernetesConfig{
				Context: "some-${VAR}-cluster",
			},
		}

		ExpandConfigVars(cfg)

		if cfg.Kubernetes.Context != "some-${VAR}-cluster" {
			t.Errorf("Kubernetes.Context = %v, should not change with empty env", cfg.Kubernetes.Context)
		}
	})

	t.Run("multiple variables in one string", func(t *testing.T) {
		cfg := &ContextConfig{
			Name: "test",
			Env: map[string]string{
				"PROVIDER": "acme-cloud",
				"CLUSTER":  "alpha",
				"REGION":   "eu-west",
			},
			Kubernetes: &KubernetesConfig{
				Context: "${PROVIDER}-${REGION}-${CLUSTER}",
			},
		}

		ExpandConfigVars(cfg)

		if cfg.Kubernetes.Context != "acme-cloud-eu-west-alpha" {
			t.Errorf("Kubernetes.Context = %v, want acme-cloud-eu-west-alpha", cfg.Kubernetes.Context)
		}
	})

	t.Run("expansion in deeply nested structs", func(t *testing.T) {
		cfg := &ContextConfig{
			Name: "test",
			Env: map[string]string{
				"CLUSTER_NAME": "alpha",
			},
			SSH: &SSHConfig{
				Bastion: BastionConfig{
					Host: "bastion.${CLUSTER_NAME}.example.com",
					User: "admin",
				},
			},
			VPN: &VPNConfig{
				ConfigFile: "/etc/vpn/${CLUSTER_NAME}.conf",
			},
		}

		ExpandConfigVars(cfg)

		if cfg.SSH.Bastion.Host != "bastion.alpha.example.com" {
			t.Errorf("SSH.Bastion.Host = %v, want bastion.alpha.example.com", cfg.SSH.Bastion.Host)
		}
		if cfg.VPN.ConfigFile != "/etc/vpn/alpha.conf" {
			t.Errorf("VPN.ConfigFile = %v, want /etc/vpn/alpha.conf", cfg.VPN.ConfigFile)
		}
	})
}

func TestManager_GenerateEnvVars_Kubernetes(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	ctx := &ContextConfig{
		Name:        "k8s-test",
		Environment: EnvDevelopment,
		Kubernetes: &KubernetesConfig{
			Context:    "my-cluster",
			Namespace:  "default",
			Kubeconfig: "/path/to/kubeconfig",
		},
	}

	envVars := m.GenerateEnvVars(ctx)

	// Check KUBECONFIG is set
	if envVars["KUBECONFIG"] != "/path/to/kubeconfig" {
		t.Errorf("envVars[KUBECONFIG] = %v, want /path/to/kubeconfig", envVars["KUBECONFIG"])
	}

	// Test with ~ expansion
	ctx2 := &ContextConfig{
		Name:        "k8s-test-home",
		Environment: EnvDevelopment,
		Kubernetes: &KubernetesConfig{
			Kubeconfig: "~/.kube/custom-config",
		},
	}

	envVars2 := m.GenerateEnvVars(ctx2)

	// KUBECONFIG should be expanded (not start with ~)
	if envVars2["KUBECONFIG"] == "~/.kube/custom-config" {
		t.Errorf("envVars[KUBECONFIG] = %v, expected ~ to be expanded", envVars2["KUBECONFIG"])
	}

	// Test without kubeconfig - should not set KUBECONFIG
	ctx3 := &ContextConfig{
		Name:        "k8s-test-no-config",
		Environment: EnvDevelopment,
		Kubernetes: &KubernetesConfig{
			Context:   "my-cluster",
			Namespace: "default",
		},
	}

	envVars3 := m.GenerateEnvVars(ctx3)

	if _, exists := envVars3["KUBECONFIG"]; exists {
		t.Errorf("envVars[KUBECONFIG] should not be set when kubeconfig is empty")
	}
}

func TestManager_WriteEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	ctx := &ContextConfig{
		Name:        "env-file-test",
		Environment: EnvDevelopment,
		Env: map[string]string{
			"TEST_VAR": "test_value",
		},
	}

	err := m.WriteEnvFile(ctx)
	if err != nil {
		t.Fatalf("WriteEnvFile() error = %v", err)
	}

	envPath := filepath.Join(m.StateDir(), CurrentEnvFile)
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("Failed to read env file: %v", err)
	}

	// Check that the file contains expected exports
	if len(content) == 0 {
		t.Error("Env file is empty")
	}
}

func TestManager_LoadContext_Inheritance(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	// Create base context
	base := &ContextConfig{
		Name:        "base-context",
		Description: "Base context",
		Environment: EnvDevelopment,
		AWS: &AWSConfig{
			Region: "us-east-1",
		},
		Kubernetes: &KubernetesConfig{
			Context:   "base-k8s",
			Namespace: "default",
		},
		Env: map[string]string{
			"BASE_VAR":   "base_value",
			"SHARED_VAR": "from_base",
		},
		URLs: map[string]string{
			"dashboard": "https://base.example.com",
		},
	}
	if err := m.SaveContext(base); err != nil {
		t.Fatalf("SaveContext(base) error = %v", err)
	}

	// Create child context that extends base
	child := &ContextConfig{
		Name:        "child-context",
		Extends:     "base-context",
		Description: "Child context",
		Environment: EnvProduction,
		AWS: &AWSConfig{
			Profile: "child-profile",
			Region:  "us-west-2",
		},
		Env: map[string]string{
			"CHILD_VAR":  "child_value",
			"SHARED_VAR": "from_child", // Override base
		},
	}
	if err := m.SaveContext(child); err != nil {
		t.Fatalf("SaveContext(child) error = %v", err)
	}

	// Load child context
	loaded, err := m.LoadContext("child-context")
	if err != nil {
		t.Fatalf("LoadContext() error = %v", err)
	}

	// Verify child values override
	if loaded.Name != "child-context" {
		t.Errorf("Name = %v, want child-context", loaded.Name)
	}
	if loaded.Environment != EnvProduction {
		t.Errorf("Environment = %v, want production", loaded.Environment)
	}
	if loaded.AWS.Region != "us-west-2" {
		t.Errorf("AWS.Region = %v, want us-west-2", loaded.AWS.Region)
	}
	if loaded.AWS.Profile != "child-profile" {
		t.Errorf("AWS.Profile = %v, want child-profile", loaded.AWS.Profile)
	}

	// Verify inherited values
	if loaded.Kubernetes == nil || loaded.Kubernetes.Context != "base-k8s" {
		t.Errorf("Kubernetes.Context should be inherited from base")
	}
	if loaded.URLs["dashboard"] != "https://base.example.com" {
		t.Errorf("URLs[dashboard] should be inherited from base")
	}

	// Verify env var merging
	if loaded.Env["BASE_VAR"] != "base_value" {
		t.Errorf("Env[BASE_VAR] = %v, want base_value", loaded.Env["BASE_VAR"])
	}
	if loaded.Env["CHILD_VAR"] != "child_value" {
		t.Errorf("Env[CHILD_VAR] = %v, want child_value", loaded.Env["CHILD_VAR"])
	}
	if loaded.Env["SHARED_VAR"] != "from_child" {
		t.Errorf("Env[SHARED_VAR] = %v, want from_child (child overrides)", loaded.Env["SHARED_VAR"])
	}
}

func TestManager_LoadContext_CircularInheritance(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	// Create circular dependency: A -> B -> A
	contextA := &ContextConfig{
		Name:    "context-a",
		Extends: "context-b",
	}
	contextB := &ContextConfig{
		Name:    "context-b",
		Extends: "context-a",
	}

	m.SaveContext(contextA)
	m.SaveContext(contextB)

	// Loading either should fail with circular dependency error
	_, err := m.LoadContext("context-a")
	if err == nil {
		t.Error("LoadContext() should fail for circular inheritance")
	}
	if err != nil && !contains(err.Error(), "circular") {
		t.Errorf("Error should mention circular dependency, got: %v", err)
	}
}

func TestManager_LoadContext_MultiLevelInheritance(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	// Create: grandparent -> parent -> child
	grandparent := &ContextConfig{
		Name:        "grandparent",
		Environment: EnvDevelopment,
		AWS: &AWSConfig{
			Region: "us-east-1",
		},
		Env: map[string]string{
			"LEVEL": "grandparent",
		},
	}
	parent := &ContextConfig{
		Name:    "parent",
		Extends: "grandparent",
		Kubernetes: &KubernetesConfig{
			Context: "parent-k8s",
		},
	}
	child := &ContextConfig{
		Name:        "child",
		Extends:     "parent",
		Environment: EnvProduction,
	}

	m.SaveContext(grandparent)
	m.SaveContext(parent)
	m.SaveContext(child)

	loaded, err := m.LoadContext("child")
	if err != nil {
		t.Fatalf("LoadContext() error = %v", err)
	}

	// Child overrides
	if loaded.Environment != EnvProduction {
		t.Errorf("Environment = %v, want production", loaded.Environment)
	}

	// Parent contribution
	if loaded.Kubernetes == nil || loaded.Kubernetes.Context != "parent-k8s" {
		t.Errorf("Kubernetes.Context should be inherited from parent")
	}

	// Grandparent contribution
	if loaded.AWS == nil || loaded.AWS.Region != "us-east-1" {
		t.Errorf("AWS.Region should be inherited from grandparent")
	}
	if loaded.Env["LEVEL"] != "grandparent" {
		t.Errorf("Env[LEVEL] should be inherited from grandparent")
	}
}

func TestManager_LoadContext_ParentNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	child := &ContextConfig{
		Name:    "orphan",
		Extends: "nonexistent-parent",
	}
	m.SaveContext(child)

	_, err := m.LoadContext("orphan")
	if err == nil {
		t.Error("LoadContext() should fail when parent not found")
	}
}

func TestManager_SecretFilesState_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	state := &SecretFilesState{
		ContextName: "test-context",
		Files: map[string]SecretFileEntry{
			"KUBECONFIG": {
				Path:     "/dev/shm/ctx-test-KUBECONFIG-12345",
				EnvVar:   "KUBECONFIG",
				Provider: "onepassword",
			},
			"SSH_KEY": {
				Path:     "/dev/shm/ctx-test-SSH_KEY-67890",
				EnvVar:   "SSH_KEY",
				Provider: "bitwarden",
			},
		},
	}

	// Save
	if err := m.SaveSecretFilesState(state); err != nil {
		t.Fatalf("SaveSecretFilesState() error = %v", err)
	}

	// Load
	loaded, err := m.LoadSecretFilesState("test-context")
	if err != nil {
		t.Fatalf("LoadSecretFilesState() error = %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadSecretFilesState() returned nil")
	}

	if loaded.ContextName != state.ContextName {
		t.Errorf("ContextName = %v, want %v", loaded.ContextName, state.ContextName)
	}
	if len(loaded.Files) != 2 {
		t.Errorf("Files count = %d, want 2", len(loaded.Files))
	}
	if loaded.Files["KUBECONFIG"].Provider != "onepassword" {
		t.Errorf("KUBECONFIG provider = %v, want onepassword", loaded.Files["KUBECONFIG"].Provider)
	}
	if loaded.Files["SSH_KEY"].Provider != "bitwarden" {
		t.Errorf("SSH_KEY provider = %v, want bitwarden", loaded.Files["SSH_KEY"].Provider)
	}
}

func TestManager_SecretFilesState_LoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	loaded, err := m.LoadSecretFilesState("nonexistent")
	if err != nil {
		t.Fatalf("LoadSecretFilesState() error = %v", err)
	}
	if loaded != nil {
		t.Error("LoadSecretFilesState() should return nil for non-existent state")
	}
}

func TestManager_CleanupSecretFiles(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	// Create temp files to simulate secret files
	secretFile1 := filepath.Join(tmpDir, "secret1.txt")
	secretFile2 := filepath.Join(tmpDir, "secret2.txt")
	os.WriteFile(secretFile1, []byte("secret-content-1"), 0o600)
	os.WriteFile(secretFile2, []byte("secret-content-2"), 0o600)

	// Save state
	state := &SecretFilesState{
		ContextName: "cleanup-test",
		Files: map[string]SecretFileEntry{
			"VAR1": {Path: secretFile1, EnvVar: "VAR1", Provider: "bitwarden"},
			"VAR2": {Path: secretFile2, EnvVar: "VAR2", Provider: "vault"},
		},
	}
	if err := m.SaveSecretFilesState(state); err != nil {
		t.Fatalf("SaveSecretFilesState() error = %v", err)
	}

	// Cleanup
	if err := m.CleanupSecretFiles("cleanup-test"); err != nil {
		t.Fatalf("CleanupSecretFiles() error = %v", err)
	}

	// Verify files are deleted
	if _, err := os.Stat(secretFile1); !os.IsNotExist(err) {
		t.Error("Secret file 1 should be deleted")
	}
	if _, err := os.Stat(secretFile2); !os.IsNotExist(err) {
		t.Error("Secret file 2 should be deleted")
	}

	// Verify state file is deleted
	loaded, err := m.LoadSecretFilesState("cleanup-test")
	if err != nil {
		t.Fatalf("LoadSecretFilesState() error = %v", err)
	}
	if loaded != nil {
		t.Error("State should be cleaned up")
	}
}

func TestManager_CleanupSecretFiles_NoState(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	// Should not error when no state exists
	if err := m.CleanupSecretFiles("nonexistent"); err != nil {
		t.Errorf("CleanupSecretFiles() should not error on non-existent state, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
