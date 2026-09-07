---
type: index
title: Output Schema
description: Index of kube-burner-ocp raw JSON artifacts and harness-parsed metrics.
tags: [output, schema, metrics, json]
timestamp: 2026-08-26T12:00:00Z
---

# Output Schema

## Raw kube-burner-ocp artifacts

Collected from `{results_dir}/{TR-ID}/` (files matching `jobSummary.json` or
`*Quantiles*`):

| File | Concept |
|------|---------|
| [jobSummary.json](jobSummary.md) | Per-job pass/fail and config |
| [latency quantiles](latency-quantiles.md) | PVC and pod latency P50/P95/P99 |

## Harness-parsed

| Concept | Description |
|---------|-------------|
| [harness-parsed-metrics](harness-parsed-metrics.md) | `core.TestResult` from `ParseResults` |

Flow: Collect → Parse → Grader ([kube-burner-ocp adapter](../adapters/kube-burner-ocp.md)).
