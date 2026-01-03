---
title: 'Aixgo v0.2.3 & v0.2.4: Phased Agent Startup with Dependency Ordering'
date: 2026-01-03
description: 'Introducing dependency-aware agent startup that eliminates race conditions in multi-agent systems using topological sort and phased initialization.'
tags: ['release', 'runtime', 'startup', 'dependencies', 'orchestration']
author: 'Aixgo Team'
---

We're excited to announce **Aixgo v0.2.3** and **v0.2.4**, featuring **phased agent startup with dependency ordering**. This eliminates race conditions in multi-agent systems and
makes startup behavior predictable and safe.

## The Problem: Startup Race Conditions

When all agents start concurrently, orchestrators can't rely on dependencies being ready during their `Start()` method.

### Before v0.2.3

```go
rt.Register(databaseAgent)      // Dependency
rt.Register(cacheAgent)          // Dependency
rt.Register(orchestratorAgent)   // Needs both above

rt.Start(ctx)  // All start concurrently - race condition!
```

**Problem**: Orchestrator might call `runtime.Call()` during `Start()`, but dependencies might not be ready yet.

## The Solution: Phased Startup

v0.2.3 introduces **phased agent startup** powered by topological sort. Agents are grouped into dependency levels and started in phases, with each phase completing before the next
begins.

### How It Works

1. **Declare Dependencies**: Use `depends_on` in agent definitions
2. **Automatic Ordering**: Aixgo builds a dependency graph and computes startup phases
3. **Phase-Based Execution**: Agents start in phases (0, 1, 2, ...) based on dependencies
4. **Concurrent Within Phases**: Agents in the same phase start concurrently
5. **Ready Polling**: Each phase waits for all agents to be `Ready()` before proceeding

### After v0.2.3

```yaml
agents:
  - name: database
    role: producer
    # No dependencies - Phase 0

  - name: cache
    role: producer
    depends_on: [database]
    # Phase 1

  - name: api
    role: react
    depends_on: [database, cache]
    # Phase 2
```

**Result**: Guaranteed startup order with no race conditions.

## Key Features

### 1. Topological Sort with Kahn's Algorithm

- **Cycle Detection**: Automatically detects and reports circular dependencies
- **Optimal Ordering**: Minimizes startup phases
- **Parallel Execution**: Independent agents start concurrently

### 2. Configurable Timeout

```yaml
config:
  agent_start_timeout: 45s # Default: 30s
```

### 3. All Runtime Support

Phased startup works across all runtime implementations:

- **LocalRuntime**: Single-process deployments
- **SimpleRuntime**: Lightweight runtime
- **DistributedRuntime**: Multi-node orchestration via gRPC

### 4. Backward Compatible

If you don't specify `depends_on`, all agents start concurrently as before. Phased startup is opt-in.

## Real-World Example

```yaml
agents:
  # Tier 1: Foundational services
  - name: config-service
    role: producer

  - name: database
    role: producer

  # Tier 2: Services depending on Tier 1
  - name: cache
    role: producer
    depends_on: [database, config-service]

  - name: auth-service
    role: react
    depends_on: [database]

  # Tier 3: Application services
  - name: user-service
    role: react
    depends_on: [database, cache, auth-service]

  - name: order-service
    role: react
    depends_on: [database, cache, auth-service]

  # Tier 4: Orchestrators
  - name: api-gateway
    role: react
    depends_on: [user-service, order-service]

config:
  agent_start_timeout: 60s
```

**Startup sequence**:

1. **Phase 0**: `config-service`, `database` (concurrent)
2. **Phase 1**: `cache`, `auth-service` (concurrent, after Phase 0 ready)
3. **Phase 2**: `user-service`, `order-service` (concurrent, after Phase 1 ready)
4. **Phase 3**: `api-gateway` (after Phase 2 ready)

## Version History

### v0.2.4 (January 3, 2026)

**Bug Fixes**:

- Fix test failures exposed by phased startup
- Add mutex to MockAgent to prevent race condition
- Fix errorAgent to use AgentDef name instead of hardcoded value

### v0.2.3 (January 2, 2026)

**Major Features**:

- Add phased agent startup with dependency ordering (topological sort)
- Add `depends_on` field to AgentDef
- Add `agent_start_timeout` config option (30s default)
- Add `internal/graph` package with DependencyGraph and Kahn's algorithm
- Improve `Start()` to block until all agents are `Ready()`

**Bug Fixes**:

- Fix unchecked error returns in agent tests
- Replace deprecated `option.WithCredentialsFile` with `WithAuthCredentialsFile`
- Fix MockAgent.Start blocking that caused test timeouts

## Migration Guide

### Adding Dependencies to Existing Systems

1. **Identify Dependencies**: Which agents need others to be ready first?

   ```yaml
   # Before
   agents:
     - name: orchestrator
     - name: worker1
     - name: worker2
   ```

2. **Add depends_on**: Declare explicit dependencies

   ```yaml
   # After
   agents:
     - name: worker1
     - name: worker2
     - name: orchestrator
       depends_on: [worker1, worker2]
   ```

3. **Test Startup**: Verify agents start in the correct order

   ```bash
   # You'll see log messages:
   # Starting Phase 0: [worker1, worker2]
   # All agents in Phase 0 are ready
   # Starting Phase 1: [orchestrator]
   # All agents in Phase 1 are ready
   ```

### No Changes Required

If your agents don't have startup dependencies, no changes are needed.

## Performance Considerations

### Startup Time Impact

- **Single Phase**: No additional delay
- **Multiple Phases**: ~100-200ms per phase for Ready() polling
- **Typical 3-Phase System**: Adds ~300-600ms to total startup time

This is negligible compared to agent initialization time (database connections, loading models, etc.).

### Optimization Tips

1. **Minimize Dependencies**: Only declare true dependencies
2. **Parallelize Within Tiers**: Agents at the same level start concurrently
3. **Fast Ready() Checks**: Keep `Ready()` implementations lightweight

## What's Next

- **Graceful Shutdown Ordering**: Apply dependency ordering to shutdown (reverse order)
- **Dynamic Dependency Resolution**: Allow runtime dependency declaration
- **Dependency Visualization**: Tools to visualize and debug dependency graphs
- **Health-Based Dependencies**: Wait for agents to be healthy, not just ready

## Get Involved

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
