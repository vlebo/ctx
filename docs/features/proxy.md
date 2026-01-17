# Proxy Configuration

ctx can configure HTTP/HTTPS proxy settings per context. This is useful when different environments require different proxy servers.

## Configuration

```yaml
proxy:
  http: http://proxy.example.com:8080
  https: http://proxy.example.com:8080
  no_proxy: localhost,127.0.0.1,.internal,.example.com
```

## Environment Variables Set

When you switch to a context with proxy configured, both uppercase and lowercase variants are set for maximum compatibility:

```bash
HTTP_PROXY=http://proxy.example.com:8080
http_proxy=http://proxy.example.com:8080
HTTPS_PROXY=http://proxy.example.com:8080
https_proxy=http://proxy.example.com:8080
NO_PROXY=localhost,127.0.0.1,.internal,.example.com
no_proxy=localhost,127.0.0.1,.internal,.example.com
```

## Testing Proxy Configuration

To verify proxy settings are working:

```bash
# 1. Switch to context with proxy
ctx use myproject-prod

# 2. Check environment variables
echo $HTTP_PROXY
echo $HTTPS_PROXY
echo $NO_PROXY

# 3. Test with curl (shows proxy being used)
curl -v https://example.com 2>&1 | grep -i proxy

# 4. Test that NO_PROXY works (should bypass proxy)
curl -v https://localhost:8080 2>&1 | grep -i proxy
```

## Common Use Cases

### Corporate Proxy

```yaml
proxy:
  http: http://corporate-proxy.internal:3128
  https: http://corporate-proxy.internal:3128
  no_proxy: localhost,127.0.0.1,.internal.company.com
```

### Authenticated Proxy

```yaml
proxy:
  http: http://user:password@proxy.example.com:8080
  https: http://user:password@proxy.example.com:8080
```

!!! warning
    Storing passwords in config files is not recommended. Consider using environment variable references or a secrets manager for proxy credentials.

### Different Proxies Per Environment

```yaml
# dev.yaml - no proxy
name: dev
# (just don't include proxy section)

# staging.yaml - staging proxy
name: staging
proxy:
  http: http://staging-proxy:8080
  https: http://staging-proxy:8080
  no_proxy: localhost,127.0.0.1,.staging.internal

# prod.yaml - production proxy
name: prod
proxy:
  http: http://prod-proxy:8080
  https: http://prod-proxy:8080
  no_proxy: localhost,127.0.0.1,.prod.internal
```

### SOCKS Proxy

Some tools support SOCKS proxies via the `ALL_PROXY` variable. ctx doesn't set this automatically, but you can use the `env` section:

```yaml
proxy:
  http: http://proxy.example.com:8080
  https: http://proxy.example.com:8080

env:
  ALL_PROXY: socks5://proxy.example.com:1080
  all_proxy: socks5://proxy.example.com:1080
```

## NO_PROXY Format

The `no_proxy` field accepts a comma-separated list of:

- Hostnames: `localhost`
- IP addresses: `127.0.0.1`
- Domain suffixes: `.internal` (matches `foo.internal`, `bar.internal`)
- CIDR notation: `192.168.0.0/16` (tool-dependent)

Example:

```yaml
proxy:
  no_proxy: localhost,127.0.0.1,.internal,.company.com,192.168.0.0/16
```

## Inheritance

Proxy settings are merged during inheritance. A child context can override specific fields:

```yaml
# base.yaml
proxy:
  http: http://proxy.example.com:8080
  https: http://proxy.example.com:8080
  no_proxy: localhost,127.0.0.1

# child.yaml
extends: base
proxy:
  no_proxy: localhost,127.0.0.1,.internal  # Extends no_proxy list
```

## Clearing Proxy

To explicitly clear proxy settings in a context that extends a base with proxy:

```yaml
# base.yaml
proxy:
  http: http://proxy.example.com:8080

# direct-access.yaml
extends: base
# Don't include proxy section - inherits from base

# But if you need to explicitly clear:
env:
  HTTP_PROXY: ""
  HTTPS_PROXY: ""
```
