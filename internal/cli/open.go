// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newOpenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "open [name]",
		Short: "Open a URL defined in the current context",
		Long: `Open a URL defined in the current context's urls section.

Without arguments, lists all available URLs.
With a name argument, opens that specific URL in the default browser.

Examples:
  ctx open           # List available URLs
  ctx open nomad     # Open Nomad UI
  ctx open consul    # Open Consul UI`,
		RunE: runOpen,
	}

	return cmd
}

func runOpen(cmd *cobra.Command, args []string) error {
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

	// Check if URLs are defined
	if len(ctx.URLs) == 0 {
		fmt.Printf("No URLs defined for context '%s'.\n", currentContext)
		fmt.Println("\nAdd URLs to your context file:")
		fmt.Println("  urls:")
		fmt.Println("    nomad: http://localhost:4646")
		fmt.Println("    consul: http://localhost:8500")
		return nil
	}

	// If no argument, list available URLs
	if len(args) == 0 {
		cyan := color.New(color.FgCyan)
		cyan.Printf("Available URLs for '%s':\n\n", currentContext)

		// Sort keys for consistent output
		keys := make([]string, 0, len(ctx.URLs))
		for k := range ctx.URLs {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, name := range keys {
			url := ctx.URLs[name]
			fmt.Printf("  %-12s %s\n", name, url)
		}
		fmt.Println("\nUsage: ctx open <name>")
		return nil
	}

	// Open specific URL
	urlName := strings.ToLower(args[0])
	url, ok := ctx.URLs[urlName]
	if !ok {
		// Try case-insensitive match
		for k, v := range ctx.URLs {
			if strings.EqualFold(k, urlName) {
				url = v
				ok = true
				break
			}
		}
	}

	if !ok {
		return fmt.Errorf("URL '%s' not found in context '%s'", urlName, currentContext)
	}

	green := color.New(color.FgGreen)
	if ctx.Browser != nil {
		green.Printf("Opening %s in %s profile '%s': %s\n", urlName, ctx.Browser.Type, ctx.Browser.Profile, url)
	} else {
		green.Printf("Opening %s: %s\n", urlName, url)
	}

	return OpenURL(ctx.Browser, url)
}
