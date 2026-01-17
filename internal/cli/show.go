// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vlebo/ctx/internal/config"
)

func newShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show context configuration details",
		Long:  "Display detailed configuration for a specific context.",
		Args:  cobra.ExactArgs(1),
		RunE:  runShow,
	}

	return cmd
}

func runShow(cmd *cobra.Command, args []string) error {
	contextName := args[0]

	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	ctx, err := mgr.LoadContext(contextName)
	if err != nil {
		return fmt.Errorf("failed to load context: %w", err)
	}

	fmt.Print(config.FormatContextDetails(ctx))

	return nil
}
