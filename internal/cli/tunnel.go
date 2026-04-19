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
		Short: "Manage tunnels",
		Long:  "Manage SSH and SSM tunnels for the current context.",
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

	ssmTunnels := 0
	if ctx.AWS != nil {
		ssmTunnels = len(ctx.AWS.Tunnels)
	}
	if len(ctx.Tunnels) == 0 && ssmTunnels == 0 {
		fmt.Printf("No tunnels defined for context '%s'.\n", ctx.Name)
		return nil
	}

	fmt.Printf("Tunnels for context '%s':\n\n", ctx.Name)

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"NAME", "TYPE", "LOCAL", "REMOTE", "DESCRIPTION"})
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
		table.Append([]string{t.Name, "ssh", local, remote, t.Description})
	}
	if ctx.AWS != nil {
		for _, t := range ctx.AWS.Tunnels {
			local := fmt.Sprintf("localhost:%d", t.LocalPort)
			remote := fmt.Sprintf("%s:%d", t.RemoteHost, t.RemotePort)
			table.Append([]string{t.Name, "ssm", local, remote, t.Description})
		}
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

	// Build candidate lists from both SSH and SSM tunnels
	var sshTunnelsToStart []config.TunnelConfig
	var ssmTunnelsToStart []config.SSMTunnelConfig

	if len(args) > 0 {
		tunnelName := args[0]
		for _, t := range ctx.Tunnels {
			if t.Name == tunnelName {
				sshTunnelsToStart = append(sshTunnelsToStart, t)
				break
			}
		}
		if ctx.AWS != nil {
			for _, t := range ctx.AWS.Tunnels {
				if t.Name == tunnelName {
					ssmTunnelsToStart = append(ssmTunnelsToStart, t)
					break
				}
			}
		}
		if len(sshTunnelsToStart) == 0 && len(ssmTunnelsToStart) == 0 {
			return fmt.Errorf("tunnel '%s' not found in context", tunnelName)
		}
	} else {
		sshTunnelsToStart = ctx.Tunnels
		if ctx.AWS != nil {
			ssmTunnelsToStart = ctx.AWS.Tunnels
		}
	}

	if len(sshTunnelsToStart) == 0 && len(ssmTunnelsToStart) == 0 {
		return fmt.Errorf("no tunnels defined for this context")
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
			ContextName:   ctx.Name,
			TunnelPIDs:    make(map[string]tunnelEntry),
			SSMTunnelPIDs: make(map[string]ssmTunnelEntry),
		}
	}
	if state.TunnelPIDs == nil {
		state.TunnelPIDs = make(map[string]tunnelEntry)
	}
	if state.SSMTunnelPIDs == nil {
		state.SSMTunnelPIDs = make(map[string]ssmTunnelEntry)
	}

	// Migrate old state format if needed
	if state.PID > 0 && len(state.TunnelPIDs) == 0 {
		if isProcessRunning(state.PID) {
			if proc, err := os.FindProcess(state.PID); err == nil {
				proc.Signal(syscall.SIGTERM)
				time.Sleep(200 * time.Millisecond)
			}
		}
		state.PID = 0
		state.Tunnels = nil
	}

	fmt.Printf("Starting tunnels for %s...\n", ctx.Name)

	tunnelTimeout := 5
	if ctx.SSH != nil && ctx.SSH.TunnelTimeout > 0 {
		tunnelTimeout = ctx.SSH.TunnelTimeout
	}

	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	startedCount := 0
	var startedTunnels []string

	// --- SSH tunnels ---
	if len(sshTunnelsToStart) > 0 {
		if ctx.SSH == nil || ctx.SSH.Bastion.Host == "" {
			yellow.Printf("⚠ SSH tunnels skipped: no SSH bastion configured\n")
		} else {
			for _, t := range sshTunnelsToStart {
				if entry, exists := state.TunnelPIDs[t.Name]; exists {
					if isProcessRunning(entry.PID) {
						yellow.Printf("• %s already running (PID: %d)\n", t.Name, entry.PID)
						continue
					}
					delete(state.TunnelPIDs, t.Name)
				}

				actualPort, portChanged := findAvailablePort(t.LocalPort)
				tunnelConfig := t
				tunnelConfig.LocalPort = actualPort

				sshArgs := buildSSHArgs(ctx.SSH, []config.TunnelConfig{tunnelConfig})
				sshCmd := exec.Command("ssh", sshArgs...)
				sshCmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

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

				exitCh := make(chan error, 1)
				go func() {
					exitCh <- sshCmd.Wait()
				}()

				select {
				case <-exitCh:
					logFd.Close()
					logContent, _ := os.ReadFile(logFile)
					errMsg := strings.TrimSpace(string(logContent))
					if idx := strings.Index(errMsg, "\n"); idx > 0 {
						errMsg = errMsg[:idx]
					}
					yellow.Printf("⚠ Tunnel %s: %s\n", t.Name, errMsg)
					continue
				case <-time.After(time.Duration(tunnelTimeout) * time.Second):
				}

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
		}
	}

	// --- SSM tunnels ---
	if len(ssmTunnelsToStart) > 0 {
		if err := checkSSMDependencies(); err != nil {
			yellow.Printf("⚠ SSM tunnels skipped: %v\n", err)
		} else {
			var awsCreds *config.AWSCredentials
			if ctx.AWS.UseVault {
				awsCreds = mgr.LoadAWSCredentials(ctx.Name)
			}
			awsEnv := buildAWSEnv(ctx.AWS, awsCreds)

			for _, t := range ssmTunnelsToStart {
				if entry, exists := state.SSMTunnelPIDs[t.Name]; exists {
					if isProcessRunning(entry.PID) {
						configChanged := entry.Config.SSMTarget != t.SSMTarget ||
							entry.Config.RemoteHost != t.RemoteHost ||
							entry.Config.RemotePort != t.RemotePort ||
							entry.Config.LocalPort != t.LocalPort
						if !configChanged {
							yellow.Printf("• %s already running (PID: %d)\n", t.Name, entry.PID)
							continue
						}
						// Config changed — stop the stale tunnel before restarting
						killSSMProcessGroup(entry.PID)
					}
					delete(state.SSMTunnelPIDs, t.Name)
				}

				actualPort, portChanged := findAvailablePort(t.LocalPort)
				tunnelConfig := t
				tunnelConfig.LocalPort = actualPort

				if err := validateSSMTarget(tunnelConfig.SSMTarget, awsEnv); err != nil {
					yellow.Printf("⚠ %s: %v\n", t.Name, err)
					continue
				}

				ssmArgs := buildSSMArgs(tunnelConfig)
				ssmCmd := exec.Command("aws", ssmArgs...)
				ssmCmd.Env = awsEnv
				ssmCmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

				logFile := filepath.Join(stateDir, fmt.Sprintf("%s-%s.log", ctx.Name, t.Name))
				logFd, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
				if err != nil {
					yellow.Printf("⚠ %s: failed to create log file: %v\n", t.Name, err)
					continue
				}
				ssmCmd.Stdout = logFd
				ssmCmd.Stderr = logFd

				if err := ssmCmd.Start(); err != nil {
					logFd.Close()
					yellow.Printf("⚠ %s: failed to start: %v\n", t.Name, err)
					continue
				}

				exitCh := make(chan error, 1)
				go func() {
					exitCh <- ssmCmd.Wait()
				}()

				select {
				case <-exitCh:
					logFd.Close()
					logContent, _ := os.ReadFile(logFile)
					errMsg := strings.TrimSpace(string(logContent))
					if idx := strings.Index(errMsg, "\n"); idx > 0 {
						errMsg = errMsg[:idx]
					}
					yellow.Printf("⚠ Tunnel %s: %s\n", t.Name, errMsg)
					continue
				case <-time.After(time.Duration(tunnelTimeout) * time.Second):
				}

				state.SSMTunnelPIDs[t.Name] = ssmTunnelEntry{
					PID:       ssmCmd.Process.Pid,
					StartedAt: time.Now(),
					Config:    tunnelConfig,
				}
				startedCount++
				startedTunnels = append(startedTunnels, t.Name)

				green.Print("✓ ")
				if portChanged {
					fmt.Printf("%-12s localhost:%d → %s:%d (PID: %d) ", t.Name, actualPort, t.RemoteHost, t.RemotePort, ssmCmd.Process.Pid)
					yellow.Printf("(port %d was in use)\n", t.LocalPort)
				} else {
					fmt.Printf("%-12s localhost:%d → %s:%d (PID: %d)\n", t.Name, t.LocalPort, t.RemoteHost, t.RemotePort, ssmCmd.Process.Pid)
				}
			}
		}
	}

	// Save state
	if err := saveTunnelState(stateFile, state); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save state: %v\n", err)
	}

	if startedCount > 0 {
		fmt.Printf("\n%d tunnel(s) started.\n", startedCount)
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

	// Add destination (omit user@ to let SSH default to current OS user)
	var dest string
	if sshConfig.Bastion.User != "" {
		dest = fmt.Sprintf("%s@%s", sshConfig.Bastion.User, sshConfig.Bastion.Host)
	} else {
		dest = sshConfig.Bastion.Host
	}
	args = append(args, dest)

	return args
}

