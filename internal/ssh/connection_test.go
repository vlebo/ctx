// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package ssh

import (
	"testing"

	"github.com/vlebo/ctx/pkg/types"
)

func TestNewConnection(t *testing.T) {
	cfg := &types.SSHConfig{
		Bastion: types.BastionConfig{
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
	cfg := &types.SSHConfig{
		Bastion: types.BastionConfig{
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

func TestConnection_Disconnect_NotConnected(t *testing.T) {
	cfg := &types.SSHConfig{
		Bastion: types.BastionConfig{
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
