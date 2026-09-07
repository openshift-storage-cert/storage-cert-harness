---
type: test-requirement
title: "TR-STOR-006: PVC Density and Provisioning"
description: >-
  Validate PersistentVolumeClaim provisioning density — storage backend bind
  latency and capacity under high PVC counts.
tags: [openshift-virt, odf, storage, pvc, provisioning, density, kube-burner, partner-level-3]
resource: TR-STOR-006
timestamp: 2026-08-26T12:00:00Z
---

# TR-STOR-006: PVC Density and Provisioning

## Summary

High-density PVC provisioning via the upstream
[pvc-density](../workloads/pvc-density.md) workload. Creates pods with
PersistentVolumeClaims using a configurable storage class and claim size
(default **256Mi**). Measures provisioner throughput, PVC bind latency, and
system stability under bulk volume creation — independent of VM workloads.

| Field | Value |
|-------|-------|
| Partner level | 3 |
| Verified | unverified |
| Automation tool | [kube-burner-ocp](../adapters/kube-burner-ocp.md) |
| SLA status | to-be-validated (`sla: []` in KB) |
| Checks status | to-be-validated |

## Target SLA (draft)

Key metrics under evaluation (not yet gated in harness):

- PVC bind time (P99)
- Provisioner throughput (PVCs/minute)
- Pod-ready latency with PVC mounts
- Storage class controller queue depth

Acceptable latency limits are **to-be-validated**. Complements TR-STOR-001
(PVC binding attach time &lt;15s at lower scale).

## Risk & failure modes

- Provisioner cannot keep pace with bulk PVC creation → scheduling backlog
- Backend volume-count / metadata limits
- Non-linear PVC bind latency growth as total PVC count rises
- CSI driver connection pool exhaustion under concurrent provision/attach

## Harness integration

- Catalog entry: `automation_tool: kube-burner-ocp`, `sla_count: 0`
- Plan param `workload: pvc-density` selects the subcommand
- Native grading: all kube-burner jobs `passed` ([jobs_passed](../output-schema/harness-parsed-metrics.md))
- Example plans: [tr-stor-006-pvc-density](../plans/tr-stor-006-pvc-density.md) (20 iter),
  [smoke](../plans/tr-stor-006-pvc-density-smoke.md) (5 iter)

## Related

- Automation: [kube-burner-ocp](../adapters/kube-burner-ocp.md) → [pvc-density](../workloads/pvc-density.md)
- Output: [latency quantiles](../output-schema/latency-quantiles.md), [jobSummary](../output-schema/jobSummary.md)
- Pipeline: [stage flow](../pipeline/kube-burner-ocp-stages.md)
