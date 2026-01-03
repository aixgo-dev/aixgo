---
title: "Aixgo v0.2.3 & v0.2.4: Phased Agent Startup with Dependency Ordering"
date: 2026-01-03
description: "Introducing dependency-aware agent startup that eliminates race conditions in multi-agent systems using topological sort and phased initialization."
tags: ["release", "runtime", "startup", "dependencies", "orchestration"]
author: "Aixgo Team"
---

We're excited to announce **Aixgo v0.2.3** and **v0.2.4**, featuring a major enhancement to the runtime system: **phased agent startup with dependency ordering**. This powerful new capability eliminates a common class of race conditions in multi-agent systems and makes startup behavior both predictable and safe.

## The Problem: Agent Startup Race Conditions

In multi-agent systems, orchestrator agents often need to check if their dependencies are ready during their own `Start()` method. However, when all agents start concurrently, there's no guarantee that dependencies have finished initializing before orchestrators try to use them.

### Before v0.2.3

```go
// Orchestrator starts at the same time as its dependencies
rt.Register(databaseAgent)      // Dependency
rt.Register(cacheAgent)          // Dependency
rt.Register(orchestratorAgent)   // Needs both above

// All start concurrently - race condition!
rt.Start(ctx)
```

**Problem**: The orchestrator might call `runtime.Call()` to check dependencies during its `Start()`, but those agents might not be ready yet, causing errors or requiring complex retry logic.

## The Solution: Phased Startup

