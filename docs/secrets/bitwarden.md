# Bitwarden

Fetch secrets from Bitwarden and inject them as environment variables.

## Configuration

```yaml
bitwarden:
  server: https://bw.example.com  # Self-hosted server URL (optional)
  email: user@example.com         # Pre-fill email for login (optional)
  auto_login: true                # Auto-run 'bw login' if not authenticated
  sso: false                      # Use SSO login (requires org SSO identifier)

secrets:
  bitwarden:
    DATABASE_PASSWORD: "prod-database-creds"    # Item name (auto-detect field)
    DB_USER: "prod-database-creds#username"     # Specific field
    CUSTOM_VALUE: "my-item#custom_field_name"   # Custom field
```

## How It Works

1. On `ctx use`, checks if Bitwarden is authenticated and unlocked
2. Tries to use saved session from keychain
3. If no valid session and `auto_login: true`, runs `bw login`
4. If vault is locked, runs `bw unlock` to get session token
5. Session is saved to system keychain for future use
6. Fetches each secret item and extracts the appropriate field

## Field Syntax

Use `item-name#field` to specify which field to extract:

```yaml
secrets:
  bitwarden:
    PASSWORD: "my-login"              # Auto: password → notes
    PASSWORD: "my-login#password"     # Explicit password field
    USERNAME: "my-login#username"     # Get username field
    NOTES: "my-item#notes"            # Get notes
    CUSTOM: "my-item#my_custom_field" # Any custom field
```

## Field Priority

When fetching a Bitwarden item without a specific field, ctx tries:

1. `password` (from login items)
2. `notes` (fallback for secure notes)

## CLI Limitations

The Bitwarden CLI can only be connected to **one server at a time**. If you switch between contexts that use different Bitwarden servers (e.g., self-hosted vs cloud), ctx will automatically:

1. Log you out of the current server
2. Configure the new server URL
3. Prompt for authentication

Sessions are saved to your system keychain to minimize re-authentication when switching back.

## Auto-Login Behavior

When `auto_login: true`:

| Setting | Behavior |
|---------|----------|
| `sso: false` | Runs `bw login` with email/password prompt |
| `sso: true` | Runs `bw login --sso` (opens browser for SSO) |

After login, ctx automatically runs `bw unlock` to get the session token.

## Manual Authentication

If `auto_login: false`, authenticate manually:

```bash
# Email/password login
bw login user@example.com
export BW_SESSION=$(bw unlock --raw)

# SSO login (if your org uses it)
bw login --sso
export BW_SESSION=$(bw unlock --raw)
```

## Self-Hosted Servers

For self-hosted Bitwarden/Vaultwarden:

```yaml
bitwarden:
  server: https://bw.example.com
  email: user@example.com
  auto_login: true
```

!!! note
    Switching between servers requires re-authentication. ctx handles this automatically but you'll need to enter your password again.

## Session Storage

Sessions are stored securely in your system keychain:

- **macOS**: Keychain
- **Linux**: Secret Service API (gnome-keyring, kwallet)
- **Windows**: Windows Credential Manager

Sessions are cleared on `ctx logout <context>`.
