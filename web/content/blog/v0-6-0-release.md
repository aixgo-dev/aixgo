---
title: "Aixgo v0.6.0: Interactive Coding Assistant with Multi-Model Support"
date: 2026-03-08
description: "Interactive coding assistant with file operations, git integration, and cost tracking"
tags: ["release", "cli", "chat", "assistant", "interactive"]
author: "Aixgo Team"
---

We're excited to announce **Aixgo v0.6.0**, introducing an interactive coding assistant that brings the power of
multi-agent AI to your command line. This release transforms Aixgo from purely an orchestration framework into a
complete AI development toolkit with the new `aixgo chat` command.

## What's New in v0.6.0

**Quick Links:**

1. [Interactive Coding Assistant](#1-interactive-coding-assistant) - Multi-model chat interface
1. [CLI Modernization](#2-cli-modernization) - Cobra-based subcommands
1. [Session Management](#3-session-management) - Persistent conversations
1. [Assistant Tools](#4-assistant-tools) - File, git, and terminal operations

### 1. Interactive Coding Assistant

The centerpiece of v0.6.0 is `aixgo chat` - an interactive coding assistant that combines conversational AI
with practical development tools, all in a lightweight Go binary.

**Start Chatting in Seconds:**

```bash
# Launch interactive session
aixgo chat

# Use a specific model
aixgo chat --model gpt-4o

# Resume a previous session
aixgo chat --session abc123
```

**What Makes It Special:**

- **Multi-Model Support** - Switch between 7+ LLM providers (Claude, GPT, Gemini, Grok) mid-conversation
- **Real-Time Streaming** - Markdown-rendered responses stream as they're generated
- **Cost Transparency** - See per-message and session-total costs automatically
- **Lightweight** - <20MB binary, <100ms startup time
- **Session Persistence** - Conversations automatically saved to `~/.aixgo/sessions/`

**Example Session:**

```text
╭──────────────────────────────────────────────────╮
│           Aixgo Interactive Assistant            │
╰──────────────────────────────────────────────────╯
  Model: claude-3-5-sonnet
  Type /help for commands, /quit to exit

> Read the main.go file

[Assistant reads and analyzes the file content...]

> Now refactor the error handling to use wrapped errors

[Assistant modifies the file with improved error handling...]

✓ Updated main.go
  - Added fmt.Errorf with %w wrapping
  - Improved error context messages

[Cost: $0.0023 | Session total: $0.0045]

> /model gpt-4o
Switched to model: gpt-4o

> Run the tests
[Confirms command execution...]
✓ Executed: go test ./...
ok      github.com/example/pkg    0.123s

[Cost: $0.0012 | Session total: $0.0057]
```

**In-Session Commands:**

- `/model <name>` - Switch models without losing context
- `/cost` - Detailed cost breakdown by message
- `/save` - Manual session save
- `/clear` - Reset conversation (with confirmation)
- `/help` - Command reference
- `/quit` - Save and exit

### 2. CLI Modernization

The entire CLI has been refactored from flag-based commands to a modern Cobra framework with clear subcommands.

**Before (v0.5.0):**

```bash
aixgo -config agents.yaml
```

**After (v0.6.0):**

```bash
# Run orchestrator
aixgo run -config agents.yaml

# Interactive assistant
aixgo chat --model claude-3-5-sonnet

# List models with pricing
aixgo models

# Manage sessions
aixgo session list
aixgo session resume <id>
aixgo session delete <id>
```

**Benefits:**

- **Better Discovery** - `aixgo --help` shows all subcommands
- **Clearer Purpose** - Each command has a specific function
- **Future Extensibility** - Easy to add new commands
- **Consistent UX** - Follows standard CLI conventions

### 3. Session Management

Sessions provide persistent conversation history with automatic cost tracking and easy resumption.

**Session Commands:**

```bash
# List all sessions
aixgo session list

# Output:
# ID: a1b2c3d4 | Model: claude-3-5-sonnet | Cost: $0.0234 | Updated: 2026-03-08
# ID: e5f6g7h8 | Model: gpt-4o | Cost: $0.0156 | Updated: 2026-03-07

# Resume a session
aixgo session resume a1b2c3d4

# Delete old sessions
aixgo session delete e5f6g7h8
```

**Session Features:**

- **Automatic Saving** - Every interaction saved to disk
- **Cost Tracking** - Per-session and per-message cost accumulation
- **Resume Anywhere** - Pick up conversations on any machine
- **JSON Storage** - Human-readable format at `~/.aixgo/sessions/`
- **Model History** - Track which models were used when

**Storage Format:**

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

### 4. Assistant Tools

The assistant comes equipped with practical development tools accessible through natural language.

**File Operations:**

- **read_file** - Read file contents with syntax highlighting context
- **write_file** - Create or update files with safety checks
- **glob** - Find files by pattern (e.g., "*.go", "src/**/*.ts")
- **grep** - Search file contents with regex support

**Examples:**

```text
> Read all Go files in the pkg directory
[Uses glob to find files, then reads each one]

> Find all TODO comments in the codebase
[Uses grep to search across files]

> Create a new utils.go file with helper functions
[Generates code and writes to file]
```

**Git Operations:**

- **git_status** - Show working tree status
- **git_diff** - View changes in working directory
- **git_commit** - Create commits with generated messages
- **git_log** - View commit history

**Examples:**

```text
> What files have I changed?
[Runs git status and summarizes changes]

> Show me the diff
[Displays git diff with context]

> Commit these changes with an appropriate message
[Analyzes changes, generates commit message, confirms with user]
```

**Terminal Execution:**

- **Safe Command Execution** - Allowlist-based security
- **User Confirmation** - Prompts before running commands
- **Output Capture** - Returns command results to assistant

**Examples:**

```text
> Run the test suite
Execute command: go test ./...
Continue? (y/n): y
[Runs tests and shows results]

> What's the current Go version?
Execute command: go version
Continue? (y/n): y
go version go1.26.0 darwin/arm64
```

**Security Features:**

- **Command Allowlist** - Only approved commands execute
- **Path Validation** - Prevents directory traversal
- **Confirmation Prompts** - User approval required
- **Output Sanitization** - Removes sensitive information

## Model Information

View all available models and their pricing with a single command.

```bash
aixgo models
```

**Output:**

```text
Available LLM Models:

Provider: Anthropic
  claude-3-5-sonnet    | Input: $0.003/1K | Output: $0.015/1K
  claude-opus-4        | Input: $0.015/1K | Output: $0.075/1K
  claude-3-haiku       | Input: $0.00025/1K | Output: $0.00125/1K

Provider: OpenAI
  gpt-4o               | Input: $0.005/1K | Output: $0.015/1K
  gpt-4-turbo          | Input: $0.01/1K | Output: $0.03/1K
  gpt-3.5-turbo        | Input: $0.0005/1K | Output: $0.0015/1K

Provider: Google
  gemini-1.5-pro       | Input: $0.00035/1K | Output: $0.0014/1K
  gemini-2.0-flash     | Input: $0.000075/1K | Output: $0.0003/1K

Provider: xAI
  grok-2               | Input: $0.002/1K | Output: $0.010/1K
```

This helps you choose the right model for your budget and use case.

## Performance & Efficiency

**Binary Size & Startup:**

- **Binary Size** - <20MB (includes all providers and tools)
- **Cold Start** - <100ms (serverless-ready)
- **Memory** - ~50MB base footprint

**Cost Optimization:**

Switching models mid-conversation enables cost-aware workflows:

```text
> /model claude-3-haiku
Switched to model: claude-3-haiku

> Run these simple code generation tasks
[Uses cheaper model for straightforward work]

> /model claude-3-5-sonnet
Switched to model: claude-3-5-sonnet

> Now review the code for security issues
[Uses more capable model for complex analysis]
```

**Comparison to Alternatives:**

| Dimension | Aixgo Chat | Cursor/Copilot | Python CLIs |
|-----------|------------|----------------|-------------|
| Binary Size | <20MB | N/A (Editor) | 100MB+ venv |
| Startup Time | <100ms | N/A | 2-5s |
| Multi-Model | Yes (7+) | Limited | Varies |
| Cost Tracking | Built-in | No | Manual |
| Offline-First | Yes | No | Varies |
| Self-Hosted | Yes | No | Yes |

## Migration Guide

### Existing Users

**No breaking changes** - The orchestrator functionality remains unchanged:

```bash
# Old way still works
aixgo run -config agents.yaml

# New subcommand (recommended)
aixgo run -config agents.yaml
```

**New capabilities** are purely additive via new subcommands.

### Getting Started

**Fresh Install:**

```bash
# Install via go install
go install github.com/aixgo-dev/aixgo/cmd/aixgo@v0.6.0

# Or download binary
curl -L https://github.com/aixgo-dev/aixgo/releases/latest/download/aixgo_Linux_x86_64.tar.gz | tar xz
sudo mv aixgo /usr/local/bin/
```

**Configure API Keys:**

Set at least one provider's API key:

```bash
export OPENAI_API_KEY=sk-...        # For GPT models
export ANTHROPIC_API_KEY=sk-ant-... # For Claude models
export GOOGLE_API_KEY=...           # For Gemini models
export XAI_API_KEY=xai-...          # For Grok models
```

**Start Chatting:**

```bash
aixgo chat
```

## Use Cases

### 1. Code Review Assistant

```text
> Review this pull request for security issues
[Reads changed files, analyzes for vulnerabilities]

> Generate a summary of changes for the PR description
[Creates structured PR description with changes categorized]
```

### 2. Refactoring Helper

```text
> Find all instances of the old API pattern
[Uses grep to locate patterns]

> Refactor them to use the new pattern
[Generates and applies changes across files]

> Show me the diff before committing
[Displays git diff for review]
```

### 3. Documentation Generator

```text
> Read all exported functions in pkg/
[Scans codebase]

> Generate API documentation in docs/api.md
[Creates comprehensive documentation]
```

### 4. Multi-Model Experimentation

```text
> /model gpt-4o
Explain this algorithm

> /model claude-3-5-sonnet
[Same question to different model]

> /model gemini-1.5-pro
[Compare responses across models]
```

## What's Next

Looking ahead to v0.7.0 (Q2 2026):

**Enhanced Assistant Features:**

- Visual workflow debugging for multi-step operations
- Custom tool integration via MCP protocol
- Session encryption at rest
- Collaborative sessions (shared context)

**Provider Improvements:**

- Function calling standardization
- Streaming support for all providers
- Provider health monitoring

**Infrastructure:**

- PostgreSQL session backend
- Web UI for session management
- Docker-based assistant deployment

## Technical Implementation

For those interested in the architecture:

**Package Structure:**

```text
pkg/assistant/
├── coordinator/     # LLM orchestration with streaming
├── session/         # JSON file session persistence
├── tools/
│   ├── file/        # read_file, write_file, glob, grep
│   ├── git/         # git operations
│   └── terminal/    # safe command execution
├── output/          # markdown streaming renderer
└── prompt/          # interactive UI components
```

**Key Design Decisions:**

1. **File-Based Sessions** - Simple, portable, human-readable
2. **MCP-Compatible Tools** - Standard tool protocol for extensibility
3. **Streaming First** - Real-time feedback for better UX
4. **Cost Tracking** - Built into every LLM call automatically
5. **Security by Default** - Allowlists, confirmations, sanitization

## Resources

- **Interactive Guide**: [aixgo.dev/guides/chat-assistant](https://aixgo.dev/guides/chat-assistant)
- **CLI Reference**: [docs/CLI.md](https://github.com/aixgo-dev/aixgo/blob/main/docs/CLI.md)
- **Feature Catalog**: [docs/FEATURES.md](https://github.com/aixgo-dev/aixgo/blob/main/docs/FEATURES.md)
- **API Documentation**: [pkg.go.dev/github.com/aixgo-dev/aixgo](https://pkg.go.dev/github.com/aixgo-dev/aixgo)

## Get Involved

We'd love to hear how you use the new interactive assistant:

- **GitHub Issues**: [github.com/aixgo-dev/aixgo/issues](https://github.com/aixgo-dev/aixgo/issues)
- **Discussions**: [github.com/orgs/aixgo-dev/discussions](https://github.com/orgs/aixgo-dev/discussions)
- **Contributing**: [docs/CONTRIBUTING.md](https://github.com/aixgo-dev/aixgo/blob/main/docs/CONTRIBUTING.md)

**Feature Requests:**

What tools would you like to see? Let us know:

- Additional file operations?
- IDE integrations?
- Custom tool support?
- Team collaboration features?

## Contributors

Thank you to everyone who contributed to this release through code, documentation, testing, and feedback.

---

**Aixgo** - Production-grade AI agents in Go.

[Website](https://aixgo.dev) | [GitHub](https://github.com/aixgo-dev/aixgo) | [Documentation](https://pkg.go.dev/github.com/aixgo-dev/aixgo)

**Upgrade to v0.6.0 today:**

```bash
go get github.com/aixgo-dev/aixgo@v0.6.0
```
