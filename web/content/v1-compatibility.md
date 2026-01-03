---
title: 'Aixgo v1 Compatibility Promise'
description: 'Our commitment to API stability, upgrade paths, and long-term maintenance for Aixgo v1.0 and beyond.'
---

# Aixgo v1 Compatibility Promise

## Introduction

Aixgo v1.0 will establish a foundation for long-term stability. Code written against the v1 specification—whether YAML workflow configurations, Go SDK usage, or protocol
integrations—will continue to work correctly with all future v1.x releases.

This document defines what "compatibility" means, what is covered by our guarantee, and how Aixgo will evolve while honoring this commitment. This stability commitment reflects our
[core philosophy](/why-aixgo) that production AI deserves production-grade tooling.

## The Compatibility Promise

When Aixgo reaches v1.0, we commit that:

**YAML workflow configurations** written for v1.0 will execute correctly on all v1.x releases without modification. Your declarative agent definitions, supervisor patterns, and
orchestration workflows will remain stable.

**Public Go SDK APIs** exposed in the `github.com/aixgo-dev/aixgo` module will maintain source-level compatibility. Code that compiles against v1.0 will continue to compile against
all v1.x versions.

**gRPC and MCP protocol wire formats** will remain backward-compatible. Services built on v1.0 will interoperate with all v1.x releases, enabling incremental upgrades in
distributed deployments.

While we expect the vast majority of programs will maintain this compatibility, it is impossible to guarantee that no future change will break any program. This document catalogs
exceptions to our compatibility promise.

## What Is Covered

### YAML Workflow Configurations

All documented YAML schema elements for workflow definitions are stable:

- Agent type definitions (`role: react`, `role: producer`, etc.)
- Supervisor configuration (`max_rounds`, `model`, etc.)
- Input/output routing (`inputs`, `outputs`, `source`, `target`)
- Model provider specifications (`model`, `temperature`, `max_tokens`)
- Tool definitions (`tools`, `input_schema`, `description`)
- Orchestration patterns (sequential, parallel, reflection, etc.)

**Example of guaranteed stability:**

```yaml
supervisor:
  name: coordinator
  model: gpt-4-turbo
  max_rounds: 10

agents:
  - name: analyzer
    role: react
    model: gpt-4-turbo
    prompt: 'Analyze incoming data'
    tools:
      - name: query_database
        description: 'Query the database'
        input_schema:
          type: object
          properties:
            query: { type: string }
```

This configuration will execute identically across all v1.x releases.

### Go SDK Public APIs

All exported types, functions, and methods in the `github.com/aixgo-dev/aixgo` module are covered:

- Core types (`Agent`, `Supervisor`, `Message`, `Tool`)
- Configuration builders (`NewSupervisor`, `NewAgent`, `WithModel`)
- Lifecycle methods (`Run`, `Stop`, `AddAgent`)
- Tool registration interfaces (`RegisterTool`, `ToolHandler`)
- Context and observability hooks

**Example of guaranteed API:**

```go
import "github.com/aixgo-dev/aixgo"

supervisor := aixgo.NewSupervisor("coordinator")
supervisor.AddAgent(analyzer)
supervisor.Run(context.Background())  // Signature stable
```

### Protocol Wire Formats

The serialized message formats for gRPC and MCP are guaranteed:

- gRPC service definitions (`.proto` files)
- MCP transport protocol messages
- Message envelope structures
- Authentication and authorization headers

Clients and servers built on v1.0 can communicate with v1.x counterparts.

## What Is Excluded

The following are explicitly **not** covered by the compatibility guarantee:

### 1. Security Fixes

If a security vulnerability is discovered, we will fix it even if doing so breaks compatibility. Security always takes precedence over backward compatibility.

**Rationale:** Protecting users from exploits is more important than API stability. We will document breaking security changes in release notes with migration guidance.

### 2. Experimental Features

Features marked as **alpha** or **beta** in documentation may change or be removed:

- Functions with `// Experimental: ...` comments
- YAML configuration fields documented as "alpha" or "beta"
- Features in packages suffixed with `/alpha` or `/beta`

**How to identify experimental features:**

```go
// Experimental: MultiModalInput may change in future releases
type MultiModalInput struct { ... }
```

```yaml
# Beta: Vision capabilities are under active development
agents:
  - name: image-analyzer
    role: vision # Beta feature
```

**Guidance:** Avoid experimental features in production code until they graduate to stable status.

### 3. Internal Packages

Packages under `internal/` directories are not covered. These are implementation details subject to change without notice.

**Example:**

- `github.com/aixgo-dev/aixgo/internal/executor` - Not stable
- `github.com/aixgo-dev/aixgo/internal/transport` - Not stable

### 4. Bugs and Unspecified Behavior

Fixing bugs may break programs that depend on incorrect behavior. Similarly, behaviors not explicitly documented are subject to change.

**Example:** If a YAML parser incorrectly accepts malformed input, fixing this bug may break configurations that relied on the bug.

### 5. Performance Characteristics

