// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/vlebo/ctx/internal/cloud"
	"github.com/vlebo/ctx/internal/config"
)

func newVPNCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vpn",
		Short: "Manage VPN connections",
		Long:  `Connect to or disconnect from VPN configured in the current context.`,
	}

	cmd.AddCommand(newVPNConnectCmd())
	cmd.AddCommand(newVPNDisconnectCmd())
	cmd.AddCommand(newVPNStatusCmd())

	return cmd
}

func newVPNConnectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect",
		Short: "Connect to VPN for current context",
		RunE:  runVPNConnect,
	}
}

func newVPNDisconnectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disconnect",
		Short: "Disconnect from VPN for current context",
		RunE:  runVPNDisconnect,
	}
}

func newVPNStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show VPN connection status",
		RunE:  runVPNStatus,
	}
}

func runVPNConnect(cmd *cobra.Command, args []string) error {
	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	ctx, err := mgr.GetCurrentContext()
	if err != nil {
		return fmt.Errorf("failed to get current context: %w", err)
	}

	if ctx == nil {
		return fmt.Errorf("no active context - use 'ctx use <name>' first")
	}

	if ctx.VPN == nil {
		return fmt.Errorf("no VPN configured for context '%s'", ctx.Name)
	}

	green := color.New(color.FgGreen)
	fmt.Printf("Connecting to VPN (%s)...\n", ctx.VPN.Type)

	if err := switchVPN(ctx.VPN); err != nil {
		// Send failure event
		sendVPNEvent(mgr, ctx.Name, string(ctx.Environment), "vpn.connect", false, err.Error())
		return fmt.Errorf("VPN connection failed: %w", err)
	}

	// Send success event
	sendVPNEvent(mgr, ctx.Name, string(ctx.Environment), "vpn.connect", true, "")

	green.Printf("✓ VPN connected\n")
	return nil
}

func runVPNDisconnect(cmd *cobra.Command, args []string) error {
	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	ctx, err := mgr.GetCurrentContext()
	if err != nil {
		return fmt.Errorf("failed to get current context: %w", err)
	}

	if ctx == nil {
		return fmt.Errorf("no active context - use 'ctx use <name>' first")
	}

	if ctx.VPN == nil {
		return fmt.Errorf("no VPN configured for context '%s'", ctx.Name)
	}

	yellow := color.New(color.FgYellow)
	fmt.Printf("Disconnecting from VPN (%s)...\n", ctx.VPN.Type)

	if err := disconnectVPN(ctx.VPN); err != nil {
		sendVPNEvent(mgr, ctx.Name, string(ctx.Environment), "vpn.disconnect", false, err.Error())
		return fmt.Errorf("VPN disconnect failed: %w", err)
	}

	sendVPNEvent(mgr, ctx.Name, string(ctx.Environment), "vpn.disconnect", true, "")

	yellow.Printf("✓ VPN disconnected\n")
	return nil
}

func runVPNStatus(cmd *cobra.Command, args []string) error {
	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	ctx, err := mgr.GetCurrentContext()
	if err != nil {
		return fmt.Errorf("failed to get current context: %w", err)
	}

	if ctx == nil {
		fmt.Println("No active context")
		return nil
	}

	if ctx.VPN == nil {
		fmt.Printf("No VPN configured for context '%s'\n", ctx.Name)
		return nil
	}

	fmt.Printf("Context: %s\n", ctx.Name)
	fmt.Printf("VPN Type: %s\n", ctx.VPN.Type)

	if ctx.VPN.Interface != "" {
		fmt.Printf("Interface: %s\n", ctx.VPN.Interface)
	}
	if ctx.VPN.ConfigFile != "" {
		fmt.Printf("Config File: %s\n", ctx.VPN.ConfigFile)
	}
	if ctx.VPN.ExitNode != "" {
		fmt.Printf("Exit Node: %s\n", ctx.VPN.ExitNode)
	}

	// Check actual connection status based on VPN type
	status := checkVPNStatus(ctx.VPN)
	if status {
		green := color.New(color.FgGreen)
		green.Println("Status: Connected")
	} else {
		yellow := color.New(color.FgYellow)
		yellow.Println("Status: Disconnected")

		// Check if a different VPN might be connected
		if otherVPN := checkAnyVPNRunning(ctx.VPN); otherVPN != "" {
			yellow.Printf("Note: A different VPN appears to be running: %s\n", otherVPN)
		}
	}

	return nil
}

// sendVPNEvent sends a VPN audit event to the cloud server.
func sendVPNEvent(mgr *config.Manager, contextName, environment, action string, success bool, errMsg string) {
	client := NewCloudClient(mgr)
	if client == nil {
		return
	}

	event := &cloud.AuditEvent{
		Action:       action,
		ContextName:  contextName,
		Environment:  environment,
		Success:      success,
		ErrorMessage: errMsg,
	}
	_ = client.SendAuditEvent(event)
}
