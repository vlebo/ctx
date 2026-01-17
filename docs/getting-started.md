# Getting Started

This guide will help you set up `ctx` and create your first context.

## Installation

### Quick Install

```bash
curl -fsSL https://github.com/vlebo/ctx/releases/latest/download/install.sh | sh
```

### From Source

```bash
git clone https://github.com/vlebo/ctx.git
cd ctx
go build -o ctx ./cmd/ctx
sudo mv ctx /usr/local/bin/
```

## Shell Integration

Add to your shell configuration file:

=== "Zsh (~/.zshrc)"

    ```bash
    eval "$(ctx shell-hook zsh)"
    ```

=== "Bash (~/.bashrc)"

    ```bash
    eval "$(ctx shell-hook bash)"
    ```

=== "Fish (~/.config/fish/config.fish)"

    ```fish
    ctx shell-hook fish | source
    ```

This enables:

- Context name in your prompt: `[ctx: myproject-prod]`
- Automatic environment variable loading on `ctx use`
- Per-shell context isolation
- Auto-load last context in new shells (can be disabled)

### Disable Auto-Load in New Shells

By default, new shell sessions automatically load the last active context. To disable this and start fresh:

1. Run `ctx shell-hook` to see the generated code
2. Find and comment out the auto-load section at the bottom

## Initialize ctx

```bash
ctx init
```

This creates the configuration directory at `~/.config/ctx/`.

## Create Your First Context

Create a file at `~/.config/ctx/contexts/myproject-dev.yaml`:

```yaml
name: myproject-dev
description: "MyProject Development Environment"
environment: development

aws:
  profile: myproject-dev
  region: us-west-2

kubernetes:
  context: arn:aws:eks:us-west-2:123456789:cluster/dev
  namespace: default
```

## Switch Context

```bash
ctx use myproject-dev
```

## Verify

```bash
# Show current context status
ctx

# Test AWS
aws sts get-caller-identity

# Test Kubernetes
kubectl get pods
```

## What's Next?

- [Configuration Overview](configuration/overview.md) - Learn all configuration options
- [Context Inheritance](configuration/inheritance.md) - Avoid repeating configuration
- [AWS Setup](cloud/aws.md) - AWS SSO, aws-vault, and more
- [SSH Tunnels](features/tunnels.md) - Auto-connect to databases and services
- [VPN](features/vpn.md) - Auto-connect VPN on context switch
