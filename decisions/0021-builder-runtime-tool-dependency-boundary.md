# 0021. Builder-owned runtime tool dependencies

- **Status:** Proposed
- **Date:** 2026-11-24
- **Tracking:** ECOPROJECT-5533

## Context

The harness image consumes a digest-pinned `harness-builder` image containing
the released kube-burner, kube-burner-ocp, OpenShift, KubeVirt, and virtbench
tool payload. Some kube-burner-ocp hooks also require runtime utilities such as
`jq` and the OpenSSH client. Installing those packages only in the harness
repository would make the tool payload incomplete and couple tool dependencies
to a frequently rebuilt harness image.

## Decision

Runtime dependencies required by packaged certification tools are owned by the
final runtime stage of `storage-cert-harness-images/images/harness-builder/Containerfile`.
The harness image consumes them through its pinned `BUILD_IMAGE`; it does not
download or install tool dependencies independently. The builder publishes the
tool version OCI labels, and the harness preserves those inherited labels.

The harness image-content check requires the builder-owned utilities in every
validated image. The pinned builder digest contains the required utilities.

## Decision notes

### Options considered

- **Install in harness-builder:** Keeps tool binaries and their runtime
  dependencies together and makes the builder digest the complete tool payload.
- **Install in the harness Containerfile:** Rejected because it duplicates
  builder ownership and can produce a harness image whose pinned tool payload
  is not independently reproducible.
- **Require host utilities:** Rejected because partner certification runs use
  the published image and must not depend on host `jq` or SSH packages.

### Consequences

Builder changes require a new builder release and a `BUILD_IMAGE` digest bump.
The harness repository retains image-content validation and provides a small
label extraction script for release and troubleshooting evidence. Developer
smoke scripts may still require host `jq` for their own cluster-lease JSON
operations; that is separate from the partner runtime dependency.
