// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

// Package shell provides shell integration for ctx.
package shell

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

// ShellType represents the type of shell.
type ShellType string

const (
	ShellBash ShellType = "bash"
	ShellZsh  ShellType = "zsh"
	ShellFish ShellType = "fish"
)

// DetectShell detects the current shell type.
func DetectShell() ShellType {
	shell := os.Getenv("SHELL")
	if strings.Contains(shell, "zsh") {
		return ShellZsh
	}
	if strings.Contains(shell, "fish") {
		return ShellFish
	}
	return ShellBash
}

// HookConfig holds configuration for generating shell hooks.
type HookConfig struct {
	ConfigDir    string
	StateDir     string
	PromptFormat string
}

// convertPromptFormatToBash converts Go template syntax to bash/zsh syntax.
func convertPromptFormatToBash(format string) string {
	result := format

	// Replace simple variables
	result = strings.ReplaceAll(result, "{{.Name}}", "${ctx_name}")
	result = strings.ReplaceAll(result, "{{.Environment}}", "${ctx_env}")

	// Handle {{if .IsProd}}...{{end}} conditionals
	// Convert to: $([ "$ctx_env" = "production" ] && echo "..." || echo "")
	ifProdRegex := regexp.MustCompile(`\{\{if \.IsProd\}\}(.*?)\{\{end\}\}`)
	result = ifProdRegex.ReplaceAllString(result, `$([ "$ctx_env" = "production" -o "$ctx_env" = "prod" ] && echo '$1')`)

	return result
}

// convertPromptFormatToFish converts Go template syntax to fish syntax.
func convertPromptFormatToFish(format string) string {
	result := format

	// Replace simple variables
	result = strings.ReplaceAll(result, "{{.Name}}", "$ctx_name")
	result = strings.ReplaceAll(result, "{{.Environment}}", "$ctx_env")

	// Handle {{if .IsProd}}...{{end}} conditionals
	// For fish, we'll use a simpler approach with string replace
	ifProdRegex := regexp.MustCompile(`\{\{if \.IsProd\}\}(.*?)\{\{end\}\}`)
	result = ifProdRegex.ReplaceAllString(result, `"(test "$ctx_env" = "production" -o "$ctx_env" = "prod"; and echo '$1')"`)

	return result
}

