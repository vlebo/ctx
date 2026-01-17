# HashiCorp Vault

ctx integrates with HashiCorp Vault for secrets management. When configured, it:

1. **Sets environment variables** - `VAULT_ADDR` and `VAULT_NAMESPACE` are set so the `vault` CLI works automatically
2. **Auto-login** - Optionally runs `vault login` when switching contexts
3. **Browser integration** - OIDC login opens in the configured browser profile
4. **Secret fetching** - Fetch secrets via the unified `secrets:` section

## Configuration

```yaml
vault:
  address: https://vault.example.com
  namespace: admin                 # Optional namespace
  auth_method: oidc                # token, oidc, aws, kubernetes, approle
  auto_login: true                 # Run 'vault login' on context switch
  skip_verify: false               # Skip TLS verification (not recommended)

# To fetch secrets from Vault:
secrets:
  vault:
    DATABASE_PASSWORD: "databases/prod#password"
    API_KEY: "services/api#key"
```

## Authentication Methods

| Method | Description |
|--------|-------------|
| `token` | Use existing VAULT_TOKEN or prompt for token |
| `oidc` | Browser-based SSO login (uses browser profile if configured) |
| `aws` | AWS IAM authentication |
| `kubernetes` | Kubernetes service account authentication |
| `approle` | AppRole authentication |

## Environment Variables Set

| Variable | Description |
|----------|-------------|
| `VAULT_ADDR` | Vault server address |
| `VAULT_NAMESPACE` | Vault namespace (if configured) |

## How It Works

When you run `ctx use <name>` with Vault configured:

1. `VAULT_ADDR` is set to the configured address
2. `VAULT_NAMESPACE` is set if specified
3. Checks for existing saved token (verifies it's still valid)
4. If no valid token and `auto_login: true`, runs `vault login -method=<auth_method>`
5. Saves the new token securely (see below)
6. If a browser profile is configured, OIDC opens in that profile

## Secret Path Formats

For Vault secrets, use the format `mount/path#field`:

- `mount/path#field` - e.g., `operations/consul#http_user`
- `mount/path` - defaults to `value` field
- Use the same path as `vault kv get -mount=<mount> <path>`
- Do NOT include `/data/` - ctx handles KV v2 automatically

ctx tries KV v2 (`vault kv get`) first, then falls back to KV v1 (`vault read`).

### Examples

```yaml
secrets:
  vault:
    # KV v2 - specify mount and path
    DB_PASS: "databases/prod/postgres#password"
    DB_USER: "databases/prod/postgres#username"

    # Different mount
    CONSUL_USER: "operations/consul#http_user"

    # Default to 'value' field
    API_KEY: "services/myapp#value"
```

## Token Storage Security

Vault tokens are stored securely using your system's keychain:

| Platform | Storage |
|----------|---------|
| **macOS** | Keychain (encrypted, requires login) |
| **Linux** | Secret Service API (gnome-keyring, kwallet) |
| **Windows** | Windows Credential Manager |
| **Headless/SSH** | Falls back to file with 0600 permissions |

**Benefits:**

- Tokens are encrypted at rest
- Requires user authentication to access
- Per-context isolation (different contexts have different tokens)
- Automatic token verification on context switch

## Managing Tokens

```bash
# View stored token (macOS)
security find-generic-password -s "ctx" -a "vault-token-mycontext" -w

# View stored token (Linux)
secret-tool lookup service ctx username vault-token-mycontext

# Remove all credentials for a context
ctx logout mycontext
```

!!! note
    `ctx logout` removes the Vault token from keychain. `ctx deactivate` keeps the token for next time.

## Usage

```bash
# Switch context (auto-login if configured)
ctx use myproject-prod

# Vault CLI now works automatically
vault kv get secret/myapp/config
vault kv list secret/

# Manual login if needed
vault login -method=oidc
```

## Browser Profile Integration

When using OIDC authentication with a browser profile configured, the login page opens in the correct profile:

```yaml
vault:
  address: https://vault.example.com
  auth_method: oidc
  auto_login: true

browser:
  type: chrome
  profile: "Work"
```
