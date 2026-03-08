---
title: "Interactive Coding Assistant"
description: "Use the aixgo chat command to interact with an AI coding assistant that can read and write files, run git operations, and execute terminal commands"
weight: 3
---

The `aixgo chat` command provides an interactive coding assistant that combines conversational AI with practical development tools. It runs as a single lightweight binary with no external runtime dependencies.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [API Key Setup](#api-key-setup)
- [Starting a Session](#starting-a-session)
- [CLI Flags](#cli-flags)
- [In-Session Commands](#in-session-commands)
- [Built-in Tools](#built-in-tools)
- [Session Management](#session-management)
- [Model Selection](#model-selection)
- [Security Model](#security-model)
- [Example Workflows](#example-workflows)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

- Go 1.26 or later (for building from source)
- At least one LLM provider API key (see [API Key Setup](#api-key-setup))

---

## Installation

**Via `go install`:**

```bash
go install github.com/aixgo-dev/aixgo/cmd/aixgo@latest
```

**Via pre-built binary:**

```bash
curl -L https://github.com/aixgo-dev/aixgo/releases/latest/download/aixgo_Linux_x86_64.tar.gz | tar xz
sudo mv aixgo /usr/local/bin/
```

Verify the installation:

```bash
aixgo --version
```

---

## API Key Setup

The assistant auto-detects which providers are available based on environment variables. Set at least one before starting a session.

```bash
export ANTHROPIC_API_KEY=<your-anthropic-api-key>   # Claude models
export OPENAI_API_KEY=<your-openai-api-key>          # GPT models
export GOOGLE_API_KEY=<your-google-api-key>          # Gemini models
export XAI_API_KEY=<your-xai-api-key>                # Grok models
```

To see which models are available with your configured keys, run `aixgo models` (see [Model Selection](#model-selection)).

---

## Starting a Session

**Start with default model (`claude-3-5-sonnet`):**

```bash
aixgo chat
```

**Start with a specific model:**

```bash
aixgo chat --model gpt-4o
aixgo chat --model gemini-2.0-flash
```

**Resume a previous session:**

```bash
aixgo chat --session <session-id>
```

**Disable streaming output:**

```bash
aixgo chat --no-stream
```

Once started, the assistant displays a welcome prompt and waits for input on a `>` line.

```text
╭──────────────────────────────────────────────────╮
│           Aixgo Interactive Assistant            │
╰──────────────────────────────────────────────────╯
  Model: claude-3-5-sonnet
  Type /help for commands, /quit to exit

>
```

To exit at any time, type `/quit` or press `Ctrl+C`. The session is saved automatically.

---

## CLI Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--model` | `-m` | `claude-3-5-sonnet` (or `$AIXGO_MODEL`) | Model to use for the session |
| `--session` | `-s` | — | Resume an existing session by ID |
| `--no-stream` | — | `false` | Disable streaming and wait for full responses |

The default model can be overridden with the `AIXGO_MODEL` environment variable:

```bash
export AIXGO_MODEL=gpt-4o
aixgo chat   # starts with gpt-4o
```

---

## In-Session Commands

Commands are prefixed with `/` and interpreted directly rather than sent to the model.

| Command | Description |
|---------|-------------|
| `/model <name>` | Switch to a different model without losing conversation context |
| `/cost` | Display the current session cost summary (total cost, message count, active model) |
| `/save` | Write the session to disk immediately |
| `/clear` | Reset conversation history (prompts for confirmation before clearing) |
| `/help` | Print the command reference |
| `/quit` | Save the session and exit |

**Example: switching models mid-conversation:**

```text
> /model gpt-4o
Switched to model: gpt-4o

> /cost

Session Cost Summary:
  Total cost: $0.0045
  Messages: 8
  Model: gpt-4o
```

**Notes:**

- `/model` accepts any model name from the supported list. If the required API key is not set, the switch will fail with an error.
- `/clear` resets both the in-memory history and the coordinator state. The session file on disk is not deleted.
- `/quit` and `/exit` are aliases.

---

## Built-in Tools

The assistant has access to three categories of tools that the model can invoke automatically in response to natural language requests. You do not call these tools directly.

### File Operations

| Tool | Description |
|------|-------------|
| `read_file` | Read the contents of a file |
| `write_file` | Create or overwrite a file |
| `glob` | Find files matching a glob pattern |
| `grep` | Search file contents using a regex pattern |

**Example prompts that use file tools:**

```text
> Read the main.go file
> Find all Go files in the pkg directory
> Search for TODO comments across the codebase
> Create a new file at internal/util/strings.go with these helper functions
```

File tools validate paths before operating on them to prevent directory traversal. See [Security Model](#security-model) for details.

### Git Operations

| Tool | Description |
|------|-------------|
| `git_status` | Show working tree status |
| `git_diff` | View uncommitted changes |
| `git_commit` | Create a commit with a generated message |
| `git_log` | View recent commit history |

**Example prompts that use git tools:**

```text
> What files have I changed?
> Show me the diff for the session package
> Commit the current changes with an appropriate message
> What were the last five commits?
```

`git_commit` analyzes the staged changes, generates a commit message, and presents it for confirmation before committing.

### Terminal Execution

The assistant can execute shell commands through the `exec` tool. All executions require explicit user confirmation before running.

**Confirmation prompt:**

```text
Execute command: go test ./...
Continue? (y/n):
```

Respond `y` to proceed or `n` to cancel. The command output is returned to the model so it can interpret results and continue the conversation.

**Example prompts that use the terminal tool:**

```text
> Run the test suite
> What Go version is installed?
> Build the project
> Show running processes
```

See [Security Model](#security-model) for the full list of allowed commands and blocking rules.

---

## Session Management

Sessions are stored as JSON files at `~/.aixgo/sessions/`. Every message exchange is automatically saved after completion. Manual saves are available with `/save`.

### Listing Sessions

```bash
aixgo session list
```

Output:

```text
Saved Sessions:
─────────────────────────────────────────────────────────────────
ID            Model                 Messages  Cost        Last Updated
─────────────────────────────────────────────────────────────────
a1b2c3d4e5f6  claude-3-5-sonnet     14        $0.0234     10:42
e5f6g7h8i9j0  gpt-4o                8         $0.0156     Mon 14:30

Resume a session with: aixgo chat --session <id>
Or: aixgo session resume <id>
```

### Resuming a Session

```bash
aixgo session resume a1b2c3d4e5f6
# equivalent to:
aixgo chat --session a1b2c3d4e5f6
```

Resuming a session restores the full message history to the model's context window. The model retains awareness of prior exchanges.

### Deleting a Session

```bash
aixgo session delete e5f6g7h8i9j0
```

This removes the session file from `~/.aixgo/sessions/`. The operation is not reversible.

### Session File Format

Each session is stored as a human-readable JSON file:

```json
{
  "id": "a1b2c3d4",
  "model": "claude-3-5-sonnet",
  "created": "2026-03-08T10:00:00Z",
  "updated": "2026-03-08T11:30:00Z",
  "total_cost": 0.0234,
  "messages": [
    {
      "role": "user",
      "content": "Read main.go",
      "timestamp": "2026-03-08T10:00:00Z"
    },
    {
      "role": "assistant",
      "content": "Here's the content of main.go...",
      "timestamp": "2026-03-08T10:00:05Z",
      "model": "claude-3-5-sonnet",
      "cost": 0.0012
    }
  ]
}
```

Session files can be read, copied, or backed up with standard file tools.

---

## Model Selection

Run `aixgo models` to view all supported models, their pricing, and which are available with your current API keys:

```bash
aixgo models
```

Output (availability indicated by checkmark):

```text
Available Models:
════════════════════════════════════════════════════════════════════════════════
Model                 Provider    Description                          Input/1M     Output/1M
────────────────────────────────────────────────────────────────────────────────
✔ claude-opus-4        Anthropic   Most capable, best for complex tas...  $15.00       $75.00
✔ claude-3-5-sonnet    Anthropic   Balanced speed and capability (rec...  $3.00        $15.00
✔ claude-3-5-haiku     Anthropic   Fastest, best for simple tasks         $0.25        $1.25
✘ gpt-4o               OpenAI      Latest GPT-4 with vision               $2.50        $10.00
✘ gpt-4o-mini          OpenAI      Smaller, faster GPT-4o                 $0.15        $0.60
✘ o1                   OpenAI      Reasoning model for complex problems   $15.00       $60.00
✘ gemini-2.0-flash     Google      Latest Gemini, fast and capable        $0.08        $0.30
✘ gemini-1.5-pro       Google      Gemini Pro with 1M context             $1.25        $5.00
✘ grok-2               xAI         Grok 2 for coding and reasoning        $2.00        $10.00

Available: 3/9 models (set API keys to enable more)
```

### Switching Models Mid-Conversation

Use `/model` inside a session to switch without losing history:

```text
> /model claude-3-5-haiku
Switched to model: claude-3-5-haiku
```

The conversation history carries over. Subsequent messages are sent to the new model. This is useful for routing straightforward tasks to lower-cost models and complex analysis to higher-capability models.

### Cost Tracking

The assistant displays per-message cost after each response when the cost exceeds $0.001:

```text
[Cost: $0.0023 | Session total: $0.0045]
```

Use `/cost` for a full session summary at any time.

> **Note:** Costs shown during streaming responses are estimated (approximately 4 characters per token). Non-streaming responses use exact token counts from the provider's API.

---

## Security Model

### Command Allowlist

The terminal tool (`exec`) only runs commands from an explicit allowlist. Commands not on this list are rejected before any confirmation prompt is shown.

**Allowed command categories:**

| Category | Commands |
|----------|----------|
| Build tools | `go`, `make`, `npm`, `yarn`, `pnpm`, `cargo`, `gradle`, `mvn`, `pip`, `poetry` |
| Version control | `git` |
| File operations (read) | `ls`, `cat`, `head`, `tail`, `wc`, `find`, `grep`, `which`, `file`, `basename` |
| System info | `pwd`, `whoami`, `uname`, `date`, `env`, `echo`, `printf`, `dirname` |
| Process | `ps` |
| Network (read) | `curl`, `wget`, `ping`, `nslookup`, `host` |
| Utilities | `jq`, `yq`, `sed`, `awk`, `sort`, `uniq`, `diff`, `tree` |
| Containers | `docker` |
| Cloud CLIs | `gcloud`, `aws`, `az`, `kubectl` |

### Blocked Subcommands

Certain subcommands are blocked even for allowed base commands:

| Command | Blocked subcommands |
|---------|---------------------|
| `git` | `push`, `reset`, `rebase`, `force-push` |
| `rm` | `-rf`, `-r`, `--recursive` |
| `docker` | `rm`, `rmi`, `prune`, `system prune` |

### Shell Operator Restrictions

The following shell operators are blocked to prevent bypass through chaining:

- `&&`, `||`, `;` — command chaining
- `` ` ``, `$(` — command substitution
- `<` — input redirection

Pipe (`|`) is permitted only when the pipe target is one of: `grep`, `head`, `tail`, `wc`, `sort`, `uniq`, `jq`, `awk`, `sed`.

Output redirection (`>`) is permitted only to `/dev/null` or with `2>&1`.

### Path Validation

File tools reject paths containing `..` components to prevent directory traversal outside the working directory.

### Confirmation Requirement

The terminal tool always requires explicit user confirmation before executing a command. There is no way to pre-approve commands or disable this prompt.

---

## Example Workflows

### Code Review

```text
> Read all Go files in the pkg/security directory
[Reads and summarizes each file]

> Identify any inputs that are not validated before use
[Analyzes code for validation gaps]

> Generate a summary of findings in SECURITY_REVIEW.md
[Creates the file]
```

### Refactoring

```text
> Find all functions that return errors without wrapping them with %w
[Uses grep to locate unwrapped errors]

> Show me the first three examples with their surrounding context
[Reads relevant file sections]

> Update each one to use fmt.Errorf with %w
[Modifies files and reports changes]

> Show me the diff before we commit
[Runs git diff]

> Commit these changes
[Generates message, prompts for confirmation, creates commit]
```

### Documentation Generation

```text
> Read all exported functions in pkg/mcp
[Scans and reads relevant files]

> Generate API reference documentation in docs/mcp-api.md
[Creates structured documentation file]

> Run the markdown linter on the new file
[Executes linter command after confirmation]
```

### Cost-Aware Multi-Model Workflow

```text
> /model claude-3-5-haiku
Switched to model: claude-3-5-haiku

> Generate boilerplate unit tests for the functions in agents/react.go
[Uses cheaper model for repetitive generation work]

> /model claude-opus-4
Switched to model: claude-opus-4

> Review the generated tests for correctness and edge cases
[Uses more capable model for analysis]

> /cost

Session Cost Summary:
  Total cost: $0.0089
  Messages: 12
  Model: claude-opus-4
```

---

## Troubleshooting

**"command not allowed" when running a terminal command**

The command is not in the allowlist. Check the [Allowed command categories](#command-allowlist) table. If you need an unlisted command, run it directly in your terminal outside the assistant.

**"failed to get provider" on startup**

The API key for the requested model is not set or is invalid. Run `aixgo models` to check which models are available, then verify the corresponding environment variable is exported correctly.

**"failed to switch model" when using `/model`**

The target model requires an API key that is not configured. Set the relevant environment variable and try again.

**Streaming output stops mid-response**

This can occur due to network interruption or provider-side rate limiting. The session up to that point is saved. Resume with `aixgo chat --session <id>` and repeat the request. Use `--no-stream` to disable streaming if the problem persists.

**High session costs**

Long conversation histories are sent with every request, accumulating token usage. Use `/clear` to reset history when starting a new task, or start a fresh session with `aixgo chat`.

---

## Next Steps

- [Session Persistence](/guides/sessions/) - Detailed session API for programmatic access
- [Provider Integration](/guides/provider-integration/) - Configure additional LLM providers
- [Cost Optimization](/guides/cost-optimization/) - Strategies for reducing LLM spend
- [Quick Start](/guides/quick-start/) - Build multi-agent pipelines with the orchestration framework
