// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package shell

import (
	"os"
	"strings"
	"testing"
)

func TestDetectShell(t *testing.T) {
	// Save original SHELL
	originalShell := os.Getenv("SHELL")
	defer os.Setenv("SHELL", originalShell)

	tests := []struct {
		shell    string
		expected ShellType
	}{
		{"/bin/bash", ShellBash},
		{"/usr/bin/bash", ShellBash},
		{"/bin/zsh", ShellZsh},
		{"/usr/local/bin/zsh", ShellZsh},
		{"/usr/bin/fish", ShellFish},
		{"/usr/local/bin/fish", ShellFish},
		{"", ShellBash}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			os.Setenv("SHELL", tt.shell)
			got := DetectShell()
			if got != tt.expected {
				t.Errorf("DetectShell() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGenerateHook_Bash(t *testing.T) {
	cfg := HookConfig{
		ConfigDir:    "/home/test/.config/ctx",
		StateDir:     "/home/test/.config/ctx/state",
		PromptFormat: "[ctx: {{.Name}}]",
	}

	hook, err := GenerateHook(ShellBash, cfg)
	if err != nil {
		t.Fatalf("GenerateHook() error = %v", err)
	}

	// Check for expected content
	expectedStrings := []string{
		"# ctx shell integration for bash",
		"ctx()",
		"command ctx",
		cfg.StateDir + "/current.env",
		"--export",
		"CTX_CURRENT",
		"__ctx_prompt",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(hook, expected) {
			t.Errorf("Bash hook missing expected string: %q", expected)
		}
	}
}

func TestGenerateHook_Zsh(t *testing.T) {
	cfg := HookConfig{
		ConfigDir:    "/home/test/.config/ctx",
		StateDir:     "/home/test/.config/ctx/state",
		PromptFormat: "[ctx: {{.Name}}]",
	}

	hook, err := GenerateHook(ShellZsh, cfg)
	if err != nil {
		t.Fatalf("GenerateHook() error = %v", err)
	}

	// Check for expected content
	expectedStrings := []string{
		"# ctx shell integration for zsh",
		"ctx()",
		"command ctx",
		cfg.StateDir + "/current.env",
		"--export",
		"CTX_CURRENT",
		"PROMPT_SUBST",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(hook, expected) {
			t.Errorf("Zsh hook missing expected string: %q", expected)
		}
	}
}

func TestGenerateHook_Fish(t *testing.T) {
	cfg := HookConfig{
		ConfigDir:    "/home/test/.config/ctx",
		StateDir:     "/home/test/.config/ctx/state",
		PromptFormat: "[ctx: {{.Name}}]",
	}

	hook, err := GenerateHook(ShellFish, cfg)
	if err != nil {
		t.Fatalf("GenerateHook() error = %v", err)
	}

	// Check for expected content
	expectedStrings := []string{
		"# ctx shell integration for fish",
		"function ctx",
		"command ctx",
		cfg.StateDir + "/current.env",
		"--export",
		"CTX_CURRENT",
		"fish_prompt",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(hook, expected) {
			t.Errorf("Fish hook missing expected string: %q", expected)
		}
	}
}

func TestGenerateHook_UnsupportedShell(t *testing.T) {
	cfg := HookConfig{
		ConfigDir: "/home/test/.config/ctx",
		StateDir:  "/home/test/.config/ctx/state",
	}

	_, err := GenerateHook(ShellType("powershell"), cfg)
	if err == nil {
		t.Error("Expected error for unsupported shell type")
	}
}
