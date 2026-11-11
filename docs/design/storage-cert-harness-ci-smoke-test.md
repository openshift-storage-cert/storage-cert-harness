# Storage certification harness — CI/CD live cluster smoke test

## Status

In progress (2026-10-12 proposed; 2026-10-14 PR smoke, edge-112, lease; **2026-10-21**
implementation on `smoke-plan-single` — local smoke path largely wired; cluster
lease wired; **2026-11-10** — lab-connected **GitHub self-hosted runner blocked**;
adopt **Model C** lab executor VM (§2.6)). See [§13 Implementation progress](#13-implementation-progress).

## Summary

Define how the `storage-cert-harness` repository runs **live-cluster smoke**
against the CI OpenShift lab (initial target: **edge-112**) using
`plans/harness-smoke-plan.yaml` with **minimal overrides** (VM counts, iterations,
timeouts) and portable CI scripts (`run-image-tests.sh`, `live-cluster-smoke.sh`).

The smoke plan covers **all non-destructive automated TRs** in `catalog.json`
(**excluding HA** / `TR-VIRT-007` and any TR that exceeds **20 minutes** at smoke
settings after tuning). **At most one smoke run may use the cluster at a time**;
there is **no queue** — a second run fails fast if the lease is held.

**Team trigger model (decided):**

- **No automatic smoke** on PR open, push to `main`, or the default `CI` workflow.
- Smoke runs only when explicitly requested:
  - **On demand:** PR comment `/run-live-smoke` (trusted association), or
    `workflow_dispatch` with PR number (Actions UI / allowed environment actors).
  - **Scheduled:** once daily (`05:00 UTC`); runs only if at least one **open**
    same-repo PR to `main` was **updated** in the **last 24 hours** and does not
    contain `[skip live-smoke]`.
- When a run is scheduled, the **same guards** apply as on-demand: canonical repo,
  same-repo head, skip token, fork rejection, cluster lease, environment secrets
  (path-filter rules in §1 remain policy for when smoke is *required* before merge,
  not a separate automatic trigger).
- Contributors **must** run the smoke plan at least once on a **personal /
  dedicated non-CI lab** before opening a PR when their diff would require smoke
  at merge time (documented in `README.md` and `docs/contributing.md`).
- **Failed PR smoke is a merge gate:** the live-smoke check must **pass** before
  the PR can merge. Investigate failures (logs, `report.json`, artifacts), fix or
  re-run on demand (§1.4) — do not merge with a failed smoke run. `[skip
  live-smoke]` is only for when the dedicated storage/cluster is **unavailable**
  (smoke not scheduled), not to bypass a failed run.
- Changes to **`go.mod` / `go.sum` / `vendor/**` alone do not trigger** smoke.

Measure **per-TR and full-suite** duration on edge-112 before locking CI
overrides. Image build/publish on `main` stays on the existing `ci-images` /
`ci-publish` path.

**Infrastructure constraint (2026-11):** The organization **cannot** register a
**GitHub self-hosted runner** that has **lab network access** (Leg 2 to
`lab.eng.tlv2.redhat.com`). GitHub-hosted `ubuntu-latest` jobs may reach GitHub
and public registries only. **Live cluster smoke therefore does not run inside
GitHub Actions** against the lab. Adopt **Model C** — a **dedicated lab VM**
that **polls** `main` on a schedule (and optionally PR smoke requests), builds or
pulls the harness image, and runs the same portable scripts as local smoke.
Model **A** (contributor laptop + VPN) remains required pre-PR validation;
Model **B** (GHA self-hosted runner on a lab VM) is **not available** — do not
block delivery on runner registration. Details:
[smoke-execution-models.md](smoke-execution-models.md) (A / B / C).

### Jira validity (2026-11, Model C constraint)

Stories and sub-tasks under [ECOPROJECT-5406](https://redhat.atlassian.net/browse/ECOPROJECT-5406)
after the lab-runner policy change:

| Key | Verdict | Notes |
| --- | --- | --- |
| [ECOPROJECT-5462](https://redhat.atlassian.net/browse/ECOPROJECT-5462) | **Superseded** | Entire AC assumed org **self-hosted GHA runner** — not allowed; use Model C |
| [ECOPROJECT-5584](https://redhat.atlassian.net/browse/ECOPROJECT-5584) | **Scope replaced** | Was GitHub secrets + self-hosted runner; superseded by **Model C** sub-task |
| [ECOPROJECT-5587](https://redhat.atlassian.net/browse/ECOPROJECT-5587) | **Retarget** | “First green run” = lab VM timer, not GHA self-hosted job |
| [ECOPROJECT-5588](https://redhat.atlassian.net/browse/ECOPROJECT-5588) | **Retarget** | On-demand = GHA enqueue + VM execution |
| [ECOPROJECT-5586](https://redhat.atlassian.net/browse/ECOPROJECT-5586) | **Blocked** | Required check until VM **report-back** to GitHub |
| [ECOPROJECT-5535](https://redhat.atlassian.net/browse/ECOPROJECT-5535) | **Done** (partial) | GHA resolve timeouts; VM wrapper timeout is Model C work |
| [ECOPROJECT-5536](https://redhat.atlassian.net/browse/ECOPROJECT-5536) | **Done** | Skip token; still applies to GHA scheduling only |
| [ECOPROJECT-5569](https://redhat.atlassian.net/browse/ECOPROJECT-5569) | **Done** | Lease core; janitor remains **D2** / **B2** |
| [ECOPROJECT-5580](https://redhat.atlassian.net/browse/ECOPROJECT-5580)–**5583**, **5589**–**5592**, **5598** | **Still valid** | Docs, sign-off, TRs, fork guard, cleanup, parent story text — adjust AC text where they mention “self-hosted runner” |

New work: **[ECOPROJECT-5612](https://redhat.atlassian.net/browse/ECOPROJECT-5612)** — Model C lab executor VM (parent **5406**).

## Architecture overview

```mermaid
flowchart LR
  subgraph github [GitHub Actions ubuntu-latest]
    CI[lint test build publish]
    RES[Optional resolve jobs bundle PR metadata]
  end
  subgraph labvm [Lab smoke executor VM Model C]
    CRON[systemd timer or cron]
    SYNC[git fetch main or PR ref]
    IMG[podman build or pull published image]
    LS[live-cluster-smoke.sh or make smoke-ci]
    OUT[results logs notify]
    CRON --> SYNC --> IMG --> LS --> OUT
  end
  subgraph cluster [edge-112 CI cluster]
    LE[Lease max 1 holder]
    PRE[Preflight]
    RUN[harness run smoke plan]
    CLN[Cleanup and release lease]
    LE --> PRE --> RUN --> CLN
  end
  CI -.->|no lab API path| cluster
  RES -.->|future queue file or API poll| labvm
  LS --> LE
```

**Building blocks:** smoke plan and thresholds (today:
`plans/harness-smoke-plan.yaml`, `thresholds-smoke.json`; target bundle:
`ci/config/smoke/` — §6.5), `ci/scripts/run-harness/run-image-tests.sh`,
`ci/scripts/run-harness/prepare-smoke-env.sh` (§6.6),
`ci/scripts/run-harness/live-cluster-smoke.sh` (lease wired; harness run pending ECOPROJECT-5406), offline `run-harness/replay-smoke.sh` (not a
substitute for live smoke).

## Goals and non-goals

| Goals                                                                                        | Non-goals                                                      |
| -------------------------------------------------------------------------------------------- | -------------------------------------------------------------- |
| Catch regressions across **all** catalog automations (smoke scale) on a real OCP+CNV cluster | Full Level-3 certification scale or HA/fencing (`TR-VIRT-007`) |
| Run on contributor on-demand and optional daily schedule when open PRs had activity in 24h | Running on every PR commit, on PR open, or on push to `main` |
| Exclusive, safe use of a shared or dedicated CI cluster                                      | Storing real threshold bars or kubeconfig in git               |
| Deterministic preflight, minimal plan params, cleanup on all exit paths                      | Publishing graded reports with partner SLA secrets             |

## Related work and Jira traceability

| Jira                                                                   | Summary                                         | Design coverage                                                      |
| ---------------------------------------------------------------------- | ----------------------------------------------- | -------------------------------------------------------------------- |
| [ECOPROJECT-5406](https://redhat.atlassian.net/browse/ECOPROJECT-5406) | Live cluster smoke (Jira text may still say post-merge) | PR smoke + edge-112; update Jira to match §1 |
| [ECOPROJECT-5462](https://redhat.atlassian.net/browse/ECOPROJECT-5462) | Hosted runners for cluster smoke                | **Superseded** — see [5612](https://redhat.atlassian.net/browse/ECOPROJECT-5612) |
| [ECOPROJECT-5612](https://redhat.atlassian.net/browse/ECOPROJECT-5612) | Model C lab smoke executor VM                   | [smoke-execution-model-c.md](smoke-execution-model-c.md) |
| [ECOPROJECT-5536](https://redhat.atlassian.net/browse/ECOPROJECT-5536) | Opt-out: skip live smoke from PR / merge commit | Section 1 — **Skip live smoke**; `docs/ci.md`, PR template           |

### ECOPROJECT-5536: skip live smoke on demand

[Jira: ECOPROJECT-5536](https://redhat.atlassian.net/browse/ECOPROJECT-5536) —
sub-task of [ECOPROJECT-5406](https://redhat.atlassian.net/browse/ECOPROJECT-5406).
Long smoke should not block every `main` push (~2+ hours end-to-end when smoke
runs). Documented in `storage-cert-harness/docs/ci.md` and
`.github/pull_request_template.md`.

#### Scope

- Recognize an opt-out token in the **commit message** that lands on `main` (squash
merge commit message = PR title/body — document where contributors put the token).
- Skip the `live-smoke` job only; lint, unit test, build, image scan, and publish
continue unchanged unless a separate gate says otherwise.
- Emit a clear log line and GitHub check conclusion **Skipped** (not **Success**),
and surface skip in the workflow summary (SHA, actor, token used).
- Restrict who may skip (optional): only org members, or require `storage-cert-harness`
maintainer — fork pushes to `main` are already impossible on protected branches.

#### Acceptance criteria

- PR title/body containing `[skip live-smoke]` does not start cluster work.
- On-demand and scheduled workflows respect the skip token; no workflow runs smoke on PR open.
- Token is case-sensitive and documented in `docs/ci.md` and PR template.
- Skipped runs do not hold the cluster lease.
- Audit: workflow run records `live_smoke_skipped=true` in the summary for later
nightly or manual smoke backlog.

**Suggested tokens** (pick one in implementation): `[skip live-smoke]` primary;
avoid reusing `[skip ci]` (too broad).

### Drift from ECOPROJECT-5406 description (AC implied in story body)

The Jira description still references `plans/virtbench-bootstorm-live-smoke.yaml` and
a boot-storm-only smoke. Implementation and this design use an expanded
`plans/harness-smoke-plan.yaml` aligned with
`plans/level3-internal-minimal.yaml` (all catalog TRs, smoke overrides) but
**without** `TR-VIRT-007`. Image build/push is implemented via GitHub Actions,
not a single combined shell script named `ci/live-smoke.sh`.
The design keeps **portable scripts** under `storage-cert-harness/ci/scripts/`
(e.g. extend `run-image-tests.sh` or add `live-cluster-smoke.sh`) as the shared
entry point for CI and local runs.

### Execution models and Jira

| Model | Document |
| --- | --- |
| A — Contributor laptop | [smoke-execution-model-a.md](smoke-execution-model-a.md) |
| B — GHA self-hosted runner (blocked) | [smoke-execution-model-b.md](smoke-execution-model-b.md) |
| C — Lab smoke executor VM | [smoke-execution-model-c.md](smoke-execution-model-c.md) |

Index: [smoke-execution-models.md](smoke-execution-models.md).

[ECOPROJECT-5462](https://redhat.atlassian.net/browse/ECOPROJECT-5462) is
**superseded** (see summary **Jira validity**). Lab automation is **ECOPROJECT-5406**
Model C sub-task. `ci-live-smoke.yml` stays **resolve-only** on `ubuntu-latest`
until VM **report-back** exists.

**Reference pattern (external):** Ansible `vmware.vmware` — hosted matrix + lab
host that polls GitHub; no inbound from GitHub to the lab (same as Model C).

---

## 1. When to trigger CI/CD smoke

### Event (team decision)

| Event | Run live smoke automatically? | Notes |
| --- | --- | --- |
| `pull_request` → `main` (any type) | **No** | No automatic smoke on open or new commits |
| Contributor **on demand** (`/run-live-smoke`, `workflow_dispatch`) | Yes when allowed | [§1.4](#14-contributor-on-demand-smoke-on-a-pr) |
| **`schedule`** (daily) | **Conditional** | Only if an open same-repo PR to `main` was updated in the last 24h and not skipped |
| Push to `main` | **No** | Lint/build/publish only |
| `workflow_dispatch` | On demand | `pr_number` only (SHA resolved on `ubuntu-latest`; **cluster step on lab VM**) |
| Fork PR → upstream | **No** | No upstream kubeconfig |

**Merge gate:** Target state: PR live-smoke is a **required** status check on pull
requests to `main` (branch protection) once the **lab executor** can **report back**
to GitHub (§2.6.6). Until then, require **Model A** smoke evidence in the PR (§1.5)
and maintainer review; do not enable a required GHA job that cannot reach the
cluster. When report-back exists, a **failed** reported run **blocks merge** until
a **passing** re-run is recorded (on-demand §1.4).

**Frequency:** Opening a PR triggers **at most one** automatic smoke. Pushing
additional commits to the PR branch does **not** trigger another automatic run;
contributors re-run smoke via comment or Actions UI when they need it.

### Forks and untrusted repositories

Live smoke must **never** run from a fork or expose upstream cluster secrets to
fork workflows. Required behavior:

| Scenario | Live smoke | `CI_CLUSTER_KUBECONFIG` / live-smoke secrets |
| --- | --- | --- |
| PR from a **fork** into upstream `main` | **Not triggered** (no `pull_request` trigger on live-smoke workflow) | **Not available** — GitHub does not pass upstream secrets to `pull_request` workflows for fork PRs |
| Push to **fork** `main` (copy of the repo) | **Not triggered** — job `if` requires canonical repository (below) | **Not available** — only secrets defined on the fork exist; upstream kubeconfig is never injected |
| Push to **upstream** `main` | No automatic smoke | N/A for default pipeline |
| Same-repo PR on demand | Yes when allowed | `environment: live-smoke` on upstream |
| Daily schedule | Yes only if open same-repo PR updated in last 24h (and not skipped) | Environment-gated |
| `workflow_dispatch` on upstream | Contributors with env access | Environment-gated |

**Implementation guards (GitHub):**

1. **Separate workflow** `ci-live-smoke.yml` with:
   - **`workflow_dispatch`** (PR number input) and **`issue_comment`** `/run-live-smoke`
     for on-demand runs — [§1.4](#14-contributor-on-demand-smoke-on-a-pr);
   - **`schedule`** (daily): smoke only when an open same-repo PR to `main` was
     updated in the last 24 hours and is not `[skip live-smoke]`;
   - **no** `pull_request`, **no** `push` to `main` for smoke.
   **Never** `pull_request_target` with secrets to execute fork-supplied code.
2. **Repository and same-repo guard:**

   ```yaml
   if: >-
     github.repository == 'openshift-storage-cert/storage-cert-harness' &&
     # same-repo PR resolved via Pulls API in resolve-* jobs; reject fork heads
     !contains(pr.title, '[skip live-smoke]') &&
     !contains(pr.body, '[skip live-smoke]')
   ```

   Adjust the string if the canonical GitHub org/repo name changes; document the
   value in `docs/ci.md`.
3. **Environment `live-smoke`** — store `CI_CLUSTER_KUBECONFIG`,
   `HARNESS_THRESHOLDS_JSON`, and `HARNESS_BACKENDS_YAML` as **environment
   secrets**, not plain repo secrets exposed to every workflow. Restrict
   deployment branches limited to upstream (PR jobs use environment only when
   same-repo PR rules are satisfied).
4. **Do not** pass cluster secrets via `secrets: inherit` on the top-level `CI`
   workflow for fork-visible jobs; the live-smoke reusable workflow (if any)
   should reference `environment: live-smoke` only on the guarded job.
5. **No lab kubeconfig in GitHub** — cluster credentials live **only** on the lab
   smoke executor VM (§2.6). Do not register lab-connected self-hosted runners.
   `ci-live-smoke.yml` must not materialize `CI_CLUSTER_KUBECONFIG` on
   `ubuntu-latest` jobs.

**Verification:** open a test PR from a personal fork — upstream `CI` lint/test/build
may run, but **no** `live-smoke` job appears and no workflow step receives
kubeconfig. Push to a fork’s `main` — same (no smoke job or job skipped with
“wrong repository”).

### Path filter (relevant changes only)

Run live smoke when **any** included path changes between the previous `main`
commit and `HEAD`. Skip the job entirely when the diff touches only excluded
paths.

**Include** (executable / packaged-tool impact):

| Path pattern                                                                                                            | Reason                                  |
| ----------------------------------------------------------------------------------------------------------------------- | --------------------------------------- |
| `cmd/**`, `internal/**`                                                                                                 | Harness logic, orchestration, tools     |
| `catalog.json`, `thresholds.json`                                                                                       | Test definitions and grading inputs     |
| `Containerfile`, `ci/config/images.env`, `ci/config/images.sh`                                                          | Runtime image composition               |
| Builder / tool pins that change image contents (e.g. `BUILD_IMAGE`, packaged tool versions in Containerfile build-args) | “Harness-builder image (packaged tool)” |
| `schemas/**`                                                                                                            | Plan/report validation behavior         |

**Exclude** (do not trigger live smoke alone):

| Path pattern                                                | Reason                                                                                                             |
| ----------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| `docs/**`, `*.md` (except if co-changed with include paths) | Documentation                                                                                                      |
| `ci/scripts/**`, `.github/**`             | CI wiring — unless paired with include paths in the same push                                                      |
| `plans/**`                                                  | Plan-only edits (smoke plan is versioned but stable; changing overrides without code should not burn cluster time) |
| `decisions/**`, `okf/**`, `skills/**`, `AGENTS.md`          | Process / KB metadata                                                                                              |
| `go.mod`, `go.sum`, `vendor/**`                             | **Dependency-only** — do **not** trigger live smoke (unit test + image build still run on PR)                      |

**Implementation notes:**

- GitHub: `paths` / `paths-ignore` on a dedicated workflow, or a first-step
`dorny/paths-filter` (or equivalent) job that sets `run_live_smoke=true`.
- If a single commit mixes doc and code changes, **run** smoke (include wins).

### Relationship to image build

- **Default:** live smoke `needs` successful `image-build` artifact for the same
SHA (load `dist/harness-image.tar` or use image tag `${NEXT-IMAGE-VERSION}` for
that commit).
- **Exception:** if only `Containerfile` / builder pins change and binary is
unchanged, still rebuild image — smoke validates the **image** that partners run.

### Skip live smoke (`[skip live-smoke]`)

Full-catalog smoke can take **hours** (see §7). The case-sensitive token
`[skip live-smoke]` in the **PR title or body** skips scheduling live smoke for
that PR (automatic and on-demand workflows must respect it).

| Mechanism | Behavior |
| --- | --- |
| PR title/body contains `[skip live-smoke]` | No live-smoke job scheduled; **merge allowed without smoke** — lab outage/maintenance only; record `live_smoke_skipped=true` |
| Dedicated storage/cluster **down** or lease unavailable | Use skip token; backlog manual smoke when the lab returns — **not** for hiding harness failures |
| Smoke job **failed** (harness error/fail verdict) | **Blocks merge** until fixed and a **passing** on-demand re-run |
| No token, smoke passes | Merge allowed when other required checks pass |

**Policy:** Skipping is for maintenance, lab outages, or emergencies — not routine.
PR template documents the token ([ECOPROJECT-5536](https://redhat.atlassian.net/browse/ECOPROJECT-5536)).
Skipped runs **never** acquire the cluster lease.

### 1.4 Contributor on-demand smoke on a PR

There is **no** automatic smoke when a PR is opened. Contributors (or maintainers)
request smoke explicitly:

| Mechanism | Suggested implementation | Who |
| --- | --- | --- |
| **PR comment** | `/run-live-smoke` on `issue_comment` where `issue.pull_request` is set; bot/workflow validates same-repo + not `[skip live-smoke]` + lease free | Contributors with `write` or `maintain` on repo (tune in workflow) |
| **GitHub CLI** | `gh pr comment <PR> --repo openshift-storage-cert/storage-cert-harness --body '/run-live-smoke'` | Same as PR comment (triggers `issue_comment` workflow) |
| **GitHub UI** | `workflow_dispatch` with input `pr_number` (no free-form `git_ref`) | Same; link run in PR summary |
| **Daily schedule** | `05:00 UTC` cron when an open same-repo PR had activity in the last 24h | Same guards; `max-parallel: 1` on cluster |

**Guards (all on-demand paths):**

- `github.event.pull_request.head.repo.full_name == github.repository` (no fork).
- Path filter optional for re-runs (often **always** allow re-run when explicitly requested).
- If lease acquire fails → job fails fast with “cluster busy” (**no queue** — §2).
- Set `live_smoke_on_demand=true`, PR number, SHA, actor in workflow summary.

**Fork PRs:** comment and UI triggers are **no-ops** (no secrets, no cluster access).

### 1.5 Contributor responsibility (local lab, non-CI cluster)

Before opening a PR that matches CI smoke **path includes** (§1 path filter —
excluding `vendor`/`go.mod`-only diffs), the author **must** run
`plans/harness-smoke-plan.yaml` at least once against a **dedicated lab cluster
they control** (personal or team lab kubeconfig — **not** the CI edge-112 lease
cluster). Document the run in the PR test plan (pass/fail, date, cluster name if
not sensitive).

| | CI shared lab (edge-112) | Contributor lab |
| --- | --- | --- |
| Purpose | On-demand + optional daily schedule (24h PR activity) | Pre-PR validation required by policy |
| Access | `live-smoke` environment secrets | Contributor `KUBECONFIG` + `work/backends.local.yaml` |
| Entry point | GitHub Actions | `make build` + `make image-build-local` + `make run-harness-smoke` |

This does not replace unit tests or replay-smoke; it catches integration issues
before consuming shared CI cluster time.

**Illustrative comment trigger** (on-demand re-run; must not run for fork PRs or
untrusted commenters):

```yaml
on:
  issue_comment:
    types: [created]

permissions:
  contents: read
  pull-requests: read

jobs:
  live-smoke-comment:
    if: >-
      vars.CI_LIVE_SMOKE_ENABLED == 'true' &&
      github.repository == 'openshift-storage-cert/storage-cert-harness' &&
      github.event.issue.pull_request &&
      contains(github.event.comment.body, '/run-live-smoke') &&
      contains(fromJSON('["MEMBER","OWNER","COLLABORATOR","MAINTAINER"]'),
        github.event.comment.author_association)
    environment: live-smoke
    # Follow-on job checks out the PR via pull_request_url and verifies:
    #   github.event.pull_request.head.repo.full_name == github.repository
    # before any step uses CI_CLUSTER_KUBECONFIG (use pull_request_target only
    # for checkout metadata, never to run untrusted code with secrets).
```

### GitHub Actions timeouts (current repo and limits)

Today’s workflows under `storage-cert-harness/.github/workflows/` set **no**
`timeout-minutes` on jobs. GitHub then applies the **default job timeout of 360
minutes (6 hours)** per job on GitHub-hosted runners (`ubuntu-latest` for lint,
test, build, image jobs). See [Actions limits](https://docs.github.com/en/actions/reference/limits).

| Limit                              | GitHub-hosted (`ubuntu-latest`)                                        | Lab smoke executor VM (Model C)                                    |
| ---------------------------------- | ---------------------------------------------------------------------- | ------------------------------------------------------------------ |
| **Per-job execution**              | **6 hours max** (360 min default; `timeout-minutes` cannot exceed 360) | **No GHA cap** — use `timeout-minutes` in wrapper script (240–360) |
| **Workflow run (entire workflow)** | **35 days** wall clock (includes waits/approvals)                      | N/A — single-host cron/systemd unit                                |
| **This repo today**                | No explicit timeout → each job may run up to **6 h** before cancel     | Executor not yet provisioned (**B1**)                              |

#### Implications for live smoke

- **Do not** run harness smoke on `ubuntu-latest` — no lab API route.
- On the **lab executor VM**, budget **240 minutes** per run (4 h); extend only
after **A1** timings justify it. Use `timeout` in systemd/cron wrapper around
`live-cluster-smoke.sh`.
- **End-to-end pipeline time** (developer-visible): if `live-smoke` runs **in
parallel** with `publish` after `images`, wall clock ≈ `max(live-smoke, lint→publish chain)` (~**30–60 min** for the hosted chain + **up to ~240 min** smoke ⇒ often **~2–4 h** total). If smoke **blocks** publish, wall clock is the **sum** (~**2.5–5 h** worst case).
- Add explicit `timeout-minutes` to **all** GitHub-hosted jobs in a follow-up
hygiene task so stuck jobs fail fast (for example lint 30, test 45, image-build 90).

---

## 2. CI cluster (edge-112) and lease

### Assumption: one smoke run per cluster

The CI lab cluster (**edge-112**, initial target) runs **at most one** live smoke
at a time. Manual QE or dev work on the same cluster must coordinate with the
lease object or use a different cluster. This is a **hard assumption** for CI
design — not configurable concurrency.

### edge-112 feasibility (required before CI cutover)

Before enabling PR smoke in production CI, **validate on edge-112** that the full
smoke plan (all **non-destructive** / non-HA TRs in §5) completes with the §6
overrides:

| Step | Action |
| --- | --- |
| 1 | Configure `work/backends.ci.yaml` (or lab `backends.local.yaml`) for edge-112 storage/snapshot classes and worker `node_name` / `target_node` values |
| 2 | Run `make build`, `make image-build-local` (or CI image artifact) + `run-image-tests.sh` / `live-cluster-smoke.sh` against edge-112 kubeconfig |
| 3 | Record per-TR and total suite duration in §6 tables; drop or tune any TR > 20m |
| 4 | Confirm node drain / migration TRs do not violate lab policy (non-destructive only — no `TR-VIRT-007`) |
| 5 | Sign off in this doc (date, owner) under **edge-112 sign-off** below |

**edge-112 sign-off:** *Pending — fill after successful full-plan run.*

### Dedicated vs shared

edge-112 is treated as the **dedicated CI smoke cluster** for harness PR smoke.
If other teams share it, the lease (below) is still mandatory.

### Concurrency: no queue

When the lease is held:

- A new smoke workflow **fails immediately** (exit code distinct from harness
  failure, e.g. `2`, message `cluster smoke lease held by run <id>`).
- **Do not** queue workflows in GitHub Actions waiting for the lease.
- Contributors may retry on demand after the active run finishes or use
  `[skip live-smoke]` if the lab is down.

### Lease mechanism (cluster-side)

Use a Kubernetes **Lease** in namespace `storage-cert-ci` (create namespace if
needed):

| Field | Value |
| --- | --- |
| Lease name | `storage-cert-harness-smoke` |
| Holder identity | `run_id` = `${GITHUB_RUN_ID}`, `${HARNESS_SMOKE_LEASE_RUN_ID}`, or local `local-<user>-…` |
| `leaseDurationSeconds` | Job timeout + 15m; **renew** every 5m while harness runs |
| Labels on workloads created by harness | `storage-cert-harness/ci-smoke=<run_id>` |

#### Annotations on the Lease (observability)

Publish **who is using the cluster** on the Lease object (no secrets):

| Annotation key | Example content |
| --- | --- |
| `storage-cert-harness/run-id` | `123456789` |
| `storage-cert-harness/plan` | `harness-smoke` |
| `storage-cert-harness/plan-file` | `plans/harness-smoke-plan.yaml` |
| `storage-cert-harness/git-sha` | `abc123def456` |
| `storage-cert-harness/pr-number` | `42` (if PR smoke) |
| `storage-cert-harness/workflow-url` | Link to GHA run (if any) or results URL (§2.6.8) |
| `storage-cert-harness/executor` | Lab VM hostname (`hostname -f`) or `model-c` |
| `storage-cert-harness/api-server` | API server host from `oc whoami --show-server` (not credentials) |
| `storage-cert-harness/started-at` | RFC3339 timestamp |

Optional **ConfigMap** `storage-cert-harness-smoke-status` (same namespace) mirrors
the above for `oc get cm -o yaml` without parsing Lease API fields.

#### Acquire

1. `oc apply` lease with holder = this `run_id` using server-side apply or
   patch with conflict detection.
2. On conflict: **fail fast** (no queue, no long retry loop). Log current holder
   annotations from the existing Lease for the operator.
3. **Stale lease break-glass:** if `renewTime` is older than `2 × leaseDuration`
   and workflow URL shows no active job, maintainer may delete the Lease (documented
   in ops runbook).

#### Release (success, error, cancel, kill)

Release must run whenever the CI job ends, including:

| Outcome | Lease | Cluster resources |
| --- | --- | --- |
| Harness **success** | Delete Lease if holder matches `run_id` | §8 janitor |
| Harness **error** / failed verdict | Same | Same |
| **CI cancel** / **timeout** | `trap` on `EXIT`, `INT`, `TERM` deletes Lease | Same |
| Runner **SIGKILL** / hard kill | Lease TTL expires; break-glass if stuck | Next successful run’s janitor |

Implementation in `live-cluster-smoke.sh`:

```bash
release_lease() {
  # delete Lease when holderIdentity == RUN_ID; ignore NotFound
}
trap 'release_lease; run_janitor' EXIT
```

Renewal loop: background subshell or step between TRs updates `renewTime` while
the main process runs `harness run`. If renewal stops, TTL eventually frees the
lease for break-glass.

**Never** leave the lease after a failed harness run — failed smoke must still
release the lease so the next contributor on-demand run can proceed.

### Runner placement

| Model | Cluster access | CI integration |
| --- | --- | --- |
| **A — Laptop** | Manual VPN + kubeconfig | Pre-PR policy (§1.5) — [model A](smoke-execution-model-a.md) |
| **B — GHA self-hosted runner** | Leg 1 + Leg 2 on runner host | **Not available** — [model B](smoke-execution-model-b.md) |
| **C — Lab smoke executor VM** | Leg 1 + Leg 2 on dedicated VM | **Adopted** — [model C](smoke-execution-model-c.md) |
| GHA `ubuntu-latest` | No lab API | Lint, test, build, publish only |

GitHub **hosted** `ubuntu-latest` cannot reach a private OpenShift API; do not
depend on it for live smoke.

### 2.6 Lab smoke executor (Model C)

Full specification: [smoke-execution-model-c.md](smoke-execution-model-c.md)
(VM install, cron, results, notify, web UI, validation checklist).

---

## 3. Storing cluster and harness configuration in CI

Never commit kubeconfig, backend passwords, or private `thresholds.json` exports.

**Model C:** Lab kubeconfig, backends, and optional threshold overlays live on
the **executor VM** ([model C](smoke-execution-model-c.md)). GitHub **must not**
store lab kubeconfig once Model C is active.

### Protected variables / secrets

| Name                               | Format                         | Used for                                                                                      |
| ---------------------------------- | ------------------------------ | --------------------------------------------------------------------------------------------- |
| `CI_CLUSTER_KUBECONFIG`            | Base64-encoded kubeconfig file | **Legacy Model B only** — use VM files for Model C                                            |
| `HARNESS_THRESHOLDS_JSON`          | Raw JSON string                | Write to `work/thresholds.json` (or path mounted in container)                                |
| `HARNESS_BACKENDS_YAML`            | Raw YAML                       | CI cluster backend (`storage_class`, `snapshot_class`, array creds) → `work/backends.ci.yaml` |
| `HARNESS_CATALOG_JSON`             | Optional if not in repo        | Only if catalog must stay private; default use git `catalog.json`                             |
| `KUBECTL_INSECURE_SKIP_TLS_VERIFY` | `true` / `false`               | Lab clusters with custom CA                                                                   |
| `REGISTRY_*` / Quay robot          | Existing publish secrets       | Pull image if not using artifact tarball                                                      |

GitHub: **Environment** `live-smoke` with required reviewers optional; secrets
only exposed to the live-smoke job on **upstream** `main` (see **Forks and
untrusted repositories**). Cluster kubeconfig and backends **must not** live in
repo-level secrets that are inherited by every workflow — use environment secrets
so fork PR workflows and unrelated jobs never receive them.

### Kubeconfig handling

```text
set +x
kube="$(mktemp)"
chmod 0600 "$kube"
echo "$CI_CLUSTER_KUBECONFIG" | base64 -d > "$kube"
export KUBECONFIG="$kube"
trap 'rm -f "$kube"' EXIT
```

Do not upload kubeconfig or backends YAML as workflow artifacts. Logs must not
print `oc whoami` tokens or backend secrets (`set +x` around secret material).

### Backend naming

`harness-smoke-plan.yaml` sets `backend: oc-edge-112`. The CI backends file **must**
define a backend with that exact name (or override `BACKEND=` in CI) matching the
CI cluster StorageClass / VolumeSnapshotClass.

---

## 4. Cluster validation before run

Validation is **layered**: cheap checks in bash, then harness `preflight`, then
`run`.

### Layer A — CI script (extend `run-image-tests.sh`)

Already implemented for local smoke:

- `podman` available
- Kubeconfig readable; `oc whoami --show-server`
- All nodes `Ready`
- Plan backend exists in backends YAML with `storage_class` and `snapshot_class`
- StorageClass and VolumeSnapshotClass exist
- Plan-referenced `node_name` / `target_node` exist (if any)

**Add for CI:**

- Acquire cluster lease (section 2)
- Minimum worker count (e.g. ≥ 3 workers) — smoke does not need HA but needs scheduling headroom
- `oc get clusterversion` — OCP ≥ 4.20 (align with cluster-setup-guide)
- CDI / KubeVirt CSV present if plan includes virt TRs
- CSI driver pods Ready for backend provisioner
- Optional: disk pressure / memory pressure node conditions not `True`

### Layer B — harness preflight

Run inside the same container invocation as `run`, or as a preceding step:

```bash
harness preflight \
  --catalog /work/catalog.json \
  --plan /plans/harness-smoke-plan.yaml \
  --backends /work/backends.ci.yaml \
  --backend "${BACKEND}"
```

Exit non-zero → fail job before provisioning workloads.

### Layer C — post-run gate

Fail the job if:

- `report.json` `health` is not `healthy`, or
- Any execution has `outcome` of `fail` or `error`, or
- Any verdict has `outcome` `fail` (treat `skip` as acceptable for smoke-scale params without SLA bars)

Upload **sanitized** artifacts: `report.md`, junit if present, redacted `report.json`
(strip backend connection strings if ever embedded).

When Layer C fails, the GitHub Actions job exits non-zero so the PR check is
**failed** and branch protection **blocks merge** until a subsequent run passes.

---

## 5. Smoke plan scope (full catalog, no HA, 20-minute cap per TR)

### Inclusion rule

| Rule                      | Detail                                                                                                                                                                                       |
| ------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Include**               | Every `test_requirements[].id` in `catalog.json` that has automation                                                                                                                         |
| **Always exclude**        | `TR-VIRT-007` (VM HA restart after node failure) — requires node remediation / fencing and is out of scope for CI smoke                                                                      |
| **Conditionally exclude** | Any other TR whose **measured** wall time exceeds **20 minutes** at the smoke overrides in section 6, after tuning — document the TR, duration, and reason in this file and in plan comments |

Today’s catalog has **11** automated TRs → **10** belong in CI smoke when all
meet the 20-minute bar.

### CI smoke selection (target)

| TR          | Title (short)                  | Tool family                | In CI smoke                          |
| ----------- | ------------------------------ | -------------------------- | ------------------------------------ |
| TR-STOR-002 | FIO storage IO (smoke profile) | virtbench fio              | Yes ( **temporarily out** of plan — [ECOPROJECT-5566](https://redhat.atlassian.net/browse/ECOPROJECT-5566)) |
| TR-STOR-006 | PVC provisioning density       | kube-burner-ocp            | Yes                                  |
| TR-VIRT-001 | Cold VM boot storm             | virtbench                  | Yes                                  |
| TR-VIRT-004 | Parallel live migration        | kube-burner-ocp            | Yes (single variant; see section 6)  |
| TR-VIRT-007 | VM HA after node failure       | virtbench failure-recovery | **No**                               |
| TR-VIRT-008 | Node drain                     | virtbench drain            | **No** (destructive / cordon — excluded from `harness-smoke-plan.yaml`) |
| TR-VIRT-010 | VM snapshot at scale           | kube-burner                | Yes                                  |
| TR-VIRT-013 | Per-node scale ceiling         | virtbench single-node      | Yes                                  |
| TR-VIRT-018 | Clone at scale                 | virtbench clone            | Yes                                  |
| TR-VIRT-019 | Parallel VM lifecycle          | kube-burner-ocp            | Yes (must stay ≤ 20m; tune timeouts) |
| TR-VIRT-027 | Snapshot depth per volume      | kube-burner                | Yes                                  |

**Plan source of truth:** expand `plans/harness-smoke-plan.yaml` to list the ten
`select.ids` rows above (same overrides as section 6). Until that lands, treat
`level3-internal-minimal.yaml` minus `TR-VIRT-007` and minus `variants` as the
reference — CI must not ship a four-TR subset.

`concurrency: 1` keeps cluster load predictable for CI.

### Per-TR duration policy

- Before merging smoke overrides, benchmark each TR on the CI cluster; if
`ended_at - started_at` in `report.json` is **> 1200s**, tighten overrides or
exclude that TR from `select.ids` until it fits.
- CI may add a post-run guard: fail the **pipeline planning** step if the plan
references a TR on the documented exclusion list.
- Re-evaluate exclusions when catalog or smoke overrides change.

---

## 6. Smoke plan configuration, overrides, and timing

### 6.1 Plan file and overrides

**Source of truth (today):** `plans/harness-smoke-plan.yaml` — committed in the repo,
not generated in CI. **Target (§6.5):** canonical copies under `ci/config/smoke/`
staged into `work/` before each local or CI smoke run.

| Plan field | CI value | Notes |
| --- | --- | --- |
| `name` | `harness-smoke` | |
| `backend` | `oc-edge-112` (or site-specific name aligned with plan) | Must match `HARNESS_BACKENDS_YAML` |
| `concurrency` | `1` | One TR at a time |
| `select.ids` | All non-HA TRs from §5 | Exclude `TR-VIRT-007` |
| `overrides` | Per-TR table below | **Minimal** VMs, iterations, timeouts |

CI may mount an optional **patch file** `plans/harness-smoke-plan.ci.yaml` later
for edge-112-only node names; prefer keeping one plan with backends driving
storage class.

**Local / lab parity:** `make run-harness-smoke` and `run-image-tests.sh` use the
same plan and overrides as CI.

### 6.2 Minimal settings per test (override contract)

Derived from `level3-internal-minimal.yaml` with edge-112 worker names. Change
only after re-benchmark on edge-112 and update §6.3 timing table.

| TR          | Parameter            | Smoke value                                           | Intent                                                     |
| ----------- | -------------------- | ----------------------------------------------------- | ---------------------------------------------------------- |
| TR-STOR-002 | `num_vms`            | `3`                                                   | Small FIO fleet                                            |
| TR-STOR-002 | `concurrency`        | `3`                                                   | Match VM count                                             |
| TR-STOR-002 | `profile`            | `load-80` or smoke-equivalent without long array soak | Prefer shortest profile that still exercises virtbench fio |
| TR-STOR-002 | `fio_runtime`        | `30`                                                  | Seconds per VM (not certification duration)                |
| TR-STOR-006 | `workload`           | `pvc-density`                                         |                                                            |
| TR-STOR-006 | `iterations`         | `3`                                                   | vs 1000-scale cert                                         |
| TR-STOR-006 | `claim_size`         | `256Mi`                                               |                                                            |
| TR-STOR-006 | `qps` / `burst`      | `5` / `10`                                            | API throttle                                               |
| TR-VIRT-001 | `num_vms`            | `3`                                                   | Boot storm                                                 |
| TR-VIRT-004 | `workload`           | `virt-migration`                                      |                                                            |
| TR-VIRT-004 | `iterations`         | `1`                                                   |                                                            |
| TR-VIRT-004 | `concurrency`        | `1`                                                   | Single variant only in CI (no `variants:` block)           |
| TR-VIRT-004 | `iteration_vms`      | `1`                                                   | One VM migration pass                                      |
| TR-VIRT-004 | `migration_qps`      | `1`                                                   |                                                            |
| TR-VIRT-008 | `start` / `end`      | `1` / `3`                                             | Three drain iterations                                     |
| TR-VIRT-008 | `target_node`        | CI worker node name                                   | Must exist in cluster                                      |
| TR-VIRT-008 | `namespace_prefix`   | `harness-smoke-drain`                                 | Cleanup prefix                                             |
| TR-VIRT-010 | `replicas`           | `3`                                                   |                                                            |
| TR-VIRT-010 | `snapshot_count`     | `3`                                                   |                                                            |
| TR-VIRT-010 | `namespace`          | `harness-smoke-snap`                                  |                                                            |
| TR-VIRT-010 | `max_wait`           | `15m`                                                 | Must be ≤ 20m TR budget                                    |
| TR-VIRT-013 | `attempt_vms`        | `3`                                                   | Ceiling probe, not cert scale                              |
| TR-VIRT-013 | `num_disks`          | `1`                                                   |                                                            |
| TR-VIRT-013 | `node_name`          | CI worker node name                                   |                                                            |
| TR-VIRT-018 | `iteration_clones`   | `3`                                                   |                                                            |
| TR-VIRT-018 | `use_snapshot`       | `true`                                                |                                                            |
| TR-VIRT-019 | `workload`           | `virt-parallel`                                       |                                                            |
| TR-VIRT-019 | `initial_vms`        | `2`–`3`                                               | Use `2` if 3 VMs exceed 20m on CI cluster                  |
| TR-VIRT-019 | `increment`          | `0`                                                   |                                                            |
| TR-VIRT-019 | `data_volume_count`  | `1`                                                   |                                                            |
| TR-VIRT-019 | `max_iterations`     | `1`                                                   |                                                            |
| TR-VIRT-019 | `skip_migration_job` | `true`                                                | Shorter path                                               |
| TR-VIRT-019 | `timeout`            | `20m`                                                 | Hard cap aligned with per-TR policy                        |
| TR-VIRT-019 | `qps` / `burst`      | `5` / `10`                                            |                                                            |
| TR-VIRT-027 | `snapshot_count`     | `3`                                                   | vs catalog default 250                                     |
| TR-VIRT-027 | `replicas`           | `1`                                                   |                                                            |

Local and CI smoke runs use **`thresholds-smoke.json`** (smoke SLA bars). Certification
`thresholds.json` remains for full cert runs only.

### 6.3 Execution time measurement (edge-112 and CI)

**Requirement:** measure duration for **each TR** and the **full suite** on
edge-112 before treating overrides as final.

| Source | Metric |
| --- | --- |
| `report.json` → `executions[]` | Per-TR `started_at` / `ended_at` (wall time per TR) |
| `report.json` → `composition.parts[]` | Full plan wall time |
| CI workflow | Job `duration`; write `smoke_suite_duration_s` and per-TR seconds to workflow **summary** and optional artifact `smoke-timing.json` |
| Harness log | Copy `run_id` and timings into GHA step summary for PR comment bot (optional) |

**Suite budget:** target p95 full suite on edge-112; job `timeout-minutes: 240`
until measured. Any TR **> 1200s** at smoke overrides → tune overrides or exclude
from `select.ids` (document in table below).

### 6.4 Documented duration exclusions

| TR          | Status   | edge-112 duration (s) | Notes                              |
| ----------- | -------- | --------------------- | ---------------------------------- |
| TR-VIRT-007 | Excluded | —                     | HA / node failure — never in smoke |
| *(others)*  | TBD      | TBD                   | Fill after edge-112 benchmark      |

**Full suite (edge-112):** *TBD* seconds — update after sign-off §2.

### 6.5 Local smoke configuration bundle (`ci/config/smoke/`)

Smoke needs three **versioned, non-secret** inputs plus cluster-specific secrets
that stay out of git. Paths are centralized under `ci/config/smoke/` alongside
`ci/config/images.env`, `golangci.yml`, etc. The smoke **plan** remains at
`plans/harness-smoke-plan.yaml`.

| File in bundle | Purpose | Secret? |
| --- | --- | --- |
| `harness-smoke-plan.yaml` | TR selection + smoke overrides (`backend`, `select.ids`, `overrides`) | No |
| `backends.yaml` | **Template** backends file: defines the plan’s `backend:` name (e.g. `oc-edge-112`), `storage_class`, `snapshot_class`, vendor params **without** credentials | No (no `inline` secrets) |
| `thresholds-smoke.json` | SLA bars for smoke grading | No |

**Migration:** Canonical smoke thresholds live only in
`ci/config/smoke/thresholds-smoke.json` (repo-root `thresholds-smoke.json` removed).
The plan remains `plans/harness-smoke-plan.yaml`. Full certification bars stay in
repo-root **`thresholds.json`** (not staged for smoke).

**Not in the bundle (remain under `work/`, gitignored):**

| Path | Purpose |
| --- | --- |
| `work/kubeconfig` | Cluster API access |
| `work/backends.local.yaml` | Operator copy of `ci/config/smoke/backends.yaml` with real `secrets:` / site-specific `storage_class` (or generated by staging — see §6.6) |
| `work/local-image-ref` | Built harness image tag |
| `work/harness-smoke-work/`, `work/harness-smoke-report/` | Run artifacts |

Optional later: `ci/config/smoke/README` listing env vars (`BACKEND`, `KUBECONFIG`,
`HARNESS_SMOKE_KEEP_WORK`).

### 6.6 Automatic staging to `work/` (`prepare-smoke-env.sh`)

**Status:** Implemented as `ci/scripts/run-harness/prepare-smoke-env.sh` (`make prepare-smoke-env`, `make smoke-ci`).

**Goal:** One helper invoked before smoke (local `make smoke-ci`; eventually
`live-cluster-smoke.sh`) so contributors do not manually copy thresholds or
backends.

**Script:** `ci/scripts/run-harness/prepare-smoke-env.sh`

1. Source `ci-utils.sh`; resolve `REPO_ROOT`.
2. Require `ci/config/smoke/harness-smoke-plan.yaml`,
   `ci/config/smoke/thresholds-smoke.json`, and `ci/config/smoke/backends.yaml`.
3. Copy in **fixed order** (deterministic, logged):

   | Source (bundle) | Destination under `work/` |
   | --- | --- |
   | `ci/config/smoke/thresholds-smoke.json` | `work/harness-run-thresholds.json` |
   | `ci/config/smoke/harness-smoke-plan.yaml` | `work/smoke/harness-smoke-plan.yaml` |
   | `ci/config/smoke/backends.yaml` | `work/backends.local.yaml` **only if** missing or `HARNESS_SMOKE_REFRESH_BACKENDS=true` |

4. Never overwrite `work/kubeconfig` or `work/local-image-ref`.
5. Print which paths were staged (for log forensics).

**`run-image-tests.sh`:**

- Expects staged `work/harness-run-thresholds.json`, `work/backends.local.yaml`, and `work/catalog.json` from `make prepare-smoke-env` (or `make smoke-ci`).
- Uses repo-root `plans/harness-smoke-plan.yaml` for the harness `--plan` path.

**`make run-harness-smoke`:** unchanged entry; inherits staging via `run-image-tests.sh`.

**Personal lab / different backend:**

- Edit `ci/config/smoke/backends.yaml` **locally** (uncommitted) or maintain a
  gitignored `work/backends.local.yaml` and set `HARNESS_SMOKE_REFRESH_BACKENDS=false`.
- Set plan `backend:` and matching `backends[].name`; optional `BACKEND=` env override.
- Optional `HARNESS_SMOKE_PLAN=/path/to/plan.yaml` to experiment without editing the bundle.

**CI:** `live-cluster-smoke.sh` materializes secrets from the environment, then
calls the same staging helper so CI and laptop use identical bundle layout; only
kubeconfig and secret-backed backend fields differ.

**Status:** Staging script and `ci/config/smoke/` directory are **not implemented**
yet; `run-image-tests.sh` still reads `plans/harness-smoke-plan.yaml` and copies
repo-root `thresholds-smoke.json` directly (see §13).

---

## 7. Execution time and scope tuning

### Per-TR budget

Each included TR must finish within **20 minutes** at smoke overrides (section 6).
The only standing exclusion is **HA** (`TR-VIRT-007`). Any other TR that still
exceeds 20 minutes after tuning is removed from `select.ids` and listed in
section 6’s exclusion table — not replaced by a smaller default four-TR plan.

### Baseline (partial reference)

An early four-TR run (`work/harness-smoke-report/report.json`, 2026-10-08) is
**not** representative of full-catalog smoke; use it only for order-of-magnitude
on TR-001, 006, 010, 019. Re-benchmark all ten TRs after expanding the plan.

| TR (partial run) | Approx. duration | Notes                      |
| ---------------- | ---------------- | -------------------------- |
| TR-VIRT-019      | ~5.5 min         | Tune to stay under 20m cap |
| TR-VIRT-001      | ~4 min           |                            |
| TR-STOR-006      | ~1.5 min         |                            |
| TR-VIRT-010      | ~4.5 min         |                            |

### CI job timeout

Full-catalog smoke is **sequential** (`concurrency: 1`). Worst case:

`10 TR × 20 min + 30 min` (preflight, image load, cleanup) ≈ **3.5 hours**.

Set the live-smoke job timeout to **240 minutes** initially; reduce after stable
p95 timings are recorded. If total wall clock is routinely below 120 minutes,
lower the timeout to p95 + 20%.

### Ongoing tuning

1. Prefer **tighter overrides** before excluding a TR.
2. Never drop tool coverage silently: an excluded TR must appear in section 6’s
  table with measured duration and owner sign-off.
3. Track p50/p95 per TR from `report.json` `executions[]` and overall job
  duration in CI metrics.

---

## 8. Cluster and resource cleanup

### Harness teardown

Rely on harness **teardown** stages for each TR where implemented. CI must not
assume teardown always succeeds.

### CI janitor (always run in `trap`)

After every run, with `KUBECONFIG` still valid and lease still held:

1. Delete namespaces with label `storage-cert-harness/ci-smoke=<run_id>`.
2. Delete leftover namespaces matching allowlist prefix:
  `harness-smoke-*`, `storage-cert-*` created during run (from plan params).
3. Delete cluster-scoped leftovers labeled `storage-cert-harness/ci-smoke=<run_id>`.
4. Optional: `oc delete vm -n ...` sweep for known smoke name patterns if namespaces stuck in `Terminating` > 10m (break-glass playbook).

### Stuck jobs

On GitHub Actions **timeout** or **cancel**:

- `trap` still runs if the runner receives SIGTERM (use `bash` trap; avoid `exec` without trap).
- Lease TTL expiry allows the next run after slack period even if trap failed once — monitor for duplicate leases.

### Local developer parity

`make run-harness-smoke` should call the same cleanup helper with
`run_id=local-$(whoami)-$(date +%s)` so manual runs do not fight CI.

---

## 9. Proposed CI implementation sketch

### New script

`ci/scripts/run-harness/live-cluster-smoke.sh`:

1. Source `ci-utils.sh`; `ci_log_init live-cluster-smoke`
2. Materialize secrets to `work/` (thresholds, backends, kubeconfig)
3. `acquire_smoke_lease` / `release_smoke_lease` + cleanup `trap`
4. Load image from `dist/harness-image.tar` or `podman load` / pull Quay tag for SHA
5. Invoke same `podman run ... harness run` as `run-image-tests.sh`
6. Evaluate report; exit code ≠ 0 on failure

`run-image-tests.sh` remains the developer entry; live-cluster-smoke wraps CI
secrets and lease.

### GitHub Actions

Implementation: [`.github/workflows/ci-live-smoke.yml`](../../.github/workflows/ci-live-smoke.yml).
Under **Model C**, GHA stays on **`ubuntu-latest` only**:

| GHA responsibility | Lab VM responsibility (§2.6) |
| --- | --- |
| Resolve PR metadata via Pulls API | `git fetch` + checkout requested SHA |
| Bundle PR source as short-lived artifact | Pull image or build locally |
| Enqueue smoke request (file, issue, or artifact — **B1**) | Run `live-cluster-smoke.sh` / `make smoke-ci` |
| (Future) publish check from executor callback | Store results, notify, serve web UI |

**Remove or gate** self-hosted `live-smoke` jobs in the workflow until org policy
changes — they cannot reach the cluster. Triggers remain **`workflow_dispatch`**,
**`issue_comment`** `/run-live-smoke`, and **daily schedule** (24h PR activity) to
**schedule lab work**, not to execute harness on GitHub runners. **No**
`pull_request` or `push` triggers for cluster steps.

Branch protection **required check** for `live-smoke` depends on **report-back**
(§2.6.6) or stays off (**B4**) while merge gate uses contributor **Model A**
evidence (§1.5).

---

## 10. Operations

| Item                 | Owner (TBD)        | Notes                                                |
| -------------------- | ------------------ | ---------------------------------------------------- |
| CI cluster lifecycle | Platform / QE      | edge-112; backend aligned with smoke plan `backend:` field |
| Kubeconfig rotation  | Cluster admin      | Update `/etc/storage-cert-smoke/kubeconfig` on executor VM |
| Lease stuck          | On-call harness CI | Delete lease object with audit log                   |
| Executor VM / VPN    | CI admin           | §2.6; TLV2 doc; Leg 1 + Leg 2 health checks          |
| Results / web / NFS  | CI admin           | §2.6.5–§2.6.8 retention and scrub rules              |
| Failure escalation   | Team channel       | Results URL + `run_id` + failing TR from `status.json` |

Log retention: keep smoke logs 14–30 days; no full cluster dumps.

---

## 11. Remaining open items

Detailed checklist with IDs (A1–D3): [plan-based-test-suite-runbook.md — Action items](plan-based-test-suite-runbook.md#action-items--finish-local-smoke-and-cicd).

**Single tracker:** edit open work only in the runbook (IDs **A1–D3**, Jira under [ECOPROJECT-5406](https://redhat.atlassian.net/browse/ECOPROJECT-5406)).
This section mirrors that list for design readers.

| ID | Jira | Summary | Status |
| --- | --- | --- | --- |
| A1 | [5582](https://redhat.atlassian.net/browse/ECOPROJECT-5582) | edge-112 sign-off — 8 TRs; record §6.3/§6.4 timings | Open |
| A2 | [5583](https://redhat.atlassian.net/browse/ECOPROJECT-5583) | Re-enable TR-STOR-002 in smoke plan | Blocked — [5566](https://redhat.atlassian.net/browse/ECOPROJECT-5566) |
| A3 | [5580](https://redhat.atlassian.net/browse/ECOPROJECT-5580) | Runbook + smoke docs aligned with `prepare-smoke-env` | Mostly done — [smoke-testing.md](../smoke-testing.md) |
| B1 | [5612](https://redhat.atlassian.net/browse/ECOPROJECT-5612) | Model C lab executor VM — [smoke-execution-model-c.md](smoke-execution-model-c.md) | Open |
| B2 | [5585](https://redhat.atlassian.net/browse/ECOPROJECT-5585) | `live-cluster-smoke.sh` — podman run, secrets, janitor, timing artifact | Open |
| B3 | [5535](https://redhat.atlassian.net/browse/ECOPROJECT-5535) | GHA `timeout-minutes` on resolve jobs; **240m** on VM wrapper | Partial — GHA done; VM in **B1** |
| B4 | [5586](https://redhat.atlassian.net/browse/ECOPROJECT-5586) | Branch protection — required `live-smoke` on PRs to `main` | Open |
| C1 | [5536](https://redhat.atlassian.net/browse/ECOPROJECT-5536) | `[skip live-smoke]` — verify **Skipped** conclusion + summary | Open |
| C2 | [5587](https://redhat.atlassian.net/browse/ECOPROJECT-5587) | First green **main** regression on lab VM timer | Blocked on **B1**–**B2** |
| C3 | [5588](https://redhat.atlassian.net/browse/ECOPROJECT-5588) | On-demand PR smoke via GHA → VM queue | Blocked on **B1** |
| C4 | [5589](https://redhat.atlassian.net/browse/ECOPROJECT-5589) | Fork PR guard — re-verify when CI enabled | Verify at enablement |
| D1 | [5590](https://redhat.atlassian.net/browse/ECOPROJECT-5590) | Consolidate duplicate smoke scripts / doc naming | Open |
| D2 | [5591](https://redhat.atlassian.net/browse/ECOPROJECT-5591) | Lease janitor + CI cancel (core lease: [5569](https://redhat.atlassian.net/browse/ECOPROJECT-5569) closed) | Partial |
| D3 | [5592](https://redhat.atlassian.net/browse/ECOPROJECT-5592) | Update [5406](https://redhat.atlassian.net/browse/ECOPROJECT-5406) story text for PR smoke model | Open |

### Delivered or out of scope (not open)

| Topic | Resolution |
| --- | --- |
| **`ci/config/smoke/` + `prepare-smoke-env.sh`** (§6.5–6.6) | Done — `make prepare-smoke-env`, `make smoke-ci` |
| **Daily scheduled smoke** (24h open-PR gate) | GHA resolver done; **cluster** run on VM (**B1**) |
| **On-demand triggers** (`/run-live-smoke`, `workflow_dispatch`) | GHA triggers done; **cluster** run on VM (**B1**–**B2**) |
| **Contributor execution runbook** | Done — [plan-based-test-suite-runbook.md](plan-based-test-suite-runbook.md) |
| **[ECOPROJECT-5462](https://redhat.atlassian.net/browse/ECOPROJECT-5462)** hosted runners | **Out of scope** — lab self-hosted runner not allowed; **Model C** (§2.6) |

**Closed team decisions (for reference):** **no** automatic smoke on PR open or
push to `main`; GHA schedule/comment **enqueue** lab smoke when enabled; **failed
PR smoke blocks merge** when a **reported** check ran (otherwise **Model A** +
review); no lease queue; `[skip live-smoke]` only when lab unavailable.

---

## 12. Verification checklist (for story closure)

Legend: **[x]** satisfied in repo today; **[~]** partial or pending CI enablement; **[ ]** not done.

- [ ] edge-112: full non-HA smoke plan passes with §6 overrides; timings recorded in §6.3/§6.4 (**A1**).
- [x] No smoke on PR **open** or **push** to `main`; `/run-live-smoke`, `workflow_dispatch`, or daily schedule (24h PR activity) start runs when enabled.
- [~] `/run-live-smoke` comment or `workflow_dispatch` runs smoke on same-repo PR at current head (**C3** — workflow present; **B2** execution).
- [x] Daily schedule skips when no open same-repo PR was updated in the last 24 hours (**C2** — resolver job).
- [ ] Failed PR smoke blocks merge; passing on-demand re-run unblocks (**B4** + live runs).
- [~] `[skip live-smoke]` in PR title/body skips smoke scheduling only (lab down); not used to bypass failures (**C1** — verify check conclusion).
- [~] Second concurrent smoke **fails fast** (no queue); Lease shows holder annotations (**D2** — local + CI acquire; janitor with **B2**).
- [ ] Harness **error** or CI **cancel** still **releases** lease and runs janitor (**D2** + **B2**).
- [~] All catalog TRs except `TR-VIRT-007` / `TR-VIRT-008` execute on edge-112 (8 TRs today; **A2** adds STOR-002).
- [ ] Per-TR and suite durations in workflow summary / `smoke-timing.json` (**B2**).
- [ ] Secrets never appear in artifacts (verify on first CI run).
- [~] **Fork** PR: no smoke, no kubeconfig (**C4** — workflow guards; verify when enabled).
- [x] Push to `main` does **not** auto-trigger smoke (default design).
- [x] Local `run-harness-smoke` / `smoke-ci` parity with CI plan/overrides and staged thresholds.
- [x] `ci/config/smoke/` staged to `work/` via `prepare-smoke-env` (§6.6).
- [ ] TR-STOR-002 included when ECOPROJECT-5566 is resolved (**A2**).

---

## 13. Implementation progress

Open work is tracked in the runbook (**A1–D3**); this section is a snapshot only.

### Done (merged or on branch)

| Area | Delivered |
| --- | --- |
| **Smoke plan** | Expanded `harness-smoke-plan.yaml`: 8 TRs (excludes TR-VIRT-007/008); TR-STOR-002 **commented out** until FIO gather reliable |
| **Smoke thresholds** | `thresholds-smoke.json`; staged → `work/harness-run-thresholds.json` |
| **TR-STOR-002 FIO runner** | `engine.go`: deploy → `waitForFIOVMs` → `gather-results` with `fio_collect_*` (no harness guest SSH) |
| **Local smoke hygiene** | `acquire_smoke_run_lock`; log archive; `clean_harness_smoke_artifacts`; `HARNESS_SMOKE_KEEP_WORK` |
| **Build vs smoke lock** | `build.sh` no longer takes image lock; image lock separate from smoke (`docs/ci.md`) |
| **Cert thresholds** | `when` scoping on selected VIRT bars in `thresholds.json` (full cert vs smoke bundle split) |
| **Smoke config bundle** | `ci/config/smoke/` + `prepare-smoke-env.sh`, `clean-smoke-env.sh`, `make smoke-ci` |
| **Cluster lease (core)** | `smoke-cluster-lease.sh` in `run-image-tests.sh` and start of `live-cluster-smoke.sh` |
| **CI workflow** | `ci-live-smoke.yml`: schedule, comment, dispatch, fork guards, `timeout-minutes: 240`; gated on `CI_LIVE_SMOKE_ENABLED` |
| **CI entry (incomplete)** | `live-cluster-smoke.sh` acquires lease then exits **5** until **B2** |
| **Docs / process** | Skip live-smoke token in `docs/ci.md` / contributing; [smoke-testing.md](../smoke-testing.md); [plan-based-test-suite-runbook.md](plan-based-test-suite-runbook.md) |

### In progress / blocked

| Area | State |
| --- | --- |
| **Full catalog smoke (9 TRs with STOR-002)** | Blocked on TR-STOR-002 (**A2** / ECOPROJECT-5566); TR-VIRT-008 excluded by policy |
| **edge-112 sign-off + CI** | **A1** local; **B1**–**B2** for lab executor VM (not GHA cluster jobs) |
| **Per-TR timing table** | §6.3/§6.4 still TBD until **A1** |

### Remaining (see runbook § Action items)

**B2** harness execution in CI, **B1** secrets/enablement, **B4** branch protection, **C1** skip verification, **C2**–**C4** first/verified live runs, **D1** script cleanup, **D2** janitor/cancel, **D3** Jira story update.

---

## References

- Contributor smoke guide: [smoke-testing.md](../smoke-testing.md)
- Runbook: [plan-based-test-suite-runbook.md](plan-based-test-suite-runbook.md)
- CI overview: `storage-cert-harness/docs/ci.md` (Dedicated CI cluster section)
- Scripts: `ci/scripts/run-harness/` (`run-image-tests.sh`, `run-quay-image-tests.sh`,
  `live-cluster-smoke.sh`, `smoke-cluster-lease.sh`, `replay-smoke.sh`,
  `prepare-smoke-env.sh`, `clean-smoke-env.sh`)
- Smoke bundle: `ci/config/smoke/` — §6.5; plan: `plans/harness-smoke-plan.yaml`
- Plans: `storage-cert-harness/plans/level3-internal-minimal.yaml` (override reference)
- FIO gather issue: [ECOPROJECT-5566](https://redhat.atlassian.net/browse/ECOPROJECT-5566)
- Catalog: `storage-cert-harness/catalog.json`
- Execution models: [smoke-execution-models.md](smoke-execution-models.md), [model A](smoke-execution-model-a.md), [model B](smoke-execution-model-b.md), [model C](smoke-execution-model-c.md)
- TLV2 lab VM networking (Leg 1/Leg 2): [storage-cert-harness-smoke-runner-tlv2.md](../../../Docs/Github/storage-cert-harness-smoke-runner-tlv2.md) — provisioning only; **skip** GitHub runner registration
- Jira: [ECOPROJECT-5406](https://redhat.atlassian.net/browse/ECOPROJECT-5406), [ECOPROJECT-5612](https://redhat.atlassian.net/browse/ECOPROJECT-5612) (Model C), [ECOPROJECT-5462](https://redhat.atlassian.net/browse/ECOPROJECT-5462) (superseded)
