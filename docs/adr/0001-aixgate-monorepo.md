# ADR 0001: Aixgate lives in the aixgo monorepo

**Status:** Accepted
**Date:** 2026-04-21
**Deciders:** Charles Green

## Context

Aixgate is a sandbox/policy-enforcement companion to the aixgo Go framework (see `docs/AIXGATE_PRD.md`). Early design discussions weighed three options for where Aixgate's code should live:

- **(A)** Separate repository (`github.com/aixgo-dev/aixgate`).
- **(B)** Same repository as aixgo, separate Go module via `go.work`.
- **(C)** Same repository and same Go module as aixgo (no `go.work`), with Aixgate as `cmd/aixgate/` + `pkg/aixgate/` + `internal/aixgate/`.

An earlier recommendation favoured **(B)** on the grounds that Aixgate would require `cgofuse`, and adding cgo to aixgo's main `go.mod` would break the framework's pure-Go guarantee (small binaries, no C toolchain required for downstream users).

That premise changed. The PRD (v0.3) replaces `cgofuse` with a pure-Go stack: `hanwen/go-fuse` + `landlock-lsm/go-landlock` + `elastic/go-seccomp-bpf` on Linux, and `sandbox-exec` invoked via `os/exec` on macOS. No cgo dependency is introduced.

## Decision

Adopt **option (C)**: Aixgate lives in the aixgo monorepo, in the same Go module, under:

- `cmd/aixgate/` — CLI entry point.
- `pkg/aixgate/` — public client library (for aixgo framework integration, e.g. `AIXGATE_ACTIVE=1` detection and audit-context streaming).
- `internal/aixgate/policy/` · `internal/aixgate/jail/` · `internal/aixgate/fs/` · `internal/aixgate/audit/` · `internal/aixgate/profiles/` — implementation.

## Consequences

**Good:**

- One `go.mod`, one version, one `go test ./...`. No workspace tooling overhead.
- Layout mirrors aixgo's existing conventions (`cmd/`, `pkg/`, `internal/`), so contributors don't need a second mental model.
- Aixgate can reuse aixgo shared code (e.g. `pkg/security`, `pkg/observability`) without synthetic module boundaries.
- Discovery: a user reading the aixgo repo immediately sees Aixgate alongside the framework.

**Accepted tradeoffs:**

- Release cadence is coupled. If Aixgate needs an emergency fix, a framework release goes with it (or vice versa). Mitigation: keep both surfaces small enough that this is rarely expensive; if it becomes expensive, promote to `go.work` without moving files.
- CI blast radius: a change to `internal/aixgate/fs/` reruns the full framework test suite. Mitigation: path-filtered workflows so FUSE-integration tests gate only on Aixgate paths.
- A future Aixgate CVE-embargo process has to cohabit with aixgo's. Mitigation: documented in `SECURITY.md`; if embargo friction ever becomes real, split-out is a mechanical refactor.

## Reconsider this ADR if

- Aixgate gains a cgo dependency that can't be avoided (e.g. macOS Endpoint Security framework if `sandbox-exec` is removed).
- Release cadences diverge to the point where coupling causes visible friction (measured in blocked releases per quarter, not anecdotes).
- A commercial variant (`aixgo-cloud`) needs a shared policy library that would cleanly extract to `pkg/aixgate` — at which point promoting to `go.work` or a separate repo becomes the right answer.

## Alternatives considered

**(A) Separate repo.** Rejected for v0.x. Adds discovery friction, duplicates CI infrastructure, and splits issue tracking without benefit while Aixgate has no independent users. Revisit at v1.0 if Aixgate has grown a standalone community and release cadence demand.

**(B) `go.work` with a separate `aixgate/go.mod`.** Rejected. This was the recommendation under the cgofuse assumption. With no cgo, the workspace boundary buys nothing except tooling complexity. Contributors hit more "which module am I in" confusion than the isolation is worth.

## References

- `docs/AIXGATE_PRD.md` — full product requirements document.
- `docs/AIXGATE_PRD.md` §14.1 — current tech stack (pure-Go jailer).
- Earlier architecture review that recommended option (B) under the cgofuse assumption (superseded by this ADR).
