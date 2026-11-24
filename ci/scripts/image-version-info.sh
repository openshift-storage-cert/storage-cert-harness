#!/usr/bin/env bash
# Print harness image metadata and the tool versions inherited from harness-builder.
# Usage: ./ci/scripts/image-version-info.sh [IMAGE]
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${REPO_ROOT}"

# shellcheck source=ci-utils.sh
source "${REPO_ROOT}/ci/scripts/ci-utils.sh"

require_cmd podman

image="${1:-${HARNESS_IMAGE:-${QUAY_IMAGE}:$(image_version)}}"
[[ -n "${image}" ]] || die "image reference is empty"

podman image exists "${image}" >/dev/null 2>&1 ||
	die "image is not present locally: ${image} (pull it or build it first)"

printf 'reference=%s\n' "${image}"
podman image inspect --format='
image.id={{.Id}}
image.created={{.Created}}
image.architecture={{.Architecture}}
image.os={{.Os}}
image.size_bytes={{.Size}}
image.version={{with index .Config.Labels "org.opencontainers.image.version"}}{{.}}{{end}}
harness.version={{with index .Config.Labels "io.storage-cert-harness.harness.version"}}{{.}}{{end}}
tool.kube-burner={{with index .Config.Labels "io.storage-cert-harness.kube-burner.version"}}{{.}}{{end}}
tool.kube-burner-ocp={{with index .Config.Labels "io.storage-cert-harness.kube-burner-ocp.version"}}{{.}}{{end}}
tool.virtbench={{with index .Config.Labels "io.storage-cert-harness.virtbench.version"}}{{.}}{{end}}
tool.openshift-client={{with index .Config.Labels "io.storage-cert-harness.openshift-client.version"}}{{.}}{{end}}
tool.kubevirt={{with index .Config.Labels "io.storage-cert-harness.kubevirt.version"}}{{.}}{{end}}
tool.kube-burner.source-mode={{with index .Config.Labels "io.storage-cert-harness.kube-burner.source-mode"}}{{.}}{{end}}
tool.kube-burner-ocp.source-mode={{with index .Config.Labels "io.storage-cert-harness.kube-burner-ocp.source-mode"}}{{.}}{{end}}
' "${image}"
