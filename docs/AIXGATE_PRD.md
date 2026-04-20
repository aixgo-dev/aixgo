# Aixgate — Product Requirements Document

**Runtime sandboxing for AI coding agents**

| | |
|---|---|
| **Project** | Aixgate |
| **Parent** | aixgo.dev |
| **Author** | Charles Green |
| **Version** | 0.1 (Draft) |
| **Date** | April 2026 |
| **Status** | Proposed |
| **License** | Apache 2.0 |

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Problem Statement](#2-problem-statement)
3. [Goals and Non-Goals](#3-goals-and-non-goals)
4. [Users and Use Cases](#4-users-and-use-cases)
5. [Product Overview](#5-product-overview)
6. [Architecture](#6-architecture)
7. [Policy Specification](#7-policy-specification)
8. [Audit Log](#8-audit-log)
9. [Packaging and Distribution](#9-packaging-and-distribution)
10. [Relationship to aixgo.dev](#10-relationship-to-aixgodev)
11. [Roadmap](#11-roadmap)
12. [Success Metrics](#12-success-metrics)
13. [Risks and Open Questions](#13-risks-and-open-questions)
14. [Implementation Notes for Contributors](#14-implementation-notes-for-contributors)
15. [v0.1 Weekend Build Plan](#15-v01-weekend-build-plan)
16. [Appendix](#16-appendix)

---

## 1. Executive Summary

Aixgate is a vendor-agnostic runtime sandbox for AI coding agents (Claude Code, Cursor, Aider, OpenAI Codex agents, and any other process that reads and writes files on behalf of an LLM). It enforces deny-by-default filesystem access policies at the OS boundary, ensuring that sensitive files such as `.env`, cloud credentials, SSH private keys, and personal documents are never exposed to an agent unless an explicit policy rule permits it.

Aixgate is an open-source component of **aixgo.dev**, the production-grade Go framework for AI agents. Where aixgo provides the primitives to **build** agents, Aixgate provides the primitives to **run** them safely on developer workstations. It is written in Go, distributed as a single binary, supports both macOS and Linux, and integrates into existing developer workflows by launching agents inside a transparent filesystem overlay.

The core hypothesis is simple: developers are adopting agentic coding tools faster than they are adopting the security controls to govern them. Today, running an AI agent in a project directory is roughly equivalent to running an untrusted script with full access to the user's home directory. Aixgate closes that gap without requiring behavioral change from the developer or cooperation from the agent vendor.

### Key outcomes for v1

- A developer can launch any AI coding agent with a single `aixgate run` command and be confident their `.env` files, credentials, and SSH keys are not accessible to that agent.
- Policy is declarative, portable across agents, and version-controllable alongside a project repository.
- Every filesystem access decision is logged to a structured audit trail that can be reviewed, searched, and shipped to a SIEM if desired.
- Installation and day-one experience are smooth enough that a developer will actually use Aixgate instead of bypassing it.

---

## 2. Problem Statement

### 2.1 The threat model

AI coding agents execute with the full privileges of the user running them. In a typical developer setup, this means:

- Unrestricted read access to the home directory, including `.env` files, `~/.aws/credentials`, `~/.ssh/`, browser-saved passwords, and personal documents.
- Unrestricted shell execution, allowing the agent to `curl`, `scp`, or otherwise exfiltrate anything it can read.
- Unrestricted write access to project files and system configuration.

This is acceptable when the agent is a trusted local tool operating on intentional input. It is **not** acceptable when:

- The agent receives input from untrusted sources (a pasted error message, a GitHub issue, a webpage it was asked to summarize) — the well-documented prompt injection attack surface.
- The agent operates autonomously for extended periods, with reduced human oversight of each action.
- The developer is operating under a compliance regime (SOC 2, ISO 27001, financial regulation) that requires access controls over sensitive data.

### 2.2 Why existing tools do not solve this

| Tool category | What it does | Why it is insufficient |
|---|---|---|
| Aqua / Falco / Tetragon | Runtime protection for containers and Kubernetes workloads | Targets server environments, not developer laptops. No macOS support. Requires privileged kernel access. |
| macOS App Sandbox | Apple's built-in sandboxing for App Store apps | Not available to arbitrary developer tools. Configuration is not portable. |
| Docker Desktop containers | Isolate agents in containers | Heavy, breaks developer workflow, agents cannot access project source as they expect, no fine-grained policy. |
| Agent-specific permission prompts | e.g. Claude Code's tool approval flow | Vendor-specific, inconsistent, bypassable, and the user quickly clicks through. |
| Vendor MCP servers with allowlists | Some agents let you allowlist tools | Only works within that vendor's tool-calling flow. An agent that shells out bypasses it entirely. |

### 2.3 Target user

The primary target user for v1 is the individual developer or small team running AI coding agents on a personal or work laptop. Within that audience:

- **Security-conscious individual developers** who want a belt-and-braces layer on top of vendor permission prompts.
- **Engineering teams at security-sensitive companies** (fintech, healthcare, infrastructure) that want to adopt agentic tooling but need a defensible answer to "how are you protecting customer data and production credentials?"
- **Consultants and contractors**, like the author, who run agents across multiple client codebases and need strong guarantees that credentials from one client are not accessible during a session scoped to another.

---

## 3. Goals and Non-Goals

### 3.1 Goals

- **G1. Vendor agnostic.** Aixgate works with any AI agent that runs as a local process, without requiring cooperation from the agent vendor.
- **G2. Deny by default.** Access to files and commands is denied unless explicitly permitted by the active policy.
- **G3. Transparent.** Agents see a filtered filesystem view and shell environment; they do not know they are sandboxed. No agent code changes required.
- **G4. Cross-platform.** macOS (Apple Silicon and Intel) and Linux (x86_64 and arm64) are first-class. No Windows support in v1.
- **G5. Low friction.** A developer already using Claude Code can adopt Aixgate without changing how they work beyond prefixing `aixgate run`.
- **G6. Auditable.** Every access decision is logged as structured data that can be reviewed or forwarded to a SIEM.
- **G7. Open source.** Permissively licensed (Apache 2.0), contribution-friendly, part of the aixgo.dev ecosystem.
- **G8. Single binary.** Install, upgrade, and uninstall must each be a single command.

### 3.2 Non-goals (for v1)

- **Windows support.** Viable v2; v1 focuses on macOS and Linux where AI agent adoption is concentrated.
- **Kubernetes or CI sandboxing.** Aixgate is designed for local developer use. Server-side enforcement is a future aixgo product.
- **Network egress control.** Out of scope for v1. We do not intercept outbound connections. A future version may add optional network policy via a local proxy.
- **Replacing agent vendor permission flows.** Aixgate is a defense-in-depth layer; it does not tell users to disable Claude Code's built-in approvals.
- **Kernel-level enforcement.** v1 uses FUSE and OS-provided process isolation. eBPF and LSM-based enforcement are explicit v2+ territory.
- **Policy marketplace or GUI.** v1 ships sensible defaults and YAML policies. A web UI or shared policy registry is a later concern.

### 3.3 Explicit anti-goals

Aixgate will **not** attempt to sandbox agents that are running as root, nor will it claim to defend against a motivated attacker with local code execution. The threat model is an agent acting on behalf of the user that may misbehave due to prompt injection, a flawed plan, or a buggy tool use — **not** an attacker who has already fully compromised the machine.

---

## 4. Users and Use Cases

### 4.1 Personas

#### Persona A: The security-conscious indie developer

Runs Claude Code on a personal laptop across side projects and contract work. Has had at least one close call where an agent attempted to read a `.env` or called a destructive command. Wants an always-on guardrail without giving up the productivity of agentic tools.

#### Persona B: The consultant with multiple clients

Works across several client codebases in a given week. Each client has its own credentials, API keys, and sensitive data. Needs hard boundaries so that a session in Client A's repo cannot see Client B's `.env` files, regardless of how "helpful" the agent tries to be.

#### Persona C: The platform engineer at a regulated company

Responsible for approving AI coding agent use across a 50- to 500-person engineering team. Needs a defensible story for the security and compliance teams: centralized policy, audit logs, and provable boundaries. Cares deeply about reproducibility and CI integration in the long run.

### 4.2 Primary use cases

#### UC1. Scoped agent session on a project

```bash
cd ~/code/client-a/app
aixgate run --profile claude-code -- claude
```

The agent launches inside a sandbox where only `~/code/client-a/app` is readable and writable, plus a small set of system paths required for tooling. The user's home directory, other clients' projects, SSH keys, AWS credentials, and personal documents return as if they do not exist.

#### UC2. Per-project policy

A project repository contains a `.aixgate.yaml` file declaring which files the agent may access and which commands it may run. When `aixgate run` is invoked inside that project, the policy is loaded automatically. The policy is checked into version control and code-reviewed.

#### UC3. Audit and review

```bash
aixgate audit tail
aixgate audit query --since 24h --denied-only
```

The developer can at any time review every read, write, and exec decision Aixgate made. A security team can ship these logs to a SIEM for monitoring.

#### UC4. Profile-based defaults

Out of the box, Aixgate ships with profiles for popular agents (Claude Code, Aider, Cursor headless, generic). Each profile encodes the minimum access that agent needs to function, which users can tighten or loosen as desired.

---

## 5. Product Overview

### 5.1 How it works in one paragraph

Aixgate launches the target agent as a child process inside a filesystem namespace (Linux) or sandbox profile (macOS) that presents a FUSE overlay of the real filesystem. The overlay consults the active policy for every read, write, and lookup operation. Allowed paths pass through transparently. Denied paths return `ENOENT`, making them invisible to the agent. Sensitive paths can optionally return redacted stubs. Every decision is written to a structured audit log. The agent cannot escape this boundary without subverting the OS process isolation primitives Aixgate builds on.

### 5.2 User-facing surface

#### CLI commands

```bash
aixgate run [--profile NAME] [--policy FILE] -- CMD [ARGS...]
```
Launches a command inside a sandbox. Policy is resolved in order: explicit `--policy` flag, project-local `.aixgate.yaml`, named profile, built-in default.

```bash
aixgate init
```
Creates a `.aixgate.yaml` in the current directory with safe starter defaults for the detected project type.

```bash
aixgate policy check [FILE]
```
Validates policy syntax and reports any unresolvable globs or unsafe rules (such as overly broad allowlists).

```bash
aixgate policy explain PATH
```
For a given file path, explains what the active policy would do: allow, deny, redact, or prompt.

```bash
aixgate audit tail [--follow] [--profile NAME]
aixgate audit query --since 24h [--denied-only] [--path GLOB]
```
Tail or search the audit log.

```bash
aixgate profile list
aixgate profile show NAME
```
Inspect available built-in and user profiles.

```bash
aixgate doctor
```
Diagnostic command that verifies FUSE is installed, permissions are correct, and the sandbox can be established. Outputs a remediation checklist for common setup issues.

---

## 6. Architecture

### 6.1 Component diagram

Aixgate is a single Go binary composed of the following internal components:

| Component | Responsibility |
|---|---|
| **CLI** (`cmd/aixgate`) | Argument parsing, user-facing commands, orchestration. |
| **Policy engine** (`internal/policy`) | Load, validate, and evaluate policies. Returns allow / deny / redact / prompt decisions for a given access request. |
| **Overlay filesystem** (`internal/fs`) | FUSE-based filesystem that mediates every syscall through the policy engine. Implements path canonicalization, redaction stubs, and `ENOENT` masking. |
| **Process jail** (`internal/jail`) | Platform-specific process isolation: mount namespaces and landlock on Linux, `sandbox-exec` on macOS. Responsible for chroot, environment stripping, and `PATH` restriction. |
| **Audit log** (`internal/audit`) | Structured JSONL writer with rotation. Supports live tailing and query. |
| **Profile registry** (`internal/profiles`) | Built-in profiles shipped with the binary, plus user profile discovery in `~/.aixgate/profiles`. |
| **Prompt daemon** (`internal/prompt`, v1.1) | Desktop notifications for interactive allow/deny prompts when policy specifies `prompt` on deny. |

### 6.2 Suggested repository layout

```
aixgate/
├── cmd/
│   └── aixgate/                 # CLI entry point
│       └── main.go
├── internal/
│   ├── policy/                 # Parse + evaluate policies
│   │   ├── schema.go           # Policy struct, YAML tags
│   │   ├── loader.go           # Load, merge, extend
│   │   ├── matcher.go          # Glob + rule evaluation
│   │   ├── matcher_test.go
│   │   └── rego.go             # Optional OPA integration (v2)
│   ├── fs/                     # FUSE filesystem
│   │   ├── overlay.go          # Core overlay logic (cgofuse)
│   │   ├── redact.go           # Stub/redaction generation
│   │   ├── canonical.go        # Path canonicalization
│   │   ├── darwin.go           # macOS-specific FUSE setup
│   │   └── linux.go            # Linux-specific FUSE setup
│   ├── jail/                   # Process isolation
│   │   ├── jail.go             # Cross-platform interface
│   │   ├── linux.go            # namespaces, landlock
│   │   └── darwin.go           # sandbox-exec profile generation
│   ├── audit/                  # Structured logging
│   │   ├── writer.go           # JSONL writer with rotation
│   │   ├── query.go            # Query + tail
│   │   └── schema.go
│   ├── profiles/               # Profile registry
│   │   ├── registry.go
│   │   └── builtin/            # Embedded default profiles
│   │       ├── claude-code.yaml
│   │       ├── aider.yaml
│   │       ├── cursor.yaml
│   │       └── generic.yaml
│   ├── proc/                   # Process lifecycle, signals
│   │   └── supervisor.go
│   └── prompt/                 # Desktop prompts (v1.1)
│       ├── prompt.go
│       ├── darwin.go           # osascript
│       └── linux.go            # notify-send
├── pkg/                        # Public Go API (for aixgo integration)
│   └── aixgate/
│       └── client.go
├── profiles/                   # Shipped example profiles (not embedded)
├── docs/
├── scripts/
├── test/
│   └── e2e/                    # End-to-end tests with real agents
├── .github/
│   └── workflows/
├── go.mod
├── go.sum
├── Makefile
├── LICENSE                     # Apache 2.0
├── CLAUDE.md                   # Instructions for Claude Code contributors
└── README.md
```

### 6.3 Enforcement boundary

#### Linux

- A mount namespace (`unshare(CLONE_NEWNS)`) is created for the agent process.
- A FUSE filesystem is mounted inside the namespace at the agent's perceived root, backed by `cgofuse`.
- Landlock is applied where available (Linux 5.13+) as a belt-and-braces restriction even if the FUSE layer is somehow bypassed.
- A restricted `PATH` and a filtered environment are injected.

#### macOS

- macFUSE provides the FUSE filesystem layer.
- `sandbox-exec` with a Aixgate-generated `.sb` profile enforces system-level restrictions: which binaries may be exec'd, which directories may be traversed, and denies network access optionally.
- The Aixgate binary is signed and notarized to avoid Gatekeeper friction.

### 6.4 Policy evaluation flow

For every filesystem operation the agent performs, Aixgate follows this sequence:

1. **Canonicalize** the path, resolving symlinks and `..` components against the real filesystem root.
2. **Match** the canonical path against the policy's explicit `deny` rules. If matched, return `ENOENT` (for reads and lookups) or `EACCES` (for writes), and log the denial.
3. **Match** against `redact` rules. If matched, return a synthesized stub instead of real contents.
4. **Match** against `allow` rules. If matched, pass through to the real filesystem.
5. **Otherwise**, apply the default action (`deny` for v1). Log the decision.

### 6.5 Why `ENOENT` and not `EACCES` by default

Returning "permission denied" signals to the agent that a file exists but it cannot access it. Well-behaved agents may retry, prompt the user, or report an error that reveals the path. Returning "no such file or directory" gives the agent no information: it proceeds as if the file does not exist.

This choice is deliberate and important; it is why Aixgate is effective against agents that might otherwise try to work around an obstacle. Policies can opt into `EACCES` semantics per rule for cases where the developer wants visibility.

### 6.6 Threat model specifics

**In scope:**
- A benign agent that is tricked by prompt injection into attempting to read sensitive files.
- A buggy agent plan that inadvertently tries to touch sensitive files.
- An agent that attempts to `curl` or `scp` credentials out — Aixgate prevents the read; exfiltration is moot.
- Forked subprocesses (`bash -c "cat .env"`) — the sandbox is inherited via process isolation, not FUSE alone.

**Out of scope:**
- An attacker with root privileges on the machine.
- An attacker with local code execution outside the Aixgate-launched process.
- Kernel-level exploits.
- Side-channel attacks (timing, filesystem metadata inference).

---

## 7. Policy Specification

### 7.1 Design principles

- **Readable by humans.** YAML, not Rego or CUE for v1. Rego may be added as an advanced-user option in v2.
- **Portable.** The same policy runs identically on macOS and Linux.
- **Composable.** Profiles can extend other profiles. Project policies can extend a profile.
- **Explicit.** No wildcarded inheritance; every effective rule can be traced to a source file and line.

### 7.2 Example policy

```yaml
profile: claude-code
extends: defaults/generic
default: deny

filesystem:
  allow_read:
    - ${PROJECT}/**
    - ~/.config/git/**
    - /usr/lib/**
    - /opt/homebrew/**
  allow_write:
    - ${PROJECT}/**
    - ${TMPDIR}/aixgate-*/**
  redact:
    - '**/.env'
    - '**/.env.*'
    - ~/.aws/credentials
  deny:
    - ~/.ssh/id_*
    - ~/Documents/**

exec:
  allow: [git, node, npm, pnpm, python3, go, make]
  deny_args:
    git: ['push --force', 'config --global']

env:
  pass_through: [PATH, HOME, LANG, LC_ALL, TERM]
  inject:
    AIXGATE_ACTIVE: '1'

audit:
  log: ~/.aixgate/audit.jsonl
  on_deny: log   # log | prompt | block_silently
```

### 7.3 Precedence rules

When a path matches multiple rules, precedence is:

```
explicit deny  >  redact  >  allow_write  >  allow_read  >  default
```

**Deny always wins.** This is unsurprising, matches firewall conventions, and prevents accidental widening.

### 7.4 Variables

| Variable | Resolves to |
|---|---|
| `${PROJECT}` | The directory `aixgate run` was invoked from. |
| `${HOME}` | The current user's home directory. |
| `${TMPDIR}` | The current user's temp directory. |
| `${AIXGATE_CONFIG_DIR}` | `~/.aixgate`. |

### 7.5 Glob syntax

Aixgate uses [doublestar](https://github.com/bmatcuk/doublestar) semantics:

- `*` matches any non-slash characters.
- `**` matches any number of path segments including zero.
- `?` matches a single character.
- `[abc]` matches a character class.
- `{foo,bar}` matches alternatives.

### 7.6 Policy schema (formal)

```yaml
# JSON Schema equivalent in YAML form
profile: string                         # Required. Profile name.
extends: string                         # Optional. Another profile to extend.
default: "deny" | "allow"               # Default action. v1 recommends deny.

filesystem:
  allow_read: [glob]                    # Paths agent may read.
  allow_write: [glob]                   # Paths agent may write.
  redact: [glob]                        # Paths returned as redacted stubs.
  deny: [glob]                          # Paths always denied (highest precedence).

exec:
  allow: [string]                       # Allowed command basenames.
  deny: [string]                        # Denied command basenames.
  deny_args:
    <command>: [string]                 # Denied argument patterns per command.

env:
  pass_through: [string]                # Env vars passed through.
  inject:
    <key>: <value>                      # Env vars injected.
  strip: [string]                       # Env vars explicitly stripped.

audit:
  log: path                             # Audit log file path.
  on_deny: "log" | "prompt" | "block_silently"
  on_allow: "log" | "none"              # Default: log
```

---

## 8. Audit Log

### 8.1 Format

One JSON object per line (JSONL), structured to be trivially queryable with `jq`, `grep`, or ingested into a SIEM.

```json
{"ts":"2026-04-20T10:12:33Z","profile":"claude-code","pid":4421,"session_id":"01HV2K...","op":"read","path":"/Users/cg/code/project/.env","decision":"redact","rule":"redact:**/.env","agent_cmd":"claude"}
```

### 8.2 Fields

| Field | Description |
|---|---|
| `ts` | ISO 8601 timestamp, UTC. |
| `profile` | Active profile name. |
| `pid` | PID of the process requesting access. |
| `session_id` | Unique identifier for this `aixgate run` invocation (ULID). |
| `op` | Operation: `read`, `write`, `stat`, `exec`, `unlink`, `rename`. |
| `path` | Canonicalized absolute path. |
| `decision` | `allow`, `deny`, `redact`, or `prompt`. |
| `rule` | Identifier of the rule that produced the decision. |
| `agent_cmd` | Top-level command that was launched by `aixgate run`. |

### 8.3 Retention and rotation

Audit logs rotate at 100MB by default, keeping 10 files. Configurable via `~/.aixgate/config.yaml`. Aixgate does not delete logs automatically beyond rotation; retention is the user's decision.

### 8.4 Query examples

```bash
# Everything denied in the last day
aixgate audit query --since 24h --denied-only

# Every time .env was touched
aixgate audit query --path '**/.env*'

# Every exec call for a specific session
aixgate audit query --session-id 01HV2K... --op exec

# Live tail filtered to one profile
aixgate audit tail --follow --profile claude-code
```

---

## 9. Packaging and Distribution

### 9.1 Artifacts

- Signed macOS universal binary (arm64 + x86_64), distributed as a `.pkg` for macFUSE dependency management.
- Linux static binary for x86_64 and arm64, distributed as `.tar.gz` and `.deb`.
- Homebrew tap: `brew install aixgo/tap/aixgate`.
- Shell installer: `curl -sSL https://aixgo.dev/aixgate/install.sh | sh`.

### 9.2 Dependencies

| Dependency | Purpose |
|---|---|
| macFUSE (macOS) | Filesystem-in-userspace driver. Requires one-time kernel extension approval. |
| FUSE (Linux) | Available by default on most distributions. Package: `fuse3`. |
| cgofuse | Go bindings for FUSE used internally; vendored as a Go module dependency. |
| **No external runtime** | Aixgate does not require Docker, VMs, or a background daemon in v1. |

### 9.3 Licensing

Apache 2.0, matching aixgo.dev. Contributions accepted under a lightweight DCO (Developer Certificate of Origin).

---

## 10. Relationship to aixgo.dev

Aixgate is the security and runtime isolation primitive in the aixgo.dev ecosystem. The relationship is intentionally layered:

| aixgo.dev project | Role |
|---|---|
| **aixgo** (framework) | Build agents: orchestration patterns, LLM providers, memory, tools. |
| **Aixgate** | Run agents safely on developer workstations. Enforces filesystem and exec policy at the OS boundary. |
| **aixgo-cloud** (future) | Run agents safely in server and CI environments. Shares policy format with Aixgate. |

This layering gives aixgo.dev a credible answer to the single most common security question asked about agent frameworks: "how do I stop it from reading my credentials?" Aixgate's existence strengthens the framework's positioning for enterprise and security-conscious adopters, and in turn the framework's community distribution accelerates Aixgate's adoption.

### 10.1 Integration points

- aixgo agents can detect the `AIXGATE_ACTIVE=1` environment variable and surface sandboxed status in their UI, giving users confidence that their boundaries are being enforced.
- aixgo's built-in tool definitions (`file_read`, `shell_exec`) can optionally emit additional context into Aixgate's audit log via a local socket, enabling per-tool-call attribution rather than per-syscall.
- Shared policy schema: the same `.aixgate.yaml` file will eventually be interpretable by aixgo-cloud's server-side enforcement, enabling "write policy once, enforce everywhere."

### 10.2 Positioning notes

- Aixgate is **not** bundled with aixgo. An agent built on aixgo may run without Aixgate, and Aixgate sandboxes agents built on anything.
- The aixgo framework does **not** depend on Aixgate. The dependency is one-way and optional.
- Marketing message: "aixgo.dev builds agents. Aixgate keeps them in their lane."

---

## 11. Roadmap

### 11.1 v0.1 — Proof of concept (2 weeks)

- FUSE overlay on macOS and Linux that hides `.env`, `~/.ssh/id_*`, `~/.aws/credentials` by default.
- `aixgate run` command with hardcoded policy.
- No configuration, no audit log, no profiles. Single purpose: prove the enforcement boundary holds across at least three agents (Claude Code, Aider, a custom Go agent from aixgo).

### 11.2 v0.2 — Usable alpha (4 weeks)

- YAML policy file support.
- Built-in profiles for Claude Code, Aider, and a generic default.
- Structured audit log with `tail` and `query` commands.
- `aixgate doctor` diagnostic command.
- Published to Homebrew and Linux packages.

### 11.3 v1.0 — Public launch (3 months)

- Prompt-on-access with desktop notifications.
- Project-local `.aixgate.yaml` discovery and `extends:`-style composition.
- Signed installers, notarization, first-run UX polish.
- Documentation site, example policies, and integration guides for the top five agents.
- Launch blog post and BSides Tokyo talk.

### 11.4 v1.x and beyond

- Windows support via a WFP-based filesystem filter.
- Optional network egress policy via a local HTTPS proxy.
- Rego policy backend for organizations with existing OPA investment.
- Server-side variant for CI and Kubernetes: **aixgo-cloud** integration.
- eBPF-based enforcement on Linux as an optional belt-and-braces layer.
- Policy marketplace: community-contributed profiles for specific frameworks, languages, and cloud providers.

---

## 12. Success Metrics

### 12.1 Adoption metrics

- GitHub stars on the `aixgate` repo: **500** within 3 months of v1.0 launch; **2,500** within 12 months.
- Homebrew install count: **1,000 weekly active installs** within 6 months of v1.0.
- At least **5 community-contributed profiles** for agents not authored by the core team.

### 12.2 Quality metrics

- **Zero known policy-bypass vulnerabilities** in v1.0. A responsible disclosure policy is in place.
- **False positive rate** (legitimate agent operations incorrectly denied): under **5%** on the default Claude Code profile, measured against a benchmark of 50 common coding tasks.
- **p95 read latency overhead** versus native filesystem access: under **10ms** on an NVMe SSD.
- **Startup overhead** of `aixgate run` versus direct launch: under **500ms**.

### 12.3 Strategic metrics

- Aixgate is cited in at least one public post-mortem where it prevented credential exposure.
- Aixgate drives measurable traffic and conversion to aixgo.dev the framework, as tracked by referrer analytics on docs and GitHub.
- At least one enterprise adopter references Aixgate in a security questionnaire response.

---

## 13. Risks and Open Questions

### 13.1 Technical risks

| Risk | Impact | Mitigation |
|---|---|---|
| macFUSE install friction drives users away on day one. | High | Invest in first-run UX: `aixgate doctor`, clear error messages, signed `.pkg` installer. Accept this as the main UX cost and budget for it. |
| Agents fork subprocesses that escape the sandbox. | High | Rely on mount namespaces (Linux) and `sandbox-exec` child inheritance (macOS) rather than FUSE-only enforcement. Add landlock where available. |
| Symlink and path traversal bugs in the overlay leak data. | High | Canonicalize before every policy check. Comprehensive test suite including adversarial traversal cases. Third-party security review before v1.0. |
| Policy complexity leads to footgun allowlists. | Medium | `aixgate policy check` warns on overly broad rules. Ship safe defaults. Document common patterns. |
| FUSE performance overhead breaks agent UX. | Medium | Benchmark against native. Cache policy decisions per path per session. Optimize hot paths. |

### 13.2 Product risks

| Risk | Impact | Mitigation |
|---|---|---|
| Agent vendors build their own sandboxing, making Aixgate redundant. | Medium | Position as defense-in-depth and cross-vendor. A per-vendor solution will always leave users with the "which one am I running today" problem Aixgate solves. |
| Users disable Aixgate because the default profile is too strict. | High | Ship permissive-but-safe defaults. Make it easy to allow specific paths on the fly. Prioritize the "it just works" path for popular agents. |
| Open source positioning conflicts with future commercial offering. | Medium | Commit clearly that the core enforcement and policy engine are Apache 2.0 forever. Commercial layer, if any, is around managed policy distribution, SIEM integration, or enterprise support — not gating core features. |

### 13.3 Open questions

- Should we support a "learning mode" that observes agent behavior and suggests policy? Useful for first-time users but potentially a security anti-pattern.
- How do we handle agents that legitimately need a one-off read of a sensitive file (for example, reading a `.env` to verify a variable is present)? Redaction? Prompt? Neither?
- Should the audit log be tamper-evident (hash chain, append-only) by default, or is that over-engineering for v1?
- Is there a meaningful difference between "Aixgate" as a standalone product and `aixgo aixgate` as a subcommand of an eventual `aixgo` CLI? Naming decision to revisit before launch.
- Should `.aixgate.yaml` support environment-specific overrides (dev vs staging vs demo contexts)?

---

## 14. Implementation Notes for Contributors

> This section is intended to be read by Claude Code and human contributors working in the aixgo.dev repo.

### 14.1 Core libraries

| Concern | Library | Notes |
|---|---|---|
| CLI | [`spf13/cobra`](https://github.com/spf13/cobra) | Consistent with aixgo's existing CLI patterns. |
| Config | [`spf13/viper`](https://github.com/spf13/viper) + [`goccy/go-yaml`](https://github.com/goccy/go-yaml) | YAML parsing with line-number errors. |
| FUSE | [`winfsp/cgofuse`](https://github.com/winfsp/cgofuse) | Cross-platform FUSE bindings. Works on macOS via macFUSE, Linux via libfuse3. |
| Globs | [`bmatcuk/doublestar`](https://github.com/bmatcuk/doublestar) | Supports `**` and brace expansion. |
| Logging | `log/slog` (stdlib) | Structured logs by default. |
| Audit | `log/slog` JSONHandler + `lumberjack` for rotation | |
| ULIDs | [`oklog/ulid`](https://github.com/oklog/ulid) | Session IDs. |
| Testing | stdlib `testing` + [`stretchr/testify`](https://github.com/stretchr/testify) | Match aixgo conventions. |

### 14.2 Key design decisions to respect

1. **The policy engine is pure.** Given a policy and an access request, it returns a decision. It does not perform I/O, log, or have side effects. This makes it trivially testable.
2. **The FUSE overlay only implements the minimum filesystem operations needed.** Prefer refusing exotic operations (`ioctl`, xattr) to implementing them partially.
3. **Path canonicalization happens exactly once per request**, at the top of the policy check. Never trust a path that has not been canonicalized.
4. **Audit log writes are fire-and-forget** from the FUSE hot path. Use a buffered channel; block on it only if the buffer is full, and prefer to drop with a counter metric rather than stall the agent.
5. **Platform-specific code lives in files suffixed `_darwin.go` / `_linux.go`.** Shared logic lives in the unsuffixed file. Use Go build tags only where necessary.
6. **Avoid cgo outside `internal/fs`.** cgofuse is the only acceptable cgo dependency. Everything else must be pure Go.

### 14.3 Testing strategy

- **Unit tests** for the policy engine (`internal/policy`). Exhaustive table-driven tests covering precedence, variable substitution, glob semantics, and malformed input.
- **Integration tests** for the FUSE overlay using a temporary mount and a known policy. Assert that specific reads return `ENOENT`, specific writes fail, and allowed paths pass through.
- **Adversarial tests** for path canonicalization: symlink loops, `..` traversal, null bytes, unicode normalization edge cases.
- **End-to-end tests** in `test/e2e` that launch real agents (or scripted agent mocks) inside Aixgate and verify behavior. These are slow and live outside the default `go test ./...`.
- **Benchmarks** for policy decision latency and FUSE throughput.

### 14.4 Coding conventions

- Follow `gofmt`, `goimports`, and `golangci-lint` with the aixgo.dev shared config.
- Errors wrap with `fmt.Errorf("context: %w", err)`; never bare-return.
- No `init()` functions except for registering cobra commands and embedded profiles.
- Every exported function in `pkg/aixgate` has a godoc comment starting with the function name.
- Internal packages (`internal/...`) are not stable API; do not depend on them from outside the repo.

### 14.5 Security review requirements

Before v1.0:
- Third-party security review of the FUSE overlay and path canonicalization.
- Fuzz testing of the policy parser and glob matcher.
- Threat model review against the documented boundary in §6.6.
- Responsible disclosure policy (`SECURITY.md`) and a signed PGP key for vulnerability reports.

### 14.6 Documentation requirements

- `README.md` — installation, quickstart, link to docs site.
- `docs/` — full user documentation, policy cookbook, profile authoring guide.
- `CLAUDE.md` — instructions for Claude Code contributors (architecture overview, dev loop, testing commands, PR conventions).
- `SECURITY.md` — threat model, reporting process.
- `CONTRIBUTING.md` — DCO, code style, PR process.
- Per-profile docs in `docs/profiles/` explaining the rationale for each rule in each default profile.

---

## 15. v0.1 Weekend Build Plan

A concrete, scoped proof-of-concept that a contributor (human or Claude Code) can complete in a weekend. The goal is to validate the enforcement boundary and the UX hypothesis, not to ship a product.

### 15.1 Scope

1. Single Go binary named `aixgate`.
2. One command: `aixgate run -- CMD [ARGS...]`.
3. Hardcoded policy: hide every file matching `**/.env`, `**/.env.*`, `~/.ssh/id_*`, `~/.aws/credentials` by returning `ENOENT`. Everything else passes through.
4. macOS + Linux support (whichever the contributor is on is fine for the PoC; the other can be stubbed).
5. No config, no audit log, no profiles, no tests beyond a minimal smoke check.

### 15.2 Milestones

1. **Mount a pass-through FUSE overlay.** `aixgate run -- ls ~` works and shows the real home directory via the FUSE mount. Proves the basic plumbing.
2. **Add the deny rules.** `aixgate run -- cat ~/.ssh/id_rsa` returns "No such file or directory." `aixgate run -- cat ~/.aws/credentials` same. `aixgate run -- ls ~/.ssh` does not show `id_*` files.
3. **Verify against a real agent.** Launch Claude Code inside `aixgate run` in a directory containing a `.env`. Confirm the agent reports it cannot find the file, and does not surface the contents in any form.
4. **Verify subprocess containment.** `aixgate run -- bash -c "cat .env"` also fails, proving that forked children inherit the sandbox.

### 15.3 Definition of done for v0.1

- [ ] `go build ./cmd/aixgate` produces a working binary on the contributor's platform.
- [ ] The four hardcoded paths are provably hidden from shell commands launched via `aixgate run`.
- [ ] At least one real AI agent (Claude Code preferred) has been tested inside the sandbox and its logs/UI confirm it cannot see the protected paths.
- [ ] A 200–400 word writeup of what worked, what didn't, and what the UX surprises were. This becomes the basis for the launch blog post and informs v0.2.

### 15.4 Out of scope for v0.1

Everything else in this PRD. Resist the urge to build the policy engine, audit log, profiles, `aixgate doctor`, installers, or documentation beyond a two-paragraph README. The goal is to learn, not to ship.

---

## 16. Appendix

### 16.1 Glossary

| Term | Meaning |
|---|---|
| Agent | A process driven by an LLM that reads and writes files, runs commands, and otherwise acts on the user's behalf. |
| FUSE | Filesystem in Userspace. Lets a userspace program provide a filesystem that the kernel exposes to other processes. |
| Landlock | A Linux kernel feature (5.13+) that lets a process voluntarily drop filesystem access, used as defense in depth. |
| `sandbox-exec` | A macOS tool that launches a process under a TrustedBSD sandbox profile. Used extensively by Apple's own system components. |
| Profile | A named set of Aixgate policy defaults, typically shipped with the binary and specialized per agent. |
| Policy | The effective rule set governing a Aixgate session, composed from profile defaults and project-local overrides. |
| Canonicalization | The process of resolving a path to its absolute, symlink-free, `..`-free form before policy evaluation. |
| ULID | Universally unique, lexicographically sortable identifier. Used for session IDs. |

### 16.2 Prior art

- **Aqua Security, Falco, and Tetragon** for container runtime protection. These informed the "enforcement at the OS boundary, not the application" thesis.
- **Bubblewrap and Flatpak** on Linux for user-space process isolation primitives.
- **Apple's `sandbox-exec` and App Sandbox** for the macOS enforcement model.
- **Deno's permission model** as an example of what built-in, policy-based runtime security looks like when done well.
- **Docker's `--read-only` and `tmpfs` mount patterns** for the "present a filtered filesystem view" primitive.
- **OpenBSD's `pledge(2)` and `unveil(2)`** for the general design principle of restricting a process to the minimum it needs.

### 16.3 References and further reading

- [macFUSE documentation](https://osxfuse.github.io/)
- [Linux FUSE documentation](https://www.kernel.org/doc/html/latest/filesystems/fuse.html)
- [Landlock LSM](https://landlock.io/)
- [cgofuse](https://github.com/winfsp/cgofuse)
- [OWASP LLM Top 10 — Prompt Injection](https://owasp.org/www-project-top-10-for-large-language-model-applications/)
- [Apple sandbox profile language reference](https://reverse.put.as/wp-content/uploads/2011/09/Apple-Sandbox-Guide-v1.0.pdf) (unofficial)

### 16.4 Document history

| Version | Date | Author | Notes |
|---|---|---|---|
| 0.1 | April 2026 | Charles Green | Initial draft. |

---

**End of document.**
