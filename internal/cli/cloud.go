// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/vlebo/ctx/internal/cloud"
	"github.com/vlebo/ctx/internal/config"
	"gopkg.in/yaml.v3"
)

func newCloudCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cloud",
		Short: "Manage ctx-cloud integration",
		Long: `Manage integration with ctx-cloud server for team features.

ctx-cloud provides:
  - Audit logging of context switches and actions
  - Live view of active users and their contexts
  - Shared context repository for teams
  - Centralized configuration management`,
	}

	cmd.AddCommand(newCloudLoginCmd())
	cmd.AddCommand(newCloudLogoutCmd())
	cmd.AddCommand(newCloudStatusCmd())
	cmd.AddCommand(newCloudConfigCmd())
	cmd.AddCommand(newCloudSyncCmd())
	cmd.AddCommand(newCloudListCmd())

	return cmd
}

func newCloudLoginCmd() *cobra.Command {
	var serverURL string
	var apiKey string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Configure ctx-cloud integration",
		Long: `Configure ctx-cloud integration by providing your server URL and API key.

You can obtain an API key from the ctx-cloud web dashboard:
  1. Log in to your ctx-cloud instance
  2. Go to Settings > API Keys
  3. Create a new API key with the required scopes

Example:
  ctx cloud login --server https://cloud.example.com --api-key sk_xxx`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCloudLogin(serverURL, apiKey)
		},
	}

	cmd.Flags().StringVar(&serverURL, "server", "", "ctx-cloud server URL (e.g., https://cloud.example.com)")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")

	return cmd
}

func runCloudLogin(serverURL, apiKey string) error {
	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	reader := bufio.NewReader(os.Stdin)

	// Prompt for server URL if not provided
	if serverURL == "" {
		fmt.Print("Enter ctx-cloud server URL: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		serverURL = strings.TrimSpace(input)
	}

	// Validate URL
	if serverURL == "" {
		return fmt.Errorf("server URL is required")
	}
	if !strings.HasPrefix(serverURL, "http://") && !strings.HasPrefix(serverURL, "https://") {
		serverURL = "https://" + serverURL
	}
	// Remove trailing slash
	serverURL = strings.TrimSuffix(serverURL, "/")

	// Prompt for API key if not provided
	if apiKey == "" {
		fmt.Print("Enter API key: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		apiKey = strings.TrimSpace(input)
	}

	if apiKey == "" {
		return fmt.Errorf("API key is required")
	}

	// Test the connection
	yellow := color.New(color.FgYellow)
	yellow.Print("• Testing connection... ")

	client := cloud.NewClient(serverURL, apiKey)
	if err := client.TestConnection(); err != nil {
		red := color.New(color.FgRed)
		red.Println("failed")
		return fmt.Errorf("connection test failed: %w", err)
	}

	green := color.New(color.FgGreen)
	green.Println("success")

	// Save API key
	if err := mgr.SaveCloudAPIKey(apiKey); err != nil {
		return fmt.Errorf("failed to save API key: %w", err)
	}

	// Update app config with cloud settings
	appConfig := mgr.GetAppConfig()
	if appConfig == nil {
		appConfig = &config.AppConfig{}
	}

	appConfig.Cloud = &config.CloudConfig{
		ServerURL:         serverURL,
		Enabled:           true,
		SendAuditEvents:   true,
		SendHeartbeat:     true,
		HeartbeatInterval: 30,
	}

	if err := mgr.SaveAppConfig(appConfig); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println()
	green.Println("✓ ctx-cloud integration configured successfully!")
	fmt.Println()
	fmt.Println("Cloud features are now enabled:")
	fmt.Println("  - Audit events will be sent when you switch contexts")
	fmt.Println("  - Heartbeats will be sent while a context is active")
	fmt.Println()
	fmt.Println("You can adjust settings in ~/.config/ctx/config.yaml")

	return nil
}

func newCloudLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove ctx-cloud integration",
		Long:  `Remove ctx-cloud integration by deleting the stored API key and disabling cloud features.`,
		RunE:  runCloudLogout,
	}
}

func runCloudLogout(cmd *cobra.Command, args []string) error {
	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	// Delete API key
	if err := mgr.DeleteCloudAPIKey(); err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	// Disable cloud in config
	appConfig := mgr.GetAppConfig()
	if appConfig != nil && appConfig.Cloud != nil {
		appConfig.Cloud.Enabled = false
		if err := mgr.SaveAppConfig(appConfig); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}

	green := color.New(color.FgGreen)
	green.Println("✓ ctx-cloud integration removed.")

	return nil
}

func newCloudStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show ctx-cloud integration status",
		RunE:  runCloudStatus,
	}
}

