// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cloud

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// HeartbeatManager manages the heartbeat background process.
type HeartbeatManager struct {
	stateDir string
}

// NewHeartbeatManager creates a new heartbeat manager.
func NewHeartbeatManager(stateDir string) *HeartbeatManager {
	return &HeartbeatManager{stateDir: stateDir}
}

// heartbeatState stores the state of the running heartbeat process.
type heartbeatState struct {
	PID         int    `json:"pid"`
	ContextName string `json:"context_name"`
	StartedAt   string `json:"started_at"`
}

func (m *HeartbeatManager) stateFile() string {
	return filepath.Join(m.stateDir, "heartbeat.json")
}

// StartHeartbeat starts a background heartbeat process.
// This is called from the main CLI process and forks a background goroutine.
func (m *HeartbeatManager) StartHeartbeat(client *Client, contextName, environment string, vpnConnected bool, tunnels []string, interval time.Duration) error {
	if !client.IsConfigured() {
		return nil
	}

	// Send initial heartbeat synchronously
	input := &HeartbeatInput{
		ContextName:  contextName,
		Environment:  environment,
		VPNConnected: vpnConnected,
		Tunnels:      tunnels,
	}

	if err := client.SendHeartbeat(input); err != nil {
		return fmt.Errorf("initial heartbeat failed: %w", err)
	}

	// Save state with current PID (the main process)
	// The shell will source the env file which keeps the context active
	// We rely on shell hooks and deactivate to properly end sessions
	state := &heartbeatState{
		PID:         os.Getpid(),
		ContextName: contextName,
		StartedAt:   time.Now().Format(time.RFC3339),
	}

	if err := m.saveState(state); err != nil {
		return fmt.Errorf("failed to save heartbeat state: %w", err)
	}

	return nil
}

// StopHeartbeat stops the heartbeat process if running.
func (m *HeartbeatManager) StopHeartbeat() error {
	state, err := m.loadState()
	if err != nil {
		return nil // No state file = nothing to stop
	}

	// Clean up state file
	os.Remove(m.stateFile())

	// Signal the process if it's still running and it's not us
	if state.PID > 0 && state.PID != os.Getpid() {
		if isProcessRunning(state.PID) {
			process, err := os.FindProcess(state.PID)
			if err == nil {
				process.Signal(syscall.SIGTERM)
			}
		}
	}

	return nil
}

// GetCurrentContext returns the context name of the running heartbeat, if any.
func (m *HeartbeatManager) GetCurrentContext() string {
	state, err := m.loadState()
	if err != nil {
		return ""
	}
	return state.ContextName
}

// IsRunning returns true if a heartbeat process is running.
func (m *HeartbeatManager) IsRunning() bool {
	state, err := m.loadState()
	if err != nil {
		return false
	}
	return state.PID > 0 && isProcessRunning(state.PID)
}

func (m *HeartbeatManager) saveState(state *heartbeatState) error {
	if err := os.MkdirAll(m.stateDir, 0o755); err != nil {
		return err
	}

	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return os.WriteFile(m.stateFile(), data, 0o644)
}

func (m *HeartbeatManager) loadState() (*heartbeatState, error) {
	data, err := os.ReadFile(m.stateFile())
	if err != nil {
		return nil, err
	}

	var state heartbeatState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// isProcessRunning checks if a process with the given PID is running.
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds, so we need to send signal 0 to check
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
