// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/vlebo/ctx/internal/config"
)

// getEditorCommand returns the editor binary path for the given editor type.
// It tries multiple candidates and returns the first one found in PATH.
func getEditorCommand(editorType config.EditorType) (string, error) {
	var candidates []string

	switch editorType {
	case config.EditorVSCode:
		candidates = []string{"code", "code-insiders"}
		if runtime.GOOS == "darwin" {
			candidates = append(candidates,
				"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code",
				"/Applications/Visual Studio Code - Insiders.app/Contents/Resources/app/bin/code-insiders",
			)
		}
	case config.EditorSublime:
		candidates = []string{"subl"}
		if runtime.GOOS == "darwin" {
			candidates = append(candidates,
				"/Applications/Sublime Text.app/Contents/SharedSupport/bin/subl",
			)
		}
	case config.EditorVim:
		candidates = []string{"nvim", "vim"}
	default:
		return "", fmt.Errorf("unsupported editor type: %s", editorType)
	}

	for _, cmd := range candidates {
		if path, err := exec.LookPath(cmd); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no binary found for editor '%s' (tried: %s)", editorType, strings.Join(candidates, ", "))
}

// isVimEditor returns true if the editor type is vim (runs in foreground terminal).
func isVimEditor(editorType config.EditorType) bool {
	return editorType == config.EditorVim
}

// isVimSession returns true if the workspace path looks like a Vim session file.
func isVimSession(workspace string) bool {
	return strings.HasSuffix(workspace, ".vim")
}

// buildEditorArgs constructs the argument list for the editor command.
// workspace and file may be empty.
func buildEditorArgs(editorType config.EditorType, workspace, file string) []string {
	switch editorType {
	case config.EditorVSCode:
		var args []string
		if workspace != "" {
			args = append(args, workspace)
		}
		if file != "" {
			args = append(args, "--goto", file)
		}
		return args

	case config.EditorSublime:
		var args []string
		if workspace != "" {
			args = append(args, "--project", workspace)
		}
		if file != "" {
			args = append(args, file)
		}
		return args

	case config.EditorVim:
		var args []string
		if workspace != "" {
			if isVimSession(workspace) {
				args = append(args, "-S", workspace)
			} else {
				// Treat as directory
				args = append(args, workspace)
			}
		}
		if file != "" {
			args = append(args, file)
		}
		return args

	default:
		if file != "" {
			return []string{file}
		}
		return nil
	}
}

// OpenEditor opens the workspace in the configured editor.
func OpenEditor(cfg *config.EditorConfig) error {
	if cfg == nil {
		return fmt.Errorf("no editor configured for this context")
	}

	cmdPath, err := getEditorCommand(cfg.Type)
	if err != nil {
		return err
	}

	workspace := cfg.Workspace
	if workspace != "" {
		workspace = expandPath(workspace)
	}

	args := buildEditorArgs(cfg.Type, workspace, "")
	cmd := exec.Command(cmdPath, args...)

	if isVimEditor(cfg.Type) {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return cmd.Start()
}

// OpenEditorFile opens a specific file in the configured editor.
func OpenEditorFile(cfg *config.EditorConfig, file string) error {
	if cfg == nil {
		return fmt.Errorf("no editor configured for this context")
	}

	cmdPath, err := getEditorCommand(cfg.Type)
	if err != nil {
		return err
	}

	workspace := cfg.Workspace
	if workspace != "" {
		workspace = expandPath(workspace)
	}

	args := buildEditorArgs(cfg.Type, workspace, file)
	cmd := exec.Command(cmdPath, args...)

	if isVimEditor(cfg.Type) {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return cmd.Start()
}
