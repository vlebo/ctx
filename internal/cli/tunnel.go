// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/vlebo/ctx/internal/cloud"
	"github.com/vlebo/ctx/internal/config"
)

var tunnelBackground bool

func newTunnelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tunnel",
		Short: "Manage SSH tunnels",
		Long:  "Manage SSH tunnels for the current context.",
	}

	cmd.AddCommand(newTunnelListCmd())
	cmd.AddCommand(newTunnelUpCmd())
	cmd.AddCommand(newTunnelDownCmd())
	cmd.AddCommand(newTunnelStatusCmd())

	return cmd
}

func newTunnelListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all tunnels for current context",
		Long:  "List all tunnel definitions for the current context.",
		RunE:  runTunnelList,
	}
}

func newTunnelUpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up [name]",
		Short: "Start tunnels",
		Long:  "Start all tunnels for the current context, or a specific tunnel by name.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runTunnelUp,
	}

	cmd.Flags().BoolVarP(&tunnelBackground, "background", "b", false, "Run tunnels in background")

	return cmd
}

func newTunnelDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down [name]",
		Short: "Stop tunnels",
		Long:  "Stop all tunnels for the current context, or a specific tunnel by name.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runTunnelDown,
	}
}

func newTunnelStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show tunnel connection status",
		Long:  "Show the status of all active tunnels for the current context.",
		RunE:  runTunnelStatus,
	}
}

func runTunnelList(cmd *cobra.Command, args []string) error {
	// Get current context from env var
	currentContext := os.Getenv("CTX_CURRENT")
	if currentContext == "" {
		return fmt.Errorf("no active context - use 'ctx use <name>' first")
	}

	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	ctx, err := mgr.LoadContext(currentContext)
	if err != nil {
		return fmt.Errorf("failed to load context '%s': %w", currentContext, err)
	}

	if len(ctx.Tunnels) == 0 {
		fmt.Printf("No tunnels defined for context '%s'.\n", ctx.Name)
		return nil
	}

	fmt.Printf("Tunnels for context '%s':\n\n", ctx.Name)

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"NAME", "LOCAL", "REMOTE", "DESCRIPTION"})
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetTablePadding("  ")
	table.SetNoWhiteSpace(true)

	for _, t := range ctx.Tunnels {
		local := fmt.Sprintf("localhost:%d", t.LocalPort)
		remote := fmt.Sprintf("%s:%d", t.RemoteHost, t.RemotePort)
		table.Append([]string{t.Name, local, remote, t.Description})
	}

	table.Render()
	return nil
}

