# Configuration Overview

Contexts are YAML files stored in `~/.config/ctx/contexts/`. Each file defines an environment with its cloud providers, clusters, tunnels, and other settings.

## Minimal Example

The simplest context just needs a name and one provider:

```yaml
name: myproject-prod
description: "Production Environment"
environment: production

aws:
  profile: myproject-prod
  region: us-east-1
```

## Complete Example

Here's a comprehensive example showing all available options:

```yaml
name: myproject-production
description: "MyProject Production Environment"
environment: production
# Prompt color: red, yellow, green, blue, cyan, magenta, white
env_color: red
tags: [myproject, production, aws]

# ─────────────────────────────────────────────────────────────
# Cloud Providers
# ─────────────────────────────────────────────────────────────
aws:
  profile: myproject-prod
  region: us-east-1
  sso_login: true                 # Auto-run 'aws sso login' on context switch
  # OR use aws-vault for accounts without SSO:
  # use_vault: true               # Get temporary credentials via aws-vault

gcp:
  project: myproject-prod-123456
  region: us-central1
  config_name: myproject-prod     # gcloud config name
  auto_login: true                # Auto-run 'gcloud auth login'

azure:
  subscription_id: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
  tenant_id: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
  auto_login: true                # Auto-run 'az login'

# ─────────────────────────────────────────────────────────────
# Orchestration
# ─────────────────────────────────────────────────────────────
kubernetes:
  context: arn:aws:eks:us-east-1:123456789:cluster/production
  namespace: production
  kubeconfig: ~/.kube/myproject-config   # Optional custom kubeconfig

nomad:
  address: https://nomad.internal:4646
  namespace: production
  skip_verify: false

consul:
  address: https://consul.internal:8500

# ─────────────────────────────────────────────────────────────
# SSH & Tunnels
# ─────────────────────────────────────────────────────────────
ssh:
  bastion:
    host: bastion.myproject.com
    port: 22
    user: admin
    identity_file: ~/.ssh/myproject_ed25519

tunnels:
  - name: nomad
    description: "Nomad UI"
    local_port: 4646
    remote_host: 10.0.1.10
    remote_port: 4646
    auto_connect: true            # Start automatically on context switch

  - name: postgres
    description: "Production Database"
    local_port: 5432
    remote_host: db.internal
    remote_port: 5432

  - name: redis
    description: "Redis Cache"
    local_port: 6379
    remote_host: redis.internal
    remote_port: 6379

# ─────────────────────────────────────────────────────────────
# VPN
# ─────────────────────────────────────────────────────────────
vpn:
  type: openvpn                   # openvpn, wireguard, tailscale, custom
  config_file: ~/vpn/myproject.ovpn
  auto_connect: true              # Connect automatically on context switch
  auto_disconnect: true           # Disconnect when switching away

# ─────────────────────────────────────────────────────────────
# Secrets & Identity
# ─────────────────────────────────────────────────────────────
bitwarden:
  auto_login: true                # Auto-run 'bw login' if not authenticated
  sso: true                       # Use SSO instead of email/password

onepassword:
  auto_login: true                # Auto-run 'op signin' if not authenticated
  account: "myproject.1password.com"

vault:
  address: https://vault.myproject.com
  namespace: admin
  auth_method: oidc               # token, oidc, aws, kubernetes, approle
  auto_login: true                # Auto-run 'vault login'

# What secrets to fetch from each provider (injected as env vars)
secrets:
  bitwarden:
    DB_PASSWORD: "prod-database-creds"
  onepassword:
    API_KEY: "api-credentials"
  vault:
    SECRET_TOKEN: "secret/prod/app#token"

git:
  user_name: "Your Name"
  user_email: "you@myproject.com"
  signing_key: ABCD1234           # GPG key ID
  gpg_sign: true

# ─────────────────────────────────────────────────────────────
# Registries
# ─────────────────────────────────────────────────────────────
docker:
  url: registry.myproject.com
  context: myproject-docker       # Docker context name

npm:
  registry: https://npm.myproject.com
  scope: "@myproject"

# ─────────────────────────────────────────────────────────────
# Databases (sets environment variables for CLI tools)
# ─────────────────────────────────────────────────────────────
databases:
  - name: primary
    type: postgres                # postgres, mysql, mongodb, redis
    host: localhost               # Use localhost with tunnel
    port: 5432
    database: production
    username: app_user
    ssl_mode: require

# ─────────────────────────────────────────────────────────────
# Proxy
# ─────────────────────────────────────────────────────────────
proxy:
  http: http://proxy.myproject.com:8080
  https: http://proxy.myproject.com:8080
  no_proxy: localhost,127.0.0.1,.internal

# ─────────────────────────────────────────────────────────────
# Browser Profile (for SSO/OAuth flows)
# ─────────────────────────────────────────────────────────────
browser:
  type: chrome                    # chrome or firefox
  profile: "MyProject"            # Profile name as shown in browser

# ─────────────────────────────────────────────────────────────
# Quick URLs
# ─────────────────────────────────────────────────────────────
urls:
  nomad: http://localhost:4646
  consul: http://localhost:8500
  grafana: https://grafana.myproject.com
  github: https://github.com/myproject-org
  jira: https://myproject.atlassian.net

# ─────────────────────────────────────────────────────────────
# Custom Environment Variables
# ─────────────────────────────────────────────────────────────
env:
  ENVIRONMENT: production
  PROJECT: myproject
  LOG_LEVEL: info
```

## Global Configuration

The global config file at `~/.config/ctx/config.yaml` controls default behavior:

```yaml
version: 1
default_context: ""              # Default context on new shells
auto_deactivate: true            # Auto-disconnect VPN/tunnels when switching
shell_integration: true
prompt_format: "[ctx: {{.Name}}{{if .IsProd}} ⚠️{{end}}]"

# Default deactivate behavior (can be overridden per-context)
deactivate:
  disconnect_vpn: true           # Disconnect VPN on deactivate
  stop_tunnels: true             # Stop tunnels on deactivate
```

## Deactivate Behavior

Control what happens when you run `ctx deactivate` or switch contexts. This can be set globally or per-context.

**Per-context override** (in context YAML):

```yaml
name: shared-vpn-context
# ... other config ...

# Override deactivate behavior for this context
deactivate:
  disconnect_vpn: false          # Keep VPN connected when switching away
  stop_tunnels: true             # Still stop tunnels
```

**Use case:** Multiple contexts sharing the same VPN - set `disconnect_vpn: false` so switching between them keeps the VPN connected.

| Setting | Default | Description |
|---------|---------|-------------|
| `disconnect_vpn` | `true` | Disconnect VPN when deactivating context |
| `stop_tunnels` | `true` | Stop SSH tunnels when deactivating context |

!!! note
    `ctx logout` always disconnects VPN and stops tunnels regardless of this setting.

## File Locations

| Path | Description |
|------|-------------|
| `~/.config/ctx/` | Main configuration directory |
| `~/.config/ctx/config.yaml` | Global settings |
| `~/.config/ctx/contexts/` | Context YAML files |
| `~/.config/ctx/state/` | Runtime state (PIDs, current context) |
