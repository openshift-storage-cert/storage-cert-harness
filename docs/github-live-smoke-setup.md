# GitHub setup — scheduled and on-demand live smoke

Operator guide for [CI Live Smoke](../.github/workflows/ci-live-smoke.yml) on
`openshift-storage-cert/storage-cert-harness`. Harness live smoke is **GitHub
Actions** plus **local scripts** — see [smoke-testing.md](smoke-testing.md) for
day-to-day contributor runs.

Design: [storage-cert-harness-ci-smoke-test.md](design/storage-cert-harness-ci-smoke-test.md).

**Shared lab:** **edge-112** is reachable only on the **Red Hat lab VPN** (not from
the public internet). API: `https://api.ocp-edge112-0.lab.eng.tlv2.redhat.com:6443`
(TLV2). VM provisioning, VPN tickets, and lab contacts:
[Docs/Github/storage-cert-harness-smoke-runner-tlv2.md](../../Docs/Github/storage-cert-harness-smoke-runner-tlv2.md)
(monorepo).

---

## Two execution models

Use the right path for your goal — they share the same **smoke plan** and
`ci/config/smoke/` bundle, but **network and credentials** differ.

| | **Model A — local / laptop** | **Model B — GitHub CI/CD** |
| --- | --- | --- |
| **Who** | Contributors, **A1** edge-112 sign-off | Maintainers; PR merge gate when enabled |
| **Trigger** | You run commands | `/run-live-smoke`, `workflow_dispatch`, daily schedule |
| **Host** | Your workstation | Org **self-hosted** runner (`storage-cert-smoke` label) |
| **VPN** | You connect **manually** before each session | **Always-on** VPN (or lab VLAN) on the runner VM |
| **Kubeconfig** | Your file (`export KUBECONFIG=…`) | `CI_CLUSTER_KUBECONFIG` in GitHub environment `live-smoke` |
| **Backends** | `work/backends.local.yaml` (prepare may preserve yours) | `HARNESS_BACKENDS_YAML` secret → materialized at job start (**B2**) |
| **Workflow** | [smoke-testing.md](smoke-testing.md): `make smoke-ci` | `ci-live-smoke.yml` → `live-cluster-smoke.sh` on runner |
| **Cluster lease** | Local lock + lease scripts | Same lease objects; one smoke at a time on edge-112 |
| **GitHub hosted `ubuntu-latest`** | N/A | **Only** PR resolve + source bundle — **never** talks to the cluster |

**Model A** does not need a self-hosted runner. **Model B** does not replace Model A
for development; it automates smoke for open PRs once **B1**–**B2** are complete.

### Model A — local / laptop (summary)

1. Connect **Red Hat lab VPN**.
2. Clone repo; place kubeconfig (never commit).
3. `make build && make image-build-local && make prepare-smoke-env`
4. `BACKEND=oc-edge-112 make smoke-ci` (or `make run-harness-smoke`).

Details: [smoke-testing.md](smoke-testing.md). No GitHub secrets required on your
machine.

### Model B — GitHub CI/CD (summary)

