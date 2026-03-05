# alaya-tui

A terminal user interface for interacting with AI agents alongside a local markdown vault. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and Go.

alaya-tui brings together a notes browser, AI agent chat, MCP server management, and real-time activity monitoring into a single terminal dashboard. It is designed to work with the [alaya](https://github.com/lukehinds/alaya) MCP server, which provides vault-aware context to agents like Claude.

---

## Table of Contents

- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
  - [Config File](#config-file)
  - [Agents](#agents)
  - [API Keys](#api-keys)
  - [Vault Directory](#vault-directory)
- [Vault Structure](#vault-structure)
- [Usage](#usage)
  - [Navigation](#navigation)
  - [Dashboard](#dashboard)
  - [Activity](#activity)
  - [Notes](#notes)
  - [Chat](#chat)
  - [Settings](#settings)
- [MCP Server](#mcp-server)
- [CLI Flags](#cli-flags)
- [Development](#development)
  - [Build](#build)
  - [Test](#test)
  - [Lint and Security](#lint-and-security)
- [Architecture](#architecture)
- [Security](#security)
- [CI / SAST](#ci--sast)

---

## Features

- **AI Agent Chat** — Spawn any CLI-based agent (e.g. `claude`, `ollama`) as a subprocess and chat with it interactively
- **Notes Browser** — Navigate your markdown vault in a two-pane tree/preview layout
- **Activity Monitor** — Live-tailing of the alaya audit log with filtering
- **Dashboard** — Vault statistics, tag cloud, server health, and recent activity at a glance
- **MCP Server Management** — Start and monitor the alaya MCP server from within the TUI
- **Secure API Key Storage** — Keys stored in the OS keychain (macOS Keychain, Linux Secret Service, Windows Credential Manager), never written to disk
- **Persistent Configuration** — TOML config with per-agent environment variables

---

## Requirements

- Go 1.23 or later
- A markdown vault directory (plain folder of `.md` files)
- An AI agent CLI installed (e.g. [Claude Code](https://github.com/anthropics/claude-code): `npm install -g @anthropic-ai/claude-code`)
- (Optional) [alaya MCP server](https://github.com/lukehinds/alaya) for vault-aware agent context
- (Optional) [uv](https://github.com/astral-sh/uv) if using the MCP server spawn feature

---

## Installation

```bash
git clone https://github.com/lukehinds/alaya-tui
cd alaya-tui
make build
# Binary output: ./alaya-tui
```

Or run directly without building:

```bash
go run ./cmd/alaya-tui/
```

---

## Quick Start

1. **Build and run:**

   ```bash
   make build
   ./alaya-tui --vault-dir ~/notes
   ```

2. **On first run** a default config is created at `~/.config/alaya-tui/config.toml` with a `claude` agent and vault at `~/notes`.

3. **Navigate** with number keys `1`–`5` or `Tab` to switch between tabs.

4. **In the Chat tab** the configured agent spawns automatically. Type a message and press `Enter` to send.

5. **Add API keys** in the Settings tab (press `5`) so they are injected into your agent's environment automatically.

---

## Configuration

### Config File

Default path: `~/.config/alaya-tui/config.toml`

Override with `--config <path>`.

If the file does not exist it is created automatically with sensible defaults.

```toml
vault_dir = "/home/user/notes"
default_agent = "claude"

[[agents]]
name = "claude"
command = "claude"
description = "Claude Code CLI"

[[agents]]
name = "ollama"
command = "ollama run qwen2.5"
description = "Local Ollama model"
```

| Field | Type | Description |
|-------|------|-------------|
| `vault_dir` | string | Path to your markdown vault |
| `default_agent` | string | Name of the agent to use at startup |
| `agents` | array | List of agent configurations |

### Agents

Each agent entry configures one AI agent subprocess:

```toml
[[agents]]
name = "claude"           # Identifier used in :agent commands
command = "claude"        # Executable (or full path) with optional args
description = "Claude"    # Shown in settings
env = { FOO = "bar" }    # Extra environment variables for this agent
```

The `command` field is split on whitespace into an executable and arguments. For example `"ollama run qwen2.5"` becomes `exec.Command("ollama", "run", "qwen2.5")`.

**Security note:** The executable is validated before use — shell metacharacters (`|`, `&`, `;`, `<`, `>`, `` ` ``, `$`, etc.) and relative path components (`../foo`) are rejected. Use a bare command name or an absolute path.

### API Keys

API keys are stored in the OS keychain, not in the config file. Manage them in the Settings tab or set them directly via the OS keychain tools.

Supported providers and their corresponding environment variable names:

| Provider | Environment Variable |
|----------|---------------------|
| Anthropic | `ANTHROPIC_API_KEY` |
| OpenAI | `OPENAI_API_KEY` |
| OpenRouter | `OPENROUTER_API_KEY` |
| Groq | `GROQ_API_KEY` |

Keys are automatically injected into the environment of every agent subprocess at spawn time. The UI shows masked values (last 4 characters visible).

### Vault Directory

Resolution order (first match wins):

1. `--vault-dir` CLI flag
2. `ALAYA_VAULT_DIR` environment variable
3. `ZK_NOTEBOOK_DIR` environment variable (backward compatibility)
4. `vault_dir` in config file
5. Default: `~/notes`

---

## Vault Structure

A vault is a directory of markdown files. alaya-tui scans it recursively, skipping `.git`, `.zk`, `.obsidian`, `.venv`, `.trash`, `__pycache__`, and `node_modules`.

```
~/notes/
├── .zk/ or .obsidian/
│   └── audit.jsonl      # Activity log written by the alaya MCP server
├── index.md
├── projects/
│   ├── project-a.md
│   └── project-b.md
└── research/
    └── topic.md
```

### Frontmatter

Metadata is extracted from YAML frontmatter at the top of each file:

```markdown
---
title: My Note
date: 2024-01-15
tags: [golang, tui, notes]
---

# Content starts here
```

Supported frontmatter keys: `title`, `date`, `tags`. Tags support both inline list syntax (`[a, b, c]`) and single-value syntax.

### Audit Log

The alaya MCP server writes activity to `audit.jsonl` in the vault data directory (`.zk/` or `.obsidian/`) as newline-delimited JSON:

```json
{"ts": 1705356000.5, "tool": "read_note", "args": {"path": "index.md"}, "status": "ok", "duration_ms": 12.3, "summary": "Read index.md"}
```

alaya-tui tails this file in real time, surfacing entries in the Activity tab and Dashboard.

---

## Usage

### Navigation

| Key | Action |
|-----|--------|
| `1` | Dashboard tab |
| `2` | Activity tab |
| `3` | Notes tab |
| `4` | Chat tab |
| `5` | Settings tab |
| `Tab` | Cycle to next tab |
| `q` / `Ctrl+C` | Quit |

### Dashboard

Shows a summary of your vault and system state:

- **Vault path** and note/directory counts
- **Active agent** name
- **MCP server status** (running / stopped / unknown)
- **Vault health** (healthy if `.zk/` or `.obsidian/` exists, or `.md` files are present)
- **Top 10 tags** by frequency across all notes
- **Recent activity** — last 5 audit log entries
- **Error count** from the audit log

Press `s` on the Dashboard to spawn the MCP server if it is stopped.

### Activity

A live table of all audit log entries from the MCP server.

| Key | Action |
|-----|--------|
| `j` / `k` | Scroll down / up |
| `g` | Jump to top |
| `G` | Jump to bottom |
| `/` | Filter by tool name (type to filter, `Enter` to confirm) |

New entries are appended in real time. The view auto-scrolls to the bottom unless a filter is active.

### Notes

A two-pane browser for your vault.

**Left pane** — Directory tree:

| Key | Action |
|-----|--------|
| `j` / `k` | Move cursor down / up |
| `Enter` | Expand or collapse a directory |

**Right pane** — Preview of the selected file (first 30 lines).

Directories show `v` when expanded and `>` when collapsed.

### Chat

An interactive terminal session with your configured AI agent.

The agent subprocess starts automatically when you open the Chat tab. Its stdout and stderr are streamed into the viewport in real time.

| Input | Action |
|-------|--------|
| Type + `Enter` | Send message to agent |
| `:agent <name>` | Switch to a different configured agent |
| `:restart` | Kill and restart the current agent |

The status line shows `[running]` or `[stopped]`. If the agent crashes, it shows `[stopped]` and you can use `:restart` to bring it back.

### Settings

Four sub-sections, cycle between them with `Tab`.

#### Agents

View all configured agents. Keys:

| Key | Action |
|-----|--------|
| `j` / `k` | Move selection |
| `e` | Edit the selected agent's command |
| `d` | Delete the selected agent |
| `a` | Add a new agent (two-step: enter name, then command) |
| `Enter` | Confirm edit or add |

#### API Keys

View, set, and delete API keys stored in the OS keychain.

| Key | Action |
|-----|--------|
| `j` / `k` | Move selection |
| `e` | Edit (enter new key value) |
| `d` | Delete from keychain |
| `Enter` | Confirm |

Keys are masked in the display.

#### Vault

Edit the vault directory path. Press `e`, type the new path, press `Enter`. The change is saved to config immediately.

#### Default Agent

Select which agent is used at startup. Press `j`/`k` to choose, `Enter` to confirm.

---

## MCP Server

alaya-tui can spawn and monitor the [alaya MCP server](https://github.com/lukehinds/alaya), which gives agents like Claude access to your vault contents via the Model Context Protocol.

**Spawning:**

Press `s` on the Dashboard tab. alaya-tui runs:

```
uv run alaya
```

with `ALAYA_VAULT_DIR` set to your vault path and the working directory set to the vault root. The process runs in the background; stdout/stderr are detached so they do not interfere with the TUI.

**Health check:**

On Linux, alaya-tui checks for a running server by scanning `/proc/*/cmdline` for `alaya.server` — no external tools required. On macOS and other systems the status will show as `Unknown`.

The check runs every 10 seconds. The Dashboard shows:
- Green `running` — server is active
- Red `stopped` — server is not running; press `s` to start
- Gray `unknown` — health check not supported on this OS

When you quit alaya-tui, any server process it spawned is sent a kill signal.

---

## CLI Flags

```
Usage: alaya-tui [flags]

Flags:
  --vault-dir <path>   Path to your markdown vault (overrides config and ZK_NOTEBOOK_DIR)
  --agent <name>       Name of the agent to use at startup (overrides config default_agent)
  --config <path>      Path to config file (default: ~/.config/alaya-tui/config.toml)
```

`~` in paths is expanded to the user home directory.

---

## Development

### Build

```bash
make build        # produces ./alaya-tui
make run          # go run ./cmd/alaya-tui/
make clean        # remove binary
```

### Test

```bash
make test         # go test -race -short ./...
```

### Lint and Security

```bash
make lint         # golangci-lint run ./...
make vuln         # govulncheck ./...
```

For local SAST scanning (requires `gosec` and `semgrep`):

```bash
gosec ./...
semgrep scan --config .semgrep/ --config p/golang .
```

---

## Architecture

```
alaya-tui/
├── cmd/alaya-tui/
│   └── main.go              # Entry point — flags, config loading, TUI init
├── internal/
│   ├── config/
│   │   ├── config.go        # TOML config struct, load/save, agent lookup
│   │   └── keyring.go       # OS keychain integration for API keys
│   ├── backend/
│   │   ├── audit.go         # Tail and load .zk/audit.jsonl
│   │   ├── server.go        # MCP server health check and spawn
│   │   └── vault.go         # Vault scanning, frontmatter parsing, path safety
│   └── tui/
│       ├── app.go           # Root model, tab routing, lifecycle
│       ├── styles.go        # Lipgloss colour palette and reusable styles
│       ├── dashboard.go     # Tab 1 — stats, tags, recent activity
│       ├── activity.go      # Tab 2 — live audit log table
│       ├── notes.go         # Tab 3 — vault tree browser and preview
│       ├── chat.go          # Tab 4 — agent subprocess I/O
│       └── settings.go      # Tab 5 — agents, keys, vault, default agent
├── .semgrep/
│   └── rules.yml            # Custom Semgrep rules for this codebase
├── .github/workflows/
│   ├── ci.yml               # Build, test, lint, govulncheck
│   └── sast.yml             # Semgrep + gosec SAST scanning
├── Makefile
├── go.mod
└── go.sum
```

**Key design choices:**

- The Bubble Tea `Model` / `Update` / `View` pattern is used throughout. Each tab is its own model; the root `AppModel` delegates messages to whichever tab is active.
- The agent subprocess runs entirely outside the Bubble Tea loop. Output is read in background goroutines and sent back into the program via `tea.Program.Send()`.
- The audit log is tailed in a separate goroutine using a channel. The root model consumes from the channel in `Update`.
- All secrets are stored in the OS keychain via `go-keyring`. The config file on disk contains no credentials.

---

## Security

### Command Injection Prevention

Agent executables are validated before being passed to `exec.Command`. The validator rejects:
- Shell metacharacters: `|`, `&`, `;`, `<`, `>`, `(`, `)`, `$`, `` ` ``, `\`, `"`, `'`, `*`, `?`, `[`, `]`, `#`, `~`, `=`, `%`, `!`, `{`, `}`
- Relative path components: `../foo` style paths

Bare command names (`claude`) and absolute paths (`/usr/local/bin/claude`) are accepted.

### Path Traversal Prevention

All file operations on vault paths go through `containedInVault()`, which resolves both the vault root and the target path to absolute paths and checks that the target is contained within the root. This prevents symlink-based traversal and `..` escapes.

### Secrets Management

API keys are stored exclusively in the OS keychain. The TOML config file contains only non-sensitive settings. A custom Semgrep rule (`alaya-tui-no-hardcoded-secrets`) enforces this in CI.

### Subprocess Isolation

The MCP server and agent processes run as independent OS processes with explicitly constructed environments. Neither inherits open file descriptors from the TUI beyond what is intentionally piped.

---

## CI / SAST

Two GitHub Actions workflows run on every push and pull request to `main` and `dev`.

### CI (`.github/workflows/ci.yml`)

| Job | Tool | What it checks |
|-----|------|----------------|
| `build` | `go build`, `go test -race` | Compiles and tests on Go 1.23 and 1.24 |
| `lint` | `golangci-lint` | Style, correctness, and common Go anti-patterns |
| `govulncheck` | `govulncheck` | Known CVEs in Go module dependencies |

### SAST (`.github/workflows/sast.yml`)

| Job | Tool | What it checks |
|-----|------|----------------|
| `semgrep` | Semgrep | Community rules (`p/default`, `p/golang`, `p/owasp-top-ten`, `p/secrets`) plus custom rules in `.semgrep/` |
| `gosec` | gosec | Go-specific security issues (CWE-78 command injection, CWE-22 path traversal, etc.) |

SARIF results are uploaded to the GitHub Security tab for both tools.

### Custom Semgrep Rules

Three project-specific rules live in `.semgrep/rules.yml`:

| Rule ID | Severity | What it flags |
|---------|----------|---------------|
| `alaya-tui-command-injection` | ERROR | `exec.Command` calls without an explicit `nosemgrep` suppression |
| `alaya-tui-path-traversal` | WARNING | `filepath.Join` with argument names that suggest user input |
| `alaya-tui-no-hardcoded-secrets` | ERROR | Variable assignments whose name matches `secret`, `password`, `token`, `api_key`, etc. |

Any `exec.Command` call that is intentionally safe must carry an inline suppression comment explaining why:

```go
// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command,alaya-tui-command-injection
cmd := exec.Command("uv", "run", "alaya") // #nosec G204 -- fully static args
```
