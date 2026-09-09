# 0008. Multi-scenario tools: a scenario table + unit-normalized grading

- **Status:** Accepted
- **Date:** 2026-08-23

## Context

Adding virtbench boot-storm and disk-ops exposed scenario-specific stage code,
incorrect comparisons between unlike units, and SLAs the assigned tool cannot
measure.

## Decision

- **Scenario table:** keep shared preflight/provision/run/collect/parse/teardown
  and replay plumbing in `engine.go`. Each `Scenario` declares `AutomationTool`,
  `ProvidesTR`, `ResultFile`, `BuildArgs`, a pluggable `Parse`, and optional
  `Applicable`. Register one integration per row using the exact KB tool name.
  New output shapes get a parser, not copied stages or core dispatch.
- **Normalize both operands in the shared grader:** convert measurement and gate
  to dimension base units (time: seconds; ratio: percent; throughput: IOPS;
  byte-rate: MiBps), then compare. Preserve original units for display. Known
  units from different dimensions produce an error. Unknown/empty units fall
  back to raw comparison; extend the unit table when needed.
- **Tools own measurability:** a scenario evaluator uses `Applicable(sla)` to
  skip structurally unmeasurable criteria with a reason, then delegates other
  criteria to `grader.GradeSLAs` / `GradeChecks`. Keep tool knowledge out of the
  core grader. Averages must not stand in for absent percentiles; a run at one
  scale must not claim results for another.

## Decision notes

### Options considered

- **Scenario table with pluggable parsers (chosen):** reuses stages across output
  shapes; adds indirection instead of a single hardcoded parser.
- **One integration dispatching all scenarios:** weakens exact registry lookup.
- **Parser-side unit conversion:** duplicates logic and couples parsers to gates;
  use the shared grader.
- **Core measurability filtering:** couples the core to tools; use tool evaluators.

### Consequences

Cluster-free scenario tests cover different result formats. All tools share unit
conversion; explained skips expose unsupported metrics without claiming a pass.

Multiple scales within one TR invocation, external storage/Prometheus telemetry,
in-cluster execution, and threshold encryption are outside this decision.
ADR-0007's execution/CSV follow-ups remain; shared setup is addressed by ADR-0009.

Related: [ADR-0003](0003-harness-architecture-ports-and-adapters.md),
[ADR-0007](0007-virtbench-local-exec-and-replay.md),
[ADR-0009](0009-scoped-setup-run-group-individual.md).
