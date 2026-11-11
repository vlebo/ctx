// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

package cli

import (
	"errors"

	"github.com/vlebo/ctx/internal/config"
)

// Process exit codes (documented in docs/commands.md).
const (
	ExitOK              = 0
	ExitError           = 1
	ExitContextNotFound = 2
	ExitAbstractContext = 3
	ExitConfirmDeclined = 4
)

// exitCodeError attaches a process exit code to an error.
type exitCodeError struct {
	code int
	err  error
}

func (e *exitCodeError) Error() string { return e.err.Error() }
func (e *exitCodeError) Unwrap() error { return e.err }

// withExitCode wraps err so the process exits with code.
func withExitCode(code int, err error) error {
	return &exitCodeError{code: code, err: err}
}

// exitCode returns the process exit code for err.
func exitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var coded *exitCodeError
	if errors.As(err, &coded) {
		return coded.code
	}
	var notFound *config.ContextNotFoundError
	if errors.As(err, &notFound) {
		return ExitContextNotFound
	}
	return ExitError
}
