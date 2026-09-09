# 0016. Named plan variants

- **Status:** Proposed
- **Date:** 2026-09-08

## Context

A TR can have threshold bars with different `when` conditions. A plan currently
supplies one parameter map per TR, so covering both requires separate runs.
Adapters identify results by catalog TR ID and may assume one configuration per
pipeline. Repeating IDs within a batch would mix state and overwrite results.

## Decision

Plans may declare `variants: {TR-ID: [{name: ..., params: {...}}]}`. Each list
replaces the default execution for that selected TR. Parameters resolve in order:
catalog defaults, plan overrides, then variant params. Existing plans are unchanged.

Keep the catalog TR ID for registry lookup and grading. Carry a separate variant
name into verdicts and measurements. Run each variant through an isolated pipeline
and artifact directory, sequentially within the tool's scheduling lock. Keep shared
setup once per run/group and ordinary TR batching unchanged. Preflight inspects
each variant independently. Existing `when` matching needs no changes.

Reports expose variant identity; the plan records parameter provenance. Do not
serialize arbitrary resolved parameter maps, which can carry sensitive tool inputs.

## Options Considered

- **Explicit plan variants (chosen)** — partner-controlled workload combinations
  with stable names and no catalog changes.
- **Generate runs from threshold bars** — would require rules for combining
  conditions across metrics and supplying unspecified inputs.
- **Rewrite TR IDs** — would break tool lookup, scenario lookup and parser joins.

## Consequences

Every variant contributes verdicts to certification. JSON consumers can use
`(tr, scenario, variant, item)` or `(tr, scenario, variant, name, percentile)` as execution-aware keys.
Artifacts are separated by encoded TR ID and variant name. Adapter APIs and parser
formats remain compatible. Variants within one tool do not execute concurrently.

## Non-Goals

Automatic parameter matrices, catalog variants, and changing threshold semantics.

## References

- [Test plans](0005-test-plans-and-backend-config.md)
- [KB contract](0006-adopt-kb-export-v2-contract.md)
- [Scoped setup](0009-scoped-setup-run-group-individual.md)
