// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/vlebo/ctx/internal/config"
	"github.com/vlebo/ctx/internal/shell"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize ctx configuration",
		Long: `Initialize ctx by creating the configuration directory structure
and displaying shell integration instructions.`,
		RunE: runInit,
	}
}

func runInit(cmd *cobra.Command, args []string) error {
	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	// Create directories
	if err := mgr.EnsureDirs(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Create default config if it doesn't exist
	if _, err := mgr.LoadAppConfig(); err != nil {
		// Create default config
		config := &config.AppConfig{
			Version:          1,
			ShellIntegration: true,
			PromptFormat:     "[ctx: {{.Name}}{{if .IsProd}} ⚠️{{end}}]",
			ContextsDir:      mgr.ContextsDir(),
			TunnelsDir:       filepath.Join(mgr.ConfigDir(), "tunnels"),
		}
		if err := mgr.SaveAppConfig(config); err != nil {
			return fmt.Errorf("failed to create config: %w", err)
		}
	}

	green := color.New(color.FgGreen)
	cyan := color.New(color.FgCyan)

	green.Println("✓ Configuration directory created:", mgr.ConfigDir())
	green.Println("✓ Contexts directory created:", mgr.ContextsDir())
	green.Println("✓ State directory created:", mgr.StateDir())

	fmt.Println()
	cyan.Println("Shell Integration")
	fmt.Println()
	fmt.Println("Add the following to your shell configuration file:")
	fmt.Println()

	shellType := shell.DetectShell()
	switch shellType {
	case shell.ShellZsh:
		fmt.Println("  # Add to ~/.zshrc")
		cyan.Println("  eval \"$(ctx shell-hook)\"")
	case shell.ShellFish:
		fmt.Println("  # Add to ~/.config/fish/config.fish")
		cyan.Println("  ctx shell-hook | source")
	default:
		fmt.Println("  # Add to ~/.bashrc")
		cyan.Println("  eval \"$(ctx shell-hook)\"")
	}

	fmt.Println()
	fmt.Println("Then restart your shell or run:")
	fmt.Println()
	switch shellType {
	case shell.ShellZsh:
		cyan.Println("  source ~/.zshrc")
	case shell.ShellFish:
		cyan.Println("  source ~/.config/fish/config.fish")
	default:
		cyan.Println("  source ~/.bashrc")
	}

	fmt.Println()
	cyan.Println("Creating a Context")
	fmt.Println()
	fmt.Printf("Create a context file in %s/\n", mgr.ContextsDir())
	fmt.Println()
	fmt.Println("Example context file (my-project.yaml):")
	fmt.Println()
	fmt.Println(`  name: my-project
  description: "My Project Development"
  environment: development

  aws:
    profile: my-project-dev
    region: us-east-1

  kubernetes:
    context: my-k8s-context
    namespace: default

  env:
    ENVIRONMENT: development`)

	fmt.Println()
	fmt.Println("Then switch to it with:")
	cyan.Println("  ctx use my-project")

	return nil
}

func newShellHookCmd() *cobra.Command {
	var shellFlag string

	cmd := &cobra.Command{
		Use:   "shell-hook",
		Short: "Output shell integration code",
		Long: `Output shell integration code for the current shell.
This command is typically used with eval:

  eval "$(ctx shell-hook)"

The shell type is auto-detected, but can be overridden with --shell.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShellHook(shellFlag)
		},
	}

	cmd.Flags().StringVar(&shellFlag, "shell", "", "Shell type (bash, zsh, fish)")

	return cmd
}

func runShellHook(shellFlag string) error {
	mgr, err := GetConfigManager()
	if err != nil {
		return err
	}

	// Load app config for prompt format
	appConfig, err := mgr.LoadAppConfig()
	if err != nil {
		appConfig = &config.AppConfig{
			PromptFormat: "[ctx: {{.Name}}{{if .IsProd}} ⚠️{{end}}]",
		}
	}

	// Determine shell type
	var shellType shell.ShellType
	if shellFlag != "" {
		shellType = shell.ShellType(shellFlag)
	} else {
		shellType = shell.DetectShell()
	}

	// Generate hook
	hook, err := shell.GenerateHook(shellType, shell.HookConfig{
		ConfigDir:    mgr.ConfigDir(),
		StateDir:     mgr.StateDir(),
		PromptFormat: appConfig.PromptFormat,
	})
	if err != nil {
		return fmt.Errorf("failed to generate shell hook: %w", err)
	}

	fmt.Print(hook)
	return nil
}

// GetShellConfigFile returns the path to the shell config file.
func GetShellConfigFile(shellType shell.ShellType) string {
	home, _ := os.UserHomeDir()

	switch shellType {
	case shell.ShellZsh:
		return filepath.Join(home, ".zshrc")
	case shell.ShellFish:
		return filepath.Join(home, ".config", "fish", "config.fish")
	default:
		return filepath.Join(home, ".bashrc")
	}
}
