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

| Flag | Default | Harness plan param |
|------|---------|-------------------|
| `--storage-class` | (cluster default) | from `--backend` / plan `backend:` |
| `--initial-vms` | `5` | `initial_vms` (optional) |
| `--increment` | `5` | `increment` (optional) |
| `--data-volume-count` | `9` | `data_volume_count` (optional) |
| `--max-iterations` | `0` (run until failure) | `max_iterations` (optional) |
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
7. **migrate-vms** — migrate all VMs off a randomly selected node; wait for completion
8. **delete-vms** — delete the namespace; wait for full cleanup

Then N += `increment`. Stops when a job fails or `max-iterations` is reached (0 = unlimited).

## Cluster prerequisites

- KubeVirt / OpenShift Virtualization (`kubevirts.kubevirt.io` CRD)
- CDI (`datavolumes.cdi.kubevirt.io` CRD)
- VolumeSnapshotClass on the storage class (snapshot job)
- ≥ 3 schedulable workers (migration job selects a random node; use `skip_migration_job: true` on 2-worker clusters)

## Output files

Collected by the harness from the run directory (`*Quantiles*` + `jobSummary.json`):

| File | Measurement |
|------|------------|
| `vmiLatencyQuantilesMeasurement-virt-parallel-create-vms-0.json` | VMI/pod latency phases at create |
| `dvLatencyQuantilesMeasurement-virt-parallel-create-vms-0.json` | DataVolume import latency |
| `pvcLatencyQuantilesMeasurement-virt-parallel-create-vms-0.json` | PVC bind latency at create |
| `pvcLatencyQuantilesMeasurement-virt-parallel-resize-volumes-0.json` | PVC resize propagation latency |
| `vmiLatencyQuantilesMeasurement-virt-parallel-restart-vms-0.json` | VMI latency phases at restart |
| `volumeSnapshotLatencyQuantilesMeasurement-virt-parallel-snapshot-vms-0.json` | Snapshot readiness latency |
| `vmimLatencyQuantilesMeasurement-virt-parallel-migrate-vms-0.json` | Migration latency (when not skipped) |
| `jobSummary.json` | Per-job pass/fail + elapsed time |
