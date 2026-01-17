# Tips & Best Practices

Practical advice for getting the most out of ctx.

## Production Safety

Contexts with `environment: production` or `environment: prod`:

- Display a warning indicator in prompts
- Require confirmation before switching (use `--confirm` to skip)
- Show a warning icon in status displays

```bash
ctx use acme-prod
# ⚠️  Switching to PRODUCTION environment
# Type 'yes' to confirm:
```

## Per-Shell Isolation

Each terminal session maintains its own context independently:

```bash
# Terminal 1
ctx use production
kubectl get pods     # → production cluster

# Terminal 2
ctx use development
kubectl get pods     # → development cluster
```

This means you can work in multiple environments simultaneously without conflicts.

## Credential Isolation

ctx stores credentials separately per context to prevent conflicts:

| Service | Isolation |
|---------|-----------|
| **Vault** | Per-context tokens in system keychain |
| **Bitwarden** | Per-context sessions in system keychain |
| **1Password** | Per-context sessions in system keychain |
| **Azure** | Per-context config in `~/.config/ctx/state/cloud/<context>/azure/` |
| **GCP** | Per-context config in `~/.config/ctx/state/cloud/<context>/gcloud/` |
| **AWS (profile)** | Uses AWS profiles (managed by AWS CLI in `~/.aws/credentials`) |
| **AWS (aws-vault)** | Per-context temp credentials in `~/.config/ctx/state/tokens/<context>.aws` |

This means you can have multiple shells with different contexts, each with their own authenticated sessions - no more 403 errors from token conflicts.

## Auto-Connect Features

Set up automatic connections on context switch to minimize manual steps:

```yaml
vpn:
  auto_connect: true      # Connect VPN automatically

tunnels:
  - name: db
    auto_connect: true    # Start tunnel automatically

aws:
  sso_login: true         # Run AWS SSO login automatically

vault:
  auto_login: true        # Run Vault login automatically
```

## Database Access via Tunnels

Combine tunnels with database configuration for seamless access:

```yaml
tunnels:
  - name: postgres
    local_port: 5432
    remote_host: db.internal
    remote_port: 5432
    auto_connect: true

databases:
  - name: primary
    type: postgres
    host: localhost       # Connect via tunnel
    port: 5432
    database: mydb
    username: app_user
```

Then:

```bash
ctx use mycontext
psql -h localhost -U app_user -d mydb
```

The tunnel is automatically established, and `PGHOST`, `PGPORT`, `PGDATABASE`, and `PGUSER` are set.

## Sharing VPN Across Contexts

If multiple contexts use the same VPN, configure them to not disconnect when switching:

```yaml
# base-vpn-context.yaml
name: base-vpn-context
abstract: true

vpn:
  type: openvpn
  config_file: ~/vpn/company.ovpn
  auto_connect: true

deactivate:
  disconnect_vpn: false    # Keep VPN when switching to another context

# project-a.yaml
name: project-a
extends: base-vpn-context
aws:
  profile: project-a

# project-b.yaml
name: project-b
extends: base-vpn-context
aws:
  profile: project-b
```

Now switching between `project-a` and `project-b` keeps the VPN connected.

## Organizing Contexts with Inheritance

Use abstract base contexts to avoid repetition:

```yaml
# company-base.yaml
name: company-base
abstract: true

ssh:
  bastion:
    host: bastion.company.com
    user: deploy
    identity_file: ~/.ssh/company_key

git:
  user_email: "you@company.com"

urls:
  jira: https://company.atlassian.net
  wiki: https://wiki.company.com
```

Then inherit in specific contexts:

```yaml
# project-prod.yaml
name: project-prod
extends: company-base
environment: production

aws:
  profile: project-prod
```

## Quick URL Access

Define frequently used URLs in your context:

```yaml
urls:
  nomad: http://localhost:4646
  consul: http://localhost:8500
  grafana: https://grafana.company.com
  github: https://github.com/company-org
```

Then open them easily:

```bash
ctx open grafana
```

Uses your configured browser profile for proper SSO session.

## Browser Profile for SSO

Keep SSO sessions isolated by using dedicated browser profiles:

```yaml
browser:
  type: chrome
  profile: "Company Work"
```

All SSO flows (AWS SSO, Vault OIDC, GCP, Azure) will open in the correct profile, keeping your personal browsing separate.

List available profiles:

```bash
ctx browser list
```
