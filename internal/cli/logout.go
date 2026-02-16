// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/vlebo/ctx/internal/cloud"
	"github.com/vlebo/ctx/internal/config"
)

func newLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout [context]",
		Short: "Fully disconnect and clear credentials for a context",
		Long: `Fully disconnect and clear credentials for a context.

This will:
1. Disconnect VPN (if configured)
2. Stop all tunnels
3. Remove Vault tokens from keychain
4. Clear Azure and GCP credentials
5. Clear AWS credentials (if using aws-vault)

If no context is specified, uses the current context.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runLogout,
	}

	return cmd
}

func runLogout(cmd *cobra.Command, args []string) error {
	var contextName string

	if len(args) > 0 {
		contextName = args[0]
	} else {
		// Use current context
		contextName = os.Getenv("CTX_CURRENT")
		if contextName == "" {
			return fmt.Errorf("no context specified and no active context")
		}
	}

	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	// Verify context exists
	ctx, err := mgr.LoadContext(contextName)
	if err != nil {
		return fmt.Errorf("context '%s' not found: %w", contextName, err)
	}

	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)

	fmt.Fprintf(os.Stderr, "Logging out of context '%s'...\n\n", contextName)

	// 1. Disconnect VPN (always, ignore deactivate config for logout)
	if ctx.VPN != nil && checkVPNStatus(ctx.VPN) {
		yellow.Fprint(os.Stderr, "• ")
		fmt.Fprintf(os.Stderr, "Disconnecting VPN (%s)... ", ctx.VPN.Type)
		if err := disconnectVPN(ctx.VPN); err != nil {
			red.Fprintf(os.Stderr, "failed: %v\n", err)
		} else {
			green.Fprintln(os.Stderr, "done")
		}
	}

	// 2. Stop tunnels (always, ignore deactivate config for logout)
	if len(ctx.Tunnels) > 0 {
		yellow.Fprint(os.Stderr, "• ")
		fmt.Fprint(os.Stderr, "Stopping tunnels... ")
		stopped, err := stopContextTunnels(mgr.StateDir(), contextName)
		if err != nil {
			red.Fprintf(os.Stderr, "failed: %v\n", err)
		} else if stopped > 0 {
			green.Fprintf(os.Stderr, "stopped %d tunnel(s)\n", stopped)
		} else {
			fmt.Fprintln(os.Stderr, "none running")
		}
	}

	// 3. Clear Vault token
	if ctx.Vault != nil {
		yellow.Fprint(os.Stderr, "• ")
		fmt.Fprint(os.Stderr, "Removing Vault token... ")
		if err := mgr.DeleteVaultToken(contextName); err != nil {
			red.Fprintf(os.Stderr, "failed: %v\n", err)
		} else {
			green.Fprintln(os.Stderr, "done")
		}
	}

	// 4. Clear Azure credentials
	if ctx.Azure != nil {
		yellow.Fprint(os.Stderr, "• ")
		fmt.Fprint(os.Stderr, "Removing Azure credentials... ")
		azureDir := mgr.AzureConfigDir(contextName)
		if err := os.RemoveAll(azureDir); err != nil {
			red.Fprintf(os.Stderr, "failed: %v\n", err)
		} else {
			green.Fprintln(os.Stderr, "done")
		}
	}

	// 5. Clear GCP credentials
	if ctx.GCP != nil {
		yellow.Fprint(os.Stderr, "• ")
		fmt.Fprint(os.Stderr, "Removing GCP credentials... ")
		gcpDir := mgr.GCPConfigDir(contextName)
		if err := os.RemoveAll(gcpDir); err != nil {
			red.Fprintf(os.Stderr, "failed: %v\n", err)
		} else {
			green.Fprintln(os.Stderr, "done")
		}
	}

	// 6. Clear AWS credentials (if using aws-vault)
	if ctx.AWS != nil && ctx.AWS.UseVault {
		yellow.Fprint(os.Stderr, "• ")
		fmt.Fprint(os.Stderr, "Removing AWS credentials... ")
		if err := mgr.DeleteAWSCredentials(contextName); err != nil {
			red.Fprintf(os.Stderr, "failed: %v\n", err)
		} else {
			green.Fprintln(os.Stderr, "done")
		}
	}

	// 7. Clear Bitwarden session
	if ctx.Bitwarden != nil {
		yellow.Fprint(os.Stderr, "• ")
		fmt.Fprint(os.Stderr, "Removing Bitwarden session... ")
		if err := mgr.DeleteBitwardenSession(contextName); err != nil {
			red.Fprintf(os.Stderr, "failed: %v\n", err)
		} else {
			green.Fprintln(os.Stderr, "done")
		}
	}

	// 8. Clear 1Password session
	if ctx.OnePassword != nil {
		yellow.Fprint(os.Stderr, "• ")
		fmt.Fprint(os.Stderr, "Removing 1Password session... ")
		if err := mgr.DeleteOnePasswordSession(contextName); err != nil {
			red.Fprintf(os.Stderr, "failed: %v\n", err)
		} else {
			green.Fprintln(os.Stderr, "done")
		}
	}

	// 9. Clean up secret files
	if err := mgr.CleanupSecretFiles(contextName); err != nil {
		yellow.Fprintf(os.Stderr, "⚠ Failed to clean up secret files: %v\n", err)
	}

	// If this was the current context, clear state files
	if os.Getenv("CTX_CURRENT") == contextName {
		if err := mgr.ClearCurrentContext(); err != nil {
			yellow.Fprintf(os.Stderr, "⚠ Failed to clear state files: %v\n", err)
		}
	}

	// Send cloud events (logout action + deactivate session)
	sendCloudLogoutEvents(mgr, contextName, string(ctx.Environment))

	fmt.Fprintln(os.Stderr)
	green.Fprintf(os.Stderr, "✓ Logged out of '%s'.\n", contextName)
	fmt.Fprintln(os.Stderr, "  You will need to re-authenticate next time you use this context.")
	if os.Getenv("CTX_CURRENT") == contextName {
		fmt.Fprintln(os.Stderr)
		yellow.Fprintln(os.Stderr, "Note: If env vars persist, run: source <(ctx deactivate --export)")
	}

	return nil
}

// sendCloudLogoutEvents sends logout audit event and deactivates cloud session.
func sendCloudLogoutEvents(mgr *config.Manager, contextName, environment string) {
	client := NewCloudClient(mgr)
	if client == nil {
		return
	}

	// Stop heartbeat if running
	hbMgr := cloud.NewHeartbeatManager(mgr.StateDir())
	_ = hbMgr.StopHeartbeat()

	// Send logout audit event (synchronous - must complete before exit)
	event := &cloud.AuditEvent{
		Action:      "logout",
		ContextName: contextName,
		Environment: environment,
		Success:     true,
	}
	_ = client.SendAuditEvent(event)

	// Deactivate cloud session
	_ = client.Deactivate(contextName)
}
