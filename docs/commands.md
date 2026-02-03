# CLI Reference

Complete reference for all ctx commands.

## Context Management

### `ctx`

Show current context status.

```bash
ctx
```

Displays:

- Current context name
- Environment (development, staging, production)
- Active cloud providers
- Active orchestration (Kubernetes, Nomad)
- VPN status
- Running tunnels

### `ctx list`

List available contexts.

```bash
ctx list                         # List usable contexts (hides abstract)
ctx list --all                   # List all contexts including abstract/base
```

Output columns:

- `NAME` - Context name (abstract contexts prefixed with `~`)
- `ENVIRONMENT` - Environment type
- `CLOUD` - Cloud providers (auto-detected from `aws`, `gcp`, `azure` configs + custom `cloud` label)
- `ORCHESTRATION` - Configured orchestrators

### `ctx use <name>`

Switch to a context.

```bash
ctx use myproject-dev            # Switch to context
ctx use myproject-prod --confirm # Switch to production (skip confirmation)
ctx use myproject-prod --replace # Switch and deactivate previous context
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--confirm` | Skip production confirmation prompt |
| `--replace` | Deactivate previous context before switching |

**What happens:**

1. Validates context exists and is not abstract
2. If production, prompts for confirmation (unless `--confirm`)
3. Sets environment variables
4. Connects VPN (if `auto_connect: true`)
5. Starts tunnels (if `auto_connect: true`)
6. Runs auto-login for cloud providers (if configured)
7. Fetches secrets and injects as environment variables

### `ctx show <name>`

Show detailed context configuration.

```bash
ctx show myproject-prod
```

For inherited contexts, shows the merged configuration.

### `ctx deactivate`

Deactivate current context.

```bash
ctx deactivate
```

**What happens:**

1. Clears context environment variables
2. Disconnects VPN (unless `deactivate.disconnect_vpn: false`)
3. Stops tunnels (unless `deactivate.stop_tunnels: false`)
4. Keeps credentials for next activation

### `ctx logout [context]`

Fully disconnect and clear all credentials.

```bash
ctx logout                       # Logout current context
ctx logout myproject-prod        # Logout specific context
```

**What happens:**

1. Everything `ctx deactivate` does
2. Removes stored tokens (Vault, etc.) from keychain
3. Clears cached credentials

### `ctx init`

Initialize ctx configuration.

```bash
ctx init
```

Creates `~/.config/ctx/` directory structure.

## SSH Tunnels

### `ctx tunnel list`

List defined tunnels for current context.

```bash
ctx tunnel list
```

### `ctx tunnel up [name]`

Start tunnels.

```bash
ctx tunnel up                    # Start all tunnels
ctx tunnel up postgres           # Start specific tunnel
```

### `ctx tunnel down [name]`

Stop tunnels.

```bash
ctx tunnel down                  # Stop all tunnels
ctx tunnel down postgres         # Stop specific tunnel
```

### `ctx tunnel status`

Show running tunnel status.

```bash
ctx tunnel status
```

## VPN

### `ctx vpn connect`

Connect VPN for current context.

```bash
ctx vpn connect
```

### `ctx vpn disconnect`

Disconnect VPN.

```bash
ctx vpn disconnect
```

### `ctx vpn status`

Show VPN connection status.

```bash
ctx vpn status
```

## Browser

### `ctx browser list`

List detected browser profiles.

```bash
ctx browser list
```

### `ctx browser open [url]`

Open browser with context profile.

```bash
ctx browser open                 # Open browser
ctx browser open https://example.com  # Open URL
```

## Quick URLs

### `ctx open [name]`

Open URLs defined in context.

```bash
ctx open                         # List available URLs
ctx open grafana                 # Open grafana URL in browser
```

Uses the configured browser profile if set.

## Shell Integration

### `ctx shell-hook <shell>`

Output shell integration script.

```bash
ctx shell-hook zsh               # Output Zsh integration
ctx shell-hook bash              # Output Bash integration
ctx shell-hook fish              # Output Fish integration
```

Add to your shell config:

=== "Zsh"

    ```bash
    eval "$(ctx shell-hook zsh)"
    ```

=== "Bash"

    ```bash
    eval "$(ctx shell-hook bash)"
    ```

=== "Fish"

    ```fish
    ctx shell-hook fish | source
    ```

## Deactivate vs Logout

| Command | VPN | Tunnels | Env Vars | Credentials |
|---------|-----|---------|----------|-------------|
| `ctx deactivate` | Disconnect* | Stop* | Clear | **Keep** |
| `ctx logout` | Disconnect | Stop | Clear | **Remove** |

*Configurable per-context via `deactivate.disconnect_vpn` and `deactivate.stop_tunnels`

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General error |
| `2` | Context not found |
| `3` | Abstract context (cannot use) |
| `4` | Production confirmation declined |
