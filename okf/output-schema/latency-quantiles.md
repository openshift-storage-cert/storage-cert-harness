---
type: output-schema
title: Latency quantile measurements
description: >-
  kube-burner-ocp *QuantilesMeasurement JSON files for PVC and pod phase latencies.
tags: [json, latency, quantiles, p99, p95, p50]
timestamp: 2026-08-26T12:00:00Z
---

# Latency Quantile Measurements

## Files (pvc-density)

| File | metricName prefix |
|------|-------------------|
| `pvcLatencyQuantilesMeasurement-pvc-density.json` | `pvcLatency` |
| `podLatencyQuantilesMeasurement-pvc-density.json` | `podLatency` |

Harness Collect walks the results tree and loads any file named `jobSummary.json`
or containing `Quantiles` in the filename.

## Row shape

JSON **array** of quantile rows:

| Field | Type | Description |
|-------|------|-------------|
| `quantileName` | string | Phase or condition name |
| `metricName` | string | e.g. `pvcLatencyQuantilesMeasurement` |
| `jobName` | string | `pvc-density` |
| `P50`, `P95`, `P99` | float | Latency ms |
| `min`, `max`, `avg` | float | Distribution stats |
| `uuid` | string | Links to jobSummary |
| `timestamp` | string | ISO8601 |
| `metadata` | object | Cluster version metadata |

## pvcLatency quantileName values

| quantileName | Meaning |
|--------------|---------|
| `Pending` | Time in Pending phase |
| `Bound` | Time to reach Bound |
| `Lost` | Lost phase (usually 0) |
| `Resize` | Resize phase (usually 0) |

**Key SLA candidate:** `Bound` P99 (golden fixture: **1000 ms** — obfuscated example).

## podLatency quantileName values

| quantileName | Meaning |
|--------------|---------|
| `PodScheduled` | Scheduled latency |
| `Initialized` | Initialized condition |
| `ContainersStarted` | Containers started |
| `PodReadyToStartContainers` | Ready to start containers |
| `ContainersReady` | Containers ready |
| `Ready` | Pod ready |

**Key SLA candidate:** `Ready` P99 (golden fixture: **1000 ms** — obfuscated example).

## Harness metric naming

`quantileMetricName` strips `QuantilesMeasurement` suffix and appends
`_` + `quantileName`:

- `pvcLatencyQuantilesMeasurement` + `Bound` → `pvcLatency_Bound`
- `podLatencyQuantilesMeasurement` + `Ready` → `podLatency_Ready`

Each becomes three `core.Metric` rows (p50, p95, p99) with `Unit: "ms"`.

## Links

- [harness-parsed-metrics](harness-parsed-metrics.md)
- Workload: [pvc-density](../workloads/pvc-density.md)
