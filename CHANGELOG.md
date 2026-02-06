# Changelog

## [0.1.6] - 2026-02-06

### Features

- **Variable Interpolation**: Use `${VAR}` syntax in config values with variables from the `env:` map. Enables template-based context configs with inheritance, reducing duplication for similar clusters.
- **Configurable Tunnel Timeout**: New `ssh.tunnel_timeout` option (default: 5s) to control how long to wait for tunnel connections.

### Improvements

- **SSH User Fallback**: SSH user now defaults to current OS user when not specified in config (matching ssh behavior).
- **Tunnel Failure Detection**: Tunnels now properly detect and report connection failures instead of showing false success.
- **Parallel Tunnel Startup**: All tunnels start in parallel, reducing total wait time from N*timeout to just timeout seconds.
- **Nomad/Consul Status**: Show warning instead of success when Nomad/Consul localhost addresses depend on failed tunnels.

## [0.1.5] - 2026-02-01

### Features

- **Custom Cloud Label**: New `cloud` field to label contexts with non-native cloud providers (e.g., DigitalOcean, OpenStack) shown in `ctx list` CLOUD column.

## [0.1.4] - 2026-01-28

### Bug Fixes

- Export `KUBECONFIG` environment variable when `kubeconfig` is set in context config.

## [0.1.3] - 2026-01-25

### Features

- **Cloud Sync**: Sync context usage and audit events to a central server for team visibility.

### Internal

- Refactored types package into internal/config for better encapsulation.

## [0.1.2] - 2026-01-20

### Features

- **Auto-fetch Kubernetes Credentials**: Automatically fetch credentials from cloud providers before switching kubectl context:
  - AWS EKS: `aws eks update-kubeconfig`
  - Google GKE: `gcloud container clusters get-credentials`
  - Azure AKS: `az aks get-credentials`

### Improvements

- Minor performance optimisations (thanks @dkorunic)

## [0.1.1] - 2026-01-18

Patch release with minor fixes.

## [0.1.0] - 2026-01-17

Initial release of ctx - a multi-environment context manager.

### Features

**Cloud Providers**
- AWS profile switching with SSO auto-login and aws-vault support
- GCP project/configuration switching with auto-login
- Azure subscription switching with auto-login

**Orchestration**
- Kubernetes context and namespace management
- Nomad address and namespace configuration

**Networking**
- SSH tunnels with bastion support and auto-reconnect
- VPN support: OpenVPN, WireGuard, Tailscale, and custom commands
- Proxy configuration (HTTP/HTTPS)

**Secrets Management**
- Bitwarden integration
- 1Password integration
- HashiCorp Vault (token, OIDC, AWS, Kubernetes, AppRole auth)
- AWS Secrets Manager
- AWS SSM Parameter Store
- GCP Secret Manager

**Developer Tools**
- Per-context Git user configuration
- Docker registry and context configuration
- NPM registry and scope configuration
- Browser profile support (Chrome/Firefox) for SSO workflows

**Shell Integration**
- Per-shell context isolation
- Colored prompts with environment indicators
- Support for Zsh, Bash, and Fish

**Configuration**
- YAML-based context definitions
- Context inheritance with abstract base contexts
- Environment tagging (development, staging, production)
- Production confirmation prompts

### Installation

```bash
curl -fsSL https://github.com/vlebo/ctx/releases/latest/download/install.sh | sh
```

### Documentation

Full documentation available at [vlebo.github.io/ctx](https://vlebo.github.io/ctx)