// checkSSMDependencies verifies that aws CLI and session-manager-plugin are available.
func checkSSMDependencies() error {
	if _, err := exec.LookPath("aws"); err != nil {
		return fmt.Errorf("AWS CLI not found: install aws-cli v2 from https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html")
	}
	if _, err := exec.LookPath("session-manager-plugin"); err != nil {
		return fmt.Errorf("session-manager-plugin not found: install from https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html")
	}
	return nil
}

// buildAWSEnv builds a cmd.Env slice with AWS credentials injected explicitly.
// This is required for auto_connect tunnels started before the shell hook sources the env file.
func buildAWSEnv(awsCfg *config.AWSConfig, awsCreds *config.AWSCredentials) []string {
	env := os.Environ()
	if awsCfg.Config != "" {
		env = append(env, "AWS_CONFIG_FILE="+expandPath(awsCfg.Config))
	}
	if awsCreds != nil {
		env = append(env, "AWS_ACCESS_KEY_ID="+awsCreds.AccessKeyID)
		env = append(env, "AWS_SECRET_ACCESS_KEY="+awsCreds.SecretAccessKey)
		if awsCreds.SessionToken != "" {
			env = append(env, "AWS_SESSION_TOKEN="+awsCreds.SessionToken)
		}
	} else if awsCfg.Profile != "" {
		env = append(env, "AWS_PROFILE="+awsCfg.Profile)
	}
	if awsCfg.Region != "" {
		env = append(env, "AWS_REGION="+awsCfg.Region, "AWS_DEFAULT_REGION="+awsCfg.Region)
	}
	return env
}

