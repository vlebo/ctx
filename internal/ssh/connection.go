// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

// Package ssh provides SSH connection and tunnel management.
package ssh

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vlebo/ctx/pkg/types"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Connection represents an SSH connection to a bastion host.
type Connection struct {
	config      *types.SSHConfig
	client      *ssh.Client
	connected   bool
	lastError   error
	connectedAt time.Time
}

// NewConnection creates a new SSH connection manager.
func NewConnection(cfg *types.SSHConfig) *Connection {
	return &Connection{
		config: cfg,
	}
}

// Connect establishes an SSH connection to the bastion host.
func (c *Connection) Connect() error {
	if c.connected && c.client != nil {
		return nil
	}

	sshConfig, err := c.buildSSHConfig()
	if err != nil {
		c.lastError = err
		return fmt.Errorf("failed to build SSH config: %w", err)
	}

	port := c.config.Bastion.Port
	if port == 0 {
		port = 22
	}

	addr := fmt.Sprintf("%s:%d", c.config.Bastion.Host, port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		c.lastError = err
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	c.client = client
	c.connected = true
	c.connectedAt = time.Now()
	c.lastError = nil

	return nil
}

// Disconnect closes the SSH connection.
func (c *Connection) Disconnect() error {
	if c.client != nil {
		err := c.client.Close()
		c.client = nil
		c.connected = false
		return err
	}
	return nil
}

// IsConnected returns true if the connection is active.
func (c *Connection) IsConnected() bool {
	if !c.connected || c.client == nil {
		return false
	}

	// Test the connection with a keepalive
	_, _, err := c.client.SendRequest("keepalive@openssh.com", true, nil)
	if err != nil {
		c.connected = false
		return false
	}

	return true
}

// Client returns the underlying SSH client.
func (c *Connection) Client() *ssh.Client {
	return c.client
}

// LastError returns the last connection error.
func (c *Connection) LastError() error {
	return c.lastError
}

// ConnectedAt returns when the connection was established.
func (c *Connection) ConnectedAt() time.Time {
	return c.connectedAt
}

// buildSSHConfig creates an ssh.ClientConfig from the connection config.
func (c *Connection) buildSSHConfig() (*ssh.ClientConfig, error) {
	var authMethods []ssh.AuthMethod

	// Try SSH agent first
	if agentAuth, err := c.getAgentAuth(); err == nil {
		authMethods = append(authMethods, agentAuth)
	}

	// Add identity file auth if specified
	if c.config.Bastion.IdentityFile != "" {
		keyAuth, err := c.getKeyAuth(c.config.Bastion.IdentityFile)
		if err == nil {
			authMethods = append(authMethods, keyAuth)
		}
	}

	// Add default key locations
	defaultKeys := []string{
		"~/.ssh/id_ed25519",
		"~/.ssh/id_rsa",
		"~/.ssh/id_ecdsa",
	}
	for _, keyPath := range defaultKeys {
		if keyAuth, err := c.getKeyAuth(keyPath); err == nil {
			authMethods = append(authMethods, keyAuth)
		}
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication methods available")
	}

	// Get host key callback
	hostKeyCallback, err := c.getHostKeyCallback()
	if err != nil {
		// Fall back to insecure if known_hosts doesn't exist
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	config := &ssh.ClientConfig{
		User:            c.config.Bastion.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         30 * time.Second,
	}

	return config, nil
}

// getAgentAuth returns an auth method using the SSH agent.
func (c *Connection) getAgentAuth() (ssh.AuthMethod, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK not set")
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH agent: %w", err)
	}

	agentClient := agent.NewClient(conn)
	return ssh.PublicKeysCallback(agentClient.Signers), nil
}

// getKeyAuth returns an auth method using a private key file.
func (c *Connection) getKeyAuth(keyPath string) (ssh.AuthMethod, error) {
	// Expand ~ to home directory
	if strings.HasPrefix(keyPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		keyPath = filepath.Join(home, keyPath[2:])
	}

	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	return ssh.PublicKeys(signer), nil
}

// getHostKeyCallback returns a host key callback for known_hosts verification.
func (c *Connection) getHostKeyCallback() (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
	return knownhosts.New(knownHostsPath)
}

// DialRemote connects to a remote host through the SSH connection.
func (c *Connection) DialRemote(network, addr string) (net.Conn, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to bastion")
	}

	return c.client.Dial(network, addr)
}