// GenerateHook generates the shell hook code for the specified shell type.
func GenerateHook(shellType ShellType, cfg HookConfig) (string, error) {
	var tmpl *template.Template
	var err error

	switch shellType {
	case ShellBash:
		tmpl, err = template.New("bash").Parse(bashHookTemplate)
	case ShellZsh:
		tmpl, err = template.New("zsh").Parse(zshHookTemplate)
	case ShellFish:
		tmpl, err = template.New("fish").Parse(fishHookTemplate)
	default:
		return "", fmt.Errorf("unsupported shell type: %s", shellType)
	}

	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Convert prompt format to shell-specific syntax
	var shellPromptFormat string
	switch shellType {
	case ShellFish:
		shellPromptFormat = convertPromptFormatToFish(cfg.PromptFormat)
	default:
		shellPromptFormat = convertPromptFormatToBash(cfg.PromptFormat)
	}

	data := map[string]string{
		"ConfigDir":    cfg.ConfigDir,
		"StateDir":     cfg.StateDir,
		"EnvFile":      filepath.Join(cfg.StateDir, "current.env"),
		"NameFile":     filepath.Join(cfg.StateDir, "current.name"),
		"PromptFormat": shellPromptFormat,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

const bashHookTemplate = `# ctx shell integration for bash
# Add this to your ~/.bashrc:
#   eval "$(ctx shell-hook)"

# ctx wrapper function - captures env vars for this shell session
ctx() {
    # Check for help flags - pass through directly
    if [[ "$*" == *"--help"* || "$*" == *"-h"* ]]; then
        command ctx "$@"
        return $?
    fi

    if [[ "$1" == "use" && $# -ge 2 ]]; then
        # First, run the actual switch (with side effects like VPN, kubectl, etc.)
        # This also writes env vars + resolved secrets to the env file
        command ctx "$@"
        local exit_code=$?

        if [[ $exit_code -eq 0 ]]; then
            # Source the env file which includes resolved secrets
            if [[ -f "{{.EnvFile}}" ]]; then
                source "{{.EnvFile}}"
            fi
        fi
        return $exit_code
    elif [[ "$1" == "deactivate" && $# -eq 1 ]]; then
        # Only intercept bare 'ctx deactivate', not 'ctx deactivate --export'
        # Capture unset commands from deactivate --export
        local unset_output
        unset_output=$(command ctx deactivate --export)

        # Run the actual deactivate for VPN/tunnel cleanup
        command ctx deactivate
        local exit_code=$?

        if [[ $exit_code -eq 0 ]]; then
            # Then unset the env vars in this shell
            eval "$unset_output"
        fi
        return $exit_code
    elif [[ "$1" == "logout" ]]; then
        # Capture unset commands before logout clears credentials
        local unset_output
        unset_output=$(command ctx deactivate --export)

        # Run the actual logout
        command ctx "$@"
        local exit_code=$?

        if [[ $exit_code -eq 0 ]]; then
            # Then unset the env vars in this shell
            eval "$unset_output"
        fi
        return $exit_code
    else
        command ctx "$@"
    fi
}

# Prompt function to show current context (uses env var, not file)
__ctx_prompt() {
    local ctx_name="${CTX_CURRENT:-}"
    if [[ -n "$ctx_name" ]]; then
        local ctx_env="${CTX_ENVIRONMENT:-}"
        local color=""
        local reset="\033[0m"

        case "$ctx_env" in
            production|prod)
                color="\033[31m"  # Red
                ;;
            staging|stage)
                color="\033[33m"  # Yellow
                ;;
            *)
                color="\033[32m"  # Green
                ;;
        esac

        echo -e "${color}{{.PromptFormat}}${reset} "
    fi
}

# Add to prompt if not already there
if [[ ! "$PROMPT_COMMAND" == *"__ctx_prompt"* ]]; then
    if [[ -z "$PROMPT_COMMAND" ]]; then
        PROMPT_COMMAND='PS1="$(__ctx_prompt)${PS1_ORIG:-$PS1}"'
    else
        PROMPT_COMMAND="${PROMPT_COMMAND}; PS1=\"\$(__ctx_prompt)\${PS1_ORIG:-\$PS1}\""
    fi
    # Save original PS1 on first load
    if [[ -z "$PS1_ORIG" ]]; then
        export PS1_ORIG="$PS1"
    fi
fi

# Optionally source last context for new shells (comment out if you want fresh shells)
if [[ -z "$CTX_CURRENT" && -f "{{.EnvFile}}" ]]; then
    source "{{.EnvFile}}"
fi
`

const zshHookTemplate = `# ctx shell integration for zsh
# Add this to your ~/.zshrc:
#   eval "$(ctx shell-hook)"

# ctx wrapper function - captures env vars for this shell session
ctx() {
    # Check for help flags - pass through directly
    if [[ "$*" == *"--help"* || "$*" == *"-h"* ]]; then
        command ctx "$@"
        return $?
    fi

    if [[ "$1" == "use" && $# -ge 2 ]]; then
        # First, run the actual switch (with side effects like VPN, kubectl, etc.)
        # This also writes env vars + resolved secrets to the env file
        command ctx "$@"
        local exit_code=$?

        if [[ $exit_code -eq 0 ]]; then
            # Source the env file which includes resolved secrets
            if [[ -f "{{.EnvFile}}" ]]; then
                source "{{.EnvFile}}"
            fi
        fi
        return $exit_code
    elif [[ "$1" == "deactivate" && $# -eq 1 ]]; then
        # Only intercept bare 'ctx deactivate', not 'ctx deactivate --export'
        # Capture unset commands from deactivate --export
        local unset_output
        unset_output=$(command ctx deactivate --export)

        # Run the actual deactivate for VPN/tunnel cleanup
        command ctx deactivate
        local exit_code=$?

        if [[ $exit_code -eq 0 ]]; then
            # Then unset the env vars in this shell
            eval "$unset_output"
        fi
        return $exit_code
    elif [[ "$1" == "logout" ]]; then
        # Capture unset commands before logout clears credentials
        local unset_output
        unset_output=$(command ctx deactivate --export)

        # Run the actual logout
        command ctx "$@"
        local exit_code=$?

        if [[ $exit_code -eq 0 ]]; then
            # Then unset the env vars in this shell
            eval "$unset_output"
        fi
        return $exit_code
    else
        command ctx "$@"
    fi
}

# Prompt function to show current context (uses env var, not file)
__ctx_prompt() {
    local ctx_name="${CTX_CURRENT:-}"
    if [[ -n "$ctx_name" ]]; then
        local ctx_env="${CTX_ENVIRONMENT:-}"
        local color=""
        local reset="%f"

        case "$ctx_env" in
            production|prod)
                color="%F{red}"
                ;;
            staging|stage)
                color="%F{yellow}"
                ;;
            *)
                color="%F{green}"
                ;;
        esac

        echo "${color}{{.PromptFormat}}${reset} "
    fi
}

# Set up prompt
setopt PROMPT_SUBST
if [[ ! "$PROMPT" == *'$(__ctx_prompt)'* ]]; then
    PROMPT='$(__ctx_prompt)'"${PROMPT}"
fi

# Optionally source last context for new shells (comment out if you want fresh shells)
if [[ -z "$CTX_CURRENT" && -f "{{.EnvFile}}" ]]; then
    source "{{.EnvFile}}"
fi
`

const fishHookTemplate = `# ctx shell integration for fish
# Add this to your ~/.config/fish/config.fish:
#   ctx shell-hook | source

# Helper to parse env output
function __ctx_parse_env
    for line in $argv
        # Remove 'export ' prefix and split on '='
        set -l clean (string replace 'export ' '' -- $line)
        set -l parts (string split '=' -- $clean)
        if test (count $parts) -ge 2
            set -l var_name $parts[1]
            set -l var_value (string join '=' -- $parts[2..-1])
            # Remove surrounding quotes
            set var_value (string trim -c '"' -- $var_value)
            set -gx $var_name $var_value
        end
    end
end

# ctx wrapper function - captures env vars for this shell session
function ctx --wraps=ctx --description 'Unified cloud context manager'
    # Check for help flags - pass through directly
    if contains -- --help $argv; or contains -- -h $argv
        command ctx $argv
        return $status
    end

    if test "$argv[1]" = "use"; and test (count $argv) -ge 2
        # First, run the actual switch (with side effects like VPN, kubectl, etc.)
        # This also writes env vars + resolved secrets to the env file
        command ctx $argv
        set -l exit_code $status

        if test $exit_code -eq 0
            # Source the env file which includes resolved secrets
            if test -f "{{.EnvFile}}"
                for line in (cat "{{.EnvFile}}")
                    set -l clean (string replace 'export ' '' -- $line)
                    set -l parts (string split '=' -- $clean)
                    if test (count $parts) -ge 2
                        set -l var_name $parts[1]
                        set -l var_value (string join '=' -- $parts[2..-1])
                        set var_value (string trim -c '"' -- $var_value)
                        set -gx $var_name $var_value
                    end
                end
            end
        end
        return $exit_code
    else if test "$argv[1]" = "deactivate"; and test (count $argv) -eq 1
        # Only intercept bare 'ctx deactivate', not 'ctx deactivate --export'
        # Capture unset commands from deactivate --export
        set -l unset_output (command ctx deactivate --export)

        # Run the actual deactivate for VPN/tunnel cleanup
        command ctx deactivate
        set -l exit_code $status

        if test $exit_code -eq 0
            # Then unset the env vars in this shell
            for line in $unset_output
                set -l var_name (string replace 'unset ' '' -- $line)
                set -e $var_name
            end
        end
        return $exit_code
    else if test "$argv[1]" = "logout"
        # Capture unset commands before logout clears credentials
        set -l unset_output (command ctx deactivate --export)

        # Run the actual logout
        command ctx $argv
        set -l exit_code $status

        if test $exit_code -eq 0
            # Then unset the env vars in this shell
            for line in $unset_output
                set -l var_name (string replace 'unset ' '' -- $line)
                set -e $var_name
            end
        end
        return $exit_code
    else
        command ctx $argv
    end
end

# Prompt function to show current context (uses env var, not file)
function __ctx_prompt
    set -l ctx_name "$CTX_CURRENT"
    if test -n "$ctx_name"
        set -l ctx_env "$CTX_ENVIRONMENT"
        set -l color

        switch "$ctx_env"
            case production prod
                set color (set_color red)
            case staging stage
                set color (set_color yellow)
            case '*'
                set color (set_color green)
        end

        set -l prompt_text "{{.PromptFormat}}"
        echo -n $color$prompt_text(set_color normal)" "
    end
end

# Add to fish_prompt if not already there
if not functions -q __ctx_original_fish_prompt
    if functions -q fish_prompt
        functions -c fish_prompt __ctx_original_fish_prompt
    else
        function __ctx_original_fish_prompt; end
    end

    function fish_prompt
        __ctx_prompt
        __ctx_original_fish_prompt
    end
end

# Optionally source last context for new shells (comment out if you want fresh shells)
if test -z "$CTX_CURRENT"; and test -f "{{.EnvFile}}"
    for line in (cat "{{.EnvFile}}")
        set -l clean (string replace 'export ' '' -- $line)
        set -l parts (string split '=' -- $clean)
        if test (count $parts) -ge 2
            set -l var_name $parts[1]
            set -l var_value (string join '=' -- $parts[2..-1])
            set var_value (string trim -c '"' -- $var_value)
            set -gx $var_name $var_value
        end
    end
end
`