We do not guarantee execution speed, memory usage, or resource consumption. Optimizations may change performance profiles between releases.

**Rationale:** Optimizations improve user experience but should not affect functional correctness. Your tests should validate behavior, not performance, unless you explicitly need
performance contracts.

### 6. Tooling and CLI

The `aixgo` CLI tool, code generators, and development utilities may change:

- Command-line flags and arguments
- Output formats (except stable machine-readable formats like JSON)
- Error messages and logging

**Guidance:** For automation, use the Go SDK or gRPC APIs rather than parsing CLI output.

### 7. External Dependencies

Compatibility with third-party LLM providers, vector databases, or external services is not guaranteed. If OpenAI changes their API, Aixgo may update its integration accordingly.

**Rationale:** We cannot control external service evolution. We will minimize disruption but cannot guarantee zero-impact migrations.

### 8. Unkeyed YAML Struct Literals

Adding new fields to YAML configurations may break workflows that rely on field ordering in unkeyed structs.

**Not recommended:**

```yaml
agents:
  - analyzer # Unkeyed positional field
  - react # Order-dependent
  - gpt-4-turbo
```

**Recommended:**

```yaml
agents:
  - name: analyzer
    role: react
    model: gpt-4-turbo
```

## How Aixgo Will Evolve

### Adding Features

New capabilities will be introduced through:

1. **New agent types** - Additional roles beyond the v1.0 set
2. **Optional configuration fields** - New YAML fields with sensible defaults
3. **New SDK functions** - Additional exported functions and types
4. **Protocol extensions** - New gRPC methods or MCP message types

**Backward compatibility:** Existing code will not need modification. New features are opt-in.

### Deprecation Policy

When features need to be replaced:

1. **Deprecation notice** - Feature marked deprecated with migration guidance
2. **Grace period** - Minimum 12 months of continued support
3. **Removal** - Only in major version bump (v2.0)

**Example:**

```go
// Deprecated: Use NewSupervisorWithOptions instead.
// This function will be removed in v2.0.
func NewSupervisor(name string) *Supervisor { ... }
```

### Semantic Versioning

Aixgo follows strict semantic versioning:

- **v1.x.y** - Patch releases (bug fixes, security patches)
- **v1.x.0** - Minor releases (new features, backward-compatible)
- **v2.0.0** - Major releases (breaking changes)

## Writing Future-Proof Code

Follow these guidelines to ensure your code remains compatible:

### 1. Use Keyed YAML Fields

Always specify field names explicitly:

```yaml
# Good
agents:
  - name: analyzer
    role: react
    model: gpt-4-turbo

# Bad - relies on field ordering
agents:
  - analyzer
  - react
  - gpt-4-turbo
```

### 2. Avoid Experimental Features

Check documentation for "alpha," "beta," or "experimental" warnings:

```go
// Good - stable API
supervisor := aixgo.NewSupervisor("coordinator")

// Bad - experimental API
supervisor := aixgo.NewExperimentalSupervisor("coordinator")  // May change
```

### 3. Depend on Public APIs Only

Import from `github.com/aixgo-dev/aixgo`, not `internal/` packages:

```go
// Good
import "github.com/aixgo-dev/aixgo"

// Bad - internal package, not stable
import "github.com/aixgo-dev/aixgo/internal/executor"
```

### 4. Handle Errors Gracefully

Don't assume specific error messages or types unless documented:

```go
// Good - handle any error
if err := supervisor.Run(ctx); err != nil {
    return fmt.Errorf("supervisor failed: %w", err)
}

// Bad - parsing error messages is fragile
if err != nil && strings.Contains(err.Error(), "timeout") {
    // Error message format not guaranteed
}
```

### 5. Pin to Minor Versions

Use Go modules to control upgrade cadence:

```go
// go.mod
require github.com/aixgo-dev/aixgo v1.2.3  // Specific version
```

For production, pin to a specific minor version and test before upgrading.

## Migration Path from Alpha

When v1.0 is released, we will provide:

1. **Migration guide** - Detailed changelog of breaking changes from v0.x
2. **Automated migration tools** - CLI tools to update YAML configs and Go code
3. **Deprecation warnings** - Advance notice of features removed in v1.0
4. **Extended support** - v0.x will receive critical security patches for 6 months after v1.0 release

## Reporting Compatibility Issues

If you encounter a compatibility regression:

1. **File an issue** on GitHub with reproduction steps
2. **Include version numbers** for both working and broken versions
3. **Provide minimal examples** demonstrating the regression

We treat compatibility issues as high-priority bugs. Regressions will be fixed in patch releases.

## Conclusion

The v1.0 compatibility promise is our commitment to building a stable foundation for production AI agent systems. By clearly defining what is covered and what is excluded, we aim
to balance innovation with reliability.

Your YAML workflows, Go integrations, and distributed deployments built on v1.0 will continue to work as Aixgo evolves. We take this responsibility seriously and will honor this
commitment throughout the v1.x lifecycle.

For questions about compatibility or to discuss migration strategies, join our [Discord community](https://discord.gg/aixgo) or open a discussion on GitHub.
