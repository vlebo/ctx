// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/vlebo/ctx/internal/config"
)

var listAllFlag bool

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all available contexts",
		Long: `List all available contexts with their environment, cloud providers, and orchestration tools.

Abstract (base/template) contexts are hidden by default. Use --all to show them.`,
		RunE: runList,
	}

	cmd.Flags().BoolVarP(&listAllFlag, "all", "a", false, "Show all contexts including abstract/base contexts")

	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	configs, err := mgr.ListContextConfigs()
	if err != nil {
		return fmt.Errorf("failed to list contexts: %w", err)
	}

	currentName, _ := mgr.GetCurrentContextName()

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"", "NAME", "ENVIRONMENT", "CLOUD", "ORCHESTRATION", "EXTRAS"})
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetTablePadding("  ")
	table.SetNoWhiteSpace(true)

	shownCount := 0
	for _, ctx := range configs {
		// Skip abstract contexts unless --all is set
		if ctx.Abstract && !listAllFlag {
			continue
		}

		summary := config.GetContextSummary(ctx, currentName)

		marker := " "
		if summary.IsCurrent {
			marker = "*"
		}
		if ctx.Abstract {
			marker = "~" // Indicate abstract/base context
		}

		envStr := formatEnvironmentWithColor(ctx)
		cloudStr := summary.CloudProvider
		orchStr := summary.Orchestration
		extrasStr := summary.Extras

		if cloudStr == "" {
			cloudStr = "-"
		}
		if orchStr == "" {
			orchStr = "-"
		}
		if extrasStr == "" {
			extrasStr = "-"
		}

		table.Append([]string{marker, summary.Name, envStr, cloudStr, orchStr, extrasStr})
		shownCount++
	}

	if shownCount == 0 {
		fmt.Println("No contexts found.")
		fmt.Printf("Create a context file in %s/\n", mgr.ContextsDir())
		return nil
	}

	table.Render()

	return nil
}

// formatEnvironmentWithColor returns a colored string for the environment.
func formatEnvironmentWithColor(ctx *config.ContextConfig) string {
	c := getEnvColor(ctx)
	return c.Sprint(string(ctx.Environment))
}
