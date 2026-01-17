// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package shell

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vlebo/ctx/pkg/types"
)

func TestFormatPrompt(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		ctx      *types.ContextConfig
		expected string
	}{
		{
			name:   "simple format",
			format: "[ctx: {{.Name}}]",
			ctx: &types.ContextConfig{
				Name:        "my-context",
				Environment: types.EnvDevelopment,
			},
			expected: "[ctx: my-context]",
		},
		{
			name:   "with prod indicator",
			format: "[ctx: {{.Name}}{{if .IsProd}} ⚠️{{end}}]",
			ctx: &types.ContextConfig{
				Name:        "prod-context",
				Environment: types.EnvProduction,
			},
			expected: "[ctx: prod-context ⚠️]",
		},
		{
			name:   "prod indicator not shown for dev",
			format: "[ctx: {{.Name}}{{if .IsProd}} ⚠️{{end}}]",
			ctx: &types.ContextConfig{
				Name:        "dev-context",
				Environment: types.EnvDevelopment,
			},
			expected: "[ctx: dev-context]",
		},
		{
			name:   "with environment",
			format: "[{{.Name}} ({{.Environment}})]",
			ctx: &types.ContextConfig{
				Name:        "staging",
				Environment: types.EnvStaging,
			},
			expected: "[staging (staging)]",
		},
		{
			name:     "nil context",
			format:   "[ctx: {{.Name}}]",
			ctx:      nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FormatPrompt(tt.format, tt.ctx)
			if err != nil {
				t.Fatalf("FormatPrompt() error = %v", err)
			}
			if got != tt.expected {
				t.Errorf("FormatPrompt() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFormatPrompt_InvalidTemplate(t *testing.T) {
	ctx := &types.ContextConfig{
		Name: "test",
	}

	_, err := FormatPrompt("{{.Invalid}", ctx)
	if err == nil {
		t.Error("Expected error for invalid template")
	}
}

func TestGetCurrentContextName(t *testing.T) {
	tmpDir := t.TempDir()

	// Test non-existent file
	name, err := GetCurrentContextName(tmpDir)
	if err != nil {
		t.Fatalf("GetCurrentContextName() error = %v", err)
	}
	if name != "" {
		t.Errorf("Expected empty string for non-existent file, got %q", name)
	}

	// Create name file
	namePath := filepath.Join(tmpDir, "current.name")
	if err := os.WriteFile(namePath, []byte("test-context"), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test existing file
	name, err = GetCurrentContextName(tmpDir)
	if err != nil {
		t.Fatalf("GetCurrentContextName() error = %v", err)
	}
	if name != "test-context" {
		t.Errorf("GetCurrentContextName() = %q, want %q", name, "test-context")
	}
}
