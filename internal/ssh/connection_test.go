// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package ssh

import (
	"os/user"
	"testing"

	"github.com/vlebo/ctx/internal/config"
)

func TestNewConnection(t *testing.T) {
	cfg := &config.SSHConfig{
		Bastion: config.BastionConfig{
			Host: "bastion.example.com",
			User: "testuser",
			Port: 22,
		},
	}

	conn := NewConnection(cfg)

	if conn == nil {
		t.Fatal("NewConnection returned nil")
	}

	if conn.IsConnected() {
		t.Error("New connection should not be connected")
	}

	if conn.Client() != nil {
		t.Error("Client should be nil before connecting")
	}
}

func TestConnection_IsConnected_NotConnected(t *testing.T) {
	cfg := &config.SSHConfig{
		Bastion: config.BastionConfig{
			Host: "bastion.example.com",
			User: "testuser",
			Port: 22,
		},
	}

	conn := NewConnection(cfg)

	if conn.IsConnected() {
		t.Error("Connection should not be connected initially")
	}
}

func TestConnection_BuildSSHConfig_UserFallback(t *testing.T) {
	// With explicit user
	cfg := &config.SSHConfig{
		Bastion: config.BastionConfig{
			Host: "bastion.example.com",
			User: "deploy",
		},
	}
	conn := NewConnection(cfg)
	sshCfg, err := conn.buildSSHConfig()
	if err != nil {
		// Skip test if no auth methods available (CI environment)
		if err.Error() == "no authentication methods available" {
			t.Skip("Skipping: no SSH auth methods available (no agent or keys)")
		}
		t.Fatalf("buildSSHConfig() error = %v", err)
	}
	if sshCfg.User != "deploy" {
		t.Errorf("User = %v, want deploy", sshCfg.User)
	}

	// Without user - should fall back to current OS user
	cfg2 := &config.SSHConfig{
		Bastion: config.BastionConfig{
			Host: "bastion.example.com",
		},
	}
	conn2 := NewConnection(cfg2)
	sshCfg2, err := conn2.buildSSHConfig()
	if err != nil {
		t.Fatalf("buildSSHConfig() error = %v", err)
	}
	currentUser, _ := user.Current()
	if sshCfg2.User != currentUser.Username {
		t.Errorf("User = %v, want current OS user %v", sshCfg2.User, currentUser.Username)
	}
}

func TestConnection_Disconnect_NotConnected(t *testing.T) {
	cfg := &config.SSHConfig{
		Bastion: config.BastionConfig{
			Host: "bastion.example.com",
			User: "testuser",
			Port: 22,
		},
	}

	conn := NewConnection(cfg)

	// Should not error when disconnecting an unconnected connection
	err := conn.Disconnect()
	if err != nil {
		t.Errorf("Disconnect() error = %v, want nil", err)
	}
}
