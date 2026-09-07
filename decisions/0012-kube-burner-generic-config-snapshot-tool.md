# 0012. TR-VIRT-010 VM snapshot test with kube-burner

- **Status:** Proposed
- **Date:** 2026-08-31

## Context

TR-VIRT-010 needs a test for the time from creating a `VirtualMachineSnapshot`
to its `readyToUse` state. Existing virtbench does not produce this measurement,
and kube-burner-ocp has no standalone VM-snapshot workload.

## Decision

Add `internal/tools/kubeburner/`, a sibling to `kubeburnerocp`, that runs the
generic `kube-burner init -c <config>` workflow from the released kube-burner
image. The adapter is registered as `kube-burner` for TR-VIRT-010.

The adapter embeds its kube-burner config and VM/VirtualMachineSnapshot
templates. A run creates the requested VMs, waits for them to run, creates the
requested snapshots, waits for `readyToUse`, and collects kube-burner's
snapshot latency measurements. When the snapshot count is larger than the VM
count, the adapter renders sequential snapshot jobs. Each job creates at most
one snapshot per VM and waits for readiness before the next job starts.

- The test plan keeps kube-burner's `replicas` setting as the VM count and
  adds an optional total `snapshot_count`. When omitted, `snapshot_count`
  defaults to `replicas` to preserve existing plan behavior.
- The selected backend supplies the StorageClass.
- The external threshold bundle supplies the `snapshot_ready_time@p99` SLA.
  No real SLA values are committed in this repository.
- The parser reports the metric in milliseconds; the shared grader normalizes
  units and grades the p99 value.

## Consequences

The adapter has embedded runtime assets and cluster-free parser fixtures. It
requires OpenShift Virtualization, CDI, a compatible StorageClass, and a
VolumeSnapshotClass.

`kubeburner` and `kubeburnerocp` remain separate because one runs generic
embedded configuration and the other runs upstream `-ocp` workloads. They can
be consolidated once their execution paths converge.

The production KB change that maps TR-VIRT-010 to `kube-burner`, and its real
threshold value, remain external to this repository.

## References

- ADR-0003: run tools from released images
- ADR-0004: threshold secret handling
- ADR-0006: KB catalog and threshold contract
- ADR-0008: unit-normalized grading
