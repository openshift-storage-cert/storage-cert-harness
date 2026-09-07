# 0008. Multi-scenario tools: a scenario table + unit-normalized grading

- **Status:** Accepted
- **Date:** 2026-08-23

## Context

ADR-0007 integrated the first virtbench scenario (datasource-clone, TR-VIRT-002)
and predicted that adding the remaining scenarios would be "another integration
keyed by its `automation_tool` string, reusing the shared helpers." We used
adding two more scenarios — boot storm (TR-VIRT-001) and disk-ops (TR-STOR-001) —
as a forcing function to test whether that was actually true, and to find what
wasn't generic enough.

Three things surfaced:

1. **The original `virtbench.go` was datasource-clone-specific.** The subcommand,
   metric map, result filename, `Provides`, and integration `Name` were all
   hardcoded. Adding a scenario meant copying the package.
2. **The core grader compared raw floats.** The KB expresses a gate in whatever
   unit reads naturally in the prose (`vm_boot_time < 10 min`); virtbench reports
   seconds. `compare(480, "<", 10)` is `false` → a passing run graded as a
   spurious FAIL. This affected every tool, not just virtbench.
3. **Not every SLA is measurable by the tool that runs the TR.** Boot storm emits
   no IOPS-latency percentiles (that is storage telemetry) and one run measures
   one scale; disk-ops emits averages, not p99. The grader's "metric not found →
   error" turned these into noise rather than a clean, explainable skip.

Jira: ECOPROJECT-5273 (KB integration), 5331 (first e2e).

## Decision

1. **Scenario table.** `internal/tools/virtbench` is split into a generic
   `engine.go` (all the stage plumbing: preflight, provision, exec, collect,
   teardown, replay) and a `scenario.go` table. A `Scenario` carries the only
   things that differ between subcommands: `AutomationTool` (the registry key),
   `ProvidesTR`, `ResultFile`, a `BuildArgs` closure, a **pluggable `Parse`
   closure**, and an optional `Applicable` predicate. `init()` loops the table and
   registers one `ToolIntegration` per scenario. Adding a metric-map scenario is a
   new row; a differently-shaped tool is a new row plus a new `Parse` closure —
   **no engine edits** (validated: both new scenarios required zero `engine.go`
   changes).
2. **Unit-normalized grading.** `internal/grader/units.go` canonicalizes a
   `(value, unit)` to its dimension's base unit (time→s, ratio→%, throughput→iops,
   byte-rate→MiBps) and the grader converts **both** the measured value and the
   SLA bar before comparing, while still displaying the original units. Unknown or
   empty units fall back to a raw compare (no silently-wrong verdict); two known
   units from different dimensions are an `error`, not a bogus pass/fail.
3. **Tool-owned "measurable" skips; core grader stays generic.** A scenario's
   `Applicable(sla)` reports whether its run actually produces a measurement for
   that SLA. The scenario's `stages.Evaluator` skips inapplicable SLAs with a
   reason and delegates the rest to the exported built-in rules
   (`grader.GradeSLAs` / `grader.GradeChecks`). The core grader gains no
   tool-specific knowledge.

## Options Considered

- **Scenario table with a pluggable `Parse` (chosen)** — (+) new scenarios are
  data, not code; the disk-ops nested shape proved a single metric-map is
  insufficient, and a per-scenario `Parse` closure absorbs that without touching
  shared stages. (−) one more indirection than a single hardcoded parser.
- **One integration dispatching all scenarios internally** — rejected in ADR-0007
  and again here: it needs orchestrator-level `automation_tool → integration`
  resolution and a fat `Provides` list; the table keeps `registry.Get` an exact
  match.
- **Unit conversion in each tool's parser (normalize to the SLA's unit there)** —
  (−) every tool would reimplement conversion and would need to know each SLA's
  unit; puts a cross-cutting concern in N places. Centralizing in the grader fixes
  it once for all tools.
- **"Measurable" filtering inside the core grader** — (−) the core would need a
  per-tool notion of what each tool can measure, coupling it to concrete tools
  (violates ADR-0003). Keeping it in the tool's Evaluator preserves the boundary.

## Consequences

- Adding a virtbench scenario is now a table row (+ a `Parse` closure only when
  the output shape is new). The generality test in `virtbench_test.go` covers
  both an args-only scenario (boot storm) and a new-shape scenario (disk-ops).
- Every tool benefits from unit-aware grading for free; gates and measurements no
  longer have to share a unit. `min`/`h`/`ms` gates against second-valued metrics
  now grade correctly (proven by the boot-storm replay: `480 s` passes `< 10 min`).
- SLAs a tool structurally cannot measure produce an explained `skip`, not an
  `error`. One-scale-per-run is handled the same way (the 1000-VM `< 1 h` gate is
  skipped by the 100-VM run) — full multi-scale-in-one-TR execution is still a
  documented non-goal.
- Follow-ups from ADR-0007 stand (in-cluster Job model, real teardown, CSV
  capture). New follow-up: a richer unit table if a tool emits units not yet
  mapped (it falls back to raw compare until then).

## Non-Goals

- Live-cluster execution / `internal/kube`, the encrypted-thresholds mechanism
  (ADR-0004), running multiple scales within a single TR in one invocation, and
  sourcing IOPS/Prometheus metrics — none are decided here.

## References

- [ECOPROJECT-5273](https://redhat.atlassian.net/browse/ECOPROJECT-5273)
- [ECOPROJECT-5331](https://redhat.atlassian.net/browse/ECOPROJECT-5331)
- ADR-0003 (ports-and-adapters, core never imports a tool), ADR-0006 (KB export
  v2.0), ADR-0007 (virtbench local-exec + replay).
