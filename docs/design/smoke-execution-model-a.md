# Model A — Contributor laptop (manual VPN)

**Purpose:** Pre-PR integration validation on a cluster the contributor controls
(personal or team lab). Required before opening PRs that touch smoke **path
includes** in the CI design.

**Status:** Active — not replaced by Model C.

## When to use

- Before opening a PR that changes harness logic, images, catalog, or other
  smoke-included paths (see [storage-cert-harness-ci-smoke-test.md §1](storage-cert-harness-ci-smoke-test.md#1-when-to-trigger-cicd-smoke)).
- Reproduce failures seen on the shared CI lab (edge-112) without consuming the
  lease cluster when you have your own lab.
- On-demand validation while **Model C** or GHA queue paths are not yet enabled.

## Network and credentials

| Item | Requirement |
| --- | --- |
| VPN | Manual connect to lab network when using an in-lab cluster |
| `KUBECONFIG` | Local file under `work/kubeconfig` (gitignored) |
| Backends | `work/backends.local.yaml` or staged from `ci/config/smoke/` |

**Kubeconfig is authentication; VPN is network** — both are required when the API
is not publicly routable.

## Entry points

Primary contributor guide: [smoke-testing.md](../smoke-testing.md).

```sh
make build
make image-build-local
make prepare-smoke-env
cp /path/to/kubeconfig work/kubeconfig
make smoke-ci
# or: make run-harness-smoke
```

Use `plans/harness-smoke-plan.yaml` and the same smoke bundle as CI
(`ci/config/smoke/`).

## Merge gate (interim)

Until Model C **report-back** to GitHub exists, PRs that require smoke should
document a **Model A** run in the PR test plan (pass/fail, date, cluster). See
[storage-cert-harness-ci-smoke-test.md §1.5](storage-cert-harness-ci-smoke-test.md#15-contributor-responsibility-local-lab-non-ci-cluster).

## Related Jira

| Key | Relevance |
| --- | --- |
| [ECOPROJECT-5580](https://redhat.atlassian.net/browse/ECOPROJECT-5580) | Contributor docs / runbook |
| [ECOPROJECT-5582](https://redhat.atlassian.net/browse/ECOPROJECT-5582) | edge-112 sign-off (may use same commands on edge-112 kubeconfig) |
