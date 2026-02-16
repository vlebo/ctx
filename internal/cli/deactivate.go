// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/vlebo/ctx/internal/cloud"
	"github.com/vlebo/ctx/internal/config"
)

var deactivateExportFlag bool

func newDeactivateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deactivate",
		Short: "Deactivate current context",
		Long: `Deactivate the current context by disconnecting VPN, stopping tunnels,
and clearing environment variables.

This only affects the current context (based on CTX_CURRENT env var).
Other contexts' VPNs and tunnels are not affected.`,
		RunE: runDeactivate,
	}

	cmd.Flags().BoolVar(&deactivateExportFlag, "export", false, "Output unset commands for shell eval (used by shell hook)")

	return cmd
}

func runDeactivate(cmd *cobra.Command, args []string) error {
	// Get current context from env var
	currentContext := os.Getenv("CTX_CURRENT")
	if currentContext == "" {
		if deactivateExportFlag {
			return nil // Nothing to unset
		}
		fmt.Println("No active context to deactivate.")
		return nil
	}

	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	ctx, err := mgr.LoadContext(currentContext)
	if err != nil {
		return fmt.Errorf("failed to load context '%s': %w", currentContext, err)
	}

	// If --export, only output unset commands (no side effects)
	if deactivateExportFlag {
		// Track all vars to unset (avoid duplicates)
		varsToUnset := make(map[string]bool)

		// Add vars from current context config
		envVars := mgr.GenerateEnvVars(ctx)
		for key := range envVars {
			varsToUnset[key] = true
		}

		// Also read the env file to get any dynamically resolved secrets
		envFile := filepath.Join(mgr.StateDir(), "current.env")
		if file, err := os.Open(envFile); err == nil {
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				// Parse "export VAR=..." or "VAR=..."
				line = strings.TrimPrefix(line, "export ")
				if idx := strings.Index(line, "="); idx > 0 {
					varName := line[:idx]
					varsToUnset[varName] = true
				}
			}
			file.Close()
		}

		// Also unset all possible ctx-managed vars that might have been set by other contexts
		// This ensures switching from a context with proxy to one without clears proxy vars
		allPossibleVars := []string{
			// AWS
			"AWS_PROFILE", "AWS_REGION", "AWS_DEFAULT_REGION",
			"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN",
			// GCP
			"CLOUDSDK_CONFIG", "CLOUDSDK_ACTIVE_CONFIG_NAME", "CLOUDSDK_CORE_PROJECT", "GOOGLE_CLOUD_PROJECT",
			// Azure
			"AZURE_CONFIG_DIR", "AZURE_SUBSCRIPTION_ID",
			// Kubernetes
			"KUBECONFIG",
			// Nomad
			"NOMAD_ADDR", "NOMAD_NAMESPACE", "NOMAD_SKIP_VERIFY",
			// Consul
			"CONSUL_HTTP_ADDR", "CONSUL_HTTP_SSL_VERIFY",
			// Vault
			"VAULT_ADDR", "VAULT_NAMESPACE", "VAULT_SKIP_VERIFY", "VAULT_TOKEN",
			// Git
			"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL",
			// Proxy
			"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "NO_PROXY", "no_proxy",
			// Databases
			"PGHOST", "PGPORT", "PGDATABASE", "PGUSER", "PGSSLMODE",
			"MYSQL_HOST", "MYSQL_TCP_PORT", "MYSQL_DATABASE", "MYSQL_USER",
			"REDIS_HOST", "REDIS_PORT",
			"MONGODB_HOST", "MONGODB_PORT",
			// Context metadata
			"CTX_CURRENT", "CTX_ENVIRONMENT",
		}
		for _, key := range allPossibleVars {
			varsToUnset[key] = true
		}

		// Output unset commands
		for key := range varsToUnset {
			fmt.Printf("unset %s\n", key)
		}
		return nil
	}

	// Clear state files so new shells don't load this context
	if err := mgr.ClearCurrentContext(); err != nil {
		// Log but don't fail - the env var clearing is more important
		fmt.Fprintf(os.Stderr, "Warning: failed to clear state files: %v\n", err)
	}

	// Send cloud deactivation event and stop heartbeat
	sendCloudDeactivateEvents(mgr, currentContext)

	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)

	fmt.Fprintf(os.Stderr, "Deactivating context '%s'...\n\n", currentContext)

	// Get effective deactivate config (context overrides global)
	deactivateCfg := getDeactivateConfig(mgr, ctx)

	// Disconnect VPN if configured, enabled, and actually connected
	if ctx.VPN != nil {
		if deactivateCfg.DisconnectVPN {
			if checkVPNStatus(ctx.VPN) {
				yellow.Fprint(os.Stderr, "• ")
				fmt.Fprintf(os.Stderr, "Disconnecting VPN (%s)... ", ctx.VPN.Type)
				if err := disconnectVPN(ctx.VPN); err != nil {
					red.Fprintf(os.Stderr, "failed: %v\n", err)
				} else {
					green.Fprintln(os.Stderr, "done")
				}
			}
		} else {
			if checkVPNStatus(ctx.VPN) {
				yellow.Fprint(os.Stderr, "• ")
				fmt.Fprintln(os.Stderr, "VPN: keeping connected (disconnect_vpn: false)")
			}
		}
	}

	// Stop tunnels if any are configured and enabled
	if len(ctx.Tunnels) > 0 {
		if deactivateCfg.StopTunnels {
			yellow.Fprint(os.Stderr, "• ")
			fmt.Fprint(os.Stderr, "Stopping tunnels... ")
			stopped, err := stopContextTunnels(mgr.StateDir(), currentContext)
			if err != nil {
				red.Fprintf(os.Stderr, "failed: %v\n", err)
			} else if stopped > 0 {
				green.Fprintf(os.Stderr, "stopped %d tunnel(s)\n", stopped)
			} else {
				fmt.Fprintln(os.Stderr, "none running")
			}
		} else {
			yellow.Fprint(os.Stderr, "• ")
			fmt.Fprintln(os.Stderr, "Tunnels: keeping running (stop_tunnels: false)")
		}
	}

	// Clean up secret files
	if err := mgr.CleanupSecretFiles(currentContext); err != nil {
		yellow.Fprintf(os.Stderr, "⚠ Failed to clean up secret files: %v\n", err)
	}

	fmt.Fprintln(os.Stderr)
	green.Fprintf(os.Stderr, "✓ Context '%s' deactivated.\n", currentContext)
	fmt.Fprintln(os.Stderr)
	yellow.Fprintln(os.Stderr, "Note: If env vars persist, run: source <(ctx deactivate --export)")

	return nil
}

