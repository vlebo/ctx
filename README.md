# ctx

**Context switcher for DevOps**

[![CI](https://github.com/vlebo/ctx/actions/workflows/ci.yml/badge.svg)](https://github.com/vlebo/ctx/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/vlebo/ctx.svg)](https://pkg.go.dev/github.com/vlebo/ctx)
[![GitHub release](https://img.shields.io/github/v/release/vlebo/ctx)](https://github.com/vlebo/ctx/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

---

`ctx` is a CLI tool that simplifies working across multiple cloud environments, projects, and infrastructure platforms. Switch between AWS profiles, Kubernetes clusters, VPN connections, and SSH tunnels with a single command.

```
$ ctx list
NAME           ENVIRONMENT  CLOUD  ORCHESTRATION
acme-dev       development  aws    kubernetes
acme-staging   staging      aws    kubernetes
acme-prod      production   aws    kubernetes

$ ctx use acme-prod
✓ AWS profile: acme-prod (us-east-1)
✓ Kubernetes: prod-cluster/default
✓ VPN connected: wireguard (wg0)
✓ Tunnels: postgres, redis

[ctx: acme-prod] $
```

## Features

| Category | Features |
|----------|----------|
| **Cloud Providers** | AWS, GCP, Azure profile switching with auto-login support |
| **Orchestration** | Kubernetes, Nomad, Consul context management |
| **Networking** | SSH tunnels with auto-reconnect, per-tunnel management |
| **VPN** | OpenVPN, WireGuard, Tailscale, custom commands |
| **Secrets** | Bitwarden, 1Password, Vault, AWS Secrets Manager, AWS SSM, GCP Secret Manager |
| **Identity** | Per-context Git user configuration |
| **Registries** | Docker and NPM registry configuration |
| **Browser** | Chrome/Firefox profile per context for SSO workflows |
| **Shell** | Per-shell contexts, colored prompts, environment isolation |
| **Inheritance** | Base/abstract contexts with multi-level inheritance |

## Quick Install

```bash
curl -fsSL https://github.com/vlebo/ctx/releases/latest/download/install.sh | sh
```

Add to your `~/.zshrc` or `~/.bashrc`:

```bash
eval "$(ctx shell-hook zsh)"   # for Zsh
eval "$(ctx shell-hook bash)"  # for Bash
```

## Quick Start

```bash
# Initialize ctx
ctx init

# Create a context file at ~/.config/ctx/contexts/myproject-dev.yaml
cat > ~/.config/ctx/contexts/myproject-dev.yaml << 'EOF'
name: myproject-dev
description: "MyProject Development Environment"
environment: development

aws:
  profile: myproject-dev
  region: us-west-2

kubernetes:
  context: arn:aws:eks:us-west-2:123456789:cluster/dev
  namespace: default
EOF

# Switch to the context
ctx use myproject-dev

# Verify
ctx                          # Show current context status
aws sts get-caller-identity  # Using your AWS profile
kubectl get pods             # Connected to your cluster
```

## Documentation

Full documentation is available at **[vlebo.github.io/ctx](https://vlebo.github.io/ctx)**

- [Getting Started](https://vlebo.github.io/ctx/getting-started/) - Installation and first context
- [Configuration Overview](https://vlebo.github.io/ctx/configuration/overview/) - Complete configuration reference
- [Context Inheritance](https://vlebo.github.io/ctx/configuration/inheritance/) - Avoid repeating configuration
- [CLI Reference](https://vlebo.github.io/ctx/commands/) - All available commands
- [AWS Setup](https://vlebo.github.io/ctx/cloud/aws/) - SSO, aws-vault, Secrets Manager
- [Secrets Management](https://vlebo.github.io/ctx/secrets/) - Bitwarden, 1Password, Vault, and more
- [VPN](https://vlebo.github.io/ctx/features/vpn/) - OpenVPN, WireGuard, Tailscale
- [SSH Tunnels](https://vlebo.github.io/ctx/features/tunnels/) - Auto-connect tunnels
- [Tips & Best Practices](https://vlebo.github.io/ctx/guides/tips/) - Production safety, credential isolation

## Platform Support

| Platform | Status |
|----------|--------|
| **Linux** | Full support |
| **macOS** | Full support |
| **Windows** | WSL required |

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT License - see [LICENSE](LICENSE) for details.
