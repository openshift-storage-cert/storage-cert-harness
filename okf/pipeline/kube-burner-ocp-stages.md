---
type: pipeline
title: kube-burner-ocp adapter stages
description: >-
  End-to-end stage flow for kube-burner-ocp workloads via podman execution.
tags: [pipeline, preflight, provision, run, collect, parse]
timestamp: 2026-08-26T12:00:00Z
---

# kube-burner-ocp Pipeline Stages

## Overview

```text
Preflight → Provision → Run → Collect → Parse → (Grader) → Report
                ↑                              ↓
           results_dir                    LogBundle.Data
```

Tool: [kube-burner-ocp](../adapters/kube-burner-ocp.md).

## 1. Preflight (`preflight.Check`)

- `resolveParams(trs)` from plan overrides
- `validate()` — requires `workload` in `workloadSpecs`
- Logs workload name and image `localhost/kube-burner-ocp:v-src`
- Logs `storage_class` from backend when set
- Cluster prereqs: union of workload `prereqs` (e.g. KubeVirt for virt-density)
- Stores `params` in bag

## 2. Provision (`provisioner.Provision`)

- Creates `{WorkDir}/kubeburner-results/`
- Sets bag `results_dir`

## 3. Run (`runner.Run`)

For each TR (parallel goroutines when multiple cmds):

1. Subdir: `{results_dir}/{TR-ID}/`
2. `buildCLIArgs(params, storageClass)` → e.g. `virt-density --vms-per-node 1 ...`
3. Execute via **podman** (default) or host binary (`KUBE_BURNER_OCP_USE_HOST=1`)
4. Podman mounts: results dir, kubeconfig, kubectl/oc binary, `--network host`
5. Records `kboRun` per TR (TRID, Workload, Subdir, RunErr, Skipped)

### Duplicate workload

Second TR with same `workload` → `Skipped` message, no cmd.

## 4. Collect (`collector.Collect`)

- Reads `kbo_runs` from bag
- `findCollectedMetrics(subdir)` — walk for `jobSummary.json` + `*Quantiles*`
- Keys in `LogBundle.Data`: `{TR-ID}/{filename}`
- Skipped runs omitted

## 5. Parse (`parser.Parse`)

- Per `kboRun`: prefix-filter log data, call `ParseResults`
- Missing summary → fail with run error reason
- Output: `[]core.TestResult` → grader

## 6. Teardown (`teardown.Teardown`)

- No-op — workload handles GC via kube-burner job `cleanup`

## Scheduling constraints

From `ToolIntegration`:

- `ParallelSafe: false`
- `ExclusivityGroups: ["kube-burner", "cdi", "storage"]`
- Plans set `concurrency: 1`

## Environment

| Variable | Effect |
|----------|--------|
| `KUBE_BURNER_OCP_USE_HOST=1` | Run host `kube-burner-ocp` binary instead of podman |

Cluster access (kubeconfig, `kubectl`/`oc` on PATH) is required for live runs.

## Example commands

```bash
# TR-STOR-006 live
make run-tr-stor-006

# TR-VIRT-013 live
make run-tr-virt-013
```

Parser unit tests use `testdata/` via `go test` — no cluster required.

## Output

See [output-schema](../output-schema/index.md) for JSON artifacts and
[harness-parsed-metrics](../output-schema/harness-parsed-metrics.md).

## Plans

- [tr-stor-006-pvc-density-smoke](../plans/tr-stor-006-pvc-density-smoke.md)
- [tr-stor-006-pvc-density](../plans/tr-stor-006-pvc-density.md)
- [tr-virt-013-virt-density-smoke](../plans/tr-virt-013-virt-density-smoke.md)
- [tr-virt-013-virt-density](../plans/tr-virt-013-virt-density.md)
- [Negative plans](../plans/tr-stor-006-negative-plans.md)