// getDeactivateConfig returns the effective deactivate config.
// Context config overrides global config, with defaults if neither is set.
func getDeactivateConfig(mgr *config.Manager, ctx *config.ContextConfig) *config.DeactivateConfig {
	// Start with defaults
	cfg := config.NewDeactivateConfigDefaults()

	// Apply global config if set
	appConfig := mgr.GetAppConfig()
	if appConfig != nil && appConfig.Deactivate != nil {
		cfg.DisconnectVPN = appConfig.Deactivate.DisconnectVPN
		cfg.StopTunnels = appConfig.Deactivate.StopTunnels
	}

	// Context config overrides global
	if ctx.Deactivate != nil {
		cfg.DisconnectVPN = ctx.Deactivate.DisconnectVPN
		cfg.StopTunnels = ctx.Deactivate.StopTunnels
	}

	return cfg
}

// stopContextTunnels stops all tunnels for a given context.
// Returns the number of tunnels stopped.
func stopContextTunnels(stateDir, contextName string) (int, error) {
	tunnelStateDir := filepath.Join(stateDir, "tunnels")
	stateFile := filepath.Join(tunnelStateDir, contextName+".json")

	// Try to load state
	state, err := loadTunnelState(stateFile)
	if err != nil {
		return 0, nil // No state file = no tunnels running
	}

	stoppedCount := 0

	// Handle old format (single PID)
	if state.PID > 0 && len(state.TunnelPIDs) == 0 {
		if isProcessRunning(state.PID) {
			process, _ := os.FindProcess(state.PID)
			process.Signal(syscall.SIGTERM)
			time.Sleep(300 * time.Millisecond)
			stoppedCount = 1
		}
		os.Remove(stateFile)
		return stoppedCount, nil
	}

	// New format: per-tunnel PIDs
	for name, entry := range state.TunnelPIDs {
		if isProcessRunning(entry.PID) {
			process, _ := os.FindProcess(entry.PID)
			process.Signal(syscall.SIGTERM)
			stoppedCount++
		}
		delete(state.TunnelPIDs, name)
	}

	// Remove state file
	os.Remove(stateFile)

	return stoppedCount, nil
}

// sendCloudDeactivateEvents sends deactivation event and stops heartbeat.
// Errors are logged but do not fail the deactivation.
func sendCloudDeactivateEvents(mgr *config.Manager, contextName string) {
	client := NewCloudClient(mgr)
	if client == nil {
		return // Cloud integration not configured
	}

	appConfig := mgr.GetAppConfig()
	if appConfig == nil || appConfig.Cloud == nil {
		return
	}

	// Stop heartbeat
	hbMgr := cloud.NewHeartbeatManager(mgr.StateDir())
	hbMgr.StopHeartbeat()

	// Send deactivation audit event (synchronous - must complete before exit)
	if appConfig.Cloud.SendAuditEvents {
		event := &cloud.AuditEvent{
			Action:      "deactivate",
			ContextName: contextName,
			Success:     true,
		}
		if err := client.SendAuditEvent(event); err != nil {
			yellow := color.New(color.FgYellow)
			yellow.Fprintf(os.Stderr, "⚠ Cloud audit event failed: %v\n", err)
		}
	}

	// Notify cloud server to deactivate session
	_ = client.Deactivate(contextName)
}