func runCloudStatus(cmd *cobra.Command, args []string) error {
	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	appConfig := mgr.GetAppConfig()
	apiKey := mgr.LoadCloudAPIKey()

	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)
	cyan := color.New(color.FgCyan)

	fmt.Println("ctx-cloud Integration Status")
	fmt.Println("============================")
	fmt.Println()

	if appConfig == nil || appConfig.Cloud == nil {
		red.Println("Status: Not configured")
		fmt.Println()
		fmt.Println("Run 'ctx cloud login' to configure ctx-cloud integration.")
		return nil
	}

	cloud := appConfig.Cloud

	// Status
	fmt.Print("Status: ")
	if cloud.Enabled && apiKey != "" {
		green.Println("Enabled")
	} else if apiKey == "" {
		red.Println("No API key")
	} else {
		yellow.Println("Disabled")
	}

	// Server URL
	fmt.Print("Server: ")
	if cloud.ServerURL != "" {
		cyan.Println(cloud.ServerURL)
	} else {
		red.Println("Not set")
	}

	// API Key
	fmt.Print("API Key: ")
	if apiKey != "" {
		// Show masked key
		if len(apiKey) > 8 {
			green.Printf("%s...%s\n", apiKey[:4], apiKey[len(apiKey)-4:])
		} else {
			green.Println("Configured")
		}
	} else {
		red.Println("Not set")
	}

	fmt.Println()
	fmt.Println("Features:")
	printFeatureStatus("  Audit Events", cloud.SendAuditEvents)
	printFeatureStatus("  Heartbeat", cloud.SendHeartbeat)
	if cloud.SendHeartbeat {
		fmt.Printf("  Heartbeat Interval: %ds\n", cloud.HeartbeatInterval)
	}

	// Test connection if configured
	if cloud.Enabled && apiKey != "" && cloud.ServerURL != "" {
		fmt.Println()
		fmt.Print("Connection: ")
		client := NewCloudClient(mgr)
		if client != nil {
			if err := client.TestConnection(); err != nil {
				red.Printf("Failed (%v)\n", err)
			} else {
				green.Println("OK")
			}
		} else {
			red.Println("Client not initialized")
		}
	}

	return nil
}

func printFeatureStatus(name string, enabled bool) {
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)

	fmt.Print(name + ": ")
	if enabled {
		green.Println("Enabled")
	} else {
		red.Println("Disabled")
	}
}

func newCloudConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configure ctx-cloud settings",
		Long: `Configure ctx-cloud settings.

Examples:
  ctx cloud config --audit-events=true
  ctx cloud config --heartbeat=false
  ctx cloud config --heartbeat-interval=60`,
		RunE: runCloudConfig,
	}

	cmd.Flags().Bool("audit-events", true, "Enable/disable audit event sending")
	cmd.Flags().Bool("heartbeat", true, "Enable/disable heartbeat sending")
	cmd.Flags().Int("heartbeat-interval", 30, "Heartbeat interval in seconds")

	return cmd
}

func runCloudConfig(cmd *cobra.Command, args []string) error {
	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	appConfig := mgr.GetAppConfig()
	if appConfig == nil || appConfig.Cloud == nil {
		return fmt.Errorf("ctx-cloud not configured. Run 'ctx cloud login' first")
	}

	// Update settings based on flags
	if cmd.Flags().Changed("audit-events") {
		val, _ := cmd.Flags().GetBool("audit-events")
		appConfig.Cloud.SendAuditEvents = val
	}

	if cmd.Flags().Changed("heartbeat") {
		val, _ := cmd.Flags().GetBool("heartbeat")
		appConfig.Cloud.SendHeartbeat = val
	}

	if cmd.Flags().Changed("heartbeat-interval") {
		val, _ := cmd.Flags().GetInt("heartbeat-interval")
		appConfig.Cloud.HeartbeatInterval = val
	}

	if err := mgr.SaveAppConfig(appConfig); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	green := color.New(color.FgGreen)
	green.Println("✓ Cloud settings updated.")

	return nil
}

// NewCloudClient creates a cloud client from the config manager.
// Returns nil if cloud integration is not configured or disabled.
func NewCloudClient(mgr interface {
	GetAppConfig() *config.AppConfig
	LoadCloudAPIKey() string
}) *cloud.Client {
	appConfig := mgr.GetAppConfig()
	if appConfig == nil || appConfig.Cloud == nil || !appConfig.Cloud.Enabled {
		return nil
	}

	apiKey := mgr.LoadCloudAPIKey()
	if apiKey == "" {
		return nil
	}

	return cloud.NewClient(appConfig.Cloud.ServerURL, apiKey)
}

func newCloudSyncCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "sync <context-name>",
		Short: "Sync a shared context from ctx-cloud",
		Long: `Download and save a shared context from the ctx-cloud server.

This fetches the context configuration from the cloud and saves it locally,
allowing you to use team-shared context configurations.

If the context already exists locally, use --force to overwrite it.

Examples:
  ctx cloud sync my-team-context
  ctx cloud sync my-team-context --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCloudSync(args[0], force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing local context")

	return cmd
}

func runCloudSync(contextName string, force bool) error {
	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	client := NewCloudClient(mgr)
	if client == nil {
		return fmt.Errorf("ctx-cloud not configured. Run 'ctx cloud login' first")
	}

	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)

	// Check if context already exists
	if mgr.ContextExists(contextName) && !force {
		return fmt.Errorf("context '%s' already exists locally. Use --force to overwrite", contextName)
	}

	// Fetch context from cloud
	yellow.Printf("• Fetching context '%s' from cloud... ", contextName)

	sharedCtx, err := client.SyncContext(contextName)
	if err != nil {
		fmt.Println()
		return fmt.Errorf("failed to fetch context: %w", err)
	}
	if sharedCtx == nil {
		fmt.Println()
		return fmt.Errorf("context '%s' not found on server", contextName)
	}

	green.Println("done")

	// Convert cloud config to local ContextConfig
	ctxConfig, err := convertCloudContext(sharedCtx)
	if err != nil {
		return fmt.Errorf("failed to convert context: %w", err)
	}

	// Save context locally
	yellow.Printf("• Saving context locally... ")

	if err := mgr.SaveContext(ctxConfig); err != nil {
		fmt.Println()
		return fmt.Errorf("failed to save context: %w", err)
	}

	green.Println("done")

	fmt.Println()
	green.Printf("✓ Context '%s' synced successfully!\n", contextName)
	fmt.Println()
	fmt.Printf("  Version: %d\n", sharedCtx.Version)
	if ctxConfig.Environment != "" {
		fmt.Printf("  Environment: %s\n", ctxConfig.Environment)
	}
	if ctxConfig.Description != "" {
		fmt.Printf("  Description: %s\n", ctxConfig.Description)
	}
	fmt.Println()
	fmt.Printf("Use 'ctx use %s' to activate this context.\n", contextName)

	return nil
}

// convertCloudContext converts a cloud SharedContext to a local ContextConfig.
func convertCloudContext(shared *cloud.SharedContext) (*config.ContextConfig, error) {
	// Convert the config map to YAML, then parse it as ContextConfig
	// This allows us to use the same YAML structure as local configs

	// First convert config map to JSON
	configJSON, err := json.Marshal(shared.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Then convert JSON to YAML-compatible map
	var configMap map[string]any
	if err := json.Unmarshal(configJSON, &configMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Add metadata to the config map
	configMap["name"] = shared.Name
	if shared.Description != "" {
		configMap["description"] = shared.Description
	}
	if shared.Environment != "" {
		configMap["environment"] = shared.Environment
	}
	if shared.IsAbstract {
		configMap["abstract"] = true
	}
	if shared.Extends != "" {
		configMap["extends"] = shared.Extends
	}

	// Convert to YAML
	yamlBytes, err := yaml.Marshal(configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to YAML: %w", err)
	}

	// Parse YAML into ContextConfig
	var ctxConfig config.ContextConfig
	if err := yaml.Unmarshal(yamlBytes, &ctxConfig); err != nil {
		return nil, fmt.Errorf("failed to parse context config: %w", err)
	}

	return &ctxConfig, nil
}

func newCloudListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List shared contexts available on ctx-cloud",
		Long:  `List all shared contexts available from the ctx-cloud server that you can sync.`,
		RunE:  runCloudList,
	}
}

func runCloudList(cmd *cobra.Command, args []string) error {
	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	client := NewCloudClient(mgr)
	if client == nil {
		return fmt.Errorf("ctx-cloud not configured. Run 'ctx cloud login' first")
	}

	contexts, err := client.GetSharedContexts()
	if err != nil {
		return fmt.Errorf("failed to fetch contexts: %w", err)
	}

	if len(contexts) == 0 {
		fmt.Println("No shared contexts available.")
		fmt.Println("Create contexts in the ctx-cloud web dashboard.")
		return nil
	}

	fmt.Println("Available shared contexts:")
	fmt.Println()

	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	cyan := color.New(color.FgCyan)

	for _, ctx := range contexts {
		// Name with abstract indicator
		if ctx.IsAbstract {
			yellow.Printf("  %s ", ctx.Name)
			fmt.Print("(abstract)")
		} else {
			cyan.Printf("  %s", ctx.Name)
		}

		// Environment badge
		if ctx.Environment != "" {
			if strings.Contains(ctx.Environment, "prod") {
				color.New(color.FgRed).Printf(" [%s]", ctx.Environment)
			} else if strings.Contains(ctx.Environment, "stag") {
				yellow.Printf(" [%s]", ctx.Environment)
			} else {
				fmt.Printf(" [%s]", ctx.Environment)
			}
		}

		fmt.Println()

		// Description
		if ctx.Description != "" {
			fmt.Printf("    %s\n", ctx.Description)
		}

		// Extends info
		if ctx.Extends != "" {
			fmt.Printf("    extends: %s\n", ctx.Extends)
		}

		// Check if synced locally
		if mgr.ContextExists(ctx.Name) {
			green.Print("    ✓ synced locally")
			fmt.Printf(" (v%d)\n", ctx.Version)
		}

		fmt.Println()
	}

	fmt.Println("Use 'ctx cloud sync <name>' to download a context.")

	return nil
}
