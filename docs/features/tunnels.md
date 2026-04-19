# Tunnels

ctx manages tunnels per context with automatic connection and health monitoring. Two transport types are supported: **SSH** (via a bastion host) and **AWS SSM** (via Session Manager, no SSH port required).

## SSH Tunnels

SSH tunnels use a bastion host to forward ports to resources inside a private network.

### Configuration

```yaml
ssh:
  bastion:
    host: bastion.myproject.com
    port: 22
    user: admin
    identity_file: ~/.ssh/myproject_ed25519

tunnels:
  - name: postgres
    description: "Production Database"
    local_port: 5432
    remote_host: db.internal
    remote_port: 5432
    auto_connect: true            # Start automatically on context switch

  - name: redis
    description: "Redis Cache"
    local_port: 6379
    remote_host: redis.internal
    remote_port: 6379

  - name: nomad
    description: "Nomad UI"
    local_port: 4646
    remote_host: 10.0.1.10
    remote_port: 4646
```

### Commands

```bash
ctx tunnel list                  # List defined tunnels for current context
ctx tunnel up                    # Start all tunnels
ctx tunnel up <name>             # Start specific tunnel
ctx tunnel down                  # Stop all tunnels
ctx tunnel down <name>           # Stop specific tunnel
ctx tunnel status                # Show running tunnel status
```

### Bastion Configuration

The `ssh.bastion` section defines the jump host used for all tunnels:

```yaml
ssh:
  bastion:
    host: bastion.example.com    # Bastion hostname
    port: 22                     # SSH port (default: 22)
    user: deploy                 # SSH username (default: current OS user)
    identity_file: ~/.ssh/id_ed25519  # Private key path
  tunnel_timeout: 5              # Seconds to wait for connection (default: 5)
```

### Tunnel Options

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | **Required.** Tunnel identifier |
| `description` | string | Human-readable description |
| `local_port` | int | **Required.** Local port to bind |
| `remote_host` | string | **Required.** Remote host to tunnel to |
| `remote_port` | int | **Required.** Remote port |
| `auto_connect` | bool | Start automatically on context switch |

### Auto-Connect

When `auto_connect: true` on a tunnel:

- Tunnel starts automatically when you run `ctx use <context>`
- Tunnel starts after VPN connection (if configured)
- Tunnel stops when you switch contexts or run `ctx deactivate`

### Database Access via Tunnels

A common pattern is to combine tunnels with database configuration:

```yaml
ssh:
  bastion:
    host: bastion.example.com
    user: deploy
    identity_file: ~/.ssh/deploy_key

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
    database: production
    username: app_user
```

Then:

```bash
ctx use myproject-prod
# Tunnel auto-connects

psql -h localhost -U app_user -d production
# Connected via tunnel to db.internal
```

### Multiple Tunnels

You can define multiple tunnels per context:

```yaml
tunnels:
  - name: postgres
    local_port: 5432
    remote_host: db.internal
    remote_port: 5432
    auto_connect: true

  - name: redis
    local_port: 6379
    remote_host: redis.internal
    remote_port: 6379
    auto_connect: true

  - name: elasticsearch
    local_port: 9200
    remote_host: es.internal
    remote_port: 9200
    # Not auto-connect - start manually when needed
```

Start specific tunnels:

```bash
ctx tunnel up postgres           # Start just postgres
ctx tunnel up                    # Start all tunnels
ctx tunnel down elasticsearch    # Stop just elasticsearch
```

### Tunnel Status

Check which tunnels are running:

```bash
ctx tunnel status
```

Output shows:

- Tunnel name and description
- Local and remote endpoints
- Connection status (running/stopped)
- Process ID if running

### Health Monitoring

ctx monitors tunnel health and automatically reconnects if:

- The SSH connection drops
- The bastion host becomes unreachable
- The tunnel process crashes

### Inheritance

Tunnels are replaced (not merged) when using context inheritance. If a child context defines any tunnels, it completely replaces the parent's tunnel list.

