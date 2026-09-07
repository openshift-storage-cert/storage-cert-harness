# 0003. Harness architecture: ports-and-adapters, data-driven, image-based tools

- **Status:** Accepted
- **Date:** 2026-08-23

## Context

The harness must:

- Make it **easy to add test tools** (virtbench, kube-burner-ocp,
  kubevirt-storage-checkup, origin CSI) **and our own tests**, without touching
  core code.
- Enable **many small, independent, parallelizable dev tasks** so a small team
  can build it quickly.
- Grade results against **secret SLA thresholds supplied at runtime** (5273),
  while also supporting **complex tests that do not reduce to a single
  metric-vs-threshold check**.
- Be **cancellable** end-to-end, and let stages pass along ad-hoc state that
  doesn't fit a fixed schema.
- Ship as a **small single container**; prefer running upstream tools from the
  images their projects already release.

## Decision

1. **Ports-and-adapters (hexagonal).** The core — orchestrator, grader dispatch,
   reporter, registry — depends only on interfaces. It never imports a concrete
   test tool.

2. **Data-driven inputs.** Two KB-exported artifacts drive a run:
   - `tests-catalog.json` — every test: id, TR mapping, owning tool, params,
     assertion type, concurrency attributes.
   - `thresholds.json` — the secret SLA numbers, supplied at runtime
     (`--thresholds <path>` / `HARNESS_THRESHOLDS`, per 5273).
   Adding a *test* is usually a catalog edit → **no code**. Adding a *tool* is a
   new adapter bundle.

3. **Stage-typed adapter contracts (the "layers").** Each is small and
   independently testable:
   `Preflight`, `Provisioner` (input/setup), `Runner` (run a list of tests),
   `LogCollector` (log ingest), `ResultParser` (output ingest → results),
   `Teardown`. A `ToolIntegration` binds a set of these (shared or
   tool-specific), declares which test ids it `Provides`, and declares its tool
   image. Shared/generic adapters (e.g. "tail logs from labeled pods") are
   reused across tools.

4. **Tools run as their released container images.** An integration declares an
   **overridable image ref**; the `Runner` launches it as a Kubernetes
   Job/Pod. We compile only our adapter *glue* into the harness binary — we do
   **not** vendor or rebuild upstream tools. A `Containerfile` in the tool's
   package is a **fallback only when no upstream image exists**. The harness
   container stays small and needs cluster + registry-pull access.

5. **In-tree registry, single binary.** Integrations register at compile time;
   the harness is one static binary in one container. Out-of-process (gRPC)
   plugins were considered and are **deferred** — not needed while tools run as
   their own images.

6. **Pluggable evaluation (for tests that don't fit the SLA mold).**
   `ResultParser` emits a normalized `TestResult` (metrics + native pass/fail +
   artifacts + raw payload). An `Evaluator`, selected per-test by the catalog's
   assertion type, produces the `Verdict`:
   - `sla_threshold` — numeric metric vs `thresholds.json` (core-provided).
   - `functional` — record the tool's own pass/fail (core-provided).
   - `custom` — a tool-provided `Evaluator` for composite/arbitrary logic.
   The grader is a dispatcher over evaluators, not a hard-coded numeric check.

7. **Cancellation + stage-to-stage state.** Every stage method takes
   `context.Context` as its first argument → Ctrl-C, timeouts, and
   abort-on-failure propagate. Alongside context, each tool pipeline carries a
   per-run, concurrency-safe **`Bag`** (`string → any`, with a typed getter) for
   state that doesn't fit the schema — the namespace `Provision` created, the Job
   name `Run` produced, a token `Teardown` must revoke. The `Bag` is
   deliberately **not** `context.Value` (an anti-pattern for mutable shared
   state).

8. **Scheduling / parallelism.** The orchestrator runs a worker pool bounded by
   `--concurrency N`, and honors per-test concurrency attributes from the
   catalog: `parallel_safe` (must this test run alone?) and `exclusivity_groups`
   (tests sharing a group never overlap, even if otherwise parallel-safe).
   Cluster-hungry tests (large volume counts, node drain, boot storm) are marked
   exclusive; light functional checks run in parallel.

## Options Considered

- **Stage-typed adapters (chosen)** vs one fat per-tool interface — (+) reuse of
  shared stages, finest-grained parallel tasks. (−) a little more wiring per
  tool.
- **Run released tool images (chosen)** vs compiling tools into our binary —
  (+) small harness, no rebuild burden, track upstream releases. (−) needs
  registry access at run time (disconnected/mirror handled later).
- **In-tree registry (chosen)** vs out-of-process gRPC plugins — (+) simple,
  type-safe, one binary. (−) third parties can't inject uncompiled tools — not a
  current requirement.
- **Pluggable evaluators (chosen)** vs numeric-only grading — (+) supports
  functional and composite tests. (−) more surface than a single threshold
  check.

## Consequences

- A tool integration decomposes into several small, cluster-free, golden-file
  unit tasks (e.g. "write the virtbench `ResultParser`") — ideal for parallel
  contributors.
- The core stays stable; a new tool is a new package + one registration.
- Image-based runners keep the container tiny but assume registry reachability;
  a disconnected/mirrored-registry story is deferred.
- The `Bag` is flexible but must be documented (known keys per stage) so it
  doesn't become a dumping ground.
- Report generation stamps provenance (KB commit, schema_version) per 5273.
- The harness container itself is built and published per
  [0011](0011-harness-ships-as-container-image.md).

## Non-Goals

- Out-of-process/gRPC plugins.
- Disconnected/air-gapped registry mirroring.
- The KB exporter internals (5273) and the final report schema (a later ADR).
- How the harness container is built, tagged, and published
  ([0011](0011-harness-ships-as-container-image.md)).

## References

- [ECOPROJECT-5328](https://redhat.atlassian.net/browse/ECOPROJECT-5328),
  [ECOPROJECT-5329](https://redhat.atlassian.net/browse/ECOPROJECT-5329),
  [ECOPROJECT-5273](https://redhat.atlassian.net/browse/ECOPROJECT-5273),
  [ECOPROJECT-5331](https://redhat.atlassian.net/browse/ECOPROJECT-5331)
- Deliberation log: `harness-foundational-decisions.md`
- Model: [osac-project/osac-workspace](https://github.com/osac-project/osac-workspace)
- [0011](0011-harness-ships-as-container-image.md)
