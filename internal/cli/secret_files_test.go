// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/vlebo/ctx/internal/config"
)

func TestWriteSecretFile(t *testing.T) {
	tmpDir := t.TempDir()

	path, err := writeSecretFile(tmpDir, "test-ctx", "KUBECONFIG", "apiVersion: v1\nkind: Config\n")
	if err != nil {
		t.Fatalf("writeSecretFile() error = %v", err)
	}

	// Verify file exists
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("File not found at %s: %v", path, err)
	}

	// Verify permissions are 0600
	if runtime.GOOS != "windows" {
		perm := info.Mode().Perm()
		if perm != 0o600 {
			t.Errorf("File permissions = %o, want 0600", perm)
		}
	}

	// Verify content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != "apiVersion: v1\nkind: Config\n" {
		t.Errorf("File content = %q, want kubeconfig yaml", string(content))
	}

	// Verify filename pattern
	base := filepath.Base(path)
	if len(base) == 0 {
		t.Error("Empty filename")
	}
}

func TestSecureDeleteFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with content
	path := filepath.Join(tmpDir, "secret.txt")
	if err := os.WriteFile(path, []byte("super-secret-content"), 0o600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Securely delete it
	if err := secureDeleteFile(path); err != nil {
		t.Fatalf("secureDeleteFile() error = %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("File should not exist after secure delete")
	}
}

func TestSecureDeleteFile_NonExistent(t *testing.T) {
	// Should not error on non-existent file
	err := secureDeleteFile("/tmp/does-not-exist-ctx-test-file")
	if err != nil {
		t.Errorf("secureDeleteFile() should not error on non-existent file, got: %v", err)
	}
}

func TestGetSecretTempDir(t *testing.T) {
	dir := getSecretTempDir()

	if dir == "" {
		t.Error("getSecretTempDir() returned empty string")
	}

	// Verify the directory exists
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("getSecretTempDir() returned non-existent dir: %v", err)
	}
	if !info.IsDir() {
		t.Error("getSecretTempDir() returned a non-directory path")
	}

	// On Linux, should prefer /dev/shm
	if runtime.GOOS == "linux" {
		if _, err := os.Stat("/dev/shm"); err == nil {
			if dir != "/dev/shm" {
				t.Errorf("On Linux with /dev/shm available, expected /dev/shm, got %s", dir)
			}
		}
	}
}

func TestGetSecretFileProvider(t *testing.T) {
	tests := []struct {
		name        string
		src         config.SecretFileSource
		wantProv    string
		wantSpec    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "bitwarden provider",
			src:      config.SecretFileSource{Bitwarden: "My Item#notes"},
			wantProv: "bitwarden",
			wantSpec: "My Item#notes",
		},
		{
			name:     "onepassword provider",
			src:      config.SecretFileSource{OnePassword: "My K8s Cluster/kubeconfig"},
			wantProv: "onepassword",
			wantSpec: "My K8s Cluster/kubeconfig",
		},
		{
			name:     "vault provider",
			src:      config.SecretFileSource{Vault: "secret/data/k8s#kubeconfig"},
			wantProv: "vault",
			wantSpec: "secret/data/k8s#kubeconfig",
		},
		{
			name:     "aws_secrets_manager provider",
			src:      config.SecretFileSource{AWSSecretsManager: "my-kubeconfig"},
			wantProv: "aws_secrets_manager",
			wantSpec: "my-kubeconfig",
		},
		{
			name:     "aws_ssm provider",
			src:      config.SecretFileSource{AWSSSM: "/config/kubeconfig"},
			wantProv: "aws_ssm",
			wantSpec: "/config/kubeconfig",
		},
		{
			name:     "gcp_secret_manager provider",
			src:      config.SecretFileSource{GCPSecretManager: "kubeconfig-secret"},
			wantProv: "gcp_secret_manager",
			wantSpec: "kubeconfig-secret",
		},
		{
			name:        "no provider",
			src:         config.SecretFileSource{},
			wantErr:     true,
			errContains: "no provider specified",
		},
		{
			name:        "multiple providers",
			src:         config.SecretFileSource{Bitwarden: "item1", OnePassword: "item2"},
			wantErr:     true,
			errContains: "exactly one provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov, spec, err := getSecretFileProvider(tt.src)

			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if tt.errContains != "" && !containsStr(err.Error(), tt.errContains) {
					t.Errorf("Error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if prov != tt.wantProv {
				t.Errorf("provider = %q, want %q", prov, tt.wantProv)
			}
			if spec != tt.wantSpec {
				t.Errorf("itemSpec = %q, want %q", spec, tt.wantSpec)
			}
		})
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
