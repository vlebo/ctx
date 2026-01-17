# AWS

ctx supports three authentication methods for AWS, plus integration with AWS Secrets Manager and Parameter Store.

## Configuration

```yaml
aws:
  profile: myproject-prod
  region: us-east-1
  sso_login: true                 # Auto-run 'aws sso login' on context switch
  # OR use aws-vault:
  # use_vault: true               # Get temporary credentials via aws-vault
```

## Authentication Methods

### Standard Profile (Default)

The simplest option - just set the profile name and ctx will set `AWS_PROFILE` so AWS CLI/SDK reads credentials from `~/.aws/credentials`:

```yaml
aws:
  profile: myproject
  region: us-east-1
```

This sets `AWS_PROFILE=myproject` and relies on your existing `~/.aws/credentials` file:

```ini
[myproject]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

### AWS SSO

For accounts using AWS IAM Identity Center (SSO):

```yaml
aws:
  profile: myproject-prod
  region: us-east-1
  sso_login: true                 # Auto-run 'aws sso login' on context switch
```

When you run `ctx use`, it will:

1. Run `aws sso login --profile myproject-prod`
2. Open the configured browser profile for SSO
3. Set `AWS_PROFILE` environment variable

### aws-vault (Enhanced Security)

For accounts using access keys where you want better security, use [aws-vault](https://github.com/99designs/aws-vault):

```yaml
aws:
  profile: myproject-prod
  region: us-east-1
  use_vault: true                 # Get temporary credentials via aws-vault
```

When you run `ctx use`, it will:

1. Check for cached temporary credentials
2. If expired/missing, run `aws-vault exec myproject-prod --json`
3. Cache the temporary credentials locally
4. Set `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_SESSION_TOKEN`

**Benefits of aws-vault:**

- Master credentials stored encrypted (not in plaintext)
- Temporary credentials are automatically rotated
- Cached credentials are reused until they expire
- Credentials are context-specific (different contexts = different credentials)

**Requirements:**

- [aws-vault](https://github.com/99designs/aws-vault) must be installed
- Profile must be configured in aws-vault: `aws-vault add myproject-prod`

## Environment Variables Set

| Variable | Description |
|----------|-------------|
| `AWS_PROFILE` | AWS CLI profile name (when not using aws-vault) |
| `AWS_REGION` | AWS region |
| `AWS_DEFAULT_REGION` | AWS default region |
| `AWS_ACCESS_KEY_ID` | Temporary access key (when using aws-vault) |
| `AWS_SECRET_ACCESS_KEY` | Temporary secret key (when using aws-vault) |
| `AWS_SESSION_TOKEN` | Session token (when using aws-vault) |

## AWS Secrets Manager

Fetch secrets from AWS Secrets Manager and inject as environment variables:

```yaml
aws:
  profile: myproject
  region: us-east-1

secrets:
  aws_secrets_manager:
    DB_PASSWORD: "prod/database"           # Secret name
    API_KEY: "prod/api-keys#stripe"        # Secret with JSON key
```

### Path Formats

- `my-secret` - returns full secret string
- `my-secret#api_key` - parses secret as JSON, returns `api_key` field

### Example

If your secret `prod/api-keys` contains:

```json
{
  "stripe": "sk_live_xxx",
  "sendgrid": "SG.xxx"
}
```

Then `API_KEY: "prod/api-keys#stripe"` sets `API_KEY=sk_live_xxx`.

## AWS Parameter Store (SSM)

Fetch parameters from AWS Systems Manager Parameter Store:

```yaml
aws:
  profile: myproject
  region: us-east-1

secrets:
  aws_ssm:
    DATABASE_URL: "/prod/myproject/database_url"
    CONFIG_VALUE: "/prod/app/config"
```

Parameters are automatically decrypted if they're SecureString type.

## Credential Isolation

When using aws-vault, ctx stores temporary credentials per-context:

| Storage | Location |
|---------|----------|
| **AWS (profile)** | Uses AWS profiles (managed by AWS CLI in `~/.aws/credentials`) |
| **AWS (aws-vault)** | Per-context temp credentials in `~/.config/ctx/state/tokens/<context>.aws` |

This means you can have multiple shells with different AWS contexts active simultaneously.
