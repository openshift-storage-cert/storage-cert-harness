---
type: output-schema
title: jobSummary.json
description: >-
  kube-burner-ocp per-job summary array — native pass/fail and workload metadata.
tags: [json, jobSummary, native-outcome]
timestamp: 2026-08-26T12:00:00Z
---

# jobSummary.json

## Role

Primary **native outcome** source. Harness `parseJobSummaries`:

- Missing or empty → **fail**
- Any row with `passed: false` → native **fail**
- All rows `passed: true` → native **pass**

Also stored in `TestResult.Checks["jobs_passed"]` and `TestResult.Raw`.

## Top-level shape

JSON **array** of job summary objects.

## Key fields

| Field | Type | Description |
|-------|------|-------------|
| `metricName` | string | `"jobSummary"` |
| `passed` | bool | Job success — drives native grading |
| `elapsedTime` | float | Elapsed seconds |
| `executionErrors` | string | Present on failure (e.g. timeout) |
| `uuid` | string | Run UUID |
| `workloadFlags` | object | CLI flags used |
| `jobConfig` | object | kube-burner job definition |

### jobConfig (selected)

| Field | Example | Meaning |
|-------|---------|---------|
| `name` | `pvc-density` | Job / namespace name |
| `jobIterations` | `5` | Iteration count |
| `jobType` | `create` | Create workload |
| `qps` | `5` | API QPS |
| `burst` | `10` | API burst |
| `gc` | `false` | Garbage collection flag |
| `verifyObjects` | `true` | Verify created objects |
| `waitWhenFinished` | `true` | Wait for completion |

### workloadFlags (pvc-density example)

```json
{
  "claimSize": "256Mi",
  "containerImage": "gcr.io/google_containers/pause:3.1",
  "iterations": "5",
  "metricsProfile": "[metrics.yml]",
  "storageClassName": "ontap-san"
}
```

## Failure example

From `jobSummary-fail.json` (negative / fixture-fail):

```json
{
  "passed": false,
  "executionErrors": "timed out waiting for actions to be completed",
  "workloadFlags": { "storageClassName": "harness-nonexistent-sc" }
}
```

## Links

- Parsed metrics: [harness-parsed-metrics](harness-parsed-metrics.md)
- Negative plans: [tr-stor-006-negative-plans](../plans/tr-stor-006-negative-plans.md)
