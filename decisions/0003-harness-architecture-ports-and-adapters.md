# 0003. Harness architecture: ports-and-adapters, data-driven, image-based tools

- **Status:** Accepted
- **Date:** 2026-08-23

## Context

The harness must accept new tools and tests without changing its core, grade
runtime-supplied secret thresholds and functional results, and support
cancellation and safe parallel execution.

## Decision

- **Ports and adapters:** orchestrator, grader, report, and registry depend on
  stage interfaces, never concrete tools. Integrations register in-tree at
  compile time; ship one static harness binary in a container.
- **Data-driven execution:** a publishable catalog defines tests and a separate
  runtime threshold file supplies secret gates (`--thresholds` /
  `HARNESS_THRESHOLDS`). New tests usually need data; new tools need adapters.
  [ADR-0006](0006-adopt-kb-export-v2-contract.md) supersedes the original
  `tests-catalog.json`/assertion model with `catalog.json` and per-TR SLA/check
  items, and moves scheduling hints from the catalog to tools/plans.
- **Small stage contracts:** `Preflight`, `Provisioner`, `Runner`,
  `LogCollector`, `ResultParser`, and `Teardown` are independently testable and
  reusable. `ToolIntegration` binds stages, `Provides` IDs, and an image reference.
- **Released tool images:** runners launch overridable upstream images as
  Kubernetes Jobs/Pods. Compile adapter glue only; do not vendor/rebuild upstream
  tools. A tool-package `Containerfile` is a fallback when no image exists.
  [ADR-0007](0007-virtbench-local-exec-and-replay.md) records the virtbench
  local-exec exception.
- **Pluggable grading:** parsers normalize metrics, native outcomes, artifacts,
  and raw output. Built-in evaluators handle numeric and functional criteria;
  tool evaluators handle composite logic. The core dispatches without tool
  knowledge; ADR-0006 defines current per-item verdicts.
- **Cancellation and state:** every stage accepts `context.Context`. A per-run,
  concurrency-safe `Bag` with typed access carries stage state; document its
  keys. Do not use `context.Value` for mutable shared state.
- **Scheduling:** bound workers by `--concurrency`; non-parallel-safe work runs
  alone, and jobs sharing an exclusivity group never overlap.
- **Reports:** stamp KB provenance and schema version into every report.

## Decision notes

### Options considered

- **Small stage interfaces (chosen) / one large tool interface:** shared stages
  and independent tests justify extra wiring.
- **Released images (chosen) / compiled-in tools:** avoid upstream rebuilds;
  require registry access.
- **In-tree registry (chosen) / gRPC plugins:** simpler and type-safe, but new
  integrations require rebuilding the harness; external plugins are deferred.
- **Pluggable evaluators (chosen) / numeric-only grading:** support functional
  and composite criteria at the cost of a broader interface.

### Consequences

Shared stages support cluster-free golden parser tests. Tool images require
cluster and registry access; new integrations require rebuilding the harness.

Out-of-process/gRPC plugins and disconnected registry mirroring are deferred.
KB exporter internals and container publishing are separate concerns; see
[ADR-0011](0011-harness-ships-as-container-image.md) for delivery.
