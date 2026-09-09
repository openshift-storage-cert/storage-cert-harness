---
type: plan
title: tr-virt-019-parallel-vm-lifecycle-smoke
description: >-
  Smoke run of TR-VIRT-019: 2 VMs, 1 DataVolume each, 1 iteration — verifies the
  full virt-parallel pipeline at minimal scale.
tags: [tr-virt-019, virt-parallel, smoke, vm, lifecycle]
resource: plans/tr-virt-019-parallel-vm-lifecycle-smoke.yaml
timestamp: 2026-09-03T00:00:00Z
---

# tr-virt-019-parallel-vm-lifecycle-smoke Plan

Smoke run of [TR-VIRT-019](../test-requirements/TR-VIRT-019.md) via
[virt-parallel](../workloads/virt-parallel.md). Exercises the full lifecycle
pipeline (create, resize, restart, snapshot, migrate, delete) at minimal scale
to confirm end-to-end wiring before a progressive run.

## Key parameters

| Param | Value | Notes |
|-------|-------|-------|
| `initial_vms` | 2 | minimal VM count |
| `increment` | 0 | no scale-up |
| `data_volume_count` | 1 | minimal DVs |
| `max_iterations` | 1 | single iteration |
| `skip_migration_job` | `true` | set on clusters with < 3 schedulable workers |

## Run

```bash
KUBECONFIG=~/.kube/config KUBE_BURNER_OCP_USE_HOST=1 bin/harness run \
  --catalog examples/catalog.tr-virt-019.json \
  --thresholds thresholds.tr-virt-019.json \
  --plan plans/tr-virt-019-parallel-vm-lifecycle-smoke.yaml \
  --backends backends.yaml \
  --workdir ./out --output ./out -v
```

For the full progressive run see [tr-virt-019-parallel-vm-lifecycle](tr-virt-019-parallel-vm-lifecycle.md).