Follow **[Model B setup — Steps 1–7](#model-b-setup--self-hosted-runner-steps-17)** below.
Default **CI** (lint/build) stays on GitHub-hosted runners; only live-smoke jobs use
the VPN runner.

**B2** must finish harness execution in `live-cluster-smoke.sh` (today exit **5**
after lease acquire is possible).

---

## Model B setup — self-hosted runner (Steps 1–7)

Model B jobs that touch **edge-112** run **only** on a self-hosted runner on the
Red Hat network. **`ubuntu-latest`** runs resolve/bundle jobs only (no cluster).

```text
GitHub Actions (public)
  resolve / bundle jobs  →  ubuntu-latest  (no VPN)
  live-smoke-* jobs      →  self-hosted runner VM
       Leg 1 (public internet)  →  github.com; registry.access.redhat.com; quay.io
                                  (agent + public UBI/harness pulls, no registry token)
       Leg 2 (lab VPN/VLAN)     →  edge-112 API (smoke)
```

### Step 1 — Provision a runner VM with VPN

Get a VM (or bare-metal host) with **two network legs**:

- **Leg 1 — public internet:** HTTPS to **GitHub** (runner agent) and **unauthenticated
  pulls** from the registries pinned in [`ci/config/images.env`](../ci/config/images.env)
  — primarily **`registry.access.redhat.com`** (UBI 9 base and tool images) and
  **`quay.io`** (for example `buildah/stable`, published harness/builder refs).
  **No Quay robot or registry token** is required on the smoke runner for these
  public pins. Standard public egress; not over the lab VPN.
- **Leg 2 — lab:** **persistent VPN** (or VLAN) to **edge-112** (TLV2). VPN must
  stay up for the **entire job** (up to **240 minutes**). Use **systemd** for
  auto-reconnect. Start the runner service **after** Leg 2 is up.

See [TLV2 runner doc](../../Docs/Github/storage-cert-harness-smoke-runner-tlv2.md#network-two-legs).

Install on the runner host:

| Component | Notes |
| --- | --- |
| `oc` / `kubectl` | Compatible with the lab cluster version |
| `podman` | Or agreed container engine for harness smoke (**B2**) |
| Git | Checkout trusted `main` CI scripts |
| GitHub Actions runner agent | [Managing self-hosted runners](https://docs.github.com/en/actions/hosting-your-own-runners/managing-self-hosted-runners) |

Coordinate with platform owners for VPN profile (Leg 2 to the API server) and
confirm Leg 1 allows **GitHub**, **`registry.access.redhat.com`**, and **`quay.io`**
through any corporate proxy/firewall (no special registry credentials on the VM).
Cluster identity is only via GitHub `live-smoke` secrets. TLV2
lab flow and ticket template:
[storage-cert-harness-smoke-runner-tlv2.md](../../Docs/Github/storage-cert-harness-smoke-runner-tlv2.md).

### Step 2 — Validate cluster access from the runner host

**Before** registering anything with GitHub, prove the runner VM can talk to the
cluster. With VPN up, on the **VM**:

```bash
export KUBECONFIG=/path/to/decoded-kubeconfig
oc whoami --show-server
oc get nodes -l node-role.kubernetes.io/worker
```

If this works on your laptop but **fails on the VM**, fix VPN / routing on the VM
first. **Do not proceed** until `oc` commands succeed from the runner host itself.

Optional: `curl -sSfI https://github.com` on the VM (runner agent needs GitHub).

### Step 3 — Register the self-hosted runner on GitHub

Register the runner on the **upstream org** (not a personal fork):

1. **GitHub org** → **Settings** → **Actions** → **Runners** → **New self-hosted runner**.
2. Follow the setup instructions to install and configure the runner agent on the VM.
3. **Labels:** `self-hosted`, `storage-cert-smoke` — must match `runs-on` in
   [ci-live-smoke.yml](../.github/workflows/ci-live-smoke.yml).
4. **Scope:** upstream repo or an org runner group with access limited to the smoke repo.
5. Install as a service (`./svc.sh install`) so the agent survives logout.

Do **not** register cluster-capable runners on fork repositories.

### Step 4 — Create the GitHub environment and secrets

In the **repository:** **Settings** → **Environments** → **New environment** →
name it exactly **`live-smoke`**.

Add these **secrets** to the `live-smoke` environment:

| Secret | Format | Purpose |
| --- | --- | --- |
| `CI_CLUSTER_KUBECONFIG` | Base64-encoded kubeconfig file bytes | Lab cluster API (edge-112) |
| `HARNESS_BACKENDS_YAML` | Raw YAML | Smoke backends (`storage_class`, `snapshot_class`, backend `oc-edge-112`, …) |
| `KUBECTL_INSECURE_SKIP_TLS_VERIFY` | `true` / `false` (variable or secret) | Set `true` only if the lab API cert is untrusted by the runner |

> **Warning:** Put cluster credentials in the **`live-smoke` environment only** —
> **never** in repository-wide secrets. That prevents fork workflows and unrelated
> jobs from seeing cluster credentials.

Smoke SLA thresholds are **not** environment secrets — they live in
`ci/config/smoke/thresholds-smoke.json` and are staged from the PR artifact.
**B2** materializes kubeconfig/backends on the runner per job (`0600`, deleted in
`trap`); avoid leaving CI kubeconfig on the VM disk permanently.

### Step 5 — Set the enable switch

**Settings** → **Secrets and variables** → **Actions** → **Variables**:

| Variable | Value |
| --- | --- |
| `CI_LIVE_SMOKE_ENABLED` | `true` |

All live-smoke jobs check this variable. Leave it unset or `false` to keep the
workflow **inert** while you are still setting up.

### Step 6 — Lock down the environment (recommended)

On environment **`live-smoke`**:

- **Deployment branches:** limit to `main` (or your default branch).
- **Required reviewers:** optional — enable if only maintainers should dispatch smoke.
- **Allowed actors** for `workflow_dispatch`: restrict to maintainers per org policy.

### Step 7 — Enable the workflow

1. Ensure [ci-live-smoke.yml](../.github/workflows/ci-live-smoke.yml) is merged to `main`.
2. **Settings** → **Actions** → **General** → allow workflows (permissions need at
   least `contents: read`, `pull-requests: read` as in the workflow).
3. Set `CI_LIVE_SMOKE_ENABLED=true` (Step 5) if not already done.

Then:

- **Schedule:** cron `0 5 * * *` UTC daily picks up qualifying open PRs (see
  [§ 2 Daily scheduled smoke](#2-daily-scheduled-smoke-once-per-day)).
- **On-demand:** `/run-live-smoke` on a PR, **Actions → Run workflow**, or
  `gh workflow run` (see [§ 3](#3-activate-smoke--cli-pr-comment-and-github-ui)).

Dry-run: `workflow_dispatch` with a test PR; confirm **`live-smoke-dispatch`** runs
on your runner. Until **B2** completes, exit **5** after lease acquire still validates
VPN, labels, and environment wiring.

### Quick summary — what goes where

```text
Runner VM (your infra)
  ├─ Persistent VPN to Red Hat lab (edge-112)
  ├─ podman, oc/kubectl
  └─ GitHub Actions runner agent
       Labels: self-hosted, storage-cert-smoke

GitHub repo settings
  ├─ Variable: CI_LIVE_SMOKE_ENABLED = true
  ├─ Environment: live-smoke
  │     ├─ Secret: CI_CLUSTER_KUBECONFIG (base64)
  │     ├─ Secret: HARNESS_BACKENDS_YAML (raw YAML)
  │     ├─ Secret/variable: KUBECTL_INSECURE_SKIP_TLS_VERIFY (if needed)
  │     └─ Protection: branch limits, optional reviewers
  └─ Actions → General → allow workflows
```

### What does not work

| Setup | Result |
| --- | --- |
| Kubeconfig secret only, no VPN on runner | API timeout / connection refused |
| `ubuntu-latest` for live-smoke jobs | Workflow uses self-hosted; hosted pool cannot reach edge-112 |
| Runner on a network without lab VPN | Same as above |

---

## 1. GitHub secrets reference (Model B)

Details are in [Step 4](#step-4--create-the-github-environment-and-secrets).

**`live-smoke` environment** needs cluster material only (`CI_CLUSTER_KUBECONFIG`,
`HARNESS_BACKENDS_YAML`, optional TLS skip). **Do not** add Quay/registry robot
tokens there for smoke: builder/runtime pins in [`ci/config/images.env`](../ci/config/images.env)
use **public UBI** on `registry.access.redhat.com` and public **`quay.io`** refs;
`podman pull` on the runner uses Leg 1 without registry authentication.

(Publishing images to Quay from **`ci-build.yml`** on `main` is separate; that job
uses its own publish secrets on GitHub hosted runners — not the smoke runner VM.)

Do not commit kubeconfig or private threshold exports to git.

---

## 2. Daily scheduled smoke (once per day)

Scheduling is defined **in the workflow file** — there is no separate GitHub
“scheduled task” product to configure.

```yaml
# .github/workflows/ci-live-smoke.yml
schedule:
  - cron: '0 5 * * *'   # 05:00 UTC daily
```

### What operators must do

1. Merge `ci-live-smoke.yml` on `main` (already in repo).
2. Set `CI_LIVE_SMOKE_ENABLED=true`.
3. Configure environment `live-smoke` and the self-hosted runner.
4. **Settings → Actions → General:** allow workflows (and default permissions
   sufficient for `contents: read`, `pull-requests: read` as declared in the workflow).

### What the schedule does

1. **`resolve-schedule`** (ubuntu-latest): lists open same-repo PRs targeting
   `main` **updated in the last 24 hours**, skips `[skip live-smoke]`, skips fork
   heads. If none qualify, the workflow ends without cluster work.
2. **`bundle-schedule`**: uploads each qualifying PR head as a short-lived artifact.
3. **`live-smoke-schedule`** (self-hosted, `environment: live-smoke`): checks out
   **trusted** `main` CI scripts only, downloads the PR tree to `pr-src/`, runs
   `bash ci/scripts/run-harness/live-cluster-smoke.sh`.

To change the time, edit the `cron` expression in the workflow and merge to `main`
(schedules always use the workflow on the default branch).

**Planned improvement (C2):** filter scheduled PRs by smoke-relevant paths, not only
`updated_at` — see the runbook.

---

## 3. Activate smoke — CLI, PR comment, and GitHub UI

All paths require `CI_LIVE_SMOKE_ENABLED=true` and a qualifying **open same-repo**
PR to `main` without `[skip live-smoke]`.

### GitHub UI (`workflow_dispatch`)

1. **Actions** → **CI Live Smoke** → **Run workflow**.
2. Branch: `main`.
3. Input **pr_number**: open pull request number.
4. Run (subject to environment approval if configured).

### PR comment

A trusted association must post the exact token (case-sensitive):

```text
/run-live-smoke
```

Allowed associations: `MEMBER`, `OWNER`, `COLLABORATOR`, `MAINTAINER`.

### GitHub CLI

```bash
export GH_REPO=openshift-storage-cert/storage-cert-harness

# On-demand via comment (triggers issue_comment workflow)
gh pr comment 123 --repo "$GH_REPO" --body '/run-live-smoke'

# On-demand via workflow_dispatch
gh workflow run "CI Live Smoke" --repo "$GH_REPO" --ref main -f pr_number=123

# Inspect runs
gh run list --repo "$GH_REPO" --workflow=ci-live-smoke.yml --limit 10
gh run view <run-id> --repo "$GH_REPO" --log-failed
```

### What does not trigger smoke

- Opening or pushing to a PR (no `pull_request` trigger).
- Push to `main`.
- Comments on **fork** PRs (resolve job fails: fork not allowed).
- `/run-live-smoke` from an untrusted association (job skipped).

---

## 4. Test triggers and schedule behavior

Use a **throwaway same-repo branch** and PR. Coordinate with the cluster owner so
lease and smoke do not collide with other work.

### Enablement smoke test

| Step | Expectation |
| --- | --- |
| With `CI_LIVE_SMOKE_ENABLED` unset/false, comment `/run-live-smoke` | Workflow does not run live jobs (all jobs skipped by `if`) |
| Set variable `true`, re-comment | `resolve-comment` → `bundle-comment` → `live-smoke-comment` run |
| `gh workflow run ... -f pr_number=N` | Same dispatch path as UI |

### Schedule resolver (without waiting for 05:00 UTC)

Full `schedule` events only fire from GitHub cron. To validate logic:

- After the daily run, open **Actions** → select the scheduled **CI Live Smoke**
  run → inspect **`resolve-schedule`** logs for  
  `No open same-repo PRs to main updated since ...` or `Scheduled live smoke for PR(s): ...`.
- **Positive test:** open a PR to `main`, push a no-op commit, wait until
  `updated_at` is within 24h, ensure next cron run picks it up (or use on-demand
  for functional smoke while schedule is validated separately).
- **Negative test:** add `[skip live-smoke]` to PR title → resolver must exclude PR.
- **Negative test:** stale PR (no update in 24h) → `resolve-schedule` sets `run=false`.

### Fork guard

| Step | Expectation |
| --- | --- |
| Open PR from a **fork** into upstream | Do not post `/run-live-smoke` expecting success — resolve fails with fork message |
| Push to fork `main` with workflows enabled | No cluster job; `github.repository` guard |

### Skip token

| Step | Expectation |
| --- | --- |
| PR body contains `[skip live-smoke]` | Dispatch/comment resolve fails with skip message |
| Failed smoke then add skip token | Skip does **not** clear a failed required check — re-run smoke after fix |

### Execution note (B2)

Until `live-cluster-smoke.sh` runs the full harness, expect exit code **5** after
lease acquire. That still validates triggers, bundling, runner placement, and
kubeconfig wiring once **B2** writes secrets to disk.

---

## 5. Branch protection and secret exposure checks

### Configure required check (B4)

After the first successful live-smoke check appears on a PR:

1. **Settings → Branches** → rule for `main`.
2. Require status checks before merge.
3. Add the check name(s) from the live-smoke jobs (exact names appear on a PR
   after a run — often under workflow **CI Live Smoke**, for example
   `live-smoke-comment` / `live-smoke-dispatch` / `live-smoke-schedule`).
4. **Do not** require checks that never run on docs-only PRs unless policy intends
   to block until on-demand smoke is run.

| Test | Expectation |
| --- | --- |
| Merge with **failed** live-smoke | Blocked when check is required |
| Merge with **passed** live-smoke | Allowed (other rules permitting) |
| Re-run smoke via `/run-live-smoke` after fix | New run; branch unblocks when green |

### Validate secrets are not revealed

| Check | How |
| --- | --- |
| Job logs | Download log for `live-smoke-*` job; search must **not** find raw `client-key-data`, `token`, backend passwords, or full kubeconfig YAML. Scripts should use `set +x` around secret material (**B2**). |
| Artifacts | Download workflow artifacts (`live-smoke-src-*`); confirm they contain **source tree only**, no `kubeconfig` or `backends` with secrets. |
| Fork workflow | On a fork PR, confirm upstream **CI** (lint/test) logs show **no** `CI_CLUSTER_*` env values ([GitHub fork secret rules](https://docs.github.com/en/actions/security-for-github-actions/security-guides/using-secrets-in-github-actions#accessing-your-secrets-in-forked-pull-requests)). |
| PR checks from fork | Upstream live-smoke workflow must **not** run for fork head; no check should expose cluster metadata. |
| Repo secrets scope | Confirm cluster credentials exist only under **environment `live-smoke`**, not duplicated as repo secrets used by `.github/workflows/ci.yml`. |
| Lease objects | Optional: on cluster, inspect Coordination Lease holder annotations — should show run id / PR number, not kubeconfig (**D2**). |

### Manual log grep (example)

```bash
gh run download <run-id> --repo openshift-storage-cert/storage-cert-harness -D /tmp/smoke-run
grep -R -E 'client-key-data|password:|BEGIN PRIVATE' /tmp/smoke-run && echo 'FAIL: possible secret leak' || echo 'OK: no obvious PEM/password patterns'
```

Treat any hit as a release blocker until the step is fixed to redact or avoid printing.

---

## Kubeconfig, VPN, and why hosted runners fail

**Kubeconfig = keys to the front door. VPN = the road to the front door.**

You need **both**. Either one alone is not enough.

### What the kubeconfig gives you (authentication)

The kubeconfig file contains:

- **API server URL** — `https://api.ocp-edge112-0.lab.eng.tlv2.redhat.com:6443`
- **Client certificate + private key** — proves identity to the cluster
- **Cluster CA certificate** — trust the API server TLS certificate

That is **authentication** (your identity badge). Storing it as a masked GitHub
secret (`CI_CLUSTER_KUBECONFIG` in environment **`live-smoke`**) is required so the
workflow can present credentials — and it keeps them out of logs when configured
correctly.

### What the kubeconfig does **not** give you (network access)

The kubeconfig has **no networking**: no VPN tunnel, no firewall rule, no route.
It is credentials in a file (or in memory after decode).

### Why it fails on a GitHub-hosted runner (`ubuntu-latest`)

```text
GitHub-hosted runner (public internet)
        │
        │  oc login https://api.ocp-edge112-0.lab.eng.tlv2.redhat.com:6443
        ▼
   DNS lookup: api.ocp-edge112-0.lab.eng.tlv2.redhat.com
        ▼
   ❌ FAIL — hostname resolves to an internal lab IP unreachable
      from the public internet. Connection hangs or times out.
      The kubeconfig is never presented to the cluster.
```

The job may hold the correct credentials in memory, but it cannot open a **TCP**
connection to the API server: there is no route from a public cloud VM to
`lab.eng.tlv2.redhat.com`. The request never reaches the cluster, so **TLS client
auth never runs**.

### Two layers (both required)

#### Layer 1 — NETWORK (can you reach the door?)

```text
┌──────────────────────────────────────────────────┐
│  Runner VM  ──── VPN tunnel ────▶  Lab network   │
│  (public IP)     (always-on)       (internal)    │
│                                        │         │
│                    api.ocp-edge112-0:6443      │
└──────────────────────────────────────────────────┘
```

Without Layer 1 → timeout / hang. **Kubeconfig does not help at this layer.**

#### Layer 2 — AUTHENTICATION (are you allowed in?)

```text
┌──────────────────────────────────────────────────┐
│  Runner has route to API (Layer 1 ✅)             │
│       ▼                                          │
│  TLS handshake with cluster CA ✅                │
│  Client cert + key from kubeconfig ✅            │
│  Cluster validates identity → access granted ✅  │
└──────────────────────────────────────────────────┘
```

Without Layer 2 → `401 Unauthorized` (or TLS failure). **VPN alone is not enough
at this layer.**

`CI_CLUSTER_KUBECONFIG` in the `live-smoke` environment is **required but not
sufficient**: the runner must sit on a network that can **dial** the API URL in the
kubeconfig (Model A: your laptop on VPN; Model B: self-hosted runner on Leg 2).

### Bottom line

| Mechanism | What it provides |
| --- | --- |
| **Masked GitHub secret** | Protects credentials from appearing in logs; supplies **authentication** to the workflow |
| **VPN / lab routing (Leg 2)** | **Connectivity** to the API — without it, smoke cannot run |
| **Self-hosted runner on VPN** | Satisfies **both** layers for Model B |

Masking is about **secrecy**, not **connectivity**. See
[Two execution models](#two-execution-models), [TLV2 runner doc](../../Docs/Github/storage-cert-harness-smoke-runner-tlv2.md),
and design [§2 Network (VPN)](design/storage-cert-harness-ci-smoke-test.md).

---

## Quick reference

| Item | Value |
| --- | --- |
| Workflow file | [`.github/workflows/ci-live-smoke.yml`](../.github/workflows/ci-live-smoke.yml) |
| Enable variable | `CI_LIVE_SMOKE_ENABLED=true` |
| Environment | `live-smoke` |
| Runner labels | `self-hosted`, `storage-cert-smoke` |
| Models | **A** laptop + VPN; **B** runner on Red Hat network — [Two execution models](#two-execution-models) |
| Model B setup | [Steps 1–7](#model-b-setup--self-hosted-runner-steps-17) |
| Schedule | `0 5 * * *` UTC |
| On-demand | `/run-live-smoke`, `workflow_dispatch`, `gh workflow run` |
| Kubeconfig + VPN | [§ Kubeconfig, VPN, and why hosted runners fail](#kubeconfig-vpn-and-why-hosted-runners-fail) |
| TLV2 runner / VPN | [Docs/Github/storage-cert-harness-smoke-runner-tlv2.md](../../Docs/Github/storage-cert-harness-smoke-runner-tlv2.md) |
| Jira | [ECOPROJECT-5406](https://redhat.atlassian.net/browse/ECOPROJECT-5406), **B1** / **B4** |
