// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vlebo/ctx/internal/config"
)

// TunnelStatus represents the current status of a tunnel.
type TunnelStatus int

const (
	StatusStopped TunnelStatus = iota
	StatusStarting
	StatusConnected
	StatusReconnecting
	StatusError
)

func (s TunnelStatus) String() string {
	switch s {
	case StatusStopped:
		return "stopped"
	case StatusStarting:
		return "starting"
	case StatusConnected:
		return "connected"
	case StatusReconnecting:
		return "reconnecting"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// Tunnel represents an SSH tunnel with local port forwarding.
type Tunnel struct {
	startedAt time.Time

	listener  net.Listener
	lastError error

	ctx         context.Context
	conn        *Connection
	cancel      context.CancelFunc
	config      config.TunnelConfig
	wg          sync.WaitGroup
	status      TunnelStatus
	activeConns int64
	mu          sync.RWMutex
}

// NewTunnel creates a new tunnel instance.
func NewTunnel(cfg config.TunnelConfig, conn *Connection) *Tunnel {
	return &Tunnel{
		config: cfg,
		conn:   conn,
		status: StatusStopped,
	}
}

// Start starts the tunnel, listening on the local port and forwarding to remote.
func (t *Tunnel) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.status == StatusConnected || t.status == StatusStarting {
		return nil
	}

	t.status = StatusStarting

	// Ensure SSH connection is established
	if !t.conn.IsConnected() {
		if err := t.conn.Connect(); err != nil {
			t.status = StatusError
			t.lastError = err
			return fmt.Errorf("failed to establish SSH connection: %w", err)
		}
	}

	// Create local listener
	addr := fmt.Sprintf("127.0.0.1:%d", t.config.LocalPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.status = StatusError
		t.lastError = err
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	t.listener = listener
	t.ctx, t.cancel = context.WithCancel(context.Background())
	t.status = StatusConnected
	t.startedAt = time.Now()
	t.lastError = nil

	// Start accepting connections
	t.wg.Add(1)
	go t.acceptLoop()

	return nil
}

// Stop stops the tunnel and closes all connections.
func (t *Tunnel) Stop() error {
	t.mu.Lock()
	if t.cancel != nil {
		t.cancel()
	}
	if t.listener != nil {
		t.listener.Close()
	}
	t.status = StatusStopped
	t.mu.Unlock()

	// Wait for all goroutines to finish
	t.wg.Wait()
	return nil
}

// Status returns the current tunnel status.
func (t *Tunnel) Status() TunnelStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status
}

// LastError returns the last error that occurred.
func (t *Tunnel) LastError() error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastError
}

// ActiveConnections returns the number of active forwarded connections.
func (t *Tunnel) ActiveConnections() int64 {
	return atomic.LoadInt64(&t.activeConns)
}

// Config returns the tunnel configuration.
func (t *Tunnel) Config() config.TunnelConfig {
	return t.config
}

// StartedAt returns when the tunnel was started.
func (t *Tunnel) StartedAt() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.startedAt
}

// acceptLoop accepts incoming connections and forwards them.
func (t *Tunnel) acceptLoop() {
	defer t.wg.Done()

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
		}

		// Set accept deadline to allow checking context
		if tcpListener, ok := t.listener.(*net.TCPListener); ok {
			tcpListener.SetDeadline(time.Now().Add(1 * time.Second))
		}

		conn, err := t.listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			// Check if context was cancelled
			select {
			case <-t.ctx.Done():
				return
			default:
				// Log error but continue accepting
				continue
			}
		}

		t.wg.Add(1)
		go t.handleConnection(conn)
	}
}

// handleConnection handles a single forwarded connection.
func (t *Tunnel) handleConnection(local net.Conn) {
	defer t.wg.Done()
	defer local.Close()

	atomic.AddInt64(&t.activeConns, 1)
	defer atomic.AddInt64(&t.activeConns, -1)

	// Connect to remote host through SSH
	remoteAddr := fmt.Sprintf("%s:%d", t.config.RemoteHost, t.config.RemotePort)
	remote, err := t.conn.DialRemote("tcp", remoteAddr)
	if err != nil {
		t.mu.Lock()
		t.lastError = err
		t.mu.Unlock()
		return
	}
	defer remote.Close()

	// Bidirectional copy
	errChan := make(chan error, 2)

	go func() {
		_, err := io.Copy(remote, local)
		errChan <- err
	}()

	go func() {
		_, err := io.Copy(local, remote)
		errChan <- err
	}()

	// Wait for either direction to finish
	select {
	case <-t.ctx.Done():
	case <-errChan:
	}
}

// TunnelInfo provides information about a tunnel for display.
type TunnelInfo struct {
	StartedAt         time.Time
	LastError         error
	Name              string
	Description       string
	LocalAddr         string
	RemoteAddr        string
	Status            TunnelStatus
	ActiveConnections int64
}

// Info returns information about the tunnel.
func (t *Tunnel) Info() TunnelInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return TunnelInfo{
		Name:              t.config.Name,
		Description:       t.config.Description,
		LocalAddr:         fmt.Sprintf("localhost:%d", t.config.LocalPort),
		RemoteAddr:        fmt.Sprintf("%s:%d", t.config.RemoteHost, t.config.RemotePort),
		Status:            t.status,
		ActiveConnections: atomic.LoadInt64(&t.activeConns),
		StartedAt:         t.startedAt,
		LastError:         t.lastError,
	}
}
