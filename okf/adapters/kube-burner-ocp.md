---
type: automation-tool
title: kube-burner-ocp harness adapter
description: >-
  storage-cert-harness adapter for kube-burner-ocp — many workloads, one tool
  integration; workload selected per TR via plan params.
tags: [kube-burner-ocp, adapter, openshift, benchmark]
resource: internal/tools/kubeburnerocp
timestamp: 2026-08-26T12:00:00Z
---

# kube-burner-ocp Adapter

## Registration

Registered in `internal/tools/kubeburnerocp/kubeburner.go` via
`registry.Register(stages.ToolIntegration{...})`.

| Property | Value |
|----------|-------|
| Name | `kube-burner-ocp` |
| Default image | `localhost/kube-burner-ocp:v-src` |
| Provides | [TR-STOR-006](../test-requirements/TR-STOR-006.md), [TR-VIRT-013](../test-requirements/TR-VIRT-013.md), [TR-VIRT-019](../test-requirements/TR-VIRT-019.md) |
| Parallel safe | `false` |
| Exclusivity groups | `kube-burner`, `cdi`, `storage` |

Upstream kube-burner-ocp ships binaries only; the harness builds a runner
image (ADR-0003). Execution uses **podman** by default, or host binary when
`KUBE_BURNER_OCP_USE_HOST=1`.

## Stages implemented

| Stage | Type | Role |
|-------|------|------|
| Preflight | `preflight` | Validate params, log workload/image, optional cluster prereqs |
| Provision | `provisioner` | Create `{workDir}/kubeburner-results/` |
| Run | `runner` | Invoke workload subcommand per TR |
| Collect | `collector` | Gather `jobSummary.json` + `*Quantiles*` JSON |
| Parse | `parser` | Map to [harness metrics](../output-schema/harness-parsed-metrics.md) |
| Teardown | `teardown` | Relies on workload native GC |

Full flow: [pipeline/kube-burner-ocp-stages.md](../pipeline/kube-burner-ocp-stages.md).

## Workloads supported

Defined in `workloadSpecs` (`workloads.go`):

| Plan `workload` param | Subcommand | TR |
|-----------------------|------------|-----|
| `pvc-density` | `pvc-density` | TR-STOR-006 |
| `virt-density` | `virt-density` | TR-VIRT-013 |
| `virt-parallel` | `virt-parallel` | TR-VIRT-019 |

Adding a workload = new `workloadSpec` entry + `testdata/` golden parse tests — no engine changes.

## Grading / evaluators

Per TR evaluator map entry:

```
TR-STOR-006 → grader.NativeOrGraded("native:jobs_passed", ...)
```

- **Native outcome** — all jobs in `jobSummary.json` have `passed: true`
- **SLA bars** — KB `sla: []` (to-be-validated); no threshold grading yet

## Plan parameters

Resolved from plan `overrides.TR-STOR-006` into `Params{Workload, Raw}`.

Required:

- `workload` — e.g. `pvc-density`

Per-workload required (pvc-density):

- `iterations` (int) → `--iterations`
- `claim_size` (string) → `--claim-size`

Optional:

- `container_image` → `--container-image`
- `qps`, `burst`, `gc`, `timeout` → global kube-burner-ocp flags

Storage class is **not** a plan param — sourced from harness `--backend`
(`rc.Backend.StorageClass`) → `--storage-class-name`.

## Environment toggles

| Variable | Effect |
|----------|--------|
| `KUBE_BURNER_OCP_USE_HOST=1` | Run host `kube-burner-ocp` binary instead of podman |

## Duplicate workload handling

If multiple TRs in one job share the same `workload`, only the first runs;
others are **skipped** (shared namespace). Recorded as `OutcomeSkip`.

## Links

- TRs: [TR-STOR-006](../test-requirements/TR-STOR-006.md), [TR-VIRT-013](../test-requirements/TR-VIRT-013.md), [TR-VIRT-019](../test-requirements/TR-VIRT-019.md)
- Workloads: [pvc-density](../workloads/pvc-density.md), [virt-parallel](../workloads/virt-parallel.md)
- Plans: [index](../plans/index.md)
- Output: [output-schema](../output-schema/index.md)
