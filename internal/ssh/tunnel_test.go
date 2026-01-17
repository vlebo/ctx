// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package ssh

import (
	"testing"

	"github.com/vlebo/ctx/pkg/types"
)

func TestTunnelStatus_String(t *testing.T) {
	tests := []struct {
		status TunnelStatus
		want   string
	}{
		{StatusStopped, "stopped"},
		{StatusStarting, "starting"},
		{StatusConnected, "connected"},
		{StatusReconnecting, "reconnecting"},
		{StatusError, "error"},
		{TunnelStatus(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("TunnelStatus.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewTunnel(t *testing.T) {
	cfg := types.TunnelConfig{
		Name:        "test-tunnel",
		Description: "Test Tunnel",
		RemoteHost:  "remote.example.com",
		RemotePort:  5432,
		LocalPort:   5432,
	}

	// We can't fully test without an SSH connection, but we can test initialization
	sshCfg := &types.SSHConfig{
		Bastion: types.BastionConfig{
			Host: "bastion.example.com",
			User: "testuser",
			Port: 22,
		},
	}
	conn := NewConnection(sshCfg)
	tunnel := NewTunnel(cfg, conn)

	if tunnel.Config().Name != "test-tunnel" {
		t.Errorf("Tunnel name = %v, want %v", tunnel.Config().Name, "test-tunnel")
	}

	if tunnel.Status() != StatusStopped {
		t.Errorf("Initial status = %v, want %v", tunnel.Status(), StatusStopped)
	}
}

func TestTunnel_Info(t *testing.T) {
	cfg := types.TunnelConfig{
		Name:        "test-tunnel",
		Description: "Test Tunnel",
		RemoteHost:  "remote.example.com",
		RemotePort:  5432,
		LocalPort:   15432,
	}

	sshCfg := &types.SSHConfig{
		Bastion: types.BastionConfig{
			Host: "bastion.example.com",
			User: "testuser",
			Port: 22,
		},
	}
	conn := NewConnection(sshCfg)
	tunnel := NewTunnel(cfg, conn)

	info := tunnel.Info()

	if info.Name != "test-tunnel" {
		t.Errorf("Info.Name = %v, want %v", info.Name, "test-tunnel")
	}
	if info.LocalAddr != "localhost:15432" {
		t.Errorf("Info.LocalAddr = %v, want %v", info.LocalAddr, "localhost:15432")
	}
	if info.RemoteAddr != "remote.example.com:5432" {
		t.Errorf("Info.RemoteAddr = %v, want %v", info.RemoteAddr, "remote.example.com:5432")
	}
	if info.Status != StatusStopped {
		t.Errorf("Info.Status = %v, want %v", info.Status, StatusStopped)
	}
}
