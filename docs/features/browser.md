# Browser Profiles

Use separate browser profiles for different projects to keep SSO sessions isolated. This prevents authentication conflicts when working with multiple cloud accounts.

## Configuration

```yaml
browser:
  type: chrome        # or firefox
  profile: "MyProject" # Profile name as shown in Chrome/Firefox
```

## List Available Profiles

```bash
ctx browser list
```

This shows all detected browser profiles on your system.

## Commands

```bash
ctx browser list                 # List detected browser profiles
ctx browser open                 # Open browser with context profile
ctx browser open <url>           # Open URL with context profile
```

## How It Works

When you have a browser profile configured:

1. `ctx open <url>` opens URLs in the specified profile
2. `aws sso login` opens the SSO page in the correct profile
3. `gcloud auth login` uses the correct profile
4. `az login` uses the correct profile
5. `vault login -method=oidc` uses the correct profile
6. `bw login --sso` uses the correct profile (Bitwarden SSO)
7. `op signin --sso` uses the correct profile (1Password SSO)

## Creating Browser Profiles

### Chrome

1. Open Chrome
2. Click your profile icon (top right)
3. Click "Add" or "Add another account"
4. Enter a name for the profile

### Firefox

1. Open Firefox
2. Enter `about:profiles` in the address bar
3. Click "Create a New Profile"
4. Follow the wizard

## Example: Multi-Account Setup

```yaml
# personal.yaml
name: personal
aws:
  profile: personal-aws
  sso_login: true
browser:
  type: chrome
  profile: "Personal"

# work.yaml
name: work
aws:
  profile: work-aws
  sso_login: true
browser:
  type: chrome
  profile: "Work"

# client-acme.yaml
name: client-acme
aws:
  profile: acme-aws
  sso_login: true
browser:
  type: chrome
  profile: "ACME Client"
```

Now when you switch contexts:

```bash
ctx use work
# AWS SSO login opens in "Work" Chrome profile

ctx use client-acme
# AWS SSO login opens in "ACME Client" Chrome profile
```

## Quick URLs

Combine browser profiles with quick URLs:

```yaml
browser:
  type: chrome
  profile: "MyProject"

urls:
  console: https://console.aws.amazon.com
  grafana: https://grafana.myproject.com
  github: https://github.com/myproject-org
  jira: https://myproject.atlassian.net
```

Open URLs in the correct profile:

```bash
ctx open console    # Opens AWS console in "MyProject" profile
ctx open grafana    # Opens Grafana in "MyProject" profile
ctx open            # Lists all available URLs
```

## WSL Support

On Windows Subsystem for Linux, ctx automatically detects browser profiles from your Windows Chrome/Firefox installations. The browser opens in Windows, not in WSL.

## Troubleshooting

### Profile Not Found

If `ctx browser list` doesn't show your profile:

- Ensure the profile exists in the browser
- For Chrome, check `~/.config/google-chrome/` (Linux) or `~/Library/Application Support/Google/Chrome/` (macOS)
- For Firefox, check `~/.mozilla/firefox/` (Linux) or `~/Library/Application Support/Firefox/` (macOS)

### Wrong Profile Opens

Ensure the profile name matches exactly as shown in the browser's profile switcher. Profile names are case-sensitive.