func runTunnelUp(cmd *cobra.Command, args []string) error {
	// Get current context from env var
	currentContext := os.Getenv("CTX_CURRENT")
	if currentContext == "" {
		return fmt.Errorf("no active context - use 'ctx use <name>' first")
	}

	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	ctx, err := mgr.LoadContext(currentContext)
	if err != nil {
		return fmt.Errorf("failed to load context '%s': %w", currentContext, err)
	}

	if ctx.SSH == nil || ctx.SSH.Bastion.Host == "" {
		return fmt.Errorf("no SSH bastion configured for this context")
	}

	if len(ctx.Tunnels) == 0 {
		return fmt.Errorf("no tunnels defined for this context")
	}

	// Determine which tunnels to start
	var tunnelsToStart []config.TunnelConfig
	if len(args) > 0 {
		// Start specific tunnel
		tunnelName := args[0]
		for _, t := range ctx.Tunnels {
			if t.Name == tunnelName {
				tunnelsToStart = append(tunnelsToStart, t)
				break
			}
		}
		if len(tunnelsToStart) == 0 {
			return fmt.Errorf("tunnel '%s' not found in context", tunnelName)
		}
	} else {
		// Start all tunnels
		tunnelsToStart = ctx.Tunnels
	}

	// Get state directory
	stateDir := filepath.Join(mgr.StateDir(), "tunnels")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Load existing state
	stateFile := filepath.Join(stateDir, ctx.Name+".json")
	state, _ := loadTunnelState(stateFile)
	if state == nil {
		state = &tunnelState{
			ContextName: ctx.Name,
			TunnelPIDs:  make(map[string]tunnelEntry),
		}
	}
	if state.TunnelPIDs == nil {
		state.TunnelPIDs = make(map[string]tunnelEntry)
	}

	// Migrate old state format if needed
	if state.PID > 0 && len(state.TunnelPIDs) == 0 {
		if isProcessRunning(state.PID) {
			// Old format with single PID - kill it first
			if proc, err := os.FindProcess(state.PID); err == nil {
				proc.Signal(syscall.SIGTERM)
				time.Sleep(200 * time.Millisecond)
			}
		}
		state.PID = 0
		state.Tunnels = nil
	}

	fmt.Printf("Starting tunnels for %s...\n", ctx.Name)

	// Start each tunnel as a separate SSH process
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	startedCount := 0
	var startedTunnels []string

	for _, t := range tunnelsToStart {
		// Check if this tunnel is already running
		if entry, exists := state.TunnelPIDs[t.Name]; exists {
			if isProcessRunning(entry.PID) {
				yellow.Printf("• %s already running (PID: %d)\n", t.Name, entry.PID)
				continue
			}
			// Clean up stale entry
			delete(state.TunnelPIDs, t.Name)
		}

		// Find available port (in case configured port is in use)
		actualPort, portChanged := findAvailablePort(t.LocalPort)
		tunnelConfig := t
		tunnelConfig.LocalPort = actualPort

		// Build SSH args for this single tunnel
		sshArgs := buildSSHArgs(ctx.SSH, []config.TunnelConfig{tunnelConfig})

		// Start SSH process in background
		sshCmd := exec.Command("ssh", sshArgs...)
		sshCmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true,
		}

		// Redirect output to log file
		logFile := filepath.Join(stateDir, fmt.Sprintf("%s-%s.log", ctx.Name, t.Name))
		logFd, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			yellow.Printf("⚠ %s: failed to create log file: %v\n", t.Name, err)
			continue
		}
		sshCmd.Stdout = logFd
		sshCmd.Stderr = logFd

		if err := sshCmd.Start(); err != nil {
			logFd.Close()
			yellow.Printf("⚠ %s: failed to start: %v\n", t.Name, err)
			continue
		}

		// Give SSH a moment to establish connection
		time.Sleep(300 * time.Millisecond)

		// Check if process is still running
		if !isProcessRunning(sshCmd.Process.Pid) {
			logFd.Close()
			logContent, _ := os.ReadFile(logFile)
			yellow.Printf("⚠ %s: failed to connect. Log: %s\n", t.Name, string(logContent))
			continue
		}

		// Save to state
		state.TunnelPIDs[t.Name] = tunnelEntry{
			PID:       sshCmd.Process.Pid,
			StartedAt: time.Now(),
			Config:    tunnelConfig,
		}
		startedCount++
		startedTunnels = append(startedTunnels, t.Name)

		green.Print("✓ ")
		if portChanged {
			fmt.Printf("%-12s localhost:%d → %s:%d (PID: %d) ", t.Name, actualPort, t.RemoteHost, t.RemotePort, sshCmd.Process.Pid)
			yellow.Printf("(port %d was in use)\n", t.LocalPort)
		} else {
			fmt.Printf("%-12s localhost:%d → %s:%d (PID: %d)\n", t.Name, t.LocalPort, t.RemoteHost, t.RemotePort, sshCmd.Process.Pid)
		}
	}

	// Save state
	if err := saveTunnelState(stateFile, state); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save state: %v\n", err)
	}

	if startedCount > 0 {
		fmt.Printf("\n%d tunnel(s) started.\n", startedCount)
		// Send tunnel.up audit event
		sendTunnelEvent(mgr, ctx.Name, string(ctx.Environment), "tunnel.up", startedTunnels, true)
	}
	fmt.Println("Use 'ctx tunnel status' to check status.")
	fmt.Println("Use 'ctx tunnel down [name]' to stop tunnels.")

	return nil
}

