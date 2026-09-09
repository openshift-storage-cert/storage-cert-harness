---
type: output-schema
title: Harness parsed TestResult
description: >-
  core.TestResult produced by kubeburnerocp.ParseResults for TR-STOR-006 —
  metrics, checks, native outcome.
tags: [harness, TestResult, grading]
timestamp: 2026-08-26T12:00:00Z
---

# Harness Parsed Metrics (TR-STOR-006)

## Function

`kubeburnerocp.ParseResults(trID, data)` in `parse.go`.

## TestResult fields

| Field | TR-STOR-006 value |
|-------|-------------------|
| `TRID` | `TR-STOR-006` |
| `Native` | `pass` if all jobs passed; else `fail` |
| `Checks` | `{"jobs_passed": pass\|fail}` |
| `Metrics` | All quantile P50/P95/P99 from `*Quantiles*` files |
| `Raw` | JSON `{"jobSummary": [...]}` |

## Golden fixture expectations (5 iterations)

From `parse_test.go` / `testdata/pvc-density-results/`:

| Metric name | Percentile | Value (ms) |
|-------------|------------|------------|
| `pvcLatency_Bound` | p99 | 1000 |
| `podLatency_Ready` | p99 | 1000 |

Native: **pass** when `jobSummary.json` has `passed: true`.

Native: **fail** when using `jobSummary-fail.json` (even if quantiles present).

## Grading today

Evaluator: `native:jobs_passed` — SLA comparison skipped until KB publishes
thresholds for TR-STOR-006 (`sla_status: to-be-validated`).

Metrics are reported but not gated.

## Parser skip conditions

Per-TR in `parser.Parse`:

- Duplicate workload → `Native: skip`, raw `{"skipped": "..."}`
- No `jobSummary.json` → `Native: fail`, raw `{"error": "..."}`
- Run error propagated in error field when no summary

## Links

- Raw inputs: [jobSummary](jobSummary.md), [latency quantiles](latency-quantiles.md)
- Tool: [kube-burner-ocp](../adapters/kube-burner-ocp.md)
- TR: [TR-STOR-006](../test-requirements/TR-STOR-006.md)
