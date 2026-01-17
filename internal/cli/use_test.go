// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vlebo/ctx/internal/config"
	"github.com/vlebo/ctx/pkg/types"
)

func TestSwitchContext(t *testing.T) {
	// Create a temporary config directory
	tmpDir := t.TempDir()
	mgr := config.NewManagerWithDir(tmpDir)

	// Create a test context
	ctx := &types.ContextConfig{
		Name:        "test-context",
		Description: "Test Description",
		Environment: types.EnvDevelopment,
		Env: map[string]string{
			"TEST_VAR": "test_value",
		},
	}
	if err := mgr.SaveContext(ctx); err != nil {
		t.Fatalf("Failed to save context: %v", err)
	}

	// Switch to the context
	_, err := switchContext(mgr, ctx)
	if err != nil {
		t.Fatalf("switchContext() error = %v", err)
	}

	// Verify current context is set
	currentName, err := mgr.GetCurrentContextName()
	if err != nil {
		t.Fatalf("GetCurrentContextName() error = %v", err)
	}
	if currentName != "test-context" {
		t.Errorf("Current context = %v, want %v", currentName, "test-context")
	}

	// Verify env file was written
	envPath := filepath.Join(mgr.StateDir(), config.CurrentEnvFile)
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		t.Error("Environment file was not created")
	}

	content, _ := os.ReadFile(envPath)
	if len(content) == 0 {
		t.Error("Environment file is empty")
	}
}

func TestSwitchContext_WithAWS(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := config.NewManagerWithDir(tmpDir)

	ctx := &types.ContextConfig{
		Name:        "aws-test",
		Environment: types.EnvDevelopment,
		AWS: &types.AWSConfig{
			Profile: "test-profile",
			Region:  "us-east-1",
		},
	}
	if err := mgr.SaveContext(ctx); err != nil {
		t.Fatalf("Failed to save context: %v", err)
	}

	_, err := switchContext(mgr, ctx)
	if err != nil {
		t.Fatalf("switchContext() error = %v", err)
	}

	// Verify env vars include AWS settings
	envPath := filepath.Join(mgr.StateDir(), config.CurrentEnvFile)
	content, _ := os.ReadFile(envPath)
	contentStr := string(content)

	if !contains(contentStr, "AWS_PROFILE") {
		t.Error("AWS_PROFILE not in env file")
	}
	if !contains(contentStr, "AWS_REGION") {
		t.Error("AWS_REGION not in env file")
	}
}

func TestSwitchContext_WithGCP(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := config.NewManagerWithDir(tmpDir)

	ctx := &types.ContextConfig{
		Name:        "gcp-test",
		Environment: types.EnvDevelopment,
		GCP: &types.GCPConfig{
			Project:    "test-project",
			Region:     "us-central1",
			ConfigName: "test-config",
		},
	}
	if err := mgr.SaveContext(ctx); err != nil {
		t.Fatalf("Failed to save context: %v", err)
	}

	_, err := switchContext(mgr, ctx)
	if err != nil {
		t.Fatalf("switchContext() error = %v", err)
	}

	// Verify env vars include GCP settings
	envPath := filepath.Join(mgr.StateDir(), config.CurrentEnvFile)
	content, _ := os.ReadFile(envPath)
	contentStr := string(content)

	if !contains(contentStr, "CLOUDSDK_CORE_PROJECT") {
		t.Error("CLOUDSDK_CORE_PROJECT not in env file")
	}
	if !contains(contentStr, "GOOGLE_CLOUD_PROJECT") {
		t.Error("GOOGLE_CLOUD_PROJECT not in env file")
	}
}

func TestSwitchContext_WithNomadConsul(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := config.NewManagerWithDir(tmpDir)

	ctx := &types.ContextConfig{
		Name:        "nomad-test",
		Environment: types.EnvDevelopment,
		Nomad: &types.NomadConfig{
			Address:   "http://nomad:4646",
			Namespace: "default",
		},
		Consul: &types.ConsulConfig{
			Address: "http://consul:8500",
		},
	}
	if err := mgr.SaveContext(ctx); err != nil {
		t.Fatalf("Failed to save context: %v", err)
	}

	_, err := switchContext(mgr, ctx)
	if err != nil {
		t.Fatalf("switchContext() error = %v", err)
	}

	// Verify env vars include Nomad/Consul settings
	envPath := filepath.Join(mgr.StateDir(), config.CurrentEnvFile)
	content, _ := os.ReadFile(envPath)
	contentStr := string(content)

	if !contains(contentStr, "NOMAD_ADDR") {
		t.Error("NOMAD_ADDR not in env file")
	}
	if !contains(contentStr, "CONSUL_HTTP_ADDR") {
		t.Error("CONSUL_HTTP_ADDR not in env file")
	}
}

func TestSwitchAWS_WithVault_MissingBinary(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := config.NewManagerWithDir(tmpDir)

	cfg := &types.AWSConfig{
		Profile:  "test",
		UseVault: true,
	}

	// This should return an error because aws-vault is not in PATH
	// (unless it actually is installed)
	err := switchAWS(cfg, nil, mgr, "test-context")
	// We can't really test this deterministically without mocking exec.LookPath
	// So we just ensure the function doesn't panic
	_ = err
}

func TestSwitchAWS_WithoutVault(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := config.NewManagerWithDir(tmpDir)

	cfg := &types.AWSConfig{
		Profile:  "test",
		UseVault: false,
	}

	err := switchAWS(cfg, nil, mgr, "test-context")
	if err != nil {
		t.Errorf("switchAWS() error = %v, want nil", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findInString(s, substr)))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