```yaml
# base.yaml
tunnels:
  - name: db
    local_port: 5432
    remote_host: db.internal
    remote_port: 5432

# child.yaml
extends: base
tunnels:
  - name: cache
    local_port: 6379
    remote_host: redis.internal
    remote_port: 6379
# Only 'cache' tunnel exists - 'db' is NOT inherited
```

To include parent tunnels, you must redefine them in the child context.

---

## AWS SSM Tunnels

SSM tunnels use [AWS Session Manager](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager.html) to forward ports to resources inside a VPC — no open SSH port, no bastion key, just IAM credentials.

### Requirements

**On the EC2 bastion instance:**

- SSM Agent installed and running
- IAM instance profile with `AmazonSSMManagedInstanceCore` policy
- No inbound rules required in the security group (only HTTPS outbound to SSM endpoints)

**On your local machine (ctx checks these before starting any SSM tunnel):**

- [AWS CLI v2](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html) — `aws --version` must return `aws-cli/2.x`
- [session-manager-plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html) — must be in PATH

### Configuration

SSM tunnels live under `aws.tunnels`, separate from SSH tunnels:

```yaml
aws:
  profile: myproject-prod
  region: eu-west-1
  sso_login: true
  tunnels:
    - name: postgres
      ssm_target: i-0abc123def456789a   # EC2 instance ID of the bastion
      remote_host: db.internal.vpc
      remote_port: 5432
      local_port: 5432
      auto_connect: true

    - name: redis
      ssm_target: i-0abc123def456789a
      remote_host: cache.internal.vpc
      remote_port: 6379
      local_port: 6379
```

!!! note
    SSH tunnels (under `tunnels:`) and SSM tunnels (under `aws.tunnels:`) are independent. Both can coexist in the same context.

### Tunnel Options

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | **Required.** Tunnel identifier |
| `description` | string | Human-readable description |
| `ssm_target` | string | **Required.** EC2 instance ID (`i-xxxxxxxxxxxxxxxxx`) |
| `remote_host` | string | **Required.** Remote host inside the VPC |
| `remote_port` | int | **Required.** Remote port |
| `local_port` | int | **Required.** Local port to bind |
| `auto_connect` | bool | Start automatically on context switch |

### Commands

The same tunnel commands work for both SSH and SSM tunnels:

```bash
ctx tunnel list                  # Shows both SSH and SSM tunnels (TYPE column)
ctx tunnel up                    # Start all tunnels (SSH + SSM)
ctx tunnel up postgres           # Start specific SSM tunnel by name
ctx tunnel down                  # Stop all tunnels
ctx tunnel status                # Shows TYPE column: ssh / ssm
```

### Authentication

SSM tunnels inherit the AWS authentication configured in the `aws:` section. Credentials are injected explicitly into the subprocess environment — they work correctly even during `auto_connect` (before the shell hook sources the env file).

| Auth method | How it works |
|-------------|--------------|
| Named profile (`profile`) | `AWS_PROFILE` injected |
| aws-vault (`use_vault: true`) | Temporary credentials injected from cache |
| SSO (`sso_login: true`) | Profile set after SSO login completes |

### Pre-connection Check

When starting a tunnel interactively (`ctx tunnel up`), ctx calls `aws ssm describe-instance-information` to verify the target instance is registered in SSM before any connection attempt. If it is not, you will see a clear error:

```
instance "i-0abc123def456789a" is not registered in SSM — verify the SSM Agent
is running and the instance profile has AmazonSSMManagedInstanceCore
```

### Mixed Context Example

SSH and SSM tunnels in the same context:

```yaml
name: myproject-prod
environment: production

aws:
  profile: myproject-prod
  region: eu-west-1
  tunnels:
    - name: postgres
      ssm_target: i-0abc123def456789a
      remote_host: db.internal.vpc
      remote_port: 5432
      local_port: 5432
      auto_connect: true

# Legacy SSH tunnel to a separate service
ssh:
  bastion:
    host: legacy-bastion.example.com
    user: deploy
    identity_file: ~/.ssh/deploy_key

tunnels:
  - name: legacy-api
    remote_host: api.internal
    remote_port: 8080
    local_port: 8080
```
