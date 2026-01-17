// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package shell

import (
	"bytes"
	"os"
	"path/filepath"
	"text/template"

	"github.com/vlebo/ctx/pkg/types"
)

// PromptData holds data for prompt rendering.
type PromptData struct {
	Name        string
	Environment string
	IsProd      bool
}

// FormatPrompt formats the context name according to the prompt format template.
func FormatPrompt(format string, ctx *types.ContextConfig) (string, error) {
	if ctx == nil {
		return "", nil
	}

	data := PromptData{
		Name:        ctx.Name,
		Environment: string(ctx.Environment),
		IsProd:      ctx.IsProd(),
	}

	tmpl, err := template.New("prompt").Parse(format)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// GetCurrentContextName reads the current context name from the state file.
func GetCurrentContextName(stateDir string) (string, error) {
	namePath := filepath.Join(stateDir, "current.name")
	data, err := os.ReadFile(namePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}
