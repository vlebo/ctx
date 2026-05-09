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
		Long:  "Manage SSH and AWS tunnels for the current context.",
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
		tunnelType := t.Type
		if tunnelType == "" {
			tunnelType = "ssh"
		}
		local := fmt.Sprintf("localhost:%d", t.LocalPort)
		remote := fmt.Sprintf("%s:%d", t.RemoteHost, t.RemotePort)
		table.Append([]string{t.Name, tunnelType, local, remote, t.Description})
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

	// Build candidate list
	var tunnelsToStart []config.TunnelConfig

	if len(args) > 0 {
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
		tunnelsToStart = ctx.Tunnels
	}

	if len(tunnelsToStart) == 0 {
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

	// Pre-flight checks — run once before the loop, not per-tunnel
	sshOK := ctx.SSH != nil && ctx.SSH.Bastion.Host != ""
	ssmOK := false
	var awsEnv []string

	hasSSH, hasAWS := false, false
	for _, t := range tunnelsToStart {
		if t.Type == "aws" {
			hasAWS = true
		} else {
			hasSSH = true
		}
	}

	if hasSSH && !sshOK {
		yellow.Printf("⚠ SSH tunnels skipped: no SSH bastion configured\n")
	}
	if hasAWS {
		if ctx.AWS == nil {
			yellow.Printf("⚠ AWS tunnels skipped: no aws config found\n")
		} else if err := checkSSMDependencies(); err != nil {
			yellow.Printf("⚠ AWS tunnels skipped: %v\n", err)
		} else {
			var awsCreds *config.AWSCredentials
			if ctx.AWS.UseVault {
				awsCreds = mgr.LoadAWSCredentials(ctx.Name)
			}
			awsEnv = buildAWSEnv(ctx.AWS, awsCreds)
			ssmOK = true
		}
	}

	for _, t := range tunnelsToStart {
		if t.Type == "aws" {
			if !ssmOK {
				continue
			}

			if entry, exists := state.TunnelPIDs[t.Name]; exists {
				if isProcessRunning(entry.PID) {
					configChanged := entry.Config.Target != t.Target ||
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
				delete(state.TunnelPIDs, t.Name)
			}

			actualPort, portChanged := findAvailablePort(t.LocalPort)
			tunnelConfig := t
			tunnelConfig.LocalPort = actualPort

			if err := validateSSMTarget(tunnelConfig.Target, awsEnv); err != nil {
				yellow.Printf("⚠ %s: %v\n", t.Name, err)
				continue
			}

			ssmArgs := buildSSMArgs(tunnelConfig)
			cmd := exec.Command("aws", ssmArgs...)
			cmd.Env = awsEnv
			cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

			logFile := filepath.Join(stateDir, fmt.Sprintf("%s-%s.log", ctx.Name, t.Name))
			logFd, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				yellow.Printf("⚠ %s: failed to create log file: %v\n", t.Name, err)
				continue
			}
			cmd.Stdout = logFd
			cmd.Stderr = logFd

			if err := cmd.Start(); err != nil {
				logFd.Close()
				yellow.Printf("⚠ %s: failed to start: %v\n", t.Name, err)
				continue
			}

			exitCh := make(chan error, 1)
			go func(c *exec.Cmd) { exitCh <- c.Wait() }(cmd)

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
				PID:       cmd.Process.Pid,
				StartedAt: time.Now(),
				Config:    tunnelConfig,
			}
			startedCount++
			startedTunnels = append(startedTunnels, t.Name)

			green.Print("✓ ")
			if portChanged {
				fmt.Printf("%-12s localhost:%d → %s:%d (PID: %d) ", t.Name, actualPort, t.RemoteHost, t.RemotePort, cmd.Process.Pid)
				yellow.Printf("(port %d was in use)\n", t.LocalPort)
			} else {
				fmt.Printf("%-12s localhost:%d → %s:%d (PID: %d)\n", t.Name, t.LocalPort, t.RemoteHost, t.RemotePort, cmd.Process.Pid)
			}
		} else {
			if !sshOK {
				continue
			}

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
			cmd := exec.Command("ssh", sshArgs...)
			cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

			logFile := filepath.Join(stateDir, fmt.Sprintf("%s-%s.log", ctx.Name, t.Name))
			logFd, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				yellow.Printf("⚠ %s: failed to create log file: %v\n", t.Name, err)
				continue
			}
			cmd.Stdout = logFd
			cmd.Stderr = logFd

			if err := cmd.Start(); err != nil {
				logFd.Close()
				yellow.Printf("⚠ %s: failed to start: %v\n", t.Name, err)
				continue
			}

			exitCh := make(chan error, 1)
			go func(c *exec.Cmd) { exitCh <- c.Wait() }(cmd)

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
				PID:       cmd.Process.Pid,
				StartedAt: time.Now(),
				Config:    tunnelConfig,
			}
			startedCount++
			startedTunnels = append(startedTunnels, t.Name)

			green.Print("✓ ")
			if portChanged {
				fmt.Printf("%-12s localhost:%d → %s:%d (PID: %d) ", t.Name, actualPort, t.RemoteHost, t.RemotePort, cmd.Process.Pid)
				yellow.Printf("(port %d was in use)\n", t.LocalPort)
			} else {
				fmt.Printf("%-12s localhost:%d → %s:%d (PID: %d)\n", t.Name, t.LocalPort, t.RemoteHost, t.RemotePort, cmd.Process.Pid)
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
func buildSSMArgs(tunnel config.TunnelConfig) []string {
	params := fmt.Sprintf(
		`{"host":["%s"],"portNumber":["%d"],"localPortNumber":["%d"]}`,
		tunnel.RemoteHost, tunnel.RemotePort, tunnel.LocalPort,
	)
	return []string{
		"ssm", "start-session",
		"--target", tunnel.Target,
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
// One entry per tunnel regardless of transport type (ssh or aws).
type tunnelState struct {
	StartedAt   time.Time              `json:"started_at"`            // Deprecated
	TunnelPIDs  map[string]tunnelEntry `json:"tunnel_pids,omitempty"` // All tunnels (ssh and aws)
	ContextName string                 `json:"context_name"`
	Tunnels     []config.TunnelConfig  `json:"tunnels,omitempty"` // Deprecated
	PID         int                    `json:"pid,omitempty"`     // Deprecated: for backwards compat
}

// tunnelEntry represents a single running tunnel (ssh or aws).
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
	if len(state.TunnelPIDs) == 0 {
		fmt.Println("No active tunnels found for this context.")
		os.Remove(stateFile)
		return nil
	}

	// Determine which tunnels to stop
	var toStop []string

	if len(args) > 0 {
		tunnelName := args[0]
		if _, exists := state.TunnelPIDs[tunnelName]; exists {
			toStop = append(toStop, tunnelName)
		} else {
			return fmt.Errorf("tunnel '%s' is not running", tunnelName)
		}
	} else {
		for name := range state.TunnelPIDs {
			toStop = append(toStop, name)
		}
	}

	stoppedCount := 0
	var stoppedNames []string

	for _, name := range toStop {
		entry := state.TunnelPIDs[name]
		if isProcessRunning(entry.PID) {
			if entry.Config.Type == "aws" {
				killSSMProcessGroup(entry.PID)
			} else {
				process, _ := os.FindProcess(entry.PID)
				process.Signal(syscall.SIGTERM)
			}
			stoppedCount++
			green.Printf("✓ Stopped %s (PID: %d)\n", name, entry.PID)
		}
		delete(state.TunnelPIDs, name)
		stoppedNames = append(stoppedNames, name)
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
		sendTunnelEvent(mgr, ctx.Name, string(ctx.Environment), "tunnel.down", stoppedNames, true)
	}

	return nil
}

// pendingTunnel holds information about a tunnel being started (ssh or aws).
type pendingTunnel struct {
	config      config.TunnelConfig
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
	var autoConnect []config.TunnelConfig
	for _, t := range ctx.Tunnels {
		if t.AutoConnect {
			autoConnect = append(autoConnect, t)
		}
	}
	if len(autoConnect) == 0 {
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
			ContextName: ctx.Name,
			TunnelPIDs:  make(map[string]tunnelEntry),
		}
	}
	if state.TunnelPIDs == nil {
		state.TunnelPIDs = make(map[string]tunnelEntry)
	}

	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)

	timeout := 5
	if ctx.SSH != nil && ctx.SSH.TunnelTimeout > 0 {
		timeout = ctx.SSH.TunnelTimeout
	}

	// Pre-flight checks — run once before starting any tunnel
	sshOK := ctx.SSH != nil && ctx.SSH.Bastion.Host != ""
	ssmOK := false
	var awsEnv []string

	hasSSH, hasAWS := false, false
	for _, t := range autoConnect {
		if t.Type == "aws" {
			hasAWS = true
		} else {
			hasSSH = true
		}
	}
	if hasSSH && !sshOK {
		yellow.Fprintf(os.Stderr, "⚠ SSH auto-connect skipped: no SSH bastion configured\n")
	}
	if hasAWS {
		if ctx.AWS == nil {
			yellow.Fprintf(os.Stderr, "⚠ AWS auto-connect skipped: no aws config found\n")
		} else if err := checkSSMDependencies(); err != nil {
			yellow.Fprintf(os.Stderr, "⚠ AWS auto-connect skipped: %v\n", err)
		} else {
			var awsCreds *config.AWSCredentials
			if ctx.AWS.UseVault {
				awsCreds = mgr.LoadAWSCredentials(ctx.Name)
			}
			awsEnv = buildAWSEnv(ctx.AWS, awsCreds)
			ssmOK = true
		}
	}

	var startedTunnels []string
	var failedTunnels []string

	// Phase 1: Start all tunnels in parallel
	var pending []pendingTunnel
	for _, t := range autoConnect {
		if t.Type == "aws" {
			if !ssmOK {
				continue
			}

			if entry, exists := state.TunnelPIDs[t.Name]; exists {
				if isProcessRunning(entry.PID) {
					configChanged := entry.Config.Target != t.Target ||
						entry.Config.RemoteHost != t.RemoteHost ||
						entry.Config.RemotePort != t.RemotePort ||
						entry.Config.LocalPort != t.LocalPort
					if !configChanged {
						green.Fprintf(os.Stderr, "✓ Tunnel %s already running (PID: %d)\n", t.Name, entry.PID)
						continue
					}
					killSSMProcessGroup(entry.PID)
				}
				delete(state.TunnelPIDs, t.Name)
			}

			actualPort, portChanged := findAvailablePort(t.LocalPort)
			tunnelConfig := t
			tunnelConfig.LocalPort = actualPort

			ssmArgs := buildSSMArgs(tunnelConfig)
			cmd := exec.Command("aws", ssmArgs...)
			cmd.Env = awsEnv
			cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

			logFile := filepath.Join(stateDir, fmt.Sprintf("%s-%s.log", ctx.Name, t.Name))
			logFd, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				yellow.Fprintf(os.Stderr, "⚠ Tunnel %s: failed to create log file: %v\n", t.Name, err)
				continue
			}
			cmd.Stdout = logFd
			cmd.Stderr = logFd

			if err := cmd.Start(); err != nil {
				logFd.Close()
				yellow.Fprintf(os.Stderr, "⚠ Tunnel %s: failed to start: %v\n", t.Name, err)
				failedTunnels = append(failedTunnels, t.Name)
				continue
			}

			exitCh := make(chan error, 1)
			go func(c *exec.Cmd) { exitCh <- c.Wait() }(cmd)

			pending = append(pending, pendingTunnel{
				config:      tunnelConfig,
				actualPort:  actualPort,
				portChanged: portChanged,
				cmd:         cmd,
				logFile:     logFile,
				logFd:       logFd,
				exitCh:      exitCh,
			})
		} else {
			if !sshOK {
				continue
			}

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
			cmd := exec.Command("ssh", sshArgs...)
			cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

			logFile := filepath.Join(stateDir, fmt.Sprintf("%s-%s.log", ctx.Name, t.Name))
			logFd, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				yellow.Fprintf(os.Stderr, "⚠ Tunnel %s: failed to create log file: %v\n", t.Name, err)
				continue
			}
			cmd.Stdout = logFd
			cmd.Stderr = logFd

			if err := cmd.Start(); err != nil {
				logFd.Close()
				yellow.Fprintf(os.Stderr, "⚠ Tunnel %s: failed to start: %v\n", t.Name, err)
				continue
			}

			exitCh := make(chan error, 1)
			go func(c *exec.Cmd) { exitCh <- c.Wait() }(cmd)

			pending = append(pending, pendingTunnel{
				config:      tunnelConfig,
				actualPort:  actualPort,
				portChanged: portChanged,
				cmd:         cmd,
				logFile:     logFile,
				logFd:       logFd,
				exitCh:      exitCh,
			})
		}
	}

	if len(pending) > 0 {
		// Phase 2: Wait once for all tunnels (ssh and aws together)
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
	if len(state.TunnelPIDs) == 0 {
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

	var stale []string

	for name, entry := range state.TunnelPIDs {
		t := entry.Config
		tunnelType := t.Type
		if tunnelType == "" {
			tunnelType = "ssh"
		}
		localAddr := fmt.Sprintf("localhost:%d", t.LocalPort)
		remoteAddr := fmt.Sprintf("%s:%d", t.RemoteHost, t.RemotePort)
		pidStr := fmt.Sprintf("%d", entry.PID)
		var statusStr string
		if isProcessRunning(entry.PID) {
			statusStr = "● connected"
		} else {
			statusStr = "○ stopped"
			stale = append(stale, name)
		}
		table.Append([]string{name, tunnelType, localAddr, remoteAddr, pidStr, statusStr})
	}

	for _, name := range stale {
		delete(state.TunnelPIDs, name)
	}
	if len(stale) > 0 {
		if len(state.TunnelPIDs) == 0 {
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
