# 1Password

Fetch secrets from 1Password and inject them as environment variables.

## Configuration

```yaml
onepassword:
  account: my.1password.com       # Account URL or shorthand (optional)
  auto_login: true                # Auto-run 'op signin' if not authenticated
  sso: false                      # Use SSO login

secrets:
  onepassword:
    API_KEY: "api-credentials"              # Item name (auto-detect field)
    DB_PASSWORD: "database-prod#password"   # Specific field
    USERNAME: "database-prod#username"      # Another field from same item
```

## Initial Setup

The 1Password CLI requires a one-time account setup before ctx can use it.

### Option 1: Desktop App Integration (Recommended)

If you have the 1Password desktop app, enable CLI integration in app settings. The CLI will use the app's authentication automatically - no manual setup needed.

### Option 2: Manual Account Setup

```bash
op account add
# Follow prompts for: account URL, email, secret key, master password
```

After setup, ctx will handle authentication automatically.

## How It Works

1. On `ctx use`, checks if 1Password CLI is authenticated
2. Tries to use saved session from keychain
3. If no valid session and `auto_login: true`, runs `op signin`
4. Session is saved to system keychain for future use
5. Fetches each secret item and extracts the appropriate field

## Field Syntax

Use `item-name#field` to specify which field to extract:

```yaml
secrets:
  onepassword:
    PASSWORD: "my-login"              # Auto: password → credential → notes
    PASSWORD: "my-login#password"     # Explicit password field
    USERNAME: "my-login#username"     # Get username field
    CUSTOM: "my-item#my_custom_field" # Any custom field
```

To see available fields in an item:

```bash
op item get "item-name" --format=json | jq '.fields[].label'
```

## Field Priority

When fetching a 1Password item without a specific field, ctx tries:

1. `password`
2. `credential` (for API keys)
3. `notesPlain` (fallback)

## CLI Limitations

The 1Password CLI can only have **one account active at a time**. If you switch between contexts that use different 1Password accounts, you'll need to re-authenticate.

Sessions are saved to your system keychain to minimize re-authentication.

## Auto-Login Behavior

When `auto_login: true`:

| Setting | Behavior |
|---------|----------|
| `sso: false` | Runs `op signin` (biometric/password) |
| `sso: true` | Runs `op signin --sso` (opens browser) |

!!! tip
    1Password with biometric unlock works seamlessly - no manual session management needed.

## Manual Authentication

If `auto_login: false`, authenticate manually:

```bash
eval $(op signin)
# or with specific account:
eval $(op signin --account my.1password.com)
```

## Multiple Accounts

If you have multiple 1Password accounts, specify which one to use:

```yaml
onepassword:
  account: work.1password.com
  auto_login: true

secrets:
  onepassword:
    WORK_API_KEY: "work-api-creds"
```

## Session Storage

Sessions are stored securely in your system keychain:

- **macOS**: Keychain
- **Linux**: Secret Service API (gnome-keyring, kwallet)
- **Windows**: Windows Credential Manager

Sessions are cleared on `ctx logout <context>`.
