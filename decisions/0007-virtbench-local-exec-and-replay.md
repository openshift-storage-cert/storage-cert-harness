# 0007. virtbench integration: local-exec runner + replay mode

- **Status:** Accepted
- **Date:** 2026-08-23

## Context

At adoption, virtbench was a client-side Python CLI with no upstream image. The
first integration, datasource-clone (TR-VIRT-002), also needed offline parser
work and re-grading of saved runs.

## Decision

- **Local execution is the initial exception to ADR-0003:** invoke the installed
  `virtbench` CLI with `exec.CommandContext`. Obtain kubeconfig, UUID, scale,
  and storage class from TR parameters/backend configuration; leave integration
  `Image` empty. A self-built image and in-cluster Job/RBAC remain a follow-up.
- **Replay:** `results_dir` points at saved output, initially
  `summary_vm_creation_results.json`. Skip binary/cluster preflight and command
  execution; collect, parse, and grade the saved results offline.
- **Metric names match the KB:** map `clone_duration_sec` → `clone_duration`,
  `running_time_sec` → `time_to_running`, and `ping_time_sec` → `time_to_ping`.
  Guard the mapping with tests and give every parser a golden fixture.
- **Registry keys exactly match `automation_tool`:** register
  `virtbench datasource-clone`, and each later scenario under its own KB name.
  Share helpers without adding scenario dispatch to the orchestrator.
- Construct commands from params/backend data, never threshold values. Fixtures
  contain measurements, never real SLA gates.

## Decision notes

### Options considered

- **Local execution first (chosen):** unblocks e2e; requires an installed CLI.
- **Self-built image and in-cluster Job:** reproducible runtime, but adds early
  image/RBAC work; deferred.
- **One integration dispatching all scenarios:** weakens exact registry lookup;
  use separate registrations with shared helpers.

### Consequences

[ADR-0008](0008-scenario-table-and-unit-normalization.md) provides the shared
scenario engine.

In-cluster execution and CSV-detail capture remain follow-ups. Cleanup was
initially manual; [ADR-0009](0009-scoped-setup-run-group-individual.md) records
shared setup and cleanup. Threshold encryption remains separate.

Related: [ECOPROJECT-5331](https://redhat.atlassian.net/browse/ECOPROJECT-5331),
[ADR-0006](0006-adopt-kb-export-v2-contract.md).
