# 0005. Test plans and storage backend configuration

- **Status:** Accepted
- **Date:** 2026-08-23

## Context

Two run-time inputs were missing from the architecture:

1. **Storage backend parameters.** Certifying a specific array needs
   vendor-specific config — storage class, snapshot class, and a grab-bag of
   fields that differ per vendor (e.g. NetApp Trident's SVM / management LIF /
   endpoint) plus credentials (username, password, sometimes a token). The shape
   varies per storage, and may be empty.
2. **Test plans.** We want to run different curated subsets of the catalog —
   some pre-made (e.g. a full Level-3 plan, a smoke plan), some hand-written per
   engagement — rather than only ad-hoc CLI filters.

## Decision

1. **Test plan (YAML).** A plan selects and parameterizes catalog tests:
   `select` (by ids / tools / TRs / tags / all), optional per-test `overrides`
   (merged into each test's params), `concurrency`, and the active `backend`
   name. Pre-made plans live in `plans/`; a user passes one via `--plan`. Ad-hoc
   CLI filters still work and override the plan when set.

2. **Backend (YAML).** A backends file defines one or more named backends:
   common typed fields (`vendor`, `storage_class`, `snapshot_class`), an open
   `params` map for vendor-specifics (arbitrary shape, may be empty), and a
   `secrets` map. The plan (or `--backend`) selects the active one; it is passed
   to every stage via `RunCtx.Backend` and is **optional**.

3. **Credentials are referenced, never committed as plaintext.** Each secret is
   one of: a **file path**, a **Kubernetes Secret** reference
   (namespace/name/key), or an **inline** value (local dev only, gitignored).
   Env-var references were considered and intentionally excluded. Secrets are
   resolved at run time into an in-memory `ResolvedBackend` that is never logged
   or serialized into the report.

4. **Format split.** Hand-authored configs (plans, backends) are YAML; the
   KB-exported catalog and thresholds stay JSON. See ADR-0002/0003.

## Options Considered

- **Separate plan + backend files (chosen)** — (+) plans are shareable/pre-made
  and free of secrets; backends are site-specific. (−) two files to point at.
- **Backend embedded in the plan** — (+) one file. (−) mixes shareable plan with
  site secrets; encourages committing credentials.
- **Env-var credential refs** — (−) rejected per this session; file/k8s/inline
  cover the needs.
- **Typed-per-vendor backend structs** — (−) rejected; an open `params` map
  keeps adding a vendor a data change, not a code change.

## Consequences

- New packages `internal/plan` and `internal/backend`; `core.TestSpec` gains
  `tags`; `core.RunCtx` gains `Backend`.
- Adds a YAML dependency (`gopkg.in/yaml.v3`) for the hand-authored configs.
- **Kubernetes Secret resolution is deferred** until `internal/kube` (client-go)
  exists; the ref type and plumbing are in place and return a clear error until
  then.
- Inline credentials are gitignored and logged as a dev-only warning.

## Non-Goals

- The encrypted-folder mechanism for thresholds (ADR-0004) — a file-path secret
  ref can later point into it.
- Live client-go Secret reading (deferred, above).

## References

- [ECOPROJECT-5273](https://redhat.atlassian.net/browse/ECOPROJECT-5273)
- [0003](0003-harness-architecture-ports-and-adapters.md),
  [0004](0004-threshold-secret-handling.md)
