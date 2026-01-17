# VPN

ctx can automatically connect and disconnect VPN when switching contexts. Supports OpenVPN, WireGuard, Tailscale, and custom VPN solutions.

## Configuration

```yaml
vpn:
  type: openvpn                   # openvpn, wireguard, tailscale, custom
  config_file: ~/vpn/myproject.ovpn
  auto_connect: true              # Connect automatically on context switch
  auto_disconnect: true           # Disconnect when switching away
```

## Supported VPN Types

### OpenVPN

```yaml
vpn:
  type: openvpn
  config_file: ~/vpn/project.ovpn
  auth_user_pass: ~/vpn/auth.txt    # Optional credentials file
  auto_connect: true
```

The `auth_user_pass` file should contain username on the first line and password on the second line.

### WireGuard

```yaml
vpn:
  type: wireguard
  interface: wg0
  auto_connect: true
```

Uses `wg-quick up/down` to manage the interface.

### Tailscale

```yaml
vpn:
  type: tailscale
  exit_node: us-west-1              # Optional exit node
  auto_connect: true
```

### Custom VPN

For any VPN that isn't directly supported, use custom commands:

```yaml
vpn:
  type: custom
  connect_cmd: "sudo vpn-connect --profile myprofile"
  disconnect_cmd: "sudo vpn-disconnect"
  status_cmd: "vpn-status"
  auto_connect: true
```

## Commands

```bash
ctx vpn connect                  # Connect VPN for current context
ctx vpn disconnect               # Disconnect VPN
ctx vpn status                   # Show VPN connection status
```

## Auto-Connect Behavior

When `auto_connect: true`:

- VPN connects automatically when you run `ctx use <context>`
- Connection happens after environment variables are set but before tunnels start

When `auto_disconnect: true`:

- VPN disconnects when you switch to a different context
- VPN disconnects when you run `ctx deactivate`

## Shared VPN Contexts

If multiple contexts share the same VPN, you can prevent disconnection when switching between them:

```yaml
# context-a.yaml
name: context-a
vpn:
  type: wireguard
  interface: wg0
  auto_connect: true

deactivate:
  disconnect_vpn: false          # Keep VPN when switching away

# context-b.yaml
name: context-b
vpn:
  type: wireguard
  interface: wg0
  auto_connect: true

deactivate:
  disconnect_vpn: false          # Keep VPN when switching away
```

Now switching between `context-a` and `context-b` keeps the VPN connected.

## Example: Corporate VPN

```yaml
name: corp-prod
environment: production

vpn:
  type: openvpn
  config_file: ~/.config/ctx/vpn/corporate.ovpn
  auth_user_pass: ~/.config/ctx/vpn/corp-auth.txt
  auto_connect: true
  auto_disconnect: true

# Once connected, access internal services
tunnels:
  - name: database
    local_port: 5432
    remote_host: db.internal.corp.com
    remote_port: 5432
    auto_connect: true
```

## Example: Tailscale Exit Node

```yaml
name: geo-restricted
vpn:
  type: tailscale
  exit_node: us-east-1
  auto_connect: true
```
