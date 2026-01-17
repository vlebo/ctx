# Context Inheritance

Avoid repeating configuration by using inheritance. Create base contexts with shared settings, then extend them for specific environments.

## Basic Inheritance

**Base context** (`~/.config/ctx/contexts/acme-base.yaml`):

```yaml
name: acme-base
abstract: true                    # Cannot be used directly, only extended
description: Base config for ACME Corp

aws:
  region: us-east-1

kubernetes:
  context: acme-eks-cluster

ssh:
  bastion:
    host: bastion.acme.com
    user: deploy
    identity_file: ~/.ssh/acme_rsa

env:
  COMPANY: acme
  TEAM: platform

urls:
  jira: https://acme.atlassian.net
  wiki: https://wiki.acme.com
```

**Child contexts** extend the base:

```yaml
# acme-dev.yaml
name: acme-dev
extends: acme-base
environment: development

aws:
  profile: acme-dev               # Adds to inherited aws config

kubernetes:
  namespace: dev                  # Adds to inherited kubernetes config

env:
  LOG_LEVEL: debug                # Merged with inherited env vars

urls:
  app: https://dev.acme.com       # Merged with inherited URLs
```

```yaml
# acme-prod.yaml
name: acme-prod
extends: acme-base
environment: production

aws:
  profile: acme-prod-admin

kubernetes:
  namespace: production

vpn:
  type: openvpn
  config_file: ~/.config/ctx/acme-prod.ovpn
  auto_connect: true

env:
  LOG_LEVEL: warn
```

## How Inheritance Works

Deep merge with child values taking precedence:

| Field Type | Behavior |
|------------|----------|
| **Strings** | Child overrides parent if non-empty |
| **Booleans** | Parent's `true` is inherited (see note below) |
| **Sections** (aws, kubernetes, etc.) | Deep merged field-by-field |
| **Maps** (env, urls) | Merged - child values override matching keys |
| **Slices** (tunnels, databases) | Child replaces parent if non-empty |
| **Tags** | Merged and deduplicated |

!!! warning "Boolean Inheritance"
    Due to how YAML/Go works, a parent's `use_vault: true` cannot be overridden to `false` by a child context. Boolean fields should only be set to `true` at the level where they're actually needed, not in base contexts.

## Multi-Level Inheritance

Contexts can extend other contexts that also extend a base:

```yaml
# acme-base.yaml (abstract)
name: acme-base
abstract: true
aws:
  region: us-east-1

# acme-prod-base.yaml (abstract)
name: acme-prod-base
abstract: true
extends: acme-base
environment: production
vpn:
  type: openvpn
  auto_connect: true

# acme-prod-us.yaml (usable)
name: acme-prod-us
extends: acme-prod-base
aws:
  profile: acme-prod-us
vpn:
  config_file: ~/.config/ctx/acme-us.ovpn
```

## Abstract Contexts

Mark contexts as `abstract: true` to:

- **Hide from `ctx list`** - Only show usable contexts
- **Prevent direct use** - `ctx use acme-base` returns an error
- **Document intent** - Clear that it's a template

```bash
# Lists only usable contexts
ctx list
   NAME         ENVIRONMENT  CLOUD  ORCHESTRATION
   acme-dev     development  aws    kubernetes
   acme-prod    production   aws    kubernetes

# Shows all including abstract (marked with ~)
ctx list --all
   NAME            ENVIRONMENT  CLOUD  ORCHESTRATION
~  acme-base       -            aws    kubernetes
   acme-dev        development  aws    kubernetes
   acme-prod       production   aws    kubernetes

# Cannot use abstract context
ctx use acme-base
# Error: context 'acme-base' is abstract and cannot be used directly

# Show displays merged configuration
ctx show acme-prod
# Name: acme-prod
# Extends: acme-base
# Environment: production
# ...
```

## Example: Multi-Region Setup

```yaml
# company-base.yaml
name: company-base
abstract: true
aws:
  sso_login: true
kubernetes:
  namespace: default
env:
  COMPANY: acme

# company-us.yaml
name: company-us
abstract: true
extends: company-base
aws:
  region: us-east-1

# company-eu.yaml
name: company-eu
abstract: true
extends: company-base
aws:
  region: eu-west-1

# company-us-prod.yaml
name: company-us-prod
extends: company-us
environment: production
aws:
  profile: acme-us-prod

# company-eu-prod.yaml
name: company-eu-prod
extends: company-eu
environment: production
aws:
  profile: acme-eu-prod
```

This creates a hierarchy:

```
company-base (abstract)
├── company-us (abstract)
│   ├── company-us-dev
│   ├── company-us-staging
│   └── company-us-prod
└── company-eu (abstract)
    ├── company-eu-dev
    ├── company-eu-staging
    └── company-eu-prod
```
