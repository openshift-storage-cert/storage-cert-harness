# 0006. Adopt KB export v2.0 two-file contract

- **Status:** Accepted
- **Date:** 2026-08-23

## Context

The KB v2.0 export represents a Test Requirement (TR) with multiple SLA gates and
functional checks. The original single-assertion test model cannot express it.

## Decision

- **Two files, joined by TR ID:** publishable `catalog.json` and sensitive
  `thresholds.json`. Each has `schema_version: "2.0"`, sorted
  `test_requirements[]`, and provenance (`kb_git_commit`, `kb_commit_date`,
  `generated_date`). Loaders validate version and unique IDs; `kb.Join` requires
  matching versions, the same KB commit, and exact 1:1 ID parity before merging.
- **TR-centric model:** `TestRequirement` carries metadata, `partner_level`,
  `automation_tool`, `SLAs`, and `Checks`. `SLA.Value` is nullable so an unset
  gate differs from zero. Results retain `TRID`, metrics with percentile, and
  a check-ID-to-outcome map. Replace the old assertion model, without dual support.
- **Per-item grading:** emit one verdict per SLA (`sla:<metric>[@<percentile>]`)
  and check (`check:<id>`). Null SLA values, `to-be-validated` criteria, and
  `manual` checks skip, never pass. Other check kinds are capability, suite,
  and scenario. Tools may supply custom per-TR evaluators.
- **Selection:** partner level is the primary tier filter; CLI `--partner-level`
  replaces `--tr`/`--tag`. Plans select `partner_levels`, `tools`, or `ids`.
- **Scheduling belongs to tools/plans:** `ToolIntegration` declares
  `parallel_safe` and `exclusivity_groups`; plan `scheduling:` overrides them.
  These are absent from the KB contract.
- **Unavailable automation skips:** empty, `manual`, or unregistered tool names
  form no jobs and yield explained not-scorable skips, never pass/fail.
- **Catalog-only use:** `kb.FromCatalog` supports `list`/`preflight` without gates.
  `run` refuses selected TRs declaring scorable criteria through
  `sla_count`/`checks_count` when no threshold bundle is supplied.
- **Reporting:** preserve schema version and KB provenance in every report,
  with TR and item identity on verdicts.

## Decision notes

### Options considered

- **TR-centric model (chosen):** matches the export and preserves TR identity;
  requires replacing the old model.
- **Synthetic tests per criterion:** smaller initial change, but loses TR identity.
- **Single merged KB file:** removes the join but breaks the publication boundary.

### Consequences

The join detects stale/mismatched exports while separating publishable metadata
from secret gates.

Update catalog, threshold, report, and plan schemas together. Loaders perform
structural checks; full JSON Schema enforcement at runtime is deferred, as are
Kubernetes Secret resolution and threshold encryption.

This refines [ADR-0003](0003-harness-architecture-ports-and-adapters.md) and
[ADR-0005](0005-test-plans-and-backend-config.md), retaining
[ADR-0004](0004-threshold-secret-handling.md)'s secret rules.
Related: [ECOPROJECT-5273](https://redhat.atlassian.net/browse/ECOPROJECT-5273).
