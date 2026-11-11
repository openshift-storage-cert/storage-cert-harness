# Model C — Lab smoke executor VM (cron / systemd)

**Purpose:** Automated live smoke against **edge-112** without GitHub Actions
cluster access. A dedicated VM in or VPN’d to **TLV2** pulls `main` (and future PR
refs), builds or pulls the harness image, and runs the same scripts as local smoke.

**Status:** Adopted (2026-11). Implementation:
[ECOPROJECT-5612](https://redhat.atlassian.net/browse/ECOPROJECT-5612) (sub-task of
[ECOPROJECT-5406](https://redhat.atlassian.net/browse/ECOPROJECT-5406)).

Provisioning (VM + VPN, **no** `actions-runner`):
[storage-cert-harness-smoke-runner-tlv2.md](../../../Docs/Github/storage-cert-harness-smoke-runner-tlv2.md).

## VM requirements

| Item | Recommendation |
| --- | --- |
| OS | RHEL 9 or agreed enterprise Linux (UBI-compatible `podman`) |
| CPU / RAM | ≥ 4 vCPU, ≥ 16 GiB RAM |
| Disk | ≥ 200 GiB — git, `dist/`, podman storage, run artifacts |
| Leg 1 | HTTPS to `github.com`, `registry.access.redhat.com`, `quay.io` (public pins in `ci/config/images.env`) |
| Leg 2 | Always-on VPN/VLAN to `api.ocp-edge112-0.lab.eng.tlv2.redhat.com:6443` |
| Identity | Service account (for example `smoke-runner`) |
| Time | NTP/chrony; UTC run directory names |

## Installation (one-time)

| Step | Action |
| --- | --- |
| 1 | Provision VM + VPN; confirm `oc whoami` and `oc get nodes` on the VM |
| 2 | Install `git`, `make`, `podman`, `jq`, `curl`; pin `oc`/`kubectl` |
| 3 | Clone repo to `/opt/storage-cert-harness` (deploy key or read-only token) |
| 4 | Secrets on disk (**0600**): `/etc/storage-cert-smoke/kubeconfig`, `backends.yaml` |
| 5 | Same staging as `make prepare-smoke-env` |
| 6 | Wrapper `ci/scripts/run-harness/lab-smoke-executor.sh` (**B2**) or `make smoke-ci` |
| 7 | **systemd timer** or **cron** (below) |

Do **not** install `actions-runner` or register with GitHub.

## Schedule and git sync

| Mode | Cadence | Git ref | Image |
| --- | --- | --- | --- |
| **Regression on `main`** | Daily (for example `05:30 UTC`) | `origin/main` | Pull Quay tag for SHA when published; else `make image-build-local` |
| **Ad-hoc** | `systemctl start lab-smoke@<sha>` | Explicit SHA | Same |
| **PR smoke (future)** | Poll queue | PR head SHA | GHA artifact or `git fetch pull/N/head` |

**Concurrency:** one cluster smoke at a time (lease). Fail fast if lease held.

Example **systemd** units:

```ini
# /etc/systemd/system/storage-cert-smoke-main.service
[Service]
Type=oneshot
User=smoke-runner
Environment=KUBECONFIG=/etc/storage-cert-smoke/kubeconfig
ExecStart=/opt/storage-cert-harness/ci/scripts/run-harness/lab-smoke-executor.sh --ref main

# /etc/systemd/system/storage-cert-smoke-main.timer
[Timer]
OnCalendar=*-*-* 05:30:00 UTC
Persistent=true
```

## Run flow (per execution)

1. `git fetch` + checkout target SHA; record in `metadata.json`.
2. `make prepare-smoke-env`.
3. Build or pull harness image for that SHA.
4. `live-cluster-smoke.sh` or `make smoke-ci` — lease, janitor, `plans/harness-smoke-plan.yaml`.
5. Exit **0** only on harness pass.

## Results layout

Base: `/var/lib/storage-cert-smoke/runs/`.

```text
20261110T033000Z-abc1234/
  metadata.json
  status.json
  smoke-timing.json
  logs/
  report.json
  work/          # scrub secrets before retain or upload
```

Publish `latest.json` or symlink for dashboards.

**Validation:** process exit `0`, harness verdict pass, lease released, no secrets
in web-visible trees.

## Notifications

| Channel | Mechanism |
| --- | --- |
| Email | `mailx` / SMTP — subject `[storage-cert-smoke] pass&#124;fail main@sha` |
| Slack | Incoming webhook on VM |
| GitHub (future) | PAT — commit status / check run for branch protection |

## Retention and off-VM copy

| Destination | Notes |
| --- | --- |
| Local disk | 14–30 days; rotate with `find … -mtime +30 -delete` |
| NFS | `rsync -a` after each run |
| Google Drive / S3 | `rclone`; no secrets in uploaded tree |

## Web access to results

Read-only HTTPS (`nginx` or `Caddy`) over `/var/lib/storage-cert-smoke/runs`,
index of last N runs, VPN-only or SSO/basic auth.

Example URL: `https://smoke-results.lab.eng.tlv2.redhat.com/runs/<run-id>/`.

## GitHub relationship

| Concern | Behavior |
| --- | --- |
| GHA | `ubuntu-latest` only — lint, build, publish, optional PR bundle jobs |
| `/run-live-smoke` | Enqueue on VM (future); no cluster steps on GitHub |
| `main` regression | VM timer independent of GHA |
| Secrets | Kubeconfig **only** on VM |

## Validation checklist (before production cron)

- [ ] VPN survives reboot; timer starts after VPN up.
- [ ] Dry-run acquire/release lease only.
- [ ] Full smoke pass on edge-112 (**A1** / [5582](https://redhat.atlassian.net/browse/ECOPROJECT-5582)).
- [ ] Fail injection → `status.json` + notify.
- [ ] Web UI + optional NFS/Drive; no leaked secrets.
- [ ] Second concurrent start fails fast (lease).

## Related Jira

| Key | Role |
| --- | --- |
| New Model C sub-task | End-to-end executor (this doc) |
| [ECOPROJECT-5585](https://redhat.atlassian.net/browse/ECOPROJECT-5585) | `live-cluster-smoke.sh` harness execution on VM |
| [ECOPROJECT-5587](https://redhat.atlassian.net/browse/ECOPROJECT-5587) | First green **main** regression on VM timer |
| [ECOPROJECT-5588](https://redhat.atlassian.net/browse/ECOPROJECT-5588) | On-demand PR smoke via GHA → VM queue |
