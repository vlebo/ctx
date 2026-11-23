// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/vlebo/ctx/internal/config"
)

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, ExitOK},
		{"general error", errors.New("boom"), ExitError},
		{"context not found", &config.ContextNotFoundError{Name: "x"}, ExitContextNotFound},
		{"wrapped context not found", fmt.Errorf("failed to load context: %w", &config.ContextNotFoundError{Name: "x"}), ExitContextNotFound},
		{"abstract context", withExitCode(ExitAbstractContext, errors.New("abstract")), ExitAbstractContext},
		{"confirmation declined", withExitCode(ExitConfirmDeclined, errors.New("declined")), ExitConfirmDeclined},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitCode(tt.err); got != tt.want {
				t.Errorf("exitCode() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestWithExitCode_KeepsMessage(t *testing.T) {
	err := withExitCode(ExitConfirmDeclined, errors.New("aborted"))
	if err.Error() != "aborted" {
		t.Errorf("Error() = %q, want %q", err.Error(), "aborted")
	}
}
