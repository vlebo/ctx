# GCP

ctx manages Google Cloud Platform configurations per context, with support for auto-login and GCP Secret Manager integration.

## Configuration

```yaml
gcp:
  project: myproject-prod-123456
  region: us-central1
  config_name: myproject-prod     # gcloud config name
  auto_login: true                # Auto-run 'gcloud auth login'
```

## How It Works

When you run `ctx use <context>` with GCP configured:

1. Sets `CLOUDSDK_ACTIVE_CONFIG_NAME` to your config name
2. Sets `CLOUDSDK_CORE_PROJECT` and `GOOGLE_CLOUD_PROJECT` to your project
3. Creates a per-context gcloud config directory for credential isolation
4. If `auto_login: true` and not authenticated, runs `gcloud auth login`
5. Opens the configured browser profile for OAuth

## Environment Variables Set

| Variable | Description |
|----------|-------------|
| `CLOUDSDK_ACTIVE_CONFIG_NAME` | gcloud configuration name |
| `CLOUDSDK_CORE_PROJECT` | GCP project ID |
| `GOOGLE_CLOUD_PROJECT` | GCP project ID |
| `CLOUDSDK_CONFIG` | Per-context gcloud config directory |

## GCP Secret Manager

Fetch secrets from GCP Secret Manager and inject as environment variables:

```yaml
gcp:
  project: myproject-123
  region: us-central1

secrets:
  gcp_secret_manager:
    SERVICE_KEY: "my-service-account"      # Secret name (uses latest version)
    API_TOKEN: "api-token"                 # Another secret
```

### Path Formats

- `my-secret` - uses latest version from current project
- `projects/my-project/secrets/my-secret/versions/2` - full resource path with specific version

### Example

```yaml
gcp:
  project: acme-prod-123456

secrets:
  gcp_secret_manager:
    # Simple - uses latest version from acme-prod-123456
    DB_PASSWORD: "database-password"

    # Full path - specific project and version
    LEGACY_KEY: "projects/acme-legacy/secrets/api-key/versions/1"
```

## Credential Isolation

ctx stores GCP credentials per-context to prevent conflicts:

| Storage | Location |
|---------|----------|
| **GCP config** | `~/.config/ctx/state/cloud/<context>/gcloud/` |

This allows you to have multiple shells authenticated to different GCP projects simultaneously.

## Multiple Configurations

You can have different gcloud configurations for different contexts:

```yaml
# dev.yaml
gcp:
  project: myproject-dev-123
  config_name: myproject-dev

# prod.yaml
gcp:
  project: myproject-prod-456
  config_name: myproject-prod
```

## Usage

```bash
# Switch context
ctx use myproject-prod

# gcloud now uses the correct project
gcloud compute instances list

# All GCP tools pick up the correct project
terraform plan  # Uses GOOGLE_CLOUD_PROJECT
```
