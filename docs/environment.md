# Environment Variables

When you run `ctx use <name>`, these environment variables are set automatically based on your context configuration.

## Context Metadata

| Variable | Description |
|----------|-------------|
| `CTX_CURRENT` | Current context name |
| `CTX_ENVIRONMENT` | Environment type (development, staging, production) |

## AWS

| Variable | Description |
|----------|-------------|
| `AWS_PROFILE` | AWS CLI profile name (when not using aws-vault) |
| `AWS_REGION` | AWS region |
| `AWS_DEFAULT_REGION` | AWS default region |
| `AWS_ACCESS_KEY_ID` | Temporary access key (when using aws-vault) |
| `AWS_SECRET_ACCESS_KEY` | Temporary secret key (when using aws-vault) |
| `AWS_SESSION_TOKEN` | Session token (when using aws-vault) |

See [AWS Provider](cloud/aws.md) for details.

## GCP

| Variable | Description |
|----------|-------------|
| `CLOUDSDK_ACTIVE_CONFIG_NAME` | gcloud configuration name |
| `CLOUDSDK_CORE_PROJECT` | GCP project ID |
| `GOOGLE_CLOUD_PROJECT` | GCP project ID |
| `CLOUDSDK_CONFIG` | Per-context gcloud config directory |

See [GCP Provider](cloud/gcp.md) for details.

## Azure

| Variable | Description |
|----------|-------------|
| `AZURE_CONFIG_DIR` | Per-context Azure CLI config directory |
| `AZURE_SUBSCRIPTION_ID` | Azure subscription ID |

See [Azure Provider](cloud/azure.md) for details.

## Kubernetes

| Variable | Description |
|----------|-------------|
| `KUBECONFIG` | Path to kubeconfig file (when `kubernetes.kubeconfig` is set) |

When you specify a custom kubeconfig path in your context, it's automatically exported so kubectl and other tools use it.

## HashiCorp Tools

| Variable | Description |
|----------|-------------|
| `NOMAD_ADDR` | Nomad server address |
| `NOMAD_NAMESPACE` | Nomad namespace |
| `CONSUL_HTTP_ADDR` | Consul server address |
| `VAULT_ADDR` | Vault server address |
| `VAULT_NAMESPACE` | Vault namespace |

See [HashiCorp Vault](secrets/vault.md) for details.

## Git

| Variable | Description |
|----------|-------------|
| `GIT_AUTHOR_NAME` | Git commit author name |
| `GIT_AUTHOR_EMAIL` | Git commit author email |
| `GIT_COMMITTER_NAME` | Git committer name |
| `GIT_COMMITTER_EMAIL` | Git committer email |

## Databases

### PostgreSQL

| Variable | Description |
|----------|-------------|
| `PGHOST` | PostgreSQL host |
| `PGPORT` | PostgreSQL port |
| `PGDATABASE` | PostgreSQL database name |
| `PGUSER` | PostgreSQL username |

### MySQL

| Variable | Description |
|----------|-------------|
| `MYSQL_HOST` | MySQL host |
| `MYSQL_TCP_PORT` | MySQL port |
| `MYSQL_DATABASE` | MySQL database name |

### Redis

| Variable | Description |
|----------|-------------|
| `REDIS_HOST` | Redis host |
| `REDIS_PORT` | Redis port |

### MongoDB

| Variable | Description |
|----------|-------------|
| `MONGODB_HOST` | MongoDB host |
| `MONGODB_PORT` | MongoDB port |

## Proxy

| Variable | Description |
|----------|-------------|
| `HTTP_PROXY` | HTTP proxy URL |
| `http_proxy` | HTTP proxy URL (lowercase) |
| `HTTPS_PROXY` | HTTPS proxy URL |
| `https_proxy` | HTTPS proxy URL (lowercase) |
| `NO_PROXY` | Proxy bypass list |
| `no_proxy` | Proxy bypass list (lowercase) |

Both uppercase and lowercase variants are set for maximum compatibility.

See [Proxy Configuration](features/proxy.md) for details.

## Custom Environment Variables

Any variables defined in the `env:` section are set directly:

```yaml
env:
  ENVIRONMENT: production
  PROJECT: myproject
  LOG_LEVEL: info
  MY_CUSTOM_VAR: custom_value
```

## Secrets

Secrets fetched from providers are injected as environment variables:

```yaml
secrets:
  bitwarden:
    DB_PASSWORD: "prod-database"      # Sets DB_PASSWORD
  aws_secrets_manager:
    API_KEY: "prod/api-keys#stripe"   # Sets API_KEY
```

See [Secrets Management](secrets/index.md) for details.

## Variable Precedence

When the same variable is set by multiple sources, the order of precedence (highest to lowest):

1. Secrets (from password managers / cloud secret services)
2. Custom `env:` section
3. Provider-specific variables (AWS, GCP, etc.)
4. Context metadata (CTX_CURRENT, CTX_ENVIRONMENT)

## Checking Variables

After switching context, check what's set:

```bash
# Check specific variable
echo $AWS_PROFILE

# Check all ctx-related variables
env | grep -E '^(CTX_|AWS_|GOOGLE_|AZURE_|VAULT_|NOMAD_|CONSUL_)'

# Check proxy variables
env | grep -i proxy
```
