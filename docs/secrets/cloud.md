# Cloud Secret Managers

Fetch secrets from AWS Secrets Manager, AWS Parameter Store (SSM), and GCP Secret Manager.

Cloud secret managers use your existing cloud authentication from the `aws:` and `gcp:` config sections - no additional setup required.

## AWS Secrets Manager

Store and retrieve secrets from AWS Secrets Manager.

### Configuration

```yaml
aws:
  profile: myprofile
  region: us-east-1

secrets:
  aws_secrets_manager:
    DB_PASSWORD: "prod/database"           # Secret name
    API_KEY: "prod/api-keys#stripe_key"    # JSON secret with key extraction
```

### Path Formats

| Format | Description |
|--------|-------------|
| `my-secret` | Returns the full secret string |
| `my-secret#api_key` | Parses as JSON, returns `api_key` field |

### JSON Secrets

If your secret contains JSON:

```json
{
  "username": "admin",
  "password": "secret123",
  "host": "db.example.com"
}
```

Extract specific fields:

```yaml
secrets:
  aws_secrets_manager:
    DB_USER: "prod/database#username"
    DB_PASS: "prod/database#password"
    DB_HOST: "prod/database#host"
```

---

## AWS Parameter Store (SSM)

Fetch parameters from AWS Systems Manager Parameter Store.

### Configuration

```yaml
aws:
  profile: myprofile
  region: us-east-1

secrets:
  aws_ssm:
    DATABASE_URL: "/prod/myproject/database_url"
    CONFIG_VALUE: "/prod/app/config"
    API_KEY: "/prod/api/key"
```

### Features

- Parameters are **automatically decrypted** if they're SecureString type
- Uses the same AWS credentials as the `aws:` config section
- Full parameter path required (including leading `/`)

### Hierarchical Parameters

Organize parameters by environment:

```yaml
# dev context
secrets:
  aws_ssm:
    DATABASE_URL: "/dev/myapp/database_url"

# prod context
secrets:
  aws_ssm:
    DATABASE_URL: "/prod/myapp/database_url"
```

---

## GCP Secret Manager

Fetch secrets from Google Cloud Secret Manager.

### Configuration

```yaml
gcp:
  project: myproject-123

secrets:
  gcp_secret_manager:
    SERVICE_KEY: "my-secret"                                    # Uses latest version
    OLD_KEY: "projects/myproject/secrets/my-secret/versions/2"  # Specific version
```

### Path Formats

| Format | Description |
|--------|-------------|
| `my-secret` | Uses latest version from current project |
| `projects/PROJECT/secrets/NAME/versions/N` | Full resource path with specific version |

### Version Control

Access specific versions when needed:

```yaml
secrets:
  gcp_secret_manager:
    # Latest version (recommended for most cases)
    CURRENT_KEY: "api-key"

    # Specific version (for rollback or testing)
    OLD_KEY: "projects/myproject-123/secrets/api-key/versions/2"
```

---

## Authentication

Cloud secret managers use your existing cloud authentication:

| Provider | Authentication Source |
|----------|----------------------|
| **AWS Secrets Manager** | `aws:` config (profile, SSO, or aws-vault) |
| **AWS SSM** | `aws:` config (profile, SSO, or aws-vault) |
| **GCP Secret Manager** | `gcp:` config (project, service account) |

No additional authentication configuration is needed - just ensure your cloud provider is configured correctly.

### Example: Full AWS Setup

```yaml
aws:
  profile: myproject-prod
  region: us-east-1
  sso_login: true              # Auto-login via SSO

secrets:
  aws_secrets_manager:
    DB_PASSWORD: "prod/database#password"

  aws_ssm:
    FEATURE_FLAGS: "/prod/app/features"
```

### Example: Full GCP Setup

```yaml
gcp:
  project: myproject-prod-123
  auto_login: true

secrets:
  gcp_secret_manager:
    SERVICE_ACCOUNT_KEY: "sa-key"
    API_TOKEN: "external-api-token"
```