// buildSSHArgs builds the SSH command arguments for port forwarding.
func buildSSHArgs(sshConfig *config.SSHConfig, tunnels []config.TunnelConfig) []string {
	args := []string{
		"-N", // No remote command
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
	}

	// Add identity file if specified
	if sshConfig.Bastion.IdentityFile != "" {
		identityFile := sshConfig.Bastion.IdentityFile
		if strings.HasPrefix(identityFile, "~/") {
			home, _ := os.UserHomeDir()
			identityFile = filepath.Join(home, identityFile[2:])
		}
		args = append(args, "-i", identityFile)
	}

	// Add port if non-standard
	port := sshConfig.Bastion.Port
	if port == 0 {
		port = 22
	}
	if port != 22 {
		args = append(args, "-p", fmt.Sprintf("%d", port))
	}

	// Add port forwarding for each tunnel
	for _, t := range tunnels {
		forward := fmt.Sprintf("%d:%s:%d", t.LocalPort, t.RemoteHost, t.RemotePort)
		args = append(args, "-L", forward)
	}

	// Add destination
	dest := fmt.Sprintf("%s@%s", sshConfig.Bastion.User, sshConfig.Bastion.Host)
	args = append(args, dest)

	return args
}

// tunnelState represents the persisted state of running tunnels.
// Now stores one PID per tunnel for independent management.
type tunnelState struct {
	StartedAt   time.Time              `json:"started_at"`            // Deprecated
	TunnelPIDs  map[string]tunnelEntry `json:"tunnel_pids,omitempty"` // New: per-tunnel PIDs
	ContextName string                 `json:"context_name"`
	Tunnels     []config.TunnelConfig  `json:"tunnels,omitempty"` // Deprecated
	PID         int                    `json:"pid,omitempty"`     // Deprecated: for backwards compat
}

// tunnelEntry represents a single running tunnel.
type tunnelEntry struct {
	StartedAt time.Time           `json:"started_at"`
	Config    config.TunnelConfig `json:"config"`
	PID       int                 `json:"pid"`
}

