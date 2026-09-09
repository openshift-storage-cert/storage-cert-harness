---
type: workload
title: pvc-density
description: >-
  kube-burner-ocp workload that provisions many PVC/pod pairs to stress storage
  provisioning density and measure bind and pod-ready latencies.
tags: [pvc, density, provisioning, kube-burner-ocp]
resource: kube-burner-ocp/pkg/workloads/pvc-density.go
timestamp: 2026-08-26T12:00:00Z
---

# pvc-density Workload

## Upstream

Implemented in `kube-burner-ocp/pkg/workloads/pvc-density.go` as cobra subcommand
`pvc-density`. Runs template `pvc-density.yml` via kube-burner's `RunWorkload`.

## CLI flags

| Flag | Default | Harness plan param |
|------|---------|-------------------|
| `--storage-class-name` | (cluster default if empty) | from `--backend` / plan `backend:` |
| `--iterations` | `0` | `iterations` |
| `--claim-size` | `256Mi` | `claim_size` |
| `--container-image` | `gcr.io/google_containers/pause:3.1` | `container_image` (optional) |
| `--metrics-profile` | `metrics.yml` | (not exposed in harness yet) |

Harness also appends `--local-indexing` and optional globals: `--qps`, `--burst`,
`--gc`, `--timeout`.

## Behavior

1. Creates a namespace (job name `pvc-density`)
2. For each iteration: creates a PVC (claim size) and a pod mounting it
3. Waits for PVC bind and pod readiness (`verifyObjects`, `waitWhenFinished`)
4. Records latency quantiles for PVC phases and pod conditions
5. Optional cleanup (`cleanup: true` in job config)

Job type: `create`. Typical harness smoke: **5** iterations; full plan: **20**.

## Metrics emitted

Written to the run results directory (collected by harness):

- [jobSummary.json](../output-schema/jobSummary.md)
- [pvcLatencyQuantilesMeasurement-pvc-density.json](../output-schema/latency-quantiles.md)
- [podLatencyQuantilesMeasurement-pvc-density.json](../output-schema/latency-quantiles.md)

## Harness mapping

Plan override `workload: pvc-density` → `workloadSpecs["pvc-density"]` → CLI:

```text
pvc-density --iterations N --claim-size SIZE [--container-image IMG] \
  --local-indexing --storage-class-name SC
```

## Certification context

Primary workload for [TR-STOR-006](../test-requirements/TR-STOR-006.md) via
[kube-burner-ocp adapter](../adapters/kube-burner-ocp.md).

## Example workloadFlags (from golden fixture)

```json
{
  "claimSize": "256Mi",
  "containerImage": "gcr.io/google_containers/pause:3.1",
  "iterations": "5",
  "metricsProfile": "[metrics.yml]",
  "storageClassName": "ontap-san"
}
```
