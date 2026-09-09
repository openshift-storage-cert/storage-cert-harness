# 0013. virtbench node-drain / VM-evacuation (TR-VIRT-008): grade from the drain log

- **Status:** Accepted
- **Date:** 2026-09-03

## Context

TR-VIRT-008 measures VM evacuation during node drain. At integration time,
`virtbench vm-ops drain-nodes` drained existing nodes without creating VMs or
emitting results JSON. Its log contained drain durations and VMI distributions
before/after the drain. Stock clone templates lacked migration settings, and
virtbench's single-node mode hard-pinned VMs, preventing evacuation.

## Decision

- **Use upstream evidence:** seed VMs with datasource-clone, then run
  `vm-ops drain-nodes --log-file <path>`. A pure, golden-tested `ParseDrain`
  reads target-node duration and before/after VMI counts from the log. Do not
  introduce harness-side cluster snapshots or timing.
- **Make seeded VMs migratable:** supply a custom template with
  `evictionStrategy: LiveMigrate`, RWX volumes, and preferred node affinity to
  the target. Never hard-pin with `nodeSelector`/required affinity or cordon
  shared nodes to force placement. Omit clone `--cleanup` so VMs survive until
  the drain; teardown deletes them afterwards.
- **Extend the scenario engine:** add a scenario row and optional hooks, not
  bespoke stages. Extra preflight appends to generic checks; extra provision
  runs after generic result-directory/Bag setup; extra teardown supplements
  the generic namespace sweep on its fresh bounded context.
- **Preflight:** require kubectl, a named `target_node`, and at least two
  schedulable workers. Missing prerequisites are errors before any drain.
- **Two grading signals:** `evacuation_time` in seconds is compared with the
  external, unit-normalized SLA. `all-vms-evacuated` passes only when no tracked
  VM remains on the target. The AFTER distribution supplies `vms_remaining`
  (zero if the target is absent). Also emit `vms_total`, `vms_evacuated`, and
  `evacuation_success_rate`. Leaving the target is the defined evacuation signal.
- **Isolation and cleanup:** set `ParallelSafe=false` and exclusivity group
  `cluster`. `TolerateRunError` allows timeout logs to produce a graded failure
  despite a nonzero process exit. Defer teardown before provisioning; on a fresh
  bounded context, uncordon every worker and delete seeded/leftover namespaces
  even after cancellation.

## Decision notes

### Options considered

- **Drain-log parsing (chosen):** works with the released tool and shared engine;
  depends on a human log format.
- **Harness snapshots:** add bespoke collection and timing.
- **Upstream JSON support:** delays integration and remains pod-counted.
- **Single-node placement:** hard-pins VMs and prevents evacuation.
- **KubeVirt API migration tests:** do not exercise the node-drain path.

### Consequences

Offline `results_dir` replay remains available. Golden fixtures protect the
parser against changes in the human log format.

RWX storage and a migration destination are backend/cluster prerequisites.
Parallel/multi-node drains, migration throughput, and validation of the custom
template against every virtualization version are outside scope.

Related: [ECOPROJECT-5285](https://redhat.atlassian.net/browse/ECOPROJECT-5285),
[ADR-0007](0007-virtbench-local-exec-and-replay.md),
[ADR-0008](0008-scenario-table-and-unit-normalization.md),
[ADR-0009](0009-scoped-setup-run-group-individual.md).
