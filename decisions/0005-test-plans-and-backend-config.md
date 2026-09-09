# 0005. Test plans and storage backend configuration

- **Status:** Accepted
- **Date:** 2026-08-23

## Context

Partners need reusable test selections and site-specific storage configuration
without mixing shareable plans with credentials.

## Decision

- **Plans are YAML:** `--plan` selects catalog tests and merges per-TR parameter
  overrides; plans also set concurrency and the active backend. Explicit CLI
  filters take precedence. Ship reusable plans in `plans/`.
  [ADR-0006](0006-adopt-kb-export-v2-contract.md) refines selection to
  `partner_levels`, `tools`, and `ids`.
- **Backends are separate YAML:** named entries contain typed `vendor`,
  `storage_class`, and `snapshot_class` fields, an open vendor-specific `params`
  map (possibly empty), and secret references. A plan or `--backend` selects an
  optional backend, passed to stages through `RunCtx.Backend`.
- **Credentials are references:** support file paths, Kubernetes Secret
  namespace/name/key references, and gitignored inline values for local
  development only. Exclude environment-variable references. Resolve values
  only in memory; never log or serialize the `ResolvedBackend` into reports.
  Warn when inline development credentials are used, without exposing values.
- Hand-authored plans/backends use YAML; KB exports remain JSON.

## Decision notes

### Options considered

- **Separate plan/backend files (chosen):** keep plans shareable; require two
  inputs instead of embedding site secrets in plans.
- **Open vendor params (chosen) / typed vendor structs:** new vendors need data
  changes rather than Go types.
- **Environment-variable secret references:** excluded; file, Kubernetes Secret,
  and local-development inline references cover the intended needs.

### Consequences

YAML adds a dependency; the loaders live in `internal/plan` and
`internal/backend`.

Kubernetes Secret resolution was deferred pending a Kubernetes client: preserve
its reference shape and return a clear unsupported error until implemented.
Threshold encryption is governed by
[ADR-0004](0004-threshold-secret-handling.md); file references can later point
at decrypted files.

Related: [ECOPROJECT-5273](https://redhat.atlassian.net/browse/ECOPROJECT-5273).
