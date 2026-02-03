// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package ssh

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/vlebo/ctx/internal/config"
)

// Manager manages multiple SSH tunnels for a context.
type Manager struct {
	ctx       context.Context
	sshConfig *config.SSHConfig

	conn    *Connection
	tunnels map[string]*Tunnel

	cancel      context.CancelFunc
	contextName string
	stateDir    string

	tunnelDefs        []config.TunnelConfig
	wg                sync.WaitGroup
	reconnectInterval time.Duration
	maxReconnectDelay time.Duration
	mu                sync.RWMutex

	// Reconnect settings
	reconnectEnabled bool
}

// ManagerConfig holds configuration for the tunnel manager.
type ManagerConfig struct {
	SSHConfig         *config.SSHConfig
	ContextName       string
	StateDir          string
	TunnelDefs        []config.TunnelConfig
	ReconnectInterval time.Duration
	MaxReconnectDelay time.Duration
	ReconnectEnabled  bool
}

// NewManager creates a new tunnel manager.
func NewManager(cfg ManagerConfig) *Manager {
	if cfg.ReconnectInterval == 0 {
		cfg.ReconnectInterval = 5 * time.Second
	}
	if cfg.MaxReconnectDelay == 0 {
		cfg.MaxReconnectDelay = 5 * time.Minute
	}

	return &Manager{
		contextName:       cfg.ContextName,
		sshConfig:         cfg.SSHConfig,
		tunnelDefs:        cfg.TunnelDefs,
		stateDir:          cfg.StateDir,
		tunnels:           make(map[string]*Tunnel),
		reconnectEnabled:  cfg.ReconnectEnabled,
		reconnectInterval: cfg.ReconnectInterval,
		maxReconnectDelay: cfg.MaxReconnectDelay,
	}
}

// Start starts all tunnels.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create SSH connection
	m.conn = NewConnection(m.sshConfig)
	if err := m.conn.Connect(); err != nil {
		return fmt.Errorf("failed to establish SSH connection: %w", err)
	}

	// Create context for managing goroutines
	m.ctx, m.cancel = context.WithCancel(context.Background())

	// Start all tunnels
	var errors []error
	for _, def := range m.tunnelDefs {
		tunnel := NewTunnel(def, m.conn)
		if err := tunnel.Start(); err != nil {
			errors = append(errors, fmt.Errorf("tunnel %s: %w", def.Name, err))
			continue
		}
		m.tunnels[def.Name] = tunnel
	}

	// Write state file
	if err := m.writeState(); err != nil {
		// Non-fatal, just log
		fmt.Fprintf(os.Stderr, "Warning: failed to write state file: %v\n", err)
	}

	// Start health check goroutine if reconnect is enabled
	if m.reconnectEnabled {
		m.wg.Add(1)
		go m.healthCheckLoop()
	}

	if len(errors) > 0 {
		return fmt.Errorf("some tunnels failed to start: %v", errors)
	}

	return nil
}

// StartTunnel starts a specific tunnel by name.
func (m *Manager) StartTunnel(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find the tunnel definition
	var tunnelDef *config.TunnelConfig
	for _, def := range m.tunnelDefs {
		if def.Name == name {
			tunnelDef = &def
			break
		}
	}
	if tunnelDef == nil {
		return fmt.Errorf("tunnel '%s' not defined", name)
	}

	// Check if already running
	if tunnel, exists := m.tunnels[name]; exists && tunnel.Status() == StatusConnected {
		return nil
	}

	// Ensure SSH connection
	if m.conn == nil {
		m.conn = NewConnection(m.sshConfig)
	}
	if !m.conn.IsConnected() {
		if err := m.conn.Connect(); err != nil {
			return fmt.Errorf("failed to establish SSH connection: %w", err)
		}
	}

	// Create and start tunnel
	tunnel := NewTunnel(*tunnelDef, m.conn)
	if err := tunnel.Start(); err != nil {
		return err
	}

	m.tunnels[name] = tunnel
	m.writeState()

	return nil
}

