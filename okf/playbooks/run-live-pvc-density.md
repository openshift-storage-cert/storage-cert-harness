---
type: playbook
title: Run TR-STOR-006 live pvc-density
description: Execute the full-scale pvc-density certification test against a real OpenShift cluster.
tags: [TR-STOR-006, live, openshift, pvc-density]
timestamp: 2026-08-26T15:00:00Z
---

# Run live pvc-density

## Trigger

Certify storage provisioning performance for TR-STOR-006 on a partner cluster.

## Prerequisites

- OpenShift cluster with a working storage class (plan default: `ontap-san`)
- `kubectl` or `oc` on PATH; valid kubeconfig
- `podman` (default) or host `kube-burner-ocp` with `KUBE_BURNER_OCP_USE_HOST=1`
- Harness image `localhost/kube-burner-ocp:v-src` built locally

## Steps

1. Point `backends.example.yaml` (or a gitignored local file) at your real storage class
2. `make build`
3. `make run-tr-stor-006` — uses
   [tr-stor-006-pvc-density](../plans/tr-stor-006-pvc-density.md) (20 iterations, 256Mi)
4. Review report under `./out/tr-stor-006/`

## Scheduling

Adapter is **not parallel-safe**; exclusivity groups: `kube-burner`, `cdi`, `storage`.
Use `concurrency: 1` unless coordinated.

## Troubleshooting

| Symptom | Likely cause |
|---------|--------------|
| `need kubectl or oc on PATH` | CLI missing for podman volume mount |
| `missing required param "claim_size"` | Incomplete plan override |
| `workload requires storage_class` | Backend not selected |
| No `jobSummary.json` | Workload failed before export; check podman logs |

## Related

- [TR-STOR-006](../test-requirements/TR-STOR-006.md)
- [kube-burner-ocp adapter](../adapters/kube-burner-ocp.md)