func loadTunnelState(path string) (*tunnelState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state tunnelState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func saveTunnelState(path string, state *tunnelState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// isPortAvailable checks if a local port is available for binding.
func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// findAvailablePort finds an available port starting from the given port.
// Returns the available port and whether it differs from the original.
func findAvailablePort(startPort int) (int, bool) {
	port := startPort
	maxAttempts := 100 // Don't search forever
	for range maxAttempts {
		if isPortAvailable(port) {
			return port, port != startPort
		}
		port++
	}
	// Give up, return original and let SSH fail with clear error
	return startPort, false
}

func runTunnelDown(cmd *cobra.Command, args []string) error {
	// Get current context from env var
	currentContext := os.Getenv("CTX_CURRENT")
	if currentContext == "" {
		return fmt.Errorf("no active context - use 'ctx use <name>' first")
	}

	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	ctx, err := mgr.LoadContext(currentContext)
	if err != nil {
		return fmt.Errorf("failed to load context '%s': %w", currentContext, err)
	}

	stateDir := filepath.Join(mgr.StateDir(), "tunnels")
	stateFile := filepath.Join(stateDir, ctx.Name+".json")

	// Try to load state
	state, err := loadTunnelState(stateFile)
	if err != nil {
		fmt.Println("No active tunnels found for this context.")
		return nil
	}

	green := color.New(color.FgGreen)

	// Handle old format (single PID)
	if state.PID > 0 && len(state.TunnelPIDs) == 0 {
		if isProcessRunning(state.PID) {
			process, _ := os.FindProcess(state.PID)
			fmt.Printf("Stopping tunnels (PID: %d)...\n", state.PID)
			process.Signal(syscall.SIGTERM)
			time.Sleep(300 * time.Millisecond)
		}
		os.Remove(stateFile)
		green.Println("✓ All tunnels stopped.")
		return nil
	}

	// New format: per-tunnel PIDs
	if len(state.TunnelPIDs) == 0 {
		fmt.Println("No active tunnels found for this context.")
		os.Remove(stateFile)
		return nil
	}

	// Determine which tunnels to stop
	var tunnelsToStop []string
	if len(args) > 0 {
		// Stop specific tunnel
		tunnelName := args[0]
		if _, exists := state.TunnelPIDs[tunnelName]; exists {
			tunnelsToStop = append(tunnelsToStop, tunnelName)
		} else {
			return fmt.Errorf("tunnel '%s' is not running", tunnelName)
		}
	} else {
		// Stop all tunnels
		for name := range state.TunnelPIDs {
			tunnelsToStop = append(tunnelsToStop, name)
		}
	}

	stoppedCount := 0
	for _, name := range tunnelsToStop {
		entry := state.TunnelPIDs[name]
		if isProcessRunning(entry.PID) {
			process, _ := os.FindProcess(entry.PID)
			process.Signal(syscall.SIGTERM)
			stoppedCount++
			green.Printf("✓ Stopped %s (PID: %d)\n", name, entry.PID)
		}
		delete(state.TunnelPIDs, name)
	}

	// Save or remove state file
	if len(state.TunnelPIDs) == 0 {
		os.Remove(stateFile)
	} else {
		saveTunnelState(stateFile, state)
	}

	if stoppedCount == 0 {
		fmt.Println("No active tunnels to stop.")
	} else {
		fmt.Printf("\n%d tunnel(s) stopped.\n", stoppedCount)
		// Send tunnel.down audit event
		sendTunnelEvent(mgr, ctx.Name, string(ctx.Environment), "tunnel.down", tunnelsToStop, true)
	}

	return nil
}

// startAutoConnectTunnels starts tunnels that have auto_connect enabled.
// Called during context switch. Returns the list of successfully started tunnel names.
func startAutoConnectTunnels(mgr *config.Manager, ctx *config.ContextConfig) ([]string, error) {
	// Filter tunnels with auto_connect
	var autoConnectTunnels []config.TunnelConfig
	for _, t := range ctx.Tunnels {
		if t.AutoConnect {
			autoConnectTunnels = append(autoConnectTunnels, t)
		}
	}

	if len(autoConnectTunnels) == 0 {
		return nil, nil
	}

	// Get state directory
	stateDir := filepath.Join(mgr.StateDir(), "tunnels")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	// Load existing state
	stateFile := filepath.Join(stateDir, ctx.Name+".json")
	state, _ := loadTunnelState(stateFile)
	if state == nil {
		state = &tunnelState{
			ContextName: ctx.Name,
			TunnelPIDs:  make(map[string]tunnelEntry),
		}
	}
	if state.TunnelPIDs == nil {
		state.TunnelPIDs = make(map[string]tunnelEntry)
	}

	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)
	startedCount := 0
	var startedTunnels []string

	for _, t := range autoConnectTunnels {
		// Check if this tunnel is already running
		if entry, exists := state.TunnelPIDs[t.Name]; exists {
			if isProcessRunning(entry.PID) {
				green.Fprintf(os.Stderr, "✓ Tunnel %s already running (PID: %d)\n", t.Name, entry.PID)
				continue
			}
			delete(state.TunnelPIDs, t.Name)
		}

		// Find available port (in case configured port is in use)
		actualPort, portChanged := findAvailablePort(t.LocalPort)
		tunnelConfig := t
		tunnelConfig.LocalPort = actualPort

		// Build SSH args for this single tunnel
		sshArgs := buildSSHArgs(ctx.SSH, []config.TunnelConfig{tunnelConfig})

		// Start SSH process in background
		sshCmd := exec.Command("ssh", sshArgs...)
		sshCmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true,
		}

		// Redirect output to log file
		logFile := filepath.Join(stateDir, fmt.Sprintf("%s-%s.log", ctx.Name, t.Name))
		logFd, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			yellow.Fprintf(os.Stderr, "⚠ %s: failed to create log file: %v\n", t.Name, err)
			continue
		}
		sshCmd.Stdout = logFd
		sshCmd.Stderr = logFd

		if err := sshCmd.Start(); err != nil {
			logFd.Close()
			yellow.Fprintf(os.Stderr, "⚠ %s: failed to start: %v\n", t.Name, err)
			continue
		}

		// Give SSH a moment to establish connection
		time.Sleep(300 * time.Millisecond)

		if !isProcessRunning(sshCmd.Process.Pid) {
			logFd.Close()
			logContent, _ := os.ReadFile(logFile)
			yellow.Fprintf(os.Stderr, "⚠ %s: failed to connect. Log: %s\n", t.Name, string(logContent))
			continue
		}

		state.TunnelPIDs[t.Name] = tunnelEntry{
			PID:       sshCmd.Process.Pid,
			StartedAt: time.Now(),
			Config:    tunnelConfig,
		}
		startedCount++
		startedTunnels = append(startedTunnels, t.Name)

		if portChanged {
			green.Fprintf(os.Stderr, "✓ Tunnel %s: localhost:%d → %s:%d ", t.Name, actualPort, t.RemoteHost, t.RemotePort)
			yellow.Fprintf(os.Stderr, "(port %d in use)\n", t.LocalPort)
		} else {
			green.Fprintf(os.Stderr, "✓ Tunnel %s: localhost:%d → %s:%d\n", t.Name, t.LocalPort, t.RemoteHost, t.RemotePort)
		}
	}

	// Save state
	if err := saveTunnelState(stateFile, state); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save state: %v\n", err)
	}

	return startedTunnels, nil
}