v0.2.3 introduces **phased agent startup** powered by topological sort (Kahn's algorithm). Agents are automatically grouped into dependency levels and started in phases, with each phase completing before the next begins.

### How It Works

1. **Declare Dependencies**: Use the `depends_on` field in your agent definitions
1. **Automatic Ordering**: Aixgo builds a dependency graph and computes startup phases
1. **Phase-Based Execution**: Agents start in phases (0, 1, 2, ...) based on their dependencies
1. **Concurrent Within Phases**: Agents in the same phase start concurrently for performance
1. **Ready Polling**: Each phase waits for all agents to be `Ready()` before proceeding

### After v0.2.3

```yaml
agents:
  - name: database
    role: producer
    # No dependencies - Phase 0

  - name: cache
    role: producer
    depends_on: [database]
    # Depends on Phase 0 agent - Phase 1

  - name: api
    role: react
    depends_on: [database, cache]
    # Depends on Phase 0 & 1 agents - Phase 2
```

**Result**: Guaranteed startup order with no race conditions. The database starts first, then the cache (once database is ready), then the API (once both are ready).

## Key Features

### 1. Topological Sort with Kahn's Algorithm

Aixgo uses a production-grade implementation of Kahn's algorithm to compute dependency order:

- **Cycle Detection**: Automatically detects and reports circular dependencies
- **Optimal Ordering**: Minimizes startup phases for fastest initialization
- **Parallel Execution**: Agents without dependencies on each other start concurrently

### 2. Configurable Timeout

Control how long to wait for agents to become ready:

```yaml
config:
  agent_start_timeout: 45s  # Default: 30s
```

If any agent doesn't report `Ready() == true` within the timeout, startup fails with a clear error message.

### 3. All Runtime Support

Phased startup works across all runtime implementations:

- **LocalRuntime**: Single-process deployments
- **SimpleRuntime**: Lightweight runtime for basic use cases
- **DistributedRuntime**: Multi-node orchestration via gRPC

### 4. Backward Compatible

If you don't specify `depends_on`, all agents start concurrently as before. Phased startup is opt-in and doesn't break existing configurations.

## Real-World Example

Here's a complete example showing a realistic multi-tier application:

```yaml
agents:
  # Tier 1: Foundational services (no dependencies)
  - name: config-service
    role: producer
    description: "Loads configuration from environment"

  - name: database
    role: producer
    description: "Connects to PostgreSQL"

  # Tier 2: Services depending on Tier 1
  - name: cache
    role: producer
    depends_on: [database, config-service]
    description: "Redis cache with DB fallback"

  - name: auth-service
    role: react
    depends_on: [database]
    description: "Authentication and JWT verification"

  # Tier 3: Application services depending on Tier 1 & 2
  - name: user-service
    role: react
    depends_on: [database, cache, auth-service]
    description: "User management API"

  - name: order-service
    role: react
    depends_on: [database, cache, auth-service]
    description: "Order processing logic"

  # Tier 4: Orchestrators depending on all application services
  - name: api-gateway
    role: react
    depends_on: [user-service, order-service]
    description: "Public API gateway"

config:
  agent_start_timeout: 60s
```

**Startup sequence**:

1. **Phase 0**: `config-service`, `database` (concurrent)
1. **Phase 1**: `cache`, `auth-service` (concurrent, after Phase 0 ready)
1. **Phase 2**: `user-service`, `order-service` (concurrent, after Phase 1 ready)
1. **Phase 3**: `api-gateway` (after Phase 2 ready)

## Implementation Details

### Dependency Graph Package

v0.2.3 adds a new `internal/graph` package with:

- **DependencyGraph**: Graph data structure for agent dependencies
- **Kahn's Algorithm**: Topological sort implementation
- **Cycle Detection**: Identifies circular dependencies
- **Error Handling**: Clear error messages for invalid configurations

```go
// Internal implementation (simplified)
graph := NewDependencyGraph()
graph.AddNode("database")
graph.AddNode("cache")
graph.AddEdge("cache", "database") // cache depends on database

phases, err := graph.TopologicalSort()
// phases = [[database], [cache]]
```

### Runtime Changes

All runtimes now implement the `PhasedStarter` interface:

```go
type PhasedStarter interface {
    StartAgentsPhased(ctx context.Context, phases [][]string) error
}
```

The `StartAgents()` method automatically detects dependencies and uses phased startup when `depends_on` is specified.

### Ready() Polling

After starting each phase, the runtime polls agent `Ready()` status:

1. Start all agents in the current phase concurrently
1. Wait for their `Start()` methods to complete
1. Poll `Ready()` every 100ms until all agents return `true`
1. If timeout expires, fail with error indicating which agents aren't ready
1. Proceed to next phase

## Version History

### v0.2.4 (January 3, 2026)

**Bug Fixes**:

- Fix test failures in agent and aixgo packages exposed by phased startup
- Add mutex to MockAgent to prevent race condition on ready field
- Fix errorAgent to use AgentDef name instead of hardcoded value

### v0.2.3 (January 2, 2026)

**Major Features**:

- Add phased agent startup with dependency ordering (topological sort)
- Add `depends_on` field to AgentDef for declaring dependencies
- Add `agent_start_timeout` config option (30s default)
- Add `internal/graph` package with DependencyGraph and Kahn's algorithm
- Improve `Start()` to block until all agents are `Ready()`

**Bug Fixes**:

- Fix unchecked error returns (errcheck) in agent tests
- Replace deprecated `option.WithCredentialsFile` with `WithAuthCredentialsFile`
- Fix MockAgent.Start blocking that caused test timeouts

## Migration Guide

### Adding Dependencies to Existing Systems

If you have an existing multi-agent system experiencing startup race conditions:

1. **Identify Dependencies**: Which agents need others to be ready first?

   ```yaml
   # Before
   agents:
     - name: orchestrator
     - name: worker1
     - name: worker2
   ```

1. **Add depends_on**: Declare explicit dependencies

   ```yaml
   # After
   agents:
     - name: worker1
     - name: worker2
     - name: orchestrator
       depends_on: [worker1, worker2]
   ```

1. **Test Startup**: Verify agents start in the correct order

   ```bash
   # You'll see log messages indicating phase-based startup:
   # Starting Phase 0: [worker1, worker2]
   # All agents in Phase 0 are ready
   # Starting Phase 1: [orchestrator]
   # All agents in Phase 1 are ready
   ```

### No Changes Required

If your agents don't have startup dependencies, no changes are needed. The runtime will start all agents concurrently as before.

## Performance Considerations

### Startup Time Impact

Phased startup adds minimal overhead:

- **Single Phase**: No additional delay (same as v0.2.2)
- **Multiple Phases**: ~100-200ms per phase for Ready() polling
- **Typical 3-Phase System**: Adds ~300-600ms to total startup time

This is negligible compared to the time agents typically spend initializing (database connections, loading models, etc.).

### Optimization Tips

1. **Minimize Dependencies**: Only declare true dependencies, not "nice to have" ordering
1. **Parallelize Within Tiers**: Agents at the same dependency level start concurrently
1. **Fast Ready() Checks**: Keep `Ready()` implementations lightweight (simple boolean checks)

## What's Next

Looking ahead:

- **Graceful Shutdown Ordering**: Apply dependency ordering to shutdown (reverse order)
- **Dynamic Dependency Resolution**: Allow agents to declare dependencies at runtime
- **Dependency Visualization**: Tools to visualize and debug dependency graphs
- **Health-Based Dependencies**: Wait for agents to be healthy, not just ready

## Get Involved

We'd love to hear your feedback on phased startup:

- **GitHub Issues**: [Report bugs or request enhancements](https://github.com/aixgo-dev/aixgo/issues)
- **Discussions**: [Share your use cases](https://github.com/orgs/aixgo-dev/discussions)
- **Contributing**: [Contribution guide](https://github.com/aixgo-dev/aixgo/blob/main/docs/CONTRIBUTING.md)

## Installation

```bash
# Upgrade to the latest version
go get github.com/aixgo-dev/aixgo@v0.2.4

# Or use the public agent package
go get github.com/aixgo-dev/aixgo/agent@v0.2.4
```

---

**Eliminate startup race conditions with dependency-aware agent initialization:**

```bash
go get github.com/aixgo-dev/aixgo@v0.2.4
```
