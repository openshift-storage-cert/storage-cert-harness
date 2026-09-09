# 0011. Harness ships as a container image

- **Status:** Accepted
- **Date:** 2026-08-25

## Context

Partners need a pullable release artifact. A short-lived CI binary artifact does
not satisfy the container delivery chosen in ADR-0001/0003.

## Decision

- **Primary artifact:** publish the harness container built from the root
  `Containerfile`. `make build` still produces a development binary.
- **Registry:** `quay.io/eco-special-projects/storage-cert-harness`, initially
  tagged with a 12-character Git SHA and `:main` for default-branch pushes.
- **No secrets:** supply real thresholds at runtime via `--thresholds` /
  `HARNESS_THRESHOLDS`; never bake thresholds or encryption keys into the image.
- **Release checks:** MRs build and scan without publishing. Default-branch
  publication requires both Trivy and Dive to pass (ADR-0010).
- **Original packaging:** build a static Go binary with `CGO_ENABLED=0`, offline
  vendored dependencies, a Red Hat UBI Go builder, and a UBI-micro runtime;
  run as non-root UID 65532 with the harness entrypoint. Prefer Red Hat base
  images over anonymous Docker Hub pulls because of CEE rate limits.

## Decision notes

### Options considered

- **Quay image (chosen):** durable and pullable; needs registry credentials and
  scan gates. Short-lived binary artifacts do not satisfy release delivery.
- **UBI (chosen) / scratch or distroless:** favors Red Hat supportability over
  the smallest possible runtime.
- **Helm/OLM:** deferred packaging that would wrap the image.

### Consequences

**Implementation has evolved:** the current image packages local tool runtimes
as well as the binary, using a UBI Python runtime and an entrypoint script.
The original binary-only/UBI-micro layout is historical, not a description of
that image. Current build inputs, tags, and contents are defined in
[`Containerfile`](../Containerfile), [image pins](../ci/config/images.env), and
[CI documentation](../docs/ci.md). Offline Go compilation does not imply offline
installation of the packaged tools.

Harness packaging is distinct from upstream tool images launched by adapters
and tool-specific fallback Containerfiles (ADR-0003). Signing, signed SBOMs,
multi-architecture manifests, disconnected mirroring, Helm/OLM, and live cluster
smoke remain separate follow-ups.

Related: [ADR-0010](0010-ci-scripts-and-multi-host.md),
[ADR-0004](0004-threshold-secret-handling.md),
[ECOPROJECT-5274](https://redhat.atlassian.net/browse/ECOPROJECT-5274).
