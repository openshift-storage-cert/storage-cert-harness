---
type: test-plan
title: tr-stor-006-pvc-density-smoke
description: >-
  Fast TR-STOR-006 smoke — 5 PVC/pod pairs, 256Mi. Live on cluster.
tags: [plan, TR-STOR-006, smoke]
resource: plans/tr-stor-006-pvc-density-smoke.yaml
timestamp: 2026-08-26T12:00:00Z
---

# Plan: tr-stor-006-pvc-density-smoke

Source: `plans/tr-stor-006-pvc-density-smoke.yaml`

## Intent

Quick validation of the [kube-burner-ocp](../adapters/kube-burner-ocp.md) adapter
and [pvc-density](../workloads/pvc-density.md) on a live cluster.

## Configuration

| Key | Value |
|-----|-------|
| `backend` | `ontap-san` |
| `select.ids` | `TR-STOR-006` |

## TR overrides

| Param | Value |
|-------|-------|
| `workload` | `pvc-density` |
| `iterations` | `5` |
| `claim_size` | `256Mi` |

## Related

- Full plan: [tr-stor-006-pvc-density](tr-stor-006-pvc-density.md)
- Pipeline: [kube-burner-ocp stages](../pipeline/kube-burner-ocp-stages.md)
