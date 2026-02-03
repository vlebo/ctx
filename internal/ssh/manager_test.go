// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package ssh

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vlebo/ctx/internal/config"
)

func TestNewManager(t *testing.T) {
	cfg := ManagerConfig{
		ContextName: "test-context",
		SSHConfig: &config.SSHConfig{
			Bastion: config.BastionConfig{
				Host: "bastion.example.com",
				User: "testuser",
				Port: 22,
			},
		},
		TunnelDefs: []config.TunnelConfig{
			{
				Name:       "postgres",
				RemoteHost: "db.internal",
				RemotePort: 5432,
				LocalPort:  5432,
			},
		},
		StateDir: "/tmp/ctx-test",
	}

	mgr := NewManager(cfg)

	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}

	if mgr.IsRunning() {
		t.Error("Manager should not be running initially")
	}
}

func TestManager_DefaultReconnectSettings(t *testing.T) {
	cfg := ManagerConfig{
		ContextName: "test-context",
		SSHConfig: &config.SSHConfig{
			Bastion: config.BastionConfig{
				Host: "bastion.example.com",
				User: "testuser",
				Port: 22,
			},
		},
	}

	mgr := NewManager(cfg)

	if mgr.reconnectInterval != 5*time.Second {
		t.Errorf("Default reconnectInterval = %v, want %v", mgr.reconnectInterval, 5*time.Second)
	}
	if mgr.maxReconnectDelay != 5*time.Minute {
		t.Errorf("Default maxReconnectDelay = %v, want %v", mgr.maxReconnectDelay, 5*time.Minute)
	}
}

func TestManager_Status_NoTunnels(t *testing.T) {
	cfg := ManagerConfig{
		ContextName: "test-context",
		SSHConfig: &config.SSHConfig{
			Bastion: config.BastionConfig{
				Host: "bastion.example.com",
				User: "testuser",
				Port: 22,
			},
		},
	}

	mgr := NewManager(cfg)

	status := mgr.Status()
	if len(status) != 0 {
		t.Errorf("Status() returned %d tunnels, want 0", len(status))
	}
}

func TestLoadState_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := LoadState(tmpDir, "nonexistent")
	if err == nil {
		t.Error("LoadState() should return error for nonexistent state")
	}
}

func TestLoadState_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Write invalid JSON
	statePath := filepath.Join(tmpDir, "test-context.json")
	err := os.WriteFile(statePath, []byte("invalid json"), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err = LoadState(tmpDir, "test-context")
	if err == nil {
		t.Error("LoadState() should return error for invalid JSON")
	}
}

func TestLoadState_Valid(t *testing.T) {
	tmpDir := t.TempDir()

	// Write valid state
	statePath := filepath.Join(tmpDir, "test-context.json")
	stateJSON := `{
		"context_name": "test-context",
		"pid": 12345,
		"started_at": "2024-01-01T00:00:00Z",
		"tunnels": []
	}`
	err := os.WriteFile(statePath, []byte(stateJSON), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	state, err := LoadState(tmpDir, "test-context")
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if state.ContextName != "test-context" {
		t.Errorf("ContextName = %v, want %v", state.ContextName, "test-context")
	}
	if state.PID != 12345 {
		t.Errorf("PID = %v, want %v", state.PID, 12345)
	}
}

func TestIsProcessRunning_InvalidPID(t *testing.T) {
	// PID 0 is usually not a valid user process
	if IsProcessRunning(0) {
		t.Error("IsProcessRunning(0) should return false")
	}

	// Very high PID that's unlikely to exist
	if IsProcessRunning(999999999) {
		t.Error("IsProcessRunning(999999999) should return false")
	}
}

func TestIsProcessRunning_CurrentProcess(t *testing.T) {
	// Current process should be running
	if !IsProcessRunning(os.Getpid()) {
		t.Error("IsProcessRunning(os.Getpid()) should return true")
	}
}
