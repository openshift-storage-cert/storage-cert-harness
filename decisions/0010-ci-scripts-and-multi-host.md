# 0010. CI logic lives in portable ci/ scripts

- **Status:** Accepted
- **Date:** 2026-08-25

## Context

[ADR-0002](0002-repository-host-and-sdlc-workflow.md) put CI behavior in
Makefile targets so GitLab → GitHub would be a wrapper swap. ECOPROJECT-5274
needs the same checks locally, in GitLab CI, in GitHub Actions, and in a YAML
pre-commit hook. Makefile-only logic cannot be invoked cleanly from pre-commit
or from job images that do not have `make` conventions.

SLA numbers must never enter git history (ADR-0002 / ADR-0004 / 5273). A
diff-based secret scan has to run the same way on a laptop and in CI.

## Decision

1. **Source of truth:** shell scripts in `ci/`. They wrap `go build` / `go test`
   (unit tests only) / lint / secret-scan / supply-chain / replay-smoke / image
   build and scan.
2. **Wrappers:** `.gitlab-ci.yml`, `.github/workflows/ci.yml`, `Makefile`, and
   `.pre-commit-config.yaml` only call those scripts.
3. **Offline Go:** `GOPROXY=off` and `GOFLAGS=-mod=vendor`; `vendor/` is
   committed.
4. **CEE runners:** every GitLab job sets `tags: [itup-alm-x86]` (same pool as
   csi-certification-kb), plus `default.tags` so a new job cannot omit it.
5. **Image scans:** Trivy and Dive are **parallel CI jobs**
   (`ci/image-scan-trivy.sh`, `ci/image-scan-dive.sh`). Push to Quay only if
   both pass. That image is the release artifact
   ([ADR-0011](0011-harness-ships-as-container-image.md)).
6. **Live cluster smoke** is not in this ADR; it is a follow-up (ECOPROJECT-5331)
   with a different runner tag and protected kubeconfig.

This supersedes ADR-0002’s bullet that CI logic lives in the Makefile. Secret
hygiene and “build as if public” still stand.

## Options Considered

- **ci/ scripts + thin wrappers (chosen)** — (+) one implementation for local,
  GitLab, GitHub, pre-commit. (−) more files than Makefile-only.
- **Makefile as CI logic (ADR-0002)** — (+) fewer files. (−) awkward for
  pre-commit and per-job container images (yamllint vs golang vs trivy).
- **Duplicate YAML in GitLab and GitHub** — (−) two sources of truth.

## Consequences

- Contributors run `./ci/install-tools.sh` then `./ci/ci.sh` or `make ci` before
  pushing. Local image build/run uses **podman** (`CONTAINER_ENGINE`; GitLab
  image jobs override to buildah, GitHub to docker).
- GitLab job images may differ (UBI python for YAML, golang for tests, Trivy /
  Dive / Buildah for image jobs); scripts only `require_cmd`.
- AI instruction files (`AGENTS.md`, `CLAUDE.md`, `skills/`) stay on internal
  GitLab and are `export-ignore` for public GitHub.

## Non-Goals

- Live 3-VM boot-storm / cluster-connected runners.
- Encrypted `secrets/` mechanism (ADR-0004).
- Cosign / signed SBOM publish.

## References

- [ECOPROJECT-5274](https://redhat.atlassian.net/browse/ECOPROJECT-5274)
- [0002](0002-repository-host-and-sdlc-workflow.md),
  [0011](0011-harness-ships-as-container-image.md)
- [docs/ci.md](../docs/ci.md)
