// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vlebo/ctx/pkg/types"
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

	config := &types.AppConfig{
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

	ctx := &types.ContextConfig{
		Name:        "test-context",
		Description: "Test Description",
		Environment: types.EnvDevelopment,
		AWS: &types.AWSConfig{
			Profile: "test-profile",
			Region:  "us-east-1",
		},
		Kubernetes: &types.KubernetesConfig{
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
	contexts := []*types.ContextConfig{
		{Name: "context-a", Environment: types.EnvDevelopment},
		{Name: "context-b", Environment: types.EnvStaging},
		{Name: "context-c", Environment: types.EnvProduction},
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

	ctx := &types.ContextConfig{
		Name:        "to-delete",
		Environment: types.EnvDevelopment,
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
	ctx := &types.ContextConfig{
		Name:        "current-test",
		Environment: types.EnvDevelopment,
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

	ctx := &types.ContextConfig{
		Name:        "env-test",
		Environment: types.EnvProduction,
		AWS: &types.AWSConfig{
			Profile: "aws-profile",
			Region:  "us-west-2",
		},
		GCP: &types.GCPConfig{
			Project:    "gcp-project",
			ConfigName: "gcp-config",
		},
		Nomad: &types.NomadConfig{
			Address:   "http://nomad:4646",
			Namespace: "default",
		},
		Consul: &types.ConsulConfig{
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

func TestManager_WriteEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	ctx := &types.ContextConfig{
		Name:        "env-file-test",
		Environment: types.EnvDevelopment,
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
	base := &types.ContextConfig{
		Name:        "base-context",
		Description: "Base context",
		Environment: types.EnvDevelopment,
		AWS: &types.AWSConfig{
			Region: "us-east-1",
		},
		Kubernetes: &types.KubernetesConfig{
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
	child := &types.ContextConfig{
		Name:        "child-context",
		Extends:     "base-context",
		Description: "Child context",
		Environment: types.EnvProduction,
		AWS: &types.AWSConfig{
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
	if loaded.Environment != types.EnvProduction {
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
	contextA := &types.ContextConfig{
		Name:    "context-a",
		Extends: "context-b",
	}
	contextB := &types.ContextConfig{
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
	grandparent := &types.ContextConfig{
		Name:        "grandparent",
		Environment: types.EnvDevelopment,
		AWS: &types.AWSConfig{
			Region: "us-east-1",
		},
		Env: map[string]string{
			"LEVEL": "grandparent",
		},
	}
	parent := &types.ContextConfig{
		Name:    "parent",
		Extends: "grandparent",
		Kubernetes: &types.KubernetesConfig{
			Context: "parent-k8s",
		},
	}
	child := &types.ContextConfig{
		Name:        "child",
		Extends:     "parent",
		Environment: types.EnvProduction,
	}

	m.SaveContext(grandparent)
	m.SaveContext(parent)
	m.SaveContext(child)

	loaded, err := m.LoadContext("child")
	if err != nil {
		t.Fatalf("LoadContext() error = %v", err)
	}

	// Child overrides
	if loaded.Environment != types.EnvProduction {
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

	child := &types.ContextConfig{
		Name:    "orphan",
		Extends: "nonexistent-parent",
	}
	m.SaveContext(child)

	_, err := m.LoadContext("orphan")
	if err == nil {
		t.Error("LoadContext() should fail when parent not found")
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
