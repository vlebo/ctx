# SSH Tunnels

ctx manages SSH tunnels per context with automatic connection, health monitoring, and reconnection.

## Configuration

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

## Commands

```bash
ctx tunnel list                  # List defined tunnels for current context
ctx tunnel up                    # Start all tunnels
ctx tunnel up <name>             # Start specific tunnel
ctx tunnel down                  # Stop all tunnels
ctx tunnel down <name>           # Stop specific tunnel
ctx tunnel status                # Show running tunnel status
```

## Bastion Configuration

The `ssh.bastion` section defines the jump host used for all tunnels:

```yaml
ssh:
  bastion:
    host: bastion.example.com    # Bastion hostname
    port: 22                     # SSH port (default: 22)
    user: deploy                 # SSH username (default: current OS user)
    identity_file: ~/.ssh/id_ed25519  # Private key path
```

## Tunnel Options

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | **Required.** Tunnel identifier |
| `description` | string | Human-readable description |
| `local_port` | int | **Required.** Local port to bind |
| `remote_host` | string | **Required.** Remote host to tunnel to |
| `remote_port` | int | **Required.** Remote port |
| `auto_connect` | bool | Start automatically on context switch |

## Auto-Connect

When `auto_connect: true` on a tunnel:

- Tunnel starts automatically when you run `ctx use <context>`
- Tunnel starts after VPN connection (if configured)
- Tunnel stops when you switch contexts or run `ctx deactivate`

## Database Access via Tunnels

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

## Multiple Tunnels

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

## Tunnel Status

Check which tunnels are running:

```bash
ctx tunnel status
```

Output shows:

- Tunnel name and description
- Local and remote endpoints
- Connection status (running/stopped)
- Process ID if running

## Health Monitoring

ctx monitors tunnel health and automatically reconnects if:

- The SSH connection drops
- The bastion host becomes unreachable
- The tunnel process crashes

## Inheritance

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
