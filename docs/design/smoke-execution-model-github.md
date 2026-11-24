# Model GitHub — GHA self-hosted runner on lab VPN

> **Not applicable.** The organization **cannot** use this model: lab-connected
> **self-hosted** GitHub Actions runners are blocked by policy. Do **not** provision
> an `actions-runner` for cluster smoke or store edge-112 kubeconfig in the GitHub
> `live-smoke` environment for execution on a runner host.
>
> **Use instead:** [Model B — GitLab smoke pipeline](smoke-execution-model-b.md)
> (release mirror) and/or [Model C — lab smoke executor VM](smoke-execution-model-c.md).

This page is kept as **historical design** and for Leg 1/Leg 2 VM guidance that
still applies to Models **B** and **C**.

## Purpose (historical)

Run `ci-live-smoke.yml` **live-smoke** jobs on an org **self-hosted** runner with:

- Leg 1 — public HTTPS (`github.com`, `registry.access.redhat.com`, `quay.io`)
- Leg 2 — lab VPN to `api.ocp-edge112-0.lab.eng.tlv2.redhat.com:6443`

## Status

**Blocked** — no runner registration with lab network access.

## What still applies elsewhere

VM provisioning **without** the runner agent:

- [TLV2 smoke VM checklist](../../../Docs/Github/storage-cert-harness-smoke-runner-tlv2.md) — Leg 1/Leg 2, VPN, `oc` validation

## Superseded artifacts

| Artifact | Notes |
| --- | --- |
| `ci-live-smoke.yml` self-hosted jobs | Resolve/bundle only on GitHub; cluster work not on GHA runner |
| GitHub `live-smoke` + `CI_CLUSTER_KUBECONFIG` for runner execution | Not used under current policy |
| Labels `storage-cert-smoke` on GitHub | Not used |

## Related Jira

| Key | Verdict |
| --- | --- |
| [ECOPROJECT-5462](https://redhat.atlassian.net/browse/ECOPROJECT-5462) | Superseded — GHA runner for cluster smoke |
| [ECOPROJECT-5584](https://redhat.atlassian.net/browse/ECOPROJECT-5584) | Scope replaced — retarget to Model B/C |

See [smoke-execution-models.md](smoke-execution-models.md).
