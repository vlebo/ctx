// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/vlebo/ctx/internal/config"
)

func newBrowserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "browser",
		Short: "Browser profile management",
		Long:  `Manage browser profiles for contexts.`,
	}

	cmd.AddCommand(newBrowserListCmd())
	cmd.AddCommand(newBrowserOpenCmd())

	return cmd
}

func newBrowserListCmd() *cobra.Command {
	var browserType string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available browser profiles",
		Long: `List all available browser profiles for Chrome or Firefox.

Examples:
  ctx browser list              # List all profiles (Chrome and Firefox)
  ctx browser list --type chrome   # List only Chrome profiles
  ctx browser list --type firefox  # List only Firefox profiles`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrowserList(browserType)
		},
	}

	cmd.Flags().StringVarP(&browserType, "type", "t", "", "Browser type (chrome, firefox)")

	return cmd
}

func runBrowserList(browserType string) error {
	cyan := color.New(color.FgCyan)

	// List Chrome profiles
	if browserType == "" || browserType == "chrome" {
		profiles, err := listChromeProfiles()
		if err == nil && len(profiles) > 0 {
			cyan.Println("Chrome profiles:")
			for _, p := range profiles {
				fmt.Printf("  %-20s (dir: %s)\n", p.Name, p.Dir)
			}
			fmt.Println()
		} else if browserType == "chrome" {
			return fmt.Errorf("no Chrome profiles found: %v", err)
		}
	}

	// List Firefox profiles
	if browserType == "" || browserType == "firefox" {
		profiles, err := listFirefoxProfiles()
		if err == nil && len(profiles) > 0 {
			cyan.Println("Firefox profiles:")
			for _, p := range profiles {
				defaultMark := ""
				if p.Default {
					defaultMark = " (default)"
				}
				fmt.Printf("  %s%s\n", p.Name, defaultMark)
			}
			fmt.Println()
		} else if browserType == "firefox" {
			return fmt.Errorf("no Firefox profiles found: %v", err)
		}
	}

	fmt.Println("Usage in context config:")
	fmt.Println("  browser:")
	fmt.Println("    type: chrome")
	fmt.Println("    profile: \"Profile Name\"")

	return nil
}

func newBrowserOpenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "open [url]",
		Short: "Open browser with context profile",
		Long: `Open the browser configured for the current context.

If a URL is provided, opens that URL. Otherwise, just opens the browser.

Examples:
  ctx browser open                    # Open browser with context profile
  ctx browser open https://google.com # Open URL in context browser`,
		RunE: runBrowserOpen,
	}

	return cmd
}

func runBrowserOpen(cmd *cobra.Command, args []string) error {
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

	if ctx.Browser == nil {
		return fmt.Errorf("no browser configured for context '%s'", currentContext)
	}

	url := ""
	if len(args) > 0 {
		url = args[0]
	}

	green := color.New(color.FgGreen)
	if url != "" {
		green.Printf("Opening %s in %s profile '%s'\n", url, ctx.Browser.Type, ctx.Browser.Profile)
	} else {
		green.Printf("Opening %s profile '%s'\n", ctx.Browser.Type, ctx.Browser.Profile)
	}

	return openWithProfile(ctx.Browser, url)
}

// openWithProfile opens a browser with the specified profile.
func openWithProfile(cfg *config.BrowserConfig, url string) error {
	switch cfg.Type {
	case config.BrowserChrome:
		return openURLChrome(cfg.Profile, url)
	case config.BrowserFirefox:
		return openURLFirefox(cfg.Profile, url)
	default:
		return fmt.Errorf("unsupported browser type: %s", cfg.Type)
	}
}
