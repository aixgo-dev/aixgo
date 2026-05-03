# ADR 0002: Aixgate ships from a separate repository

**Status:** Accepted
**Date:** 2026-05-03
**Deciders:** Charles Green
**Supersedes:** [ADR 0001](0001-aixgate-monorepo.md)

## Context

[ADR 0001](0001-aixgate-monorepo.md) chose to keep Aixgate inside the aixgo monorepo, on the reasoning that Aixgate had no independent users, no commercial variant, and no cgo dependency — and so the operational cost of two repos was not yet justified. ADR 0001 explicitly named three trigger conditions for reconsideration:

1. Aixgate gains a cgo dependency.
2. Release cadences diverge to the point where coupling causes visible friction.
3. A commercial variant (e.g. `aixgo-cloud`) needs a shared policy library that would cleanly extract to `pkg/aixgate`.

Since ADR 0001 was written, the [Aixgate PRD](../AIXGATE_PRD.md) has been substantially elaborated, and three of its design statements directly contradict the monorepo assumption:

- **§10.2** declares "Aixgate is not bundled with aixgo. The dependency is one-way and optional." Aixgate is not a feature of the framework; it consumes a small subset of `aixgo/pkg/...` and is otherwise independent.
- **§11.4** documents a v1.x roadmap (Linux Landlock ABI versions, macOS `sandbox-exec` deprecation contingency, FUSE-T fallback, Windows WFP, eBPF, network egress proxy, policy marketplace) that shares zero items with aixgo's framework roadmap.
- **§14.6** specifies that Aixgate ships its own PRD, CHANGELOG, and SECURITY.md — independent docs surface from the framework.
- **§10** identifies `aixgo-cloud` as a planned commercial variant for fleet aixgate (centralised policy distribution, SIEM integration, audit log shipping). This is exactly the trigger condition ADR 0001 named.

Two of the three reconsider triggers are firing pre-v0.1: release-cadence divergence (Aixgate's v0.1→v1.0 will be months of weekly platform-fix releases against a stable framework cadence) and the planned commercial variant. The third — cgo dependency — remains absent and is no longer the controlling factor it was at the time of ADR 0001.

Three independent strategic reviews (CRO, CMO, product-manager) recommended the split unanimously on 2026-05-03 on the basis of:

- **Audience disjointness.** Framework users are Go engineers building agentic systems. Aixgate users are developers running third-party AI coding agents (Claude Code, Cursor, Aider, Codex) who want filesystem policy and audit logs. Intersection is small.
- **Category creation.** Aixgate is creating the category "runtime sandbox for AI coding agents." Category creation needs its own banner, README, SEO surface, and star count.
- **Conversion path.** A standalone repo gives the future `aixgo-cloud` commercial offering a clean wedge product with its own funnel.

The cost of splitting now (one ADR, a few config files, a `git filter-repo` that does not yet need to run because no Go code exists) is low. The cost of splitting after v0.1 ships to Homebrew (broken `brew install` URLs, redirected stars, migrated issue trackers) is meaningfully higher.

## Decision

Aixgate ships from `github.com/aixgo-dev/aixgate`. The aixgo monorepo no longer hosts Aixgate code, PRD, or release tooling.

Aixgate consumes shared code (e.g. `pkg/security`, `pkg/observability`) from aixgo via `go.mod` versioning, the same pattern Atlas (`github.com/charlesgreen/atlas`) already uses. No `replace` directives in shipped releases.

Aixgate maintains its own ADR sequence, beginning with `aixgate/docs/adr/0001-...`. ADR numbering in the two repos is independent; do not cross-reference numerically.

## Consequences

**Good:**

- Independent semver, CHANGELOG, SECURITY.md, and GitHub Discussions per [AIXGATE_PRD.md](../AIXGATE_PRD.md) §14.6.
- Aixgate's audience lands on a focused README ("runtime sandbox for AI coding agents") instead of a framework-flavoured one.
- Release cadence decoupled — Aixgate platform-fix releases don't gate framework releases and vice versa.
- Star/SEO equity accrues to the right repo for category creation. HN/Reddit shareability of a focused security tool is materially better.
- The future `aixgo-cloud` commercial offering has a natural sibling repo to extend (`pkg/aixgate` style shared policy lib if/when needed).
- CI cost no longer cohabits — a change to Linux Landlock integration tests doesn't rerun the framework's full suite.

**Accepted tradeoffs:**

- Aixgate must depend only on aixgo's `pkg/` (already public API) — no in-tree leakage. Same constraint Atlas already operates under, so the discipline is well-rehearsed in the wider system.
- Discovery cost: a user scanning the aixgo repo no longer immediately sees Aixgate alongside the framework. Mitigated by an explicit pointer in `aixgo/docs/AIXGATE.md` (stub linking to the new repo) and a "See also" footer in the aixgo README.
- Two repos to maintain instead of one. Mitigated by mirroring governance (.gitignore, .gitleaks.toml, dependabot.yml, .markdownlint.json, .goreleaser.yaml, Makefile conventions) so contributors moving between them face zero context switch.

## Reconsider this ADR if

- Aixgate adopts a Go module dependency that aixgo itself needs (motivating remerge to share a single `go.sum`).
- Maintenance overhead of two repos exceeds the engineering benefit, measured in blocked releases per quarter (not anecdotes).
- The audience for Aixgate turns out to be a strict subset of the framework's audience and the brand-architecture argument inverts.

## References

- [ADR 0001](0001-aixgate-monorepo.md) — the superseded monorepo decision.
- [AIXGATE_PRD.md](../AIXGATE_PRD.md) §10.2 (one-way optional dependency), §11.4 (orthogonal roadmap), §14.6 (independent docs surface), §10 (`aixgo-cloud` commercial variant).
- [factory/apps/atlas/docs/OPEN_CORE_STRUCTURE.md](https://github.com/charlesgreen/atlas) — the import-direction discipline that Aixgate inherits.
- Strategic reviews from CRO, CMO, and product-manager perspectives (2026-05-03 planning session) — converged on split.