func runTunnelStatus(cmd *cobra.Command, args []string) error {
	// Get current context from env var
	currentContext := os.Getenv("CTX_CURRENT")
	if currentContext == "" {
		return fmt.Errorf("no active context - use 'ctx use <name>' first")
	}

	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	ctx, err := mgr.LoadContext(currentContext)
	if err != nil {
		return fmt.Errorf("failed to load context '%s': %w", currentContext, err)
	}

	stateDir := filepath.Join(mgr.StateDir(), "tunnels")
	stateFile := filepath.Join(stateDir, ctx.Name+".json")

	// Try to load state
	state, err := loadTunnelState(stateFile)
	if err != nil {
		fmt.Println("No active tunnels for this context.")
		return nil
	}

	// Handle old format (single PID)
	if state.PID > 0 && len(state.TunnelPIDs) == 0 {
		if !isProcessRunning(state.PID) {
			os.Remove(stateFile)
			fmt.Println("No active tunnels for this context.")
			return nil
		}
		fmt.Printf("Tunnels for context '%s' (PID: %d)\n\n", ctx.Name, state.PID)
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"TUNNEL", "LOCAL", "REMOTE", "STATUS"})
		table.SetBorder(false)
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetCenterSeparator("")
		table.SetColumnSeparator("")
		table.SetRowSeparator("")
		table.SetHeaderLine(false)
		table.SetTablePadding("  ")
		table.SetNoWhiteSpace(true)
		for _, t := range state.Tunnels {
			localAddr := fmt.Sprintf("localhost:%d", t.LocalPort)
			remoteAddr := fmt.Sprintf("%s:%d", t.RemoteHost, t.RemotePort)
			statusStr := color.GreenString("● connected")
			table.Append([]string{t.Name, localAddr, remoteAddr, statusStr})
		}
		table.Render()
		return nil
	}

	// New format: per-tunnel PIDs
	if len(state.TunnelPIDs) == 0 {
		fmt.Println("No active tunnels for this context.")
		return nil
	}

	fmt.Printf("Tunnels for context '%s'\n\n", ctx.Name)

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"TUNNEL", "LOCAL", "REMOTE", "PID", "STATUS"})
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetTablePadding("  ")
	table.SetNoWhiteSpace(true)

	activeCount := 0
	staleNames := []string{}
	for name, entry := range state.TunnelPIDs {
		t := entry.Config
		localAddr := fmt.Sprintf("localhost:%d", t.LocalPort)
		remoteAddr := fmt.Sprintf("%s:%d", t.RemoteHost, t.RemotePort)
		pidStr := fmt.Sprintf("%d", entry.PID)
		var statusStr string
		if isProcessRunning(entry.PID) {
			statusStr = "● connected"
			activeCount++
		} else {
			statusStr = "○ stopped"
			staleNames = append(staleNames, name)
		}
		table.Append([]string{name, localAddr, remoteAddr, pidStr, statusStr})
	}

	// Clean up stale entries
	for _, name := range staleNames {
		delete(state.TunnelPIDs, name)
	}
	if len(staleNames) > 0 {
		if len(state.TunnelPIDs) == 0 {
			os.Remove(stateFile)
		} else {
			saveTunnelState(stateFile, state)
		}
	}

	table.Render()

	// Show log file location
	logFile := filepath.Join(stateDir, ctx.Name+".log")
	fmt.Printf("\nLog file: %s\n", logFile)

	return nil
}

// sendTunnelEvent sends a tunnel audit event to the cloud server.
func sendTunnelEvent(mgr *config.Manager, contextName, environment, action string, tunnelNames []string, success bool) {
	client := NewCloudClient(mgr)
	if client == nil {
		return
	}

	details := map[string]interface{}{
		"tunnels": tunnelNames,
	}

	event := &cloud.AuditEvent{
		Action:      action,
		ContextName: contextName,
		Environment: environment,
		Details:     details,
		Success:     success,
	}
	_ = client.SendAuditEvent(event)
}
