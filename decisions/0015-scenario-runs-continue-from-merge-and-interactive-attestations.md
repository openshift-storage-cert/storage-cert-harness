# 0015. Scenario runs, continue-from merge, and interactive attestations

- **Status:** Proposed
- **Date:** 2026-09-07

## Context

Certification needs results under multiple conditions, such as idle and loaded,
in one report. Independent runs duplicate environment collection, and partners
currently must pre-author their unobservable claims.

## Decision

- **One scenario label per run:** optional plan `scenario`, overridden by
  `--scenario`, is copied to every verdict and measurement. It is uninterpreted
  free text; grading is unchanged. Per-TR scenario labels are outside scope.
- **Continue and merge:** `run --continue-from <report.json>` loads a prior
  report, verifies the backend matches, reuses its Environment without collecting
  again, executes, then calls reusable `report.Merge(base, next)`. Refuse
  different KB commits. `--skip-env` also permits standalone collection skipping.
- **Merge semantics:** union results by TR + item + scenario; distinct scenarios
  coexist, while a rerun replaces the same scenario's earlier result. Recompute
  summary, level rollup, and certified level through `report.Build`. Carry one
  shared Environment and Attestations set.
- **Interactive attestations:** embed `internal/attestation/questions.yaml`,
  overridable with `--attestation-questions`. Resolve in order: inherited answers
  (never re-ask), `--attestations` file, `--no-attestations` (silent skip), TTY
  prompt (default-no consent, signer, questions), otherwise warn and skip.
  Attach answers to the report and save `<output>/attestations.json` for reuse.
  Noninteractive CI/replay must never block for input.

## Decision notes

### Options considered

- **Continue-from merge (chosen):** fits sequential idle/loaded runs and reuses
  environment data; cannot combine independent reports directly.
- **Standalone merge command:** more flexible, but adds a step and environment
  collection coordination; deferred.
- **Catalog-driven attestation questions:** need KB schema/data changes; use an
  embedded, overridable question set initially.

### Consequences

One submission artifact set covers multiple conditions without recollecting the
environment. Scenario fields are additive. Attestation files retain reusable,
auditable answers under
[ADR-0014](0014-report-model-environment-attestations-and-exporters.md)'s rule:
claims remain unobservable, ungraded, and not cross-checked.

A standalone merge command for independently produced reports is deferred; the
merge function remains reusable. Catalog-driven attestation questions need KB
schema/data changes and are also deferred.
