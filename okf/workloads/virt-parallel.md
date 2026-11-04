---
type: workload
title: virt-parallel
description: >-
  kube-burner-ocp workload that exercises the full VM lifecycle (create, resize,
  restart, snapshot, migrate, delete) at progressive scale with DataVolumes.
tags: [vm, lifecycle, parallel, progressive, kube-burner-ocp, kubevirt, cdi]
resource: kube-burner-ocp/pkg/workloads/virt-parallel.go
timestamp: 2026-09-03T00:00:00Z
---

# virt-parallel Workload

## Upstream

Implemented in `kube-burner-ocp/pkg/workloads/virt-parallel.go` as cobra subcommand
`virt-parallel`. Each iteration runs a sequence of kube-burner jobs.

## CLI flags

| Flag | Harness default | Harness plan param |
|------|---------|-------------------|
| `--storage-class` | (cluster default) | from `--backend` / plan `backend:` |
| `--initial-vms` | `5` | `initial_vms` (optional); catalog `vms` takes precedence and must match if both are set |
| `--increment` | `0` | `increment` (optional) |
| `--data-volume-count` | `9` | `data_volume_count` (optional) |
| `--max-iterations` | `1` | `max_iterations` (optional) |
| `--vm-image` | `quay.io/containerdisks/fedora:41` | `vm_image` (optional) |
| `--vm-cpu` | `1` | `vm_cpu` (optional) |
| `--vm-memory` | `512Mi` | `vm_memory` (optional) |
| `--namespace` | `virt-parallel` | `namespace` (optional) |
| `--min-vol-size` | `0` | `min_vol_size` (optional) |
| `--min-vol-inc-size` | `0` | `min_vol_inc_size` (optional) |
| `--skip-migration-job` | `false` | `skip_migration_job` (optional) |
| `--skip-resize-job` | `false` | `skip_resize_job` (optional) |
| `--skip-restart-job` | `false` | `skip_restart_job` (optional) |
| `--skip-snapshot-job` | `false` | `skip_snapshot_job` (optional) |

Harness also appends `--local-indexing` and optional globals: `--qps`, `--burst`,
`--gc`, `--timeout`.

## Behavior (per iteration)

1. **start-fresh** — delete any leftover namespace from previous runs
2. **create-vms** — create N VMs each with `data-volume-count` DataVolumes; wait for VMI Running
3. **resize-volumes** — patch PVCs to a larger size; wait for propagation
4. **restart-vms** — stop and start each VM; wait for VMI Running
5. **snapshot-vms** — create a VolumeSnapshot per VM; wait for ReadyToUse
6. **propagate-labels** — label nodes for migration targeting (hook)
7. **migrate-vms** — migrate every VM created in the current iteration; wait for successful completion
8. **delete-vms** — delete the namespace; wait for full cleanup

Then N += `increment`. Stops when a job fails or the positive `max-iterations` limit is reached.
The harness rejects `max_iterations: 0` (upstream unlimited mode) for certification.
All lifecycle stages are required for TR-VIRT-019; every `skip_*_job` must be
absent or `false`. In particular, skipping migration cannot certify this test.

## Cluster prerequisites

- KubeVirt / OpenShift Virtualization (`kubevirts.kubevirt.io` CRD)
- CDI (`datavolumes.cdi.kubevirt.io` CRD)
- VolumeSnapshotClass on the storage class (snapshot job)
- Schedulable migration targets with compatible CPU, storage, and network configuration; migration cannot be skipped for certification

## Output files

Collected recursively from the current invocation’s unique run directory. Preserve
relative directories and every iteration’s summaries and individual measurements
when assembling a replay archive; quantiles alone cannot prove migration coverage.

| File | Measurement |
|------|------------|
| `vmiLatencyQuantilesMeasurement-virt-parallel-create-vms-0.json` | VMI/pod latency phases at create |
| `dvLatencyQuantilesMeasurement-virt-parallel-create-vms-0.json` | DataVolume import latency |
| `pvcLatencyQuantilesMeasurement-virt-parallel-create-vms-0.json` | PVC bind latency at create |
| `pvcLatencyQuantilesMeasurement-virt-parallel-resize-volumes-0.json` | PVC resize propagation latency |
| `vmiLatencyQuantilesMeasurement-virt-parallel-restart-vms-0.json` | VMI latency phases at restart |
| `volumeSnapshotLatencyQuantilesMeasurement-virt-parallel-snapshot-vms-0.json` | Snapshot readiness latency |
| `vmimLatencyQuantilesMeasurement-virt-parallel-migrate-vms-0.json` | Migration latency |
| `vmiLatencyMeasurement-virt-parallel-create-vms-*.json` | Required distinct created VM identities for each iteration |
| `vmimLatencyMeasurement-virt-parallel-migrate-vms-*.json` | Required successful migrations, matched by run UUID, job, namespace, and VMI name |
| `jobSummary.json` (every iteration) | Required per-job pass/fail, workload flags, run UUID, and elapsed time |

The `*` in individual measurement filenames is the iteration index. Each created
VM must have a successful individual migration measurement (`vmimName` and
nonnegative `succeededLatency`). Missing or malformed evidence reports ERROR;
incomplete migration or missing/disabled lifecycle stages reports FAIL. Partial
runs retain created/migrated counts and stage outcomes in `lifecycle` diagnostics.
Repeated latency quantiles are reduced to the maximum value per metric and
percentile across iterations for conservative, deterministic SLA grading; this
is not a percentile recomputed over the combined population.

Complete-population migration requires the upstream change in
[kube-burner-ocp #542](https://github.com/kube-burner/kube-burner-ocp/pull/542).
Older binaries that only migrate VMs from one randomly chosen worker cannot
prove TR-VIRT-019 when that leaves other test VMs unmigrated. The harness checks
evidence independently of the tool version and will reject incomplete coverage.
