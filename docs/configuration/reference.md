# Configuration Reference

Complete reference for all context configuration options.

## Root Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | **Required.** Unique context identifier |
| `description` | string | Human-readable description |
| `environment` | string | Environment type: `development`, `staging`, `production` |
| `extends` | string | Parent context name to inherit from |
| `abstract` | bool | If true, context cannot be used directly |
| `env_color` | string | Prompt color: `red`, `yellow`, `green`, `blue`, `cyan`, `magenta`, `white` |
| `tags` | []string | Tags for filtering and organization |

## AWS

```yaml
aws:
  profile: string           # AWS CLI profile name
  region: string            # AWS region (us-east-1, eu-west-1, etc.)
  sso_login: bool           # Auto-run 'aws sso login' on context switch
  use_vault: bool           # Use aws-vault for temporary credentials
```

See [AWS Provider](../cloud/aws.md) for details.

## GCP

```yaml
gcp:
  project: string           # GCP project ID
  region: string            # GCP region
  config_name: string       # gcloud configuration name
  auto_login: bool          # Auto-run 'gcloud auth login'
```

See [GCP Provider](../cloud/gcp.md) for details.

## Azure

```yaml
azure:
  subscription_id: string   # Azure subscription ID
  tenant_id: string         # Azure tenant ID
  auto_login: bool          # Auto-run 'az login'
```

See [Azure Provider](../cloud/azure.md) for details.

## Kubernetes

```yaml
kubernetes:
  context: string           # kubectl context name (optional if using cloud auto-fetch)
  namespace: string         # Default namespace
  kubeconfig: string        # Path to kubeconfig file (optional)

  # Auto-fetch credentials from cloud providers (mutually exclusive)
  aks:                      # Azure Kubernetes Service
    cluster: string         # AKS cluster name (required)
    resource_group: string  # Azure resource group (required)

  eks:                      # AWS Elastic Kubernetes Service
    cluster: string         # EKS cluster name (required)
    region: string          # AWS region (optional, falls back to aws.region)

  gke:                      # Google Kubernetes Engine
    cluster: string         # GKE cluster name (required)
    zone: string            # Cluster zone (for zonal clusters)
    region: string          # Cluster region (for regional clusters)
    project: string         # GCP project (optional, falls back to gcp.project)
```

When `aks`, `eks`, or `gke` is configured, ctx will automatically fetch credentials using the respective cloud CLI before switching kubectl context.

## Nomad

```yaml
nomad:
  address: string           # Nomad server address (https://nomad:4646)
  namespace: string         # Nomad namespace
  skip_verify: bool         # Skip TLS verification
```

## Consul

```yaml
consul:
  address: string           # Consul server address (https://consul:8500)
```

## SSH

```yaml
ssh:
  bastion:
    host: string            # Bastion hostname
    port: int               # SSH port (default: 22)
    user: string            # SSH username
    identity_file: string   # Path to SSH private key
```

## Tunnels

```yaml
tunnels:
  - name: string            # Tunnel identifier
    description: string     # Human-readable description
    local_port: int         # Local port to bind
    remote_host: string     # Remote host to tunnel to
    remote_port: int        # Remote port
    auto_connect: bool      # Start automatically on context switch
```

See [SSH Tunnels](../features/tunnels.md) for details.

## VPN

```yaml
vpn:
  type: string              # openvpn, wireguard, tailscale, custom
  config_file: string       # Path to VPN config file
  interface: string         # WireGuard interface name
  exit_node: string         # Tailscale exit node
  connect_cmd: string       # Custom connect command
  disconnect_cmd: string    # Custom disconnect command
  status_cmd: string        # Custom status command
  auth_user_pass: string    # OpenVPN credentials file
  auto_connect: bool        # Connect on context switch
  auto_disconnect: bool     # Disconnect when switching away
```

See [VPN](../features/vpn.md) for details.

## Vault

```yaml
vault:
  address: string           # Vault server address
  namespace: string         # Vault namespace
  auth_method: string       # token, oidc, aws, kubernetes, approle
  auto_login: bool          # Auto-run 'vault login'
  skip_verify: bool         # Skip TLS verification
```

See [HashiCorp Vault](../secrets/vault.md) for details.

## Bitwarden

```yaml
bitwarden:
  auto_login: bool          # Auto-run 'bw login' if not authenticated
  sso: bool                 # Use SSO login
  org_identifier: string    # Organization identifier for SSO login
  server: string            # Self-hosted Bitwarden server URL
  email: string             # Email for login (pre-fills prompt)
```

## 1Password

```yaml
onepassword:
  auto_login: bool          # Auto-run 'op signin' if not authenticated
  account: string           # 1Password account URL
```

## Secrets

```yaml
secrets:
  bitwarden:
    ENV_VAR_NAME: "item-name"     # Bitwarden item name
  onepassword:
    ENV_VAR_NAME: "item-name"     # 1Password item name
  vault:
    ENV_VAR_NAME: "path#field"    # Vault path and field
```

See [Secrets Management](../secrets/index.md) for details.

## Git

```yaml
git:
  user_name: string         # Git author/committer name
  user_email: string        # Git author/committer email
  signing_key: string       # GPG key ID for signing
  gpg_sign: bool            # Enable commit signing
```

## Docker

```yaml
docker:
  url: string               # Docker registry URL
  context: string           # Docker context name
```

## NPM

```yaml
npm:
  registry: string          # NPM registry URL
  scope: string             # NPM scope (@mycompany)
```

## Databases

```yaml
databases:
  - name: string            # Database identifier
    type: string            # postgres, mysql, mongodb, redis
    host: string            # Database host
    port: int               # Database port
    database: string        # Database name
    username: string        # Database username
    ssl_mode: string        # SSL mode (postgres)
```

## Proxy

```yaml
proxy:
  http: string              # HTTP proxy URL
  https: string             # HTTPS proxy URL
  no_proxy: string          # Comma-separated bypass list
```

See [Proxy Configuration](../features/proxy.md) for details.

## Browser

```yaml
browser:
  type: string              # chrome or firefox
  profile: string           # Browser profile name
```

See [Browser Profiles](../features/browser.md) for details.

## URLs

```yaml
urls:
  name: string              # URL for 'ctx open name'
```

## Environment Variables

```yaml
env:
  VAR_NAME: value           # Custom environment variable
```

## Deactivate Behavior

```yaml
deactivate:
  disconnect_vpn: bool      # Disconnect VPN on deactivate (default: true)
  stop_tunnels: bool        # Stop tunnels on deactivate (default: true)
```
