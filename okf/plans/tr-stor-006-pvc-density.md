---
type: test-plan
title: tr-stor-006-pvc-density
description: >-
  Full TR-STOR-006 pvc-density plan — 20 PVC/pod pairs, 256Mi claims, ontap-san
  backend. SLA bars to-be-validated.
tags: [plan, TR-STOR-006, pvc-density, ontap-san]
resource: plans/tr-stor-006-pvc-density.yaml
timestamp: 2026-08-26T12:00:00Z
---

# Plan: tr-stor-006-pvc-density

Source: `plans/tr-stor-006-pvc-density.yaml`

## Intent

TR-STOR-006 pvc-density as authored in the KB: high-density PVC provisioning.
**Do not invent thresholds** — KB `sla: []` until partner bars exist in
`thresholds.json`.

## Configuration

| Key | Value |
|-----|-------|
| `schema_version` | `"1"` |
| `name` | `tr-stor-006-pvc-density` |
| `concurrency` | `1` |
| `backend` | `ontap-san` |
| `select.ids` | `TR-STOR-006` |

## TR overrides (`TR-STOR-006`)

| Param | Value |
|-------|-------|
| `workload` | `pvc-density` |
| `iterations` | `20` |
| `claim_size` | `256Mi` |

## Run example

```bash
bin/harness run \
  --catalog examples/catalog.tr-stor-006.json \
  --thresholds examples/thresholds.example.json \
  --plan plans/tr-stor-006-pvc-density.yaml \
  --backends examples/backends.example.yaml
```

## Related

- TR: [TR-STOR-006](../test-requirements/TR-STOR-006.md)
- Smaller run: [tr-stor-006-pvc-density-smoke](tr-stor-006-pvc-density-smoke.md)
