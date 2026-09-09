#!/usr/bin/env bash
# Run the container smoke plan against the local kubeconfig and work files.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"
# shellcheck source=ci-utils.sh
source "${repo_root}/ci/scripts/ci-utils.sh"
ci_log_init "run-image-tests"

command -v podman >/dev/null 2>&1 || {
	echo "error: podman is required" >&2
	exit 1
}
KUBECONFIG="${1:-work/kubeconfig}"
[[ -s "${KUBECONFIG}" && -r "${KUBECONFIG}" ]] || {
	echo "error: kubeconfig is missing or unreadable: ${KUBECONFIG}" >&2
	exit 1
}
[[ -s work/local-image-ref ]] || {
	echo "error: work/local-image-ref is missing; run 'make image-build-local' first" >&2
	exit 1
}
[[ -s work/backends.local.yaml ]] || {
	echo "error: backend configuration is missing or empty: work/backends.local.yaml" >&2
	exit 1
}

mkdir -p work/smoke-work work/smoke-report

image="$(tr -d '[:space:]' < work/local-image-ref)"
backend="${BACKEND:-$(awk '$1 == "-" && $2 == "name:" { print $3; exit }' work/backends.local.yaml)}"
[[ -n "${backend}" ]] || {
	echo "error: no backend found in work/backends.local.yaml; set BACKEND explicitly" >&2
	exit 1
}
arch="${GOARCH:-$(uname -m)}"
case "${arch}" in
	x86_64) arch=amd64 ;;
	aarch64) arch=arm64 ;;
esac
podman run --rm --userns=keep-id --user "$(id -u):$(id -g)" --network=host \
	--platform "linux/${arch}" \
	--privileged \
	-v "${repo_root}/work:/work:Z" \
	-v "$(realpath "${KUBECONFIG}"):/work/kubeconfig:ro,Z" \
	-w /work \
	-e KUBECONFIG=/work/kubeconfig \
	-e KUBE_BURNER_OCP_USE_HOST=1 \
	"${image}" run \
	--catalog /work/catalog.json \
	--thresholds /work/thresholds.json \
	--plan /work/container-smoke.yaml \
	--backends /work/backends.local.yaml \
	--backend "${backend}" \
	--workdir /work/smoke-work \
	--output /work/smoke-report