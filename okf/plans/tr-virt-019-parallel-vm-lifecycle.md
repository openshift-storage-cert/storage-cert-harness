---
type: plan
title: tr-virt-019-parallel-vm-lifecycle
description: >-
  Full progressive-scale TR-VIRT-019 plan: starts at 5 VMs, adds 5 per iteration,
  9 DataVolumes per VM, runs until failure or max-iterations reached.
tags: [tr-virt-019, virt-parallel, progressive, vm, lifecycle]
resource: plans/tr-virt-019-parallel-vm-lifecycle.yaml
timestamp: 2026-09-03T00:00:00Z
---

# tr-virt-019-parallel-vm-lifecycle Plan

Full progressive-scale run of [TR-VIRT-019](../test-requirements/TR-VIRT-019.md)
via [virt-parallel](../workloads/virt-parallel.md).

## Key parameters

| Param | Value | Notes |
|-------|-------|-------|
| `initial_vms` | 5 | starting VM count |
| `increment` | 5 | VMs added per iteration |
| `data_volume_count` | 9 | DataVolumes per VM |
| `vm_image` | `quay.io/containerdisks/fedora:41` | |
| `max_iterations` | (unset) | runs until failure |

## Requirements

- ≥ 3 schedulable worker nodes (migration phase)
- VolumeSnapshotClass on the backend storage class
- KubeVirt + CDI installed

## Run

```bash
KUBECONFIG=~/.kube/config KUBE_BURNER_OCP_USE_HOST=1 bin/harness run \
  --catalog examples/catalog.tr-virt-019.json \
  --thresholds thresholds.tr-virt-019.json \
  --plan plans/tr-virt-019-parallel-vm-lifecycle.yaml \
  --backends backends.yaml \
  --workdir ./out --output ./out -v
```

See [smoke plan](tr-virt-019-parallel-vm-lifecycle-smoke.md) for a quick
end-to-end validation at minimal scale.
