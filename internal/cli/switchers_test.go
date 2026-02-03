// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vlebo/ctx/internal/config"
)

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input    string
		expected string
	}{
		{"~/test", filepath.Join(home, "test")},
		{"~/.ssh/id_rsa", filepath.Join(home, ".ssh", "id_rsa")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := expandPath(tt.input)
			if result != tt.expected {
				t.Errorf("expandPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSwitchVPN_NilConfig(t *testing.T) {
	err := switchVPN(nil)
	if err != nil {
		t.Errorf("switchVPN(nil) = %v, want nil", err)
	}
}

func TestSwitchVPN_UnsupportedType(t *testing.T) {
	cfg := &config.VPNConfig{
		Type: "unsupported",
	}

	err := switchVPN(cfg)
	if err == nil {
		t.Error("Expected error for unsupported VPN type")
	}
}

func TestSwitchVPN_CustomWithoutCmd(t *testing.T) {
	cfg := &config.VPNConfig{
		Type: config.VPNTypeCustom,
	}

	err := switchVPN(cfg)
	if err == nil {
		t.Error("Expected error for custom VPN without connect_cmd")
	}
}

func TestSwitchVault_NilConfig(t *testing.T) {
	err := switchVault(nil, nil, nil, "")
	if err != nil {
		t.Errorf("switchVault(nil) = %v, want nil", err)
	}
}

func TestSwitchGit_NilConfig(t *testing.T) {
	err := switchGit(nil)
	if err != nil {
		t.Errorf("switchGit(nil) = %v, want nil", err)
	}
}

func TestSwitchDocker_NilConfig(t *testing.T) {
	err := switchDocker(nil)
	if err != nil {
		t.Errorf("switchDocker(nil) = %v, want nil", err)
	}
}

func TestSwitchNPM_NilConfig(t *testing.T) {
	err := switchNPM(nil)
	if err != nil {
		t.Errorf("switchNPM(nil) = %v, want nil", err)
	}
}

func TestDisconnectVPN_NilConfig(t *testing.T) {
	err := disconnectVPN(nil)
	if err != nil {
		t.Errorf("disconnectVPN(nil) = %v, want nil", err)
	}
}

func TestSwitchOpenVPN_NoConfigFile(t *testing.T) {
	cfg := &config.VPNConfig{
		Type: config.VPNTypeOpenVPN,
	}

	err := switchOpenVPN(cfg)
	if err == nil {
		t.Error("Expected error when config_file is not set")
	}
}

func TestSwitchWireGuard_NoInterface(t *testing.T) {
	cfg := &config.VPNConfig{
		Type: config.VPNTypeWireGuard,
	}

	err := switchWireGuard(cfg)
	if err == nil {
		t.Error("Expected error when interface is not set")
	}
}
