# 0011. Harness ships as a container image

- **Status:** Accepted
- **Date:** 2026-08-25

## Context

[ADR-0001](0001-language-go.md) chose Go so the harness can be a single static
binary in a **small container**. [ADR-0003](0003-harness-architecture-ports-and-adapters.md)
repeats that: one binary in one container; tools run from *their* released
images. Partners and Red Hat run the harness against an OpenShift cluster, so
the distribution unit is an image they can pull, not a GitLab job artifact.

Until ECOPROJECT-5274, CI only compiled `bin/harness` and kept it as a one-week
artifact. That did not match the founding decision.

SLA numbers must never be baked into the image
([ADR-0004](0004-threshold-secret-handling.md)).

## Decision

1. **Primary release artifact** is a container image built from the repo-root
   `Containerfile`. Local `make build` still produces `bin/harness` for
   development; that binary is not what we publish.
2. **Multi-stage build:** Red Hat `ubi10/go-toolset` builder (`BUILD_IMAGE` in
   [`ci/images.env`](../ci/images.env); `GOPROXY=off`, `-mod=vendor`,
   `CGO_ENABLED=0`) copies the static binary into `RUNTIME_IMAGE` (UBI micro).
   Non-root `USER 65532`, `ENTRYPOINT ["/usr/bin/harness"]`. Do not use
   `docker.io/library/golang` (CEE Hub anonymous rate limits).
3. **Registry:** `quay.io/eco-special-projects/storage-cert-harness:<git-sha>`
   (12-char SHA). Pushes to the default branch also tag `:main`.
4. **No secrets in the image.** Real `thresholds.json` is supplied at run time
   (`--thresholds` / `HARNESS_THRESHOLDS`). The image contains only the binary.
5. **How CI builds, scans, and pushes** is [ADR-0010](0010-ci-scripts-and-multi-host.md)
   (`ci/image-build.sh`, parallel Trivy + Dive, push only if both pass). This
   ADR decides *what* we ship, not the wrapper scripts.

Do not confuse this image with tool `Image:` refs in adapters — those are
upstream test-tool images the harness *launches*, not the harness itself.

## Options Considered

- **Container image on Quay (chosen)** — (+) matches ADR-0001/0003; partners
  pull one ref; UBI-micro stays small and supportable. (−) needs registry
  credentials and image-scan gates.
- **Binary-only GitLab artifacts** — (+) simpler CI. (−) contradicts the
  founding ship-as-container constraint; 1-week artifacts are not a release.
- **Distroless / scratch instead of UBI-micro** — (+) even smaller. (−) leaves
  Red Hat supportability and UBI policy; micro is already tiny with a static
  binary.
- **Helm chart / OLM operator as the primary artifact** — deferred. The
  container is the unit those would wrap.

## Consequences

- Every MR builds the image (no push) and scans it; `main` pushes git-sha +
  `:main` only after Trivy and Dive pass (ADR-0010).
- Partners consume `quay.io/eco-special-projects/storage-cert-harness:<sha>`.
- Image build is offline against committed `vendor/`.
- Follow-ups: Cosign / signed SBOM, multi-arch, disconnected-registry mirroring
  (ADR-0003).

## Non-Goals

- Cosign, signed SBOM publish, multi-arch manifests.
- Helm / OLM packaging.
- Live cluster smoke (ECOPROJECT-5331).
- Tool-package `Containerfile` fallbacks (ADR-0003) — those are for upstream
  tools that have no released image, not for the harness.

## References

- [ECOPROJECT-5274](https://redhat.atlassian.net/browse/ECOPROJECT-5274)
- [0001](0001-language-go.md), [0003](0003-harness-architecture-ports-and-adapters.md),
  [0004](0004-threshold-secret-handling.md), [0010](0010-ci-scripts-and-multi-host.md)
- [`Containerfile`](../Containerfile), [`docs/ci.md`](../docs/ci.md)
