# Smoke lab configuration (tracked)

Files in this directory are the **canonical** harness smoke bundle staged into
`work/` by `make prepare-smoke-env` (see
[`ci/scripts/run-harness/README.md`](../../scripts/run-harness/README.md)).
The tracked `backends.local.yaml` contains the shared smoke-lab backend
definitions and no credentials. Do not add secrets to it; use local secret
files or Kubernetes Secrets as documented in `backends.example.yaml`.

## Change control

**Do not edit these files without inspection and review** from harness smoke
maintainers. They drive local smoke, CI staging, and merge gates; unintended
changes can hide regressions or break shared lab clusters.

| File | Staged to `work/` |
| --- | --- |
| `backends.local.yaml` | `backends.local.yaml` |
| `thresholds-smoke.json` | `harness-run-thresholds.json` |
| `thresholds.json` | `thresholds.json` (Quay / container-smoke path) |
| `cluster-smoke-lease.yaml.template` | (used by lease script, not copied) |

Repo-root `catalog.json` is copied to `work/catalog.json` during prepare (not
duplicated here).

**Not staged:** `work/kubeconfig` — you must provide cluster credentials locally
or via CI secrets.
