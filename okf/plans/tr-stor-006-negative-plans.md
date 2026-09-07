---
type: test-plan
title: TR-STOR-006 negative plans
description: >-
  Harness plans that use non-existent storage classes so PVCs never bind; harness
  must report non-pass verdict.
tags: [plan, negative-test, TR-STOR-006]
timestamp: 2026-08-26T12:00:00Z
---

# Negative Test Plans

Two plans verify failure paths when storage class is invalid.

## tr-stor-006-pvc-density-negative

Source: `plans/tr-stor-006-pvc-density-negative.yaml`

| Field | Value |
|-------|-------|
| `backend` | `no-such-sc` |
| `iterations` | `3` |
| `claim_size` | `256Mi` |
| `timeout` | `3m` |

PVCs cannot bind → kube-burner times out → `jobSummary.passed: false`.

## tr-stor-006-pvc-density-negative2

Source: `plans/tr-stor-006-pvc-density-negative2.yaml`

| Field | Value |
|-------|-------|
| `backend` | `typo-sc` (maps to typo'd class `ontap-sann`) |
| `iterations` | `3` |
| `timeout` | `2m` |

## Expected harness behavior

- Native outcome: **fail** (`jobs_passed` check fails)
- `executionErrors` example: `timed out waiting for actions to be completed`

## Related

- [jobSummary schema](../output-schema/jobSummary.md)
- [TR-STOR-006](../test-requirements/TR-STOR-006.md)
