// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

// Package main is the entry point for the ctx CLI tool.
package main

import (
	"fmt"

	"github.com/vlebo/ctx/internal/cli"
)

// Build information set at build time via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli.Version = fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date)
	cli.Execute()
}
