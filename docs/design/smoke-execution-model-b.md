# Model B — GitLab smoke pipeline (GitHub `main` mirror)

**Purpose:** Run **live cluster smoke** against the dedicated lab (edge-112) from a
**GitLab CI project** that tracks the **canonical public repository**:

[openshift-storage-cert/storage-cert-harness](https://github.com/openshift-storage-cert/storage-cert-harness)

That GitLab project is also the home for **release, versioning, and stable Quay
image tags** on the harness. **This document covers only the smoke flow** in
GitLab — not the full tag/publish pipeline (see project README and image jobs
when they exist).

**Status:** Design / phase 1 — create GitLab project and wire smoke job.

**Complements:** [Model A](smoke-execution-model-a.md) (laptop), [Model C](smoke-execution-model-c.md)
(cron/systemd on a lab VM without GitLab). Model B centralizes smoke in GitLab
CI with the same scripts as local smoke (`make smoke-ci`, `ci/scripts/run-harness/`).

Index: [smoke-execution-models.md](smoke-execution-models.md).

---

## Phase 1 — Create the GitLab project

| Item | Detail |
| --- | --- |
| **Why a GitLab project** | GitHub hosts **source of truth** and public CI (lint, test, image build on `main`). Lab smoke needs **VPN + kubeconfig** on a runner GitHub cannot use ([Model GitHub](smoke-execution-model-github.md) — not applicable). GitLab holds **internal** CI with a lab-connected runner. |
| **Sibling scope (not detailed here)** | Version bumps, release tags, and promoting a **stable** `quay.io` image after smoke passes. |
| **Smoke scope (this doc)** | On each smoke pipeline: fetch **latest `main`** from GitHub → build → run `harness-smoke-plan.yaml` on edge-112. |

### Project checklist (phase 1)

1. Create a **private** GitLab project (for example under `eco-special-projects` on
   `gitlab.cee.redhat.com`) named for harness release/smoke (exact path TBD by owners).
2. Document in the project description: **mirror of GitHub `main` for lab smoke +
   release**; upstream is `github.com/openshift-storage-cert/storage-cert-harness`.
3. Add a **read-only** Git remote to GitHub (`https` or deploy key) — **do not**
   make GitLab the canonical edit path for application code; merges land on GitHub first.
4. Register a **GitLab runner** on a VM with **Leg 1** (internet: GitHub, Quay, UBI)
   and **Leg 2** (VPN to edge-112) — same network rules as
   [TLV2 smoke VM](../../../Docs/Github/storage-cert-harness-smoke-runner-tlv2.md).
5. Store **CI/CD variables** (masked, protected): kubeconfig (file or base64),
   `HARNESS_BACKENDS_YAML` for `oc-edge-112`, optional `KUBECTL_INSECURE_SKIP_TLS_VERIFY`.
   **Never** commit kubeconfig to the GitLab repo.
6. Scaffold `.gitlab-ci.yml` with a **`smoke`** stage (details below); leave **build
   tag / Quay promote** jobs as placeholders until release owners define them.

---

## Smoke flow (GitLab pipeline)

High level:

```text
GitHub main (canonical)
        │
        │  git fetch / clone (read-only)
        ▼
GitLab project workspace
        │
        ├─ make build
        ├─ make image-build-local   (or pull published image at SHA — policy TBD)
        ├─ make prepare-smoke-env
        └─ make smoke-ci  (or run-image-tests.sh)
                │
                ▼
        edge-112 (lease + harness-smoke-plan.yaml)
```

### Trigger options (smoke only)

| Trigger | Use |
| --- | --- |
| **Manual** | Pipeline run after merge to GitHub `main` (maintainer) |
| **Scheduled** | Nightly regression on `origin/main` (cron in `.gitlab-ci.yml`) |
| **Webhook (optional)** | GitHub `push` to `main` → GitLab pipeline (fetch + smoke) |

PR smoke from GitHub is **out of scope** for phase 1 unless a follow-up passes an
explicit SHA or MR IID; default is **tip of `main`**.

### Job steps (logical)

Run on the **lab-connected** GitLab runner (not `shared` SaaS runners without VPN).

| Step | Command / action |
| --- | --- |
| 1. Sync source | `git fetch origin main && git checkout FETCH_HEAD` (or clone fresh each job) |
| 2. Record SHA | Echo `GITHUB_MAIN_SHA=$(git rev-parse HEAD)` in job log / artifact |
| 3. Toolchain | `make build` |
| 4. Image | `make image-build-local` (smoke uses local ref in `work/local-image-ref`) |
| 5. Stage bundle | `make prepare-smoke-env` |
| 6. Kubeconfig | Materialize from `CI_CLUSTER_KUBECONFIG` (or File variable) → `work/kubeconfig` mode `0600` |
| 7. Backends | Materialize backends YAML → `work/backends.local.yaml` |
| 8. Smoke | `BACKEND=oc-edge-112 make run-harness-smoke` or `make smoke-ci` |
| 9. Lease | Same as local: `smoke-cluster-lease.sh` — one smoke at a time |
| 10. Artifacts | Upload `work/harness-smoke-report/` (scrub secrets); junit/summary TBD |

Failure policy: job **fails** if harness reports error/fail verdict (same as local
smoke). Success on GitLab does **not** auto-merge GitHub PRs — it validates **`main`**
after GitHub CI.

### Example `.gitlab-ci.yml` sketch (smoke stage only)

```yaml
stages:
  - smoke

variables:
  GIT_DEPTH: "0"

smoke-main:
  stage: smoke
  tags:
    - storage-cert-smoke   # lab VPN runner
  rules:
    - if: $CI_PIPELINE_SOURCE == "schedule"
    - if: $CI_PIPELINE_SOURCE == "web"
    - if: $CI_PIPELINE_SOURCE == "api"
  before_script:
    - git remote add github https://github.com/openshift-storage-cert/storage-cert-harness.git || true
    - git fetch github main
    - git checkout FETCH_HEAD
  script:
    - make build
    - make image-build-local
    - install -m 0600 "$KUBECONFIG_FILE" work/kubeconfig
  # prepare-smoke-env + make smoke-ci
```

Adjust variable names to match GitLab File/Masked variable conventions on your
instance. Implement materialization in a small `ci/scripts/gitlab-smoke.sh` when
ready (reuse `prepare-smoke-env.sh`, `run-image-tests.sh`).

---

## Network and secrets

| Layer | Model B |
| --- | --- |
| **Network** | GitLab runner VM — Leg 1 + Leg 2 ([TLV2 doc](../../../Docs/Github/storage-cert-harness-smoke-runner-tlv2.md)) |
| **Auth** | Kubeconfig + backends in **GitLab CI variables** (not in repo) |
| **GitHub** | Public read of `main`; no cluster secrets on GitHub for this path |

Kubeconfig = authentication; VPN = network — both required on the **runner host**.

---

## Relationship to other models

| Model | Role |
| --- | --- |
| **A** | Contributors still run `make smoke-ci` before smoke-affecting PRs |
| **B (this doc)** | Post-merge (or scheduled) smoke of **GitHub `main`** via GitLab |
| **C** | Optional standalone executor (cron/systemd) without GitLab |
| **GitHub** | Lint, test, build, publish; `ci-live-smoke.yml` resolve-only — [not applicable](smoke-execution-model-github.md) |

Avoid running **B** and **C** smoke against edge-112 **at the same time** — share
the cluster lease.

---

## Validation (phase 1 done)

- [ ] GitLab project exists; GitHub `main` fetch works from runner
- [ ] `oc get nodes` on runner with CI kubeconfig
- [ ] One green `make smoke-ci` for current `main` SHA
- [ ] Failed smoke fails the GitLab job
- [ ] Report artifacts contain no kubeconfig or backend passwords

## Related docs

- [smoke-testing.md](../smoke-testing.md) — local commands (same as step 8)
- [storage-cert-harness-ci-smoke-test.md](storage-cert-harness-ci-smoke-test.md) — plan, lease, edge-112
- [ECOPROJECT-5406](https://redhat.atlassian.net/browse/ECOPROJECT-5406) — umbrella
