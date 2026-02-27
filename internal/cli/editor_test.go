// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"testing"

	"github.com/vlebo/ctx/internal/config"
)

func TestBuildEditorArgs(t *testing.T) {
	tests := []struct {
		name       string
		editorType config.EditorType
		workspace  string
		file       string
		want       []string
	}{
		{
			name:       "vscode workspace only",
			editorType: config.EditorVSCode,
			workspace:  "~/project.code-workspace",
			want:       []string{"~/project.code-workspace"},
		},
		{
			name:       "vscode file only",
			editorType: config.EditorVSCode,
			file:       "main.go",
			want:       []string{"--goto", "main.go"},
		},
		{
			name:       "vscode workspace and file",
			editorType: config.EditorVSCode,
			workspace:  "~/project.code-workspace",
			file:       "main.go",
			want:       []string{"~/project.code-workspace", "--goto", "main.go"},
		},
		{
			name:       "vscode no workspace no file",
			editorType: config.EditorVSCode,
			want:       nil,
		},
		{
			name:       "sublime project only",
			editorType: config.EditorSublime,
			workspace:  "~/project.sublime-project",
			want:       []string{"--project", "~/project.sublime-project"},
		},
		{
			name:       "sublime file only",
			editorType: config.EditorSublime,
			file:       "main.go",
			want:       []string{"main.go"},
		},
		{
			name:       "sublime project and file",
			editorType: config.EditorSublime,
			workspace:  "~/project.sublime-project",
			file:       "main.go",
			want:       []string{"--project", "~/project.sublime-project", "main.go"},
		},
		{
			name:       "vim session file",
			editorType: config.EditorVim,
			workspace:  "~/session.vim",
			want:       []string{"-S", "~/session.vim"},
		},
		{
			name:       "vim directory",
			editorType: config.EditorVim,
			workspace:  "~/project",
			want:       []string{"~/project"},
		},
		{
			name:       "vim file only",
			editorType: config.EditorVim,
			file:       "main.go",
			want:       []string{"main.go"},
		},
		{
			name:       "vim session and file",
			editorType: config.EditorVim,
			workspace:  "~/session.vim",
			file:       "main.go",
			want:       []string{"-S", "~/session.vim", "main.go"},
		},
		{
			name:       "vim no workspace no file",
			editorType: config.EditorVim,
			want:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildEditorArgs(tt.editorType, tt.workspace, tt.file)
			if !sliceEqual(got, tt.want) {
				t.Errorf("buildEditorArgs(%s, %q, %q) = %v, want %v",
					tt.editorType, tt.workspace, tt.file, got, tt.want)
			}
		})
	}
}

func TestIsVimSession(t *testing.T) {
	tests := []struct {
		workspace string
		want      bool
	}{
		{"~/session.vim", true},
		{"/path/to/project.vim", true},
		{"session.vim", true},
		{"~/project", false},
		{"~/project.code-workspace", false},
		{"~/project.sublime-project", false},
		{"", false},
		{"vim", false},
	}

	for _, tt := range tests {
		t.Run(tt.workspace, func(t *testing.T) {
			got := isVimSession(tt.workspace)
			if got != tt.want {
				t.Errorf("isVimSession(%q) = %v, want %v", tt.workspace, got, tt.want)
			}
		})
	}
}

func TestIsVimEditor(t *testing.T) {
	tests := []struct {
		editorType config.EditorType
		want       bool
	}{
		{config.EditorVim, true},
		{config.EditorVSCode, false},
		{config.EditorSublime, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.editorType), func(t *testing.T) {
			got := isVimEditor(tt.editorType)
			if got != tt.want {
				t.Errorf("isVimEditor(%s) = %v, want %v", tt.editorType, got, tt.want)
			}
		})
	}
}

// sliceEqual compares two string slices for equality.
func sliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
