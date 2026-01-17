# Contributing to ctx

Thank you for your interest in contributing to ctx! This document provides everything you need to understand the codebase, add features, and submit contributions.

## Table of Contents

- [Quick Start](#quick-start)
- [Project Structure](#project-structure)
- [Architecture](#architecture)
- [How to Add Features](#how-to-add-features)
- [Documentation](#documentation)
- [Testing](#testing)
- [Code Style](#code-style)
- [Submitting Changes](#submitting-changes)

---

## Quick Start

```bash
# Clone and build
git clone https://github.com/vlebo/ctx.git
cd ctx
go build -o ctx ./cmd/ctx

# Run tests
go test ./...

# Run linter
golangci-lint run

# Check REUSE compliance
reuse lint
```

### Key Technologies

| Library | Purpose |
|---------|---------|
| [Cobra](https://github.com/spf13/cobra) | CLI framework |
| [Viper](https://github.com/spf13/viper) | Configuration |
| [keyring](https://github.com/zalando/go-keyring) | Secure credential storage |
| [Color](https://github.com/fatih/color) | Terminal colors |

---

## Project Structure

```
ctx/
├── cmd/ctx/                    # Entry point
│   └── main.go
│
├── internal/                   # Private packages
│   ├── cli/                    # CLI commands
│   │   ├── root.go             # Root command, status display
│   │   ├── use.go              # Context switching
│   │   ├── list.go             # List contexts
│   │   ├── show.go             # Show context details
│   │   ├── deactivate.go       # Deactivate context
│   │   ├── logout.go           # Full logout with credential clearing
│   │   ├── tunnel.go           # SSH tunnel management
│   │   ├── vpn.go              # VPN commands
│   │   ├── secrets.go          # Secrets resolution (BW, 1Pass, Vault, etc.)
│   │   ├── switchers.go        # Cloud/tool switching functions
│   │   ├── browser.go          # Browser profile detection
│   │   └── browser_cmd.go      # Browser commands
│   │
│   ├── config/                 # Configuration management
│   │   ├── config.go           # Config manager, keychain operations
│   │   └── context.go          # Context validation
│   │
│   ├── shell/                  # Shell integration
│   │   ├── hook.go             # Shell hook generation
│   │   └── prompt.go           # Prompt formatting
│   │
│   └── ssh/                    # SSH tunnel management
│       ├── manager.go          # Tunnel lifecycle
│       ├── tunnel.go           # Tunnel operations
│       └── connection.go       # SSH connections
│
├── pkg/types/                  # Public type definitions
│   └── types.go
│
├── docs/                       # Documentation (mkdocs)
│   ├── index.md
│   ├── getting-started.md
│   ├── commands.md
│   ├── environment.md
│   ├── cloud/                  # Cloud provider docs
│   │   ├── aws.md
│   │   ├── gcp.md
│   │   └── azure.md
│   ├── secrets/                # Secrets provider docs
│   │   ├── index.md
│   │   ├── bitwarden.md
│   │   ├── onepassword.md
│   │   ├── vault.md
│   │   └── cloud.md
│   ├── features/               # Feature docs
│   │   ├── vpn.md
│   │   ├── tunnels.md
│   │   ├── browser.md
│   │   └── proxy.md
│   ├── configuration/          # Config docs
│   │   ├── overview.md
│   │   ├── inheritance.md
│   │   └── reference.md
│   └── guides/                 # Guides and tips
│       └── tips.md
│
├── examples/                   # Example context files
│   ├── minimal.yaml
│   ├── aws-sso.yaml
│   ├── multi-cloud.yaml
│   ├── with-secrets.yaml
│   └── with-inheritance.yaml
│
├── test/e2e/                   # End-to-end tests
│   └── run_tests.sh
│
├── .github/workflows/          # CI/CD
│   ├── ci.yml                  # Tests, build, lint
│   └── docs.yml                # Docs deployment
│
├── mkdocs.yml                  # Docs configuration
├── REUSE.toml                  # License compliance
└── .golangci.yml               # Linter configuration
```

---

## Architecture

### High-Level Flow

```
User runs 'ctx use dev'
         │
         ▼
    cmd/ctx/main.go
    └── cli.Execute()
         │
         ▼
    internal/cli/root.go
    └── Cobra routes to use.go
         │
         ▼
    internal/cli/use.go
    ├── Load context from YAML
    ├── Resolve inheritance
    ├── Switch cloud providers (switchers.go)
    ├── Connect VPN if configured
    ├── Start tunnels if configured
    ├── Resolve secrets (secrets.go)
    └── Write env file for shell hook
         │
         ▼
    Shell hook sources env file
    └── Environment variables active
```

### Key Design Decisions

1. **Per-Shell Contexts**: Each terminal maintains its own context via environment variables
2. **Keychain Storage**: Tokens (Vault, Bitwarden, 1Password) stored securely in system keychain
3. **Graceful Degradation**: Missing tools (kubectl, gcloud) cause warnings, not failures
4. **Inheritance**: Contexts can extend base contexts to reduce duplication

---

## How to Add Features

### Adding a New Secrets Provider

1. **Add config type** in `pkg/types/types.go`:

```go
type NewProviderConfig struct {
    AutoLogin bool   `yaml:"auto_login" mapstructure:"auto_login"`
    Server    string `yaml:"server" mapstructure:"server"`
}
```

2. **Add to ContextConfig**:

```go
type ContextConfig struct {
    // ...
    NewProvider *NewProviderConfig `yaml:"newprovider,omitempty"`
}
```

3. **Add to SecretsConfig**:

```go
type SecretsConfig struct {
    // ...
    NewProvider map[string]string `yaml:"newprovider,omitempty"`
}
```

4. **Implement in `internal/cli/secrets.go`**:

```go
func getNewProviderSecret(item string, cfg *types.NewProviderConfig) (string, error) {
    // Fetch secret from provider
}
```

5. **Add keychain functions** in `internal/config/config.go`:

```go
func (m *Manager) SaveNewProviderSession(ctx, session string) error
func (m *Manager) LoadNewProviderSession(ctx string) string
func (m *Manager) DeleteNewProviderSession(ctx string) error
```

6. **Add logout handling** in `internal/cli/logout.go`

7. **Add documentation** in `docs/secrets/newprovider.md`

### Adding a New Command

1. Create `internal/cli/mycommand.go`:

```go
func newMyCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "mycommand",
        Short: "Description",
        RunE:  runMyCommand,
    }
}

func runMyCommand(cmd *cobra.Command, args []string) error {
    // Implementation
    return nil
}
```

2. Register in `internal/cli/root.go`:

```go
rootCmd.AddCommand(newMyCmd())
```

---

## Documentation

Documentation is built with [MkDocs Material](https://squidfunk.github.io/mkdocs-material/) and hosted on GitHub Pages.

### Local Preview

```bash
pip install mkdocs-material
mkdocs serve
# Open http://localhost:8000
```

### Structure

- `docs/index.md` - Homepage
- `docs/cloud/` - Cloud provider docs (AWS, GCP, Azure)
- `docs/secrets/` - Secrets provider docs (Bitwarden, 1Password, Vault, etc.)
- `docs/features/` - Feature docs (VPN, tunnels, browser, proxy)
- `docs/configuration/` - Configuration reference
- `docs/guides/` - Tips and best practices

### Adding Documentation

1. Create/edit markdown files in `docs/`
2. Update `mkdocs.yml` nav section if adding new pages
3. Use admonitions for notes/warnings:

```markdown
!!! note
    Important information here.

!!! warning
    Be careful about this.
```

---

## Testing

### Running Tests

```bash
# All tests
go test ./...

# Verbose
go test -v ./...

# Specific package
go test -v ./internal/cli/

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### E2E Tests

```bash
./test/e2e/run_tests.sh
```

### Writing Tests

```go
func TestSomething(t *testing.T) {
    // Table-driven tests preferred
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"case1", "input1", "expected1"},
        {"case2", "input2", "expected2"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := doSomething(tt.input)
            if result != tt.expected {
                t.Errorf("got %s, want %s", result, tt.expected)
            }
        })
    }
}
```

---

## Code Style

### Linting

```bash
# Run linter
golangci-lint run

# REUSE compliance (license headers)
reuse lint
```

### Conventions

1. **Nil checks for optional config**:
```go
if ctx.AWS != nil {
    // AWS is configured
}
```

2. **Graceful degradation**:
```go
if _, err := exec.LookPath("kubectl"); err != nil {
    return nil  // Skip if not installed
}
```

3. **Error wrapping**:
```go
return fmt.Errorf("failed to load context: %w", err)
```

4. **Status indicators**:
```go
green := color.New(color.FgGreen)
green.Print("✓ ")
fmt.Println("Success")

yellow := color.New(color.FgYellow)
yellow.Fprintf(os.Stderr, "⚠ Warning: %v\n", err)
```

### REUSE Compliance

All files need copyright/license headers. For new files:

**Go files**: Add header comment
```go
// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT
```

**Other files**: Add to `REUSE.toml` if they can't have headers

---

## Submitting Changes

### Before Submitting

```bash
# Run all checks
go test ./...
golangci-lint run
reuse lint
go build ./...
```

### Pull Request Guidelines

1. **One feature per PR**
2. **Update docs** for user-facing changes
3. **Add tests** for new functionality
4. **Update examples/** if adding new config options

### Commit Messages

```
Add support for NewProvider secrets

- Add NewProviderConfig type
- Implement secret fetching
- Add keychain session storage
- Add documentation
```

---

## Questions?

Open an issue on GitHub or check the [documentation](https://vlebo.github.io/ctx).
