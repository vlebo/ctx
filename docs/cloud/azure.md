# Azure

ctx manages Azure CLI configurations per context, with support for auto-login and credential isolation.

## Configuration

```yaml
azure:
  subscription_id: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
  tenant_id: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
  auto_login: true                # Auto-run 'az login'
```

## How It Works

When you run `ctx use <context>` with Azure configured:

1. Sets `AZURE_SUBSCRIPTION_ID` to your subscription
2. Creates a per-context Azure CLI config directory for credential isolation
3. If `auto_login: true` and not authenticated, runs `az login`
4. Opens the configured browser profile for OAuth

## Environment Variables Set

| Variable | Description |
|----------|-------------|
| `AZURE_CONFIG_DIR` | Per-context Azure CLI config directory |
| `AZURE_SUBSCRIPTION_ID` | Azure subscription ID |

## Credential Isolation

ctx stores Azure credentials per-context to prevent conflicts:

| Storage | Location |
|---------|----------|
| **Azure config** | `~/.config/ctx/state/cloud/<context>/azure/` |

This allows you to have multiple shells authenticated to different Azure subscriptions simultaneously.

## Example Configurations

### Basic Setup

```yaml
name: myproject-prod
azure:
  subscription_id: 12345678-1234-1234-1234-123456789abc
  tenant_id: 87654321-4321-4321-4321-cba987654321
  auto_login: true
```

### Multiple Subscriptions

```yaml
# dev.yaml
name: myproject-dev
azure:
  subscription_id: dev-subscription-id
  tenant_id: company-tenant-id

# prod.yaml
name: myproject-prod
azure:
  subscription_id: prod-subscription-id
  tenant_id: company-tenant-id
```

## Usage

```bash
# Switch context
ctx use myproject-prod

# Azure CLI now uses the correct subscription
az vm list

# All Azure tools pick up the correct subscription
terraform plan  # Uses AZURE_SUBSCRIPTION_ID
```

## Browser Profile Integration

When `auto_login: true` is set and a browser profile is configured, `az login` will open the authentication page in the correct browser profile:

```yaml
azure:
  subscription_id: xxx
  auto_login: true

browser:
  type: chrome
  profile: "Work"
```
