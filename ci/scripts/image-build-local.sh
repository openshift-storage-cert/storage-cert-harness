#!/usr/bin/env bash
# Build a uniquely tagged local image from the already-built bin/harness.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

command -v podman >/dev/null 2>&1 || {
	echo "error: podman is required" >&2
	exit 1
}
[[ -x bin/harness ]] || {
	echo "error: bin/harness is missing; run 'make build' first" >&2
	exit 1
}

image_version="$(tr -d '[:space:]' < NEXT-IMAGE-VERSION)"
suffix="$(od -An -N4 -tx1 /dev/urandom | tr -d '[:space:]')"
image="storage-cert-harness:${image_version}-${suffix}"
arch="${GOARCH:-$(uname -m)}"
case "${arch}" in
	x86_64) arch=amd64 ;;
	aarch64) arch=arm64 ;;
esac
platform="linux/${arch}"

podman build \
	--platform "${platform}" \
	--build-arg "TARGETARCH=${arch}" \
	--build-arg "HARNESS_VERSION=$(tr -d '[:space:]' < NEXT-VERSION)" \
	--build-arg "IMAGE_VERSION=${image_version}" \
	-t "${image}" \
	-f Containerfile .

mkdir -p work
printf '%s\n' "${image}" > work/local-image-ref
echo "built ${image}"
echo "platform ${platform}"
echo "saved image reference to work/local-image-ref"