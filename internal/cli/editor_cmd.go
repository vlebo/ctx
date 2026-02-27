// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit [file]",
		Short: "Open editor/IDE for the current context",
		Long: `Open the editor configured for the current context.

If a file path is provided, opens that file in the editor.
Otherwise, opens the configured workspace.

Requires an active context with an 'editor' section configured.

Examples:
  ctx edit                    # Open workspace in configured editor
  ctx edit main.go            # Open specific file in editor`,
		Args: cobra.MaximumNArgs(1),
		RunE: runEdit,
	}

	return cmd
}

func runEdit(cmd *cobra.Command, args []string) error {
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

	if ctx.Editor == nil {
		return fmt.Errorf("no editor configured for context '%s'", currentContext)
	}

	green := color.New(color.FgGreen)

	if len(args) > 0 {
		file := args[0]
		green.Printf("Opening %s in %s", file, ctx.Editor.Type)
		if ctx.Editor.Workspace != "" {
			fmt.Printf(" (%s)", ctx.Editor.Workspace)
		}
		fmt.Println()
		return OpenEditorFile(ctx.Editor, file)
	}

	green.Printf("Opening %s", ctx.Editor.Type)
	if ctx.Editor.Workspace != "" {
		fmt.Printf(" (%s)", ctx.Editor.Workspace)
	}
	fmt.Println()
	return OpenEditor(ctx.Editor)
}
