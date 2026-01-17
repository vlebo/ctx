// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

// Package cli implements the command-line interface for ctx.
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/vlebo/ctx/internal/config"
	"github.com/vlebo/ctx/pkg/types"
)

var (
	// Version is set at build time.
	Version = "dev"

	cfgManager *config.Manager
)

// NewRootCmd creates the root command.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "ctx",
		Short: "Unified cloud context manager",
		Long: `CTX is a CLI tool that manages cloud/infrastructure contexts for DevOps engineers
working across multiple clients, environments, and platforms.

It provides unified switching between AWS/GCP/Azure profiles, Kubernetes clusters,
Nomad/Consul clusters, and manages persistent SSH connections with local port
forwarding for tunnel access to remote services.`,
		Version:       Version,
		RunE:          runRoot,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Add subcommands
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newShowCmd())
	rootCmd.AddCommand(newUseCmd())
	rootCmd.AddCommand(newDeactivateCmd())
	rootCmd.AddCommand(newLogoutCmd())
	rootCmd.AddCommand(newTunnelCmd())
	rootCmd.AddCommand(newVPNCmd())
	rootCmd.AddCommand(newOpenCmd())
	rootCmd.AddCommand(newBrowserCmd())
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newShellHookCmd())

	return rootCmd
}

// Execute runs the root command.
func Execute() {
	rootCmd := NewRootCmd()
	// Update version after it's been set by main.go
	rootCmd.Version = Version
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// GetConfigManager returns the config manager, initializing it if needed.
func GetConfigManager() (*config.Manager, error) {
	if cfgManager != nil {
		return cfgManager, nil
	}

	var err error
	cfgManager, err = config.NewManager()
	if err != nil {
		return nil, err
	}

	return cfgManager, nil
}

// runRoot executes when ctx is called without subcommands.
// It shows the current context status.
func runRoot(cmd *cobra.Command, args []string) error {
	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	ctx, err := mgr.GetCurrentContext()
	if err != nil {
		return fmt.Errorf("failed to get current context: %w", err)
	}

	if ctx == nil {
		fmt.Println("No context currently active.")
		fmt.Println("Use 'ctx list' to see available contexts.")
		fmt.Println("Use 'ctx use <name>' to switch to a context.")
		return nil
	}

	printCurrentContext(ctx)
	return nil
}

// getEnvColor returns the color for the environment display.
func getEnvColor(ctx *types.ContextConfig) *color.Color {
	// Use custom color if specified
	if ctx.EnvColor != "" {
		switch strings.ToLower(ctx.EnvColor) {
		case "red":
			return color.New(color.FgRed, color.Bold)
		case "yellow":
			return color.New(color.FgYellow, color.Bold)
		case "green":
			return color.New(color.FgGreen, color.Bold)
		case "blue":
			return color.New(color.FgBlue, color.Bold)
		case "cyan":
			return color.New(color.FgCyan, color.Bold)
		case "magenta":
			return color.New(color.FgMagenta, color.Bold)
		case "white":
			return color.New(color.FgWhite, color.Bold)
		}
	}

	// Fall back to defaults for common environment names
	envLower := strings.ToLower(string(ctx.Environment))
	switch envLower {
	case "production", "prod":
		return color.New(color.FgRed, color.Bold)
	case "staging", "stage":
		return color.New(color.FgYellow, color.Bold)
	case "development", "dev":
		return color.New(color.FgGreen, color.Bold)
	case "beta":
		return color.New(color.FgCyan, color.Bold)
	default:
		return color.New(color.FgWhite, color.Bold)
	}
}

// printCurrentContext displays the current context information.
func printCurrentContext(ctx *types.ContextConfig) {
	// Get color based on env_color or fall back to defaults
	envColor := getEnvColor(ctx)

	// Header
	fmt.Print("Current context: ")
	envColor.Print(ctx.Name)
	fmt.Printf(" (%s)", ctx.Environment)
	if ctx.IsProd() {
		color.New(color.FgRed).Print(" ⚠️")
	}
	fmt.Println()
	fmt.Println()

	// Description
	if ctx.Description != "" {
		fmt.Printf("Description: %s\n\n", ctx.Description)
	}

	// Cloud
	if ctx.AWS != nil || ctx.GCP != nil || ctx.Azure != nil {
		fmt.Println("Cloud:")
		if ctx.AWS != nil {
			fmt.Printf("  AWS Profile: %s\n", ctx.AWS.Profile)
			if ctx.AWS.Region != "" {
				fmt.Printf("  Region: %s\n", ctx.AWS.Region)
			}
		}
		if ctx.GCP != nil {
			fmt.Printf("  GCP Project: %s\n", ctx.GCP.Project)
			if ctx.GCP.Region != "" {
				fmt.Printf("  Region: %s\n", ctx.GCP.Region)
			}
		}
		if ctx.Azure != nil {
			fmt.Printf("  Azure Subscription: %s\n", ctx.Azure.SubscriptionID)
		}
		fmt.Println()
	}

	// Orchestration
	if ctx.Kubernetes != nil || ctx.Nomad != nil || ctx.Consul != nil {
		fmt.Println("Orchestration:")
		if ctx.Kubernetes != nil {
			namespace := ctx.Kubernetes.Namespace
			if namespace == "" {
				namespace = "default"
			}
			fmt.Printf("  Kubernetes: %s (namespace: %s)\n", ctx.Kubernetes.Context, namespace)
		}
		if ctx.Nomad != nil {
			fmt.Printf("  Nomad: %s\n", ctx.Nomad.Address)
		}
		if ctx.Consul != nil {
			fmt.Printf("  Consul: %s\n", ctx.Consul.Address)
		}
		fmt.Println()
	}

	// SSH
	if ctx.SSH != nil && ctx.SSH.Bastion.Host != "" {
		fmt.Println("SSH:")
		fmt.Printf("  Bastion: %s@%s\n", ctx.SSH.Bastion.User, ctx.SSH.Bastion.Host)
		fmt.Println()
	}

	// Tunnels
	if len(ctx.Tunnels) > 0 {
		fmt.Printf("Tunnels: %d defined\n", len(ctx.Tunnels))
		fmt.Println()
	}

	// Tags
	if len(ctx.Tags) > 0 {
		fmt.Printf("Tags: %s\n", strings.Join(ctx.Tags, ", "))
	}
}
