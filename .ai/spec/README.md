# Quay Operator — Specifications

Behavioral contracts and codebase navigation for the quay-operator v1 API. These specs capture invariants, lifecycle rules, and cross-cutting constraints that aren't obvious from reading any single source file.

## Structure

| Layer | Path | Purpose |
|---|---|---|
| **what/** | `.ai/spec/what/` | Behavioral rules. What the operator must do. Implementation-agnostic. |
| **how/** | `.ai/spec/how/` | Codebase navigation. How the code is organized. Implementation-specific. |

## Scope

- **Covered:** QuayRegistry v1 API, all 11 managed components, reconciliation lifecycle, config bundle management, TLS, monitoring
- **Out of scope:** v2alpha1 API (separate branch), Quay application internals, external dependencies (PostgreSQL, Redis, Clair internals)

## Audience

AI agents. Content is optimized for precision and machine consumption.

## Quick Start

| Task | Start here |
|---|---|
| Understand the system | `what/system-overview.md` |
| Understand component lifecycle | `what/component-management.md` |
| Understand config secret behavior | `what/config-bundle.md` |
| Understand TLS management | `what/tls.md` |
| Understand monitoring modes | `what/monitoring.md` |
| Navigate the codebase | `how/project-structure.md` |
| Understand reconciliation flow | `how/reconciliation.md` |
| Understand status evaluation | `how/status-evaluation.md` |
| Look up a domain term | `glossary.md` |

## Cross-Reference

| what/ | how/ |
|---|---|
| `what/system-overview.md` | `how/reconciliation.md`, `how/project-structure.md` |
| `what/component-management.md` | `how/reconciliation.md` (kustomize inflation, middleware) |
| `what/config-bundle.md` | `how/reconciliation.md` (config bundle step, middleware) |
| `what/tls.md` | `how/reconciliation.md` (context gathering, TLS profile) |
| `what/monitoring.md` | `how/reconciliation.md` (feature detection, object application) |
| — | `how/status-evaluation.md` (independent of what/ — evaluates all components) |

## Conventions

- **Rule numbering:** behavioral rules are numbered sequentially within each what/ file.
- **Planned changes:** unimplemented behavior is marked with `[PLANNED]` or `[PLANNED: reference]` inline next to the rule it affects.
- **Constraints:** component-specific and cross-cutting constraints go in the relevant what/ file's Constraints section, co-located with behavioral rules. Development conventions go in CLAUDE.md.
- **Authority:** what/ specs are authoritative for behavior. how/ specs are authoritative for implementation. When they conflict, what/ wins.
- **When to create a new file vs. extend an existing one:** if the new concern has its own lifecycle, configuration surface, and can be understood independently, it gets its own file. If it's a capability added to an existing component, it goes in that component's file.
