# Editor/IDE Support

Configure a per-context editor/IDE so `ctx edit` opens your project in the right editor with the right workspace. This does **not** set `EDITOR` or `VISUAL` environment variables to avoid clobbering your global preferences.

## Configuration

```yaml
editor:
  type: vscode          # vscode, sublime, or vim
  workspace: ~/path/to/project.code-workspace  # optional
```

## Commands

```bash
ctx edit                # Open workspace in configured editor
ctx edit <file>         # Open specific file in editor
```

## Supported Editors

### VS Code

```yaml
editor:
  type: vscode
  workspace: ~/projects/myproject.code-workspace
```

Binary lookup: `code` → `code-insiders` → macOS app path.

- `ctx edit` runs `code ~/projects/myproject.code-workspace`
- `ctx edit main.go` runs `code ~/projects/myproject.code-workspace --goto main.go`

### Sublime Text

```yaml
editor:
  type: sublime
  workspace: ~/projects/myproject.sublime-project
```

Binary lookup: `subl` → macOS app path.

- `ctx edit` runs `subl --project ~/projects/myproject.sublime-project`
- `ctx edit main.go` runs `subl --project ~/projects/myproject.sublime-project main.go`

### Vim / Neovim

```yaml
# With a session file (created via :mksession)
editor:
  type: vim
  workspace: ~/projects/myproject.vim

# With a project directory
editor:
  type: vim
  workspace: ~/projects/myproject

# No workspace (just opens vim)
editor:
  type: vim
```

Binary lookup: `nvim` → `vim` (prefers Neovim when available).

- Session file (`.vim`): runs `nvim -S ~/projects/myproject.vim`
- Directory: runs `nvim ~/projects/myproject`
- `ctx edit main.go` runs `nvim -S ~/projects/myproject.vim main.go`

Vim runs in the foreground (attached to your terminal). VS Code and Sublime run detached.

## Foreground vs Detached

| Editor | Mode | Why |
|--------|------|-----|
| VS Code | Detached (`cmd.Start()`) | GUI application, returns control to shell |
| Sublime | Detached (`cmd.Start()`) | GUI application, returns control to shell |
| Vim | Foreground (`cmd.Run()`) | Terminal application, needs stdin/stdout |

## Example: Full Context

```yaml
name: myproject-dev
description: "Development with VS Code workspace"
environment: development

aws:
  profile: myproject-dev
  region: us-west-2

editor:
  type: vscode
  workspace: ~/projects/myproject.code-workspace

browser:
  type: chrome
  profile: "MyProject"

urls:
  console: https://console.aws.amazon.com
  grafana: https://grafana.myproject.com
```

After switching context:

```bash
ctx use myproject-dev
ctx edit              # Opens VS Code with workspace
ctx edit src/main.go  # Opens file in VS Code
ctx open console      # Opens AWS console in Chrome "MyProject" profile
```
