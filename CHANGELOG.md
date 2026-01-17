# Changelog

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
