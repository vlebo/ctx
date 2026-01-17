# ctx_

**Multi-environment context manager**

Switch between AWS profiles, Kubernetes clusters, VPN connections, and SSH tunnels with a single command.

<div class="terminal">
  <div class="terminal-header">
    <span class="terminal-dot"></span>
    <span class="terminal-dot"></span>
    <span class="terminal-dot"></span>
  </div>
  <div class="terminal-body">
    <div class="term-line l1"><span class="term-prompt">$</span> <span class="term-cmd">ctx list</span></div>
    <div class="term-line l2"><span class="term-dim">   NAME           ENVIRONMENT  CLOUD  ORCHESTRATION</span></div>
    <div class="term-line l3">   acme-dev       development  aws    kubernetes</div>
    <div class="term-line l4">   acme-staging   staging      aws    kubernetes</div>
    <div class="term-line l5">   acme-prod      production   aws    kubernetes</div>
    <div class="term-line l6"></div>
    <div class="term-line l7"><span class="term-prompt">$</span> <span class="term-cmd">ctx use acme-prod</span></div>
    <div class="term-line l8"><span class="term-green">✓</span> AWS profile: acme-prod (us-east-1)</div>
    <div class="term-line l9"><span class="term-green">✓</span> Kubernetes: prod-cluster/default</div>
    <div class="term-line l10"><span class="term-green">✓</span> VPN connected: wireguard (wg0)</div>
    <div class="term-line l11"><span class="term-green">✓</span> Tunnels: postgres, redis</div>
    <div class="term-line l12"><span class="term-ctx">[ctx: acme-prod]</span> <span class="term-prompt">$</span></div>
  </div>
</div>

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

## Platform Support

| Platform | Status | Notes |
|----------|--------|-------|
| **Linux** | Full support | Native support |
| **macOS** | Full support | Native support |
| **Windows** | WSL required | Full WSL support including browser profiles |

!!! note "Windows Users"
    `ctx` uses Unix shell scripts and process signals. Please use [WSL (Windows Subsystem for Linux)](https://docs.microsoft.com/en-us/windows/wsl/install). Browser profile detection works automatically with your Windows Chrome/Firefox installations.

## Quick Install

```bash
curl -fsSL https://github.com/vlebo/ctx/releases/latest/download/install.sh | sh
```

Then add the shell hook to your `~/.zshrc` or `~/.bashrc`:

=== "Zsh"

    ```bash
    eval "$(ctx shell-hook zsh)"
    ```

=== "Bash"

    ```bash
    eval "$(ctx shell-hook bash)"
    ```

## Next Steps

- [Getting Started](getting-started.md) - Create your first context
- [Configuration Overview](configuration/overview.md) - Learn the configuration format
- [CLI Reference](commands.md) - All available commands
