# Model B — GitHub Actions self-hosted runner (blocked)

**Purpose (historical):** Run `ci-live-smoke.yml` **live-smoke** jobs on an org
**self-hosted** runner VM with Leg 1 (GitHub + registries) and Leg 2 (lab VPN) on
the same host as the `actions-runner` agent.

**Status:** **Not available** — the organization cannot register a GitHub
self-hosted runner with lab network access to `lab.eng.tlv2.redhat.com`.

Do **not** block smoke delivery on runner registration.

## What remains useful from Model B docs

Provisioning **without** the runner agent still applies to **Model C**:

- [TLV2 smoke VM checklist](../../../Docs/Github/storage-cert-harness-smoke-runner-tlv2.md) — Leg 1/Leg 2, VPN, `oc` validation (Steps **B–C** only).

## Superseded implementation artifacts

| Artifact | Notes |
| --- | --- |
| `ci-live-smoke.yml` self-hosted jobs | Should be removed or gated; cluster work moves to Model C |
| GitHub `live-smoke` environment with `CI_CLUSTER_KUBECONFIG` | Do not store lab kubeconfig in GitHub under Model C |
| Runner labels `storage-cert-smoke` | Not used |

## Related Jira (invalid / superseded scope)

| Key | Verdict |
| --- | --- |
| [ECOPROJECT-5462](https://redhat.atlassian.net/browse/ECOPROJECT-5462) | **Superseded** — hosted/self-hosted GHA runner for cluster smoke; close or cancel in favor of Model C |
| [ECOPROJECT-5584](https://redhat.atlassian.net/browse/ECOPROJECT-5584) | **Scope replaced** — was “secrets + `CI_LIVE_SMOKE_ENABLED` + self-hosted runner”; retarget to Model C executor (new sub-task) |

See [smoke-execution-model-c.md](smoke-execution-model-c.md) and
[storage-cert-harness-ci-smoke-test.md](storage-cert-harness-ci-smoke-test.md).