// buildSSMArgs builds the aws ssm start-session arguments for port forwarding to a remote host.
func buildSSMArgs(tunnel config.SSMTunnelConfig) []string {
	params := fmt.Sprintf(
		`{"host":["%s"],"portNumber":["%d"],"localPortNumber":["%d"]}`,
		tunnel.RemoteHost, tunnel.RemotePort, tunnel.LocalPort,
	)
	return []string{
		"ssm", "start-session",
		"--target", tunnel.SSMTarget,
		"--document-name", "AWS-StartPortForwardingSessionToRemoteHost",
		"--parameters", params,
	}
}

// validateSSMTarget checks that the given EC2 instance ID is registered in SSM.
func validateSSMTarget(instanceID string, awsEnv []string) error {
	cmd := exec.Command("aws", "ssm", "describe-instance-information",
		"--filters", fmt.Sprintf("Key=InstanceIds,Values=%s", instanceID),
		"--output", "json",
	)
	cmd.Env = awsEnv
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check SSM registration for %s: %w", instanceID, err)
	}
	var result struct {
		InstanceInformationList []struct{} `json:"InstanceInformationList"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return fmt.Errorf("failed to parse SSM response: %w", err)
	}
	if len(result.InstanceInformationList) == 0 {
		return fmt.Errorf("instance %q is not registered in SSM — verify the SSM Agent is running and the instance profile has AmazonSSMManagedInstanceCore", instanceID)
	}
	return nil
}

// tunnelState represents the persisted state of running tunnels.
// Now stores one PID per tunnel for independent management.
type tunnelState struct {
	StartedAt     time.Time                `json:"started_at"`              // Deprecated
	TunnelPIDs    map[string]tunnelEntry   `json:"tunnel_pids,omitempty"`   // SSH tunnels
	SSMTunnelPIDs map[string]ssmTunnelEntry `json:"ssm_tunnel_pids,omitempty"` // SSM tunnels
	ContextName   string                   `json:"context_name"`
	Tunnels       []config.TunnelConfig    `json:"tunnels,omitempty"` // Deprecated
	PID           int                      `json:"pid,omitempty"`     // Deprecated: for backwards compat
}

// tunnelEntry represents a single running SSH tunnel.
type tunnelEntry struct {
	StartedAt time.Time           `json:"started_at"`
	Config    config.TunnelConfig `json:"config"`
	PID       int                 `json:"pid"`
}

// ssmTunnelEntry represents a single running SSM tunnel.
type ssmTunnelEntry struct {
	StartedAt time.Time              `json:"started_at"`
	Config    config.SSMTunnelConfig `json:"config"`
	PID       int                    `json:"pid"`
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

// killSSMProcessGroup sends SIGTERM to the entire process group of an SSM tunnel.
// aws ssm start-session spawns session-manager-plugin as a child — killing only
// the aws PID leaves the plugin alive. Since we start with Setsid=true the
// process group ID equals the aws PID, so Kill(-pid) reaches both.
func killSSMProcessGroup(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGTERM)
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
	if len(state.TunnelPIDs) == 0 && len(state.SSMTunnelPIDs) == 0 {
		fmt.Println("No active tunnels found for this context.")
		os.Remove(stateFile)
		return nil
	}

	// Determine which tunnels to stop
	var sshToStop []string
	var ssmToStop []string

	if len(args) > 0 {
		tunnelName := args[0]
		if _, exists := state.TunnelPIDs[tunnelName]; exists {
			sshToStop = append(sshToStop, tunnelName)
		} else if _, exists := state.SSMTunnelPIDs[tunnelName]; exists {
			ssmToStop = append(ssmToStop, tunnelName)
		} else {
			return fmt.Errorf("tunnel '%s' is not running", tunnelName)
		}
	} else {
		for name := range state.TunnelPIDs {
			sshToStop = append(sshToStop, name)
		}
		for name := range state.SSMTunnelPIDs {
			ssmToStop = append(ssmToStop, name)
		}
	}

	stoppedCount := 0
	var stoppedNames []string

	for _, name := range sshToStop {
		entry := state.TunnelPIDs[name]
		if isProcessRunning(entry.PID) {
			process, _ := os.FindProcess(entry.PID)
			process.Signal(syscall.SIGTERM)
			stoppedCount++
			green.Printf("✓ Stopped %s (PID: %d)\n", name, entry.PID)
		}
		delete(state.TunnelPIDs, name)
		stoppedNames = append(stoppedNames, name)
	}
	for _, name := range ssmToStop {
		entry := state.SSMTunnelPIDs[name]
		if isProcessRunning(entry.PID) {
			killSSMProcessGroup(entry.PID)
			stoppedCount++
			green.Printf("✓ Stopped %s (PID: %d)\n", name, entry.PID)
		}
		delete(state.SSMTunnelPIDs, name)
		stoppedNames = append(stoppedNames, name)
	}

	// Save or remove state file
	if len(state.TunnelPIDs) == 0 && len(state.SSMTunnelPIDs) == 0 {
		os.Remove(stateFile)
	} else {
		saveTunnelState(stateFile, state)
	}

	if stoppedCount == 0 {
		fmt.Println("No active tunnels to stop.")
	} else {
		fmt.Printf("\n%d tunnel(s) stopped.\n", stoppedCount)
		sendTunnelEvent(mgr, ctx.Name, string(ctx.Environment), "tunnel.down", stoppedNames, true)
	}

	return nil
}

// pendingTunnel holds information about an SSH tunnel being started.
type pendingTunnel struct {
	config      config.TunnelConfig
	actualPort  int
	portChanged bool
	cmd         *exec.Cmd
	logFile     string
	logFd       *os.File
	exitCh      chan error
}

// ssmPendingTunnel holds information about an SSM tunnel being started.
type ssmPendingTunnel struct {
	config      config.SSMTunnelConfig
	actualPort  int
	portChanged bool
	cmd         *exec.Cmd
	logFile     string
	logFd       *os.File
	exitCh      chan error
}

// startAutoConnectTunnels starts tunnels that have auto_connect enabled.
// Called during context switch. Returns (successfulTunnels, failedTunnels, error).
func startAutoConnectTunnels(mgr *config.Manager, ctx *config.ContextConfig) ([]string, []string, error) {
	// Filter SSH tunnels with auto_connect
	var autoConnectSSH []config.TunnelConfig
	for _, t := range ctx.Tunnels {
		if t.AutoConnect {
			autoConnectSSH = append(autoConnectSSH, t)
		}
	}

	// Filter SSM tunnels with auto_connect
	var autoConnectSSM []config.SSMTunnelConfig
	if ctx.AWS != nil {
		for _, t := range ctx.AWS.Tunnels {
			if t.AutoConnect {
				autoConnectSSM = append(autoConnectSSM, t)
			}
		}
	}

	if len(autoConnectSSH) == 0 && len(autoConnectSSM) == 0 {
		return nil, nil, nil
	}

	// Get state directory
	stateDir := filepath.Join(mgr.StateDir(), "tunnels")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	// Load existing state
	stateFile := filepath.Join(stateDir, ctx.Name+".json")
	state, _ := loadTunnelState(stateFile)
	if state == nil {
		state = &tunnelState{
			ContextName:   ctx.Name,
			TunnelPIDs:    make(map[string]tunnelEntry),
			SSMTunnelPIDs: make(map[string]ssmTunnelEntry),
		}
	}
	if state.TunnelPIDs == nil {
		state.TunnelPIDs = make(map[string]tunnelEntry)
	}
	if state.SSMTunnelPIDs == nil {
		state.SSMTunnelPIDs = make(map[string]ssmTunnelEntry)
	}

	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)

	timeout := 5
	if ctx.SSH != nil && ctx.SSH.TunnelTimeout > 0 {
		timeout = ctx.SSH.TunnelTimeout
	}

	var startedTunnels []string
	var failedTunnels []string

	// --- SSH auto-connect tunnels ---
	if len(autoConnectSSH) > 0 && ctx.SSH != nil && ctx.SSH.Bastion.Host != "" {
		// Phase 1: Start all SSH tunnels in parallel
		var pending []pendingTunnel
		for _, t := range autoConnectSSH {
			if entry, exists := state.TunnelPIDs[t.Name]; exists {
				if isProcessRunning(entry.PID) {
					green.Fprintf(os.Stderr, "✓ Tunnel %s already running (PID: %d)\n", t.Name, entry.PID)
					continue
				}
				delete(state.TunnelPIDs, t.Name)
			}

			actualPort, portChanged := findAvailablePort(t.LocalPort)
			tunnelConfig := t
			tunnelConfig.LocalPort = actualPort

			sshArgs := buildSSHArgs(ctx.SSH, []config.TunnelConfig{tunnelConfig})
			sshCmd := exec.Command("ssh", sshArgs...)
			sshCmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

			logFile := filepath.Join(stateDir, fmt.Sprintf("%s-%s.log", ctx.Name, t.Name))
			logFd, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				yellow.Fprintf(os.Stderr, "⚠ Tunnel %s: failed to create log file: %v\n", t.Name, err)
				continue
			}
			sshCmd.Stdout = logFd
			sshCmd.Stderr = logFd

			if err := sshCmd.Start(); err != nil {
				logFd.Close()
				yellow.Fprintf(os.Stderr, "⚠ Tunnel %s: failed to start: %v\n", t.Name, err)
				continue
			}

			exitCh := make(chan error, 1)
			go func(cmd *exec.Cmd) {
				exitCh <- cmd.Wait()
			}(sshCmd)

			pending = append(pending, pendingTunnel{
				config:      tunnelConfig,
				actualPort:  actualPort,
				portChanged: portChanged,
				cmd:         sshCmd,
				logFile:     logFile,
				logFd:       logFd,
				exitCh:      exitCh,
			})
		}

		if len(pending) > 0 {
			// Phase 2: Wait for tunnels to either connect or fail
			time.Sleep(time.Duration(timeout) * time.Second)

			// Phase 3: Check results
			for _, p := range pending {
				t := p.config
				select {
				case <-p.exitCh:
					p.logFd.Close()
					logContent, _ := os.ReadFile(p.logFile)
					errMsg := strings.TrimSpace(string(logContent))
					if idx := strings.Index(errMsg, "\n"); idx > 0 {
						errMsg = errMsg[:idx]
					}
					yellow.Fprintf(os.Stderr, "⚠ Tunnel %s: %s\n", t.Name, errMsg)
					failedTunnels = append(failedTunnels, t.Name)
				default:
					state.TunnelPIDs[t.Name] = tunnelEntry{
						PID:       p.cmd.Process.Pid,
						StartedAt: time.Now(),
						Config:    t,
					}
					startedTunnels = append(startedTunnels, t.Name)

					if p.portChanged {
						green.Fprintf(os.Stderr, "✓ Tunnel %s: localhost:%d → %s:%d ", t.Name, p.actualPort, t.RemoteHost, t.RemotePort)
						yellow.Fprintf(os.Stderr, "(port %d in use)\n", t.LocalPort)
					} else {
						green.Fprintf(os.Stderr, "✓ Tunnel %s: localhost:%d → %s:%d\n", t.Name, t.LocalPort, t.RemoteHost, t.RemotePort)
					}
				}
			}
		}
	}

	// --- SSM auto-connect tunnels ---
	if len(autoConnectSSM) > 0 {
		if err := checkSSMDependencies(); err != nil {
			yellow.Fprintf(os.Stderr, "⚠ SSM auto-connect skipped: %v\n", err)
		} else {
			var awsCreds *config.AWSCredentials
			if ctx.AWS.UseVault {
				awsCreds = mgr.LoadAWSCredentials(ctx.Name)
			}
			awsEnv := buildAWSEnv(ctx.AWS, awsCreds)

			// Phase 1: Start all SSM tunnels in parallel
			var ssmPending []ssmPendingTunnel
			for _, t := range autoConnectSSM {
				if entry, exists := state.SSMTunnelPIDs[t.Name]; exists {
					if isProcessRunning(entry.PID) {
						configChanged := entry.Config.SSMTarget != t.SSMTarget ||
							entry.Config.RemoteHost != t.RemoteHost ||
							entry.Config.RemotePort != t.RemotePort ||
							entry.Config.LocalPort != t.LocalPort
						if !configChanged {
							green.Fprintf(os.Stderr, "✓ Tunnel %s already running (PID: %d)\n", t.Name, entry.PID)
							continue
						}
						killSSMProcessGroup(entry.PID)
					}
					delete(state.SSMTunnelPIDs, t.Name)
				}

				actualPort, portChanged := findAvailablePort(t.LocalPort)
				tunnelConfig := t
				tunnelConfig.LocalPort = actualPort

				ssmArgs := buildSSMArgs(tunnelConfig)
				ssmCmd := exec.Command("aws", ssmArgs...)
				ssmCmd.Env = awsEnv
				ssmCmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

				logFile := filepath.Join(stateDir, fmt.Sprintf("%s-%s.log", ctx.Name, t.Name))
				logFd, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
				if err != nil {
					yellow.Fprintf(os.Stderr, "⚠ Tunnel %s: failed to create log file: %v\n", t.Name, err)
					continue
				}
				ssmCmd.Stdout = logFd
				ssmCmd.Stderr = logFd

				if err := ssmCmd.Start(); err != nil {
					logFd.Close()
					yellow.Fprintf(os.Stderr, "⚠ Tunnel %s: failed to start: %v\n", t.Name, err)
					failedTunnels = append(failedTunnels, t.Name)
					continue
				}

				exitCh := make(chan error, 1)
				go func(cmd *exec.Cmd) {
					exitCh <- cmd.Wait()
				}(ssmCmd)

				ssmPending = append(ssmPending, ssmPendingTunnel{
					config:      tunnelConfig,
					actualPort:  actualPort,
					portChanged: portChanged,
					cmd:         ssmCmd,
					logFile:     logFile,
					logFd:       logFd,
					exitCh:      exitCh,
				})
			}

			if len(ssmPending) > 0 {
				// Phase 2: Wait once for all SSM tunnels
				time.Sleep(time.Duration(timeout) * time.Second)

				// Phase 3: Check results
				for _, p := range ssmPending {
					t := p.config
					select {
					case <-p.exitCh:
						p.logFd.Close()
						logContent, _ := os.ReadFile(p.logFile)
						errMsg := strings.TrimSpace(string(logContent))
						if idx := strings.Index(errMsg, "\n"); idx > 0 {
							errMsg = errMsg[:idx]
						}
						yellow.Fprintf(os.Stderr, "⚠ Tunnel %s: %s\n", t.Name, errMsg)
						failedTunnels = append(failedTunnels, t.Name)
					default:
						state.SSMTunnelPIDs[t.Name] = ssmTunnelEntry{
							PID:       p.cmd.Process.Pid,
							StartedAt: time.Now(),
							Config:    t,
						}
						startedTunnels = append(startedTunnels, t.Name)

						if p.portChanged {
							green.Fprintf(os.Stderr, "✓ Tunnel %s: localhost:%d → %s:%d ", t.Name, p.actualPort, t.RemoteHost, t.RemotePort)
							yellow.Fprintf(os.Stderr, "(port %d in use)\n", t.LocalPort)
						} else {
							green.Fprintf(os.Stderr, "✓ Tunnel %s: localhost:%d → %s:%d\n", t.Name, t.LocalPort, t.RemoteHost, t.RemotePort)
						}
					}
				}
			}
		}
	}

	// Save state
	if err := saveTunnelState(stateFile, state); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save state: %v\n", err)
	}

	return startedTunnels, failedTunnels, nil
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
	if len(state.TunnelPIDs) == 0 && len(state.SSMTunnelPIDs) == 0 {
		fmt.Println("No active tunnels for this context.")
		return nil
	}

	fmt.Printf("Tunnels for context '%s'\n\n", ctx.Name)

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"TUNNEL", "TYPE", "LOCAL", "REMOTE", "PID", "STATUS"})
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetTablePadding("  ")
	table.SetNoWhiteSpace(true)

	var staleSSH, staleSSM []string

	for name, entry := range state.TunnelPIDs {
		t := entry.Config
		localAddr := fmt.Sprintf("localhost:%d", t.LocalPort)
		remoteAddr := fmt.Sprintf("%s:%d", t.RemoteHost, t.RemotePort)
		pidStr := fmt.Sprintf("%d", entry.PID)
		var statusStr string
		if isProcessRunning(entry.PID) {
			statusStr = "● connected"
		} else {
			statusStr = "○ stopped"
			staleSSH = append(staleSSH, name)
		}
		table.Append([]string{name, "ssh", localAddr, remoteAddr, pidStr, statusStr})
	}
	for name, entry := range state.SSMTunnelPIDs {
		t := entry.Config
		localAddr := fmt.Sprintf("localhost:%d", t.LocalPort)
		remoteAddr := fmt.Sprintf("%s:%d", t.RemoteHost, t.RemotePort)
		pidStr := fmt.Sprintf("%d", entry.PID)
		var statusStr string
		if isProcessRunning(entry.PID) {
			statusStr = "● connected"
		} else {
			statusStr = "○ stopped"
			staleSSM = append(staleSSM, name)
		}
		table.Append([]string{name, "ssm", localAddr, remoteAddr, pidStr, statusStr})
	}

	for _, name := range staleSSH {
		delete(state.TunnelPIDs, name)
	}
	for _, name := range staleSSM {
		delete(state.SSMTunnelPIDs, name)
	}
	if len(staleSSH) > 0 || len(staleSSM) > 0 {
		if len(state.TunnelPIDs) == 0 && len(state.SSMTunnelPIDs) == 0 {
			os.Remove(stateFile)
		} else {
			saveTunnelState(stateFile, state)
		}
	}

	table.Render()

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

	details := map[string]any{
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
