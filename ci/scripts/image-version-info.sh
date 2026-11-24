#!/usr/bin/env bash
# Usage: ./ci/scripts/image-version-info.sh [IMAGE]
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${REPO_ROOT}"

# shellcheck source=ci-utils.sh
source "${REPO_ROOT}/ci/scripts/ci-utils.sh"

require_cmd podman

image="${1:-${HARNESS_IMAGE:-${QUAY_IMAGE}:$(image_version)}}"
[[ -n "${image}" ]] || die "image reference is empty"

if ! podman image exists "${image}" >/dev/null 2>&1; then
	echo "image is not present locally; pulling ${image}"
	podman pull "${image}"
fi
podman image exists "${image}" >/dev/null 2>&1 ||
	die "image is not available locally after pull: ${image}"

printf 'reference=%s\n' "${image}"
podman image inspect --format='image.id={{.Id}}
image.created={{.Created}}
image.architecture={{.Architecture}}
image.os={{.Os}}
image.size_bytes={{.Size}}
image.version={{with index .Config.Labels "org.opencontainers.image.version"}}{{printf "%q" .}}{{end}}
harness.version={{with index .Config.Labels "io.storage-cert-harness.harness.version"}}{{printf "%q" .}}{{end}}
tool.kube-burner={{with index .Config.Labels "io.storage-cert-harness.kube-burner.version"}}{{printf "%q" .}}{{end}}
tool.kube-burner-ocp={{with index .Config.Labels "io.storage-cert-harness.kube-burner-ocp.version"}}{{printf "%q" .}}{{end}}
tool.virtbench={{with index .Config.Labels "io.storage-cert-harness.virtbench.version"}}{{printf "%q" .}}{{end}}
tool.openshift-client={{with index .Config.Labels "io.storage-cert-harness.openshift-client.version"}}{{printf "%q" .}}{{end}}
tool.kubevirt={{with index .Config.Labels "io.storage-cert-harness.kubevirt.version"}}{{printf "%q" .}}{{end}}
tool.kube-burner.source-mode={{with index .Config.Labels "io.storage-cert-harness.kube-burner.source-mode"}}{{printf "%q" .}}{{end}}
tool.kube-burner-ocp.source-mode={{with index .Config.Labels "io.storage-cert-harness.kube-burner-ocp.source-mode"}}{{printf "%q" .}}{{end}}
' "${image}"
