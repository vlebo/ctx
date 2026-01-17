# Secrets Management

ctx can fetch secrets from password managers and cloud secret services, injecting them as environment variables on context switch.

## Supported Providers

| Provider | Type | CLI | Install |
|----------|------|-----|---------|
| [Bitwarden](bitwarden.md) | Password Manager | `bw` | [bitwarden.com/help/cli](https://bitwarden.com/help/cli/) |
| [1Password](onepassword.md) | Password Manager | `op` | [developer.1password.com/docs/cli](https://developer.1password.com/docs/cli/) |
| [HashiCorp Vault](vault.md) | Secrets Engine | `vault` | [developer.hashicorp.com/vault](https://developer.hashicorp.com/vault/downloads) |
| [AWS Secrets Manager](cloud.md#aws-secrets-manager) | Cloud | `aws` | Uses `aws:` config |
| [AWS Parameter Store](cloud.md#aws-parameter-store-ssm) | Cloud | `aws` | Uses `aws:` config |
| [GCP Secret Manager](cloud.md#gcp-secret-manager) | Cloud | `gcloud` | Uses `gcp:` config |

## Configuration Overview

Each provider has its own config section for authentication, and a unified `secrets:` section for what to fetch:

```yaml
# Provider Authentication
bitwarden:
  auto_login: true

onepassword:
  auto_login: true
  account: "my.1password.com"

vault:
  address: https://vault.example.com
  auth_method: oidc
  auto_login: true

# Cloud providers use existing auth from aws:/gcp: sections

# What secrets to fetch (injected as env vars)
secrets:
  bitwarden:
    DB_PASSWORD: "prod-database"
  onepassword:
    API_KEY: "api-credentials"
  vault:
    SECRET_TOKEN: "databases/prod#password"
  aws_secrets_manager:
    STRIPE_KEY: "prod/stripe#secret_key"
  aws_ssm:
    DATABASE_URL: "/prod/myproject/database_url"
  gcp_secret_manager:
    SERVICE_ACCOUNT: "myproject-sa-key"
```

## How It Works

1. On `ctx use`, ctx checks if you're authenticated with each configured provider
2. If `auto_login: true` and not authenticated, runs the login command automatically
3. Fetches each item and extracts the appropriate field
4. Injects values as environment variables

## Field Syntax

All providers support the `item#field` syntax to specify which field to extract:

```yaml
secrets:
  bitwarden:
    PASSWORD: "my-login"           # Auto-detect field
    PASSWORD: "my-login#password"  # Explicit field
  vault:
    DB_PASS: "databases/prod#password"
  aws_secrets_manager:
    API_KEY: "prod/keys#stripe"    # JSON key extraction
```

## Field Priority

When no `#field` is specified, each provider has a default priority:

| Provider | Field Priority |
|----------|----------------|
| **Bitwarden** | `password` → `notes` |
| **1Password** | `password` → `credential` → `notesPlain` |
| **Vault** | Specified field, defaults to `value` |
| **AWS Secrets Manager** | Full string, or JSON key if `#key` specified |
| **AWS SSM** | Parameter value (auto-decrypted) |
| **GCP Secret Manager** | Secret payload data |

## Session Storage

ctx stores authentication sessions securely in your system keychain:

| Platform | Storage |
|----------|---------|
| **macOS** | Keychain |
| **Linux** | Secret Service API (gnome-keyring, kwallet) |
| **Windows** | Windows Credential Manager |

Sessions are cleared on `ctx logout <context>`.

## Multiple Providers

You can use multiple providers in the same context:

```yaml
bitwarden:
  auto_login: true

aws:
  profile: myproject
  region: us-east-1

secrets:
  # Personal credentials from Bitwarden
  bitwarden:
    GPG_PASSPHRASE: "gpg-key"

  # Application secrets from AWS
  aws_secrets_manager:
    DB_PASSWORD: "prod/database#password"

  aws_ssm:
    FEATURE_FLAGS: "/prod/app/features"
```
