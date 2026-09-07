---
type: index
title: TR-STOR-006 Knowledge Bundle
description: >-
  Root index for OKF concepts covering TR-STOR-006 PVC density certification
  via the storage-cert-harness kube-burner-ocp adapter.
tags: [okf, storage-cert-harness, TR-STOR-006, pvc-density, kube-burner-ocp]
timestamp: 2026-08-26T12:00:00Z
---

# TR-STOR-006 / pvc-density Knowledge Bundle

This bundle helps an agent understand, configure, run, and interpret
[TR-STOR-006](test-requirements/TR-STOR-006.md) using the
[kube-burner-ocp](adapters/kube-burner-ocp.md) harness adapter and the upstream
[pvc-density](workloads/pvc-density.md) workload.

## Start here

| Goal | Concept |
|------|---------|
| What is being certified? | [TR-STOR-006](test-requirements/TR-STOR-006.md) |
| How does the harness run it? | [kube-burner-ocp adapter](adapters/kube-burner-ocp.md) |
| What does upstream execute? | [pvc-density workload](workloads/pvc-density.md) |
| Which YAML plan to use? | [Test plans](plans/index.md) |
| What JSON comes back? | [Output schema](output-schema/index.md) |
| Stage-by-stage flow | [Pipeline stages](pipeline/kube-burner-ocp-stages.md) |
| How do I run it? | [Playbooks](playbooks/index.md) |

## Concept directories

- [test-requirements/](test-requirements/index.md) — certification TR definitions
- [adapters/](adapters/index.md) — automation-tool adapters in the harness
- [workloads/](workloads/index.md) — kube-burner-ocp workload subcommands
- [plans/](plans/index.md) — harness test-plan parameterization
- [output-schema/](output-schema/index.md) — raw and parsed result formats
- [pipeline/](pipeline/index.md) — adapter execution pipeline
- [playbooks/](playbooks/index.md) — live run steps

## Quick run (harness)

```bash
make run-tr-stor-006
```

See [pipeline/kube-burner-ocp-stages.md](pipeline/kube-burner-ocp-stages.md) for
stage-by-stage flow.