// Stop stops all tunnels and closes the SSH connection.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Cancel context
	if m.cancel != nil {
		m.cancel()
	}

	// Stop all tunnels
	for _, tunnel := range m.tunnels {
		tunnel.Stop()
	}
	m.tunnels = make(map[string]*Tunnel)

	// Close SSH connection
	if m.conn != nil {
		m.conn.Disconnect()
	}

	// Remove state file
	m.removeState()

	// Wait for goroutines
	m.wg.Wait()

	return nil
}

// StopTunnel stops a specific tunnel by name.
func (m *Manager) StopTunnel(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel, exists := m.tunnels[name]
	if !exists {
		return fmt.Errorf("tunnel '%s' not running", name)
	}

	tunnel.Stop()
	delete(m.tunnels, name)
	m.writeState()

	return nil
}

// Status returns the status of all tunnels.
func (m *Manager) Status() []TunnelInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var infos []TunnelInfo
	for _, tunnel := range m.tunnels {
		infos = append(infos, tunnel.Info())
	}
	return infos
}

// GetTunnel returns a specific tunnel by name.
func (m *Manager) GetTunnel(name string) *Tunnel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tunnels[name]
}

// IsRunning returns true if the manager has active tunnels.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.tunnels) > 0
}

// healthCheckLoop periodically checks tunnel health and reconnects if needed.
func (m *Manager) healthCheckLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.reconnectInterval)
	defer ticker.Stop()

	backoff := m.reconnectInterval

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if !m.conn.IsConnected() {
				// Try to reconnect
				if err := m.reconnect(); err != nil {
					// Exponential backoff
					backoff = min(backoff*2, m.maxReconnectDelay)
					ticker.Reset(backoff)
					continue
				}
				// Reset backoff on successful reconnect
				backoff = m.reconnectInterval
				ticker.Reset(backoff)
			}
		}
	}
}

// reconnect attempts to reconnect the SSH connection and restart tunnels.
func (m *Manager) reconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Mark all tunnels as reconnecting
	for _, tunnel := range m.tunnels {
		tunnel.mu.Lock()
		tunnel.status = StatusReconnecting
		tunnel.mu.Unlock()
	}

	// Disconnect old connection
	if m.conn != nil {
		m.conn.Disconnect()
	}

	// Create new connection
	m.conn = NewConnection(m.sshConfig)
	if err := m.conn.Connect(); err != nil {
		return err
	}

	// Restart all tunnels with new connection
	for name, tunnel := range m.tunnels {
		tunnel.Stop()
		newTunnel := NewTunnel(tunnel.config, m.conn)
		if err := newTunnel.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to restart tunnel %s: %v\n", name, err)
			continue
		}
		m.tunnels[name] = newTunnel
	}

	m.writeState()
	return nil
}

// State represents the persisted state of the tunnel manager.
type State struct {
	StartedAt   time.Time    `json:"started_at"`
	ContextName string       `json:"context_name"`
	Tunnels     []TunnelInfo `json:"tunnels"`
	PID         int          `json:"pid"`
}

// writeState writes the current state to a file.
func (m *Manager) writeState() error {
	if m.stateDir == "" {
		return nil
	}

	statePath := filepath.Join(m.stateDir, m.contextName+".json")

	state := State{
		ContextName: m.contextName,
		PID:         os.Getpid(),
		StartedAt:   time.Now(),
		Tunnels:     m.Status(),
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(m.stateDir, 0o755); err != nil {
		return err
	}

	return os.WriteFile(statePath, data, 0o644)
}

// removeState removes the state file.
func (m *Manager) removeState() error {
	if m.stateDir == "" {
		return nil
	}

	statePath := filepath.Join(m.stateDir, m.contextName+".json")
	return os.Remove(statePath)
}

// LoadState loads the state from a file.
func LoadState(stateDir, contextName string) (*State, error) {
	statePath := filepath.Join(stateDir, contextName+".json")

	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// IsProcessRunning checks if a process with the given PID is running.
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. We need to send signal 0 to check.
	// Signal 0 doesn't actually send a signal but checks if the process exists.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
