# 0012. TR-VIRT-010 VM snapshot test with kube-burner

- **Status:** Proposed
- **Date:** 2026-08-31

## Context

TR-VIRT-010 measures `VirtualMachineSnapshot` creation to `readyToUse`.
virtbench lacks this measurement; kube-burner-ocp lacks a standalone VM-snapshot
workload.

## Decision

Register `internal/tools/kubeburner` as `kube-burner` for TR-VIRT-010, using the
released image's `kube-burner init -c <config>` with embedded configuration and
VM/snapshot templates.

- Create VMs and wait for them to run, then create snapshots and wait for
  readiness. Collect kube-burner's snapshot latency measurements.
- `replicas` is the VM count; optional `snapshot_count` is the total snapshot
  count and defaults to `replicas`. For more snapshots than VMs, render sequential
  jobs, each creating at most one snapshot per VM and waiting before the next job.
- Take the StorageClass from the selected backend. The external threshold bundle
  supplies `snapshot_ready_time@p99`; commit no real SLA values. Parse milliseconds
  and use shared unit-normalized grading.

## Decision notes

### Options considered

- **Generic kube-burner adapter (chosen):** supplies the missing snapshot
  workload through embedded assets; adds a separate adapter to maintain.
- **Existing virtbench / kube-burner-ocp workloads:** lack the required standalone
  measurement. Consolidation can follow if execution paths converge.

### Consequences

Include cluster-free parser fixtures alongside embedded runtime assets.

The test requires OpenShift Virtualization, CDI, a compatible StorageClass, and
a VolumeSnapshotClass. The production KB mapping and real threshold remain
external to this repository.

Related: [ADR-0003](0003-harness-architecture-ports-and-adapters.md),
[ADR-0004](0004-threshold-secret-handling.md),
[ADR-0006](0006-adopt-kb-export-v2-contract.md),
[ADR-0008](0008-scenario-table-and-unit-normalization.md).
