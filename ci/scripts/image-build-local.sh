#!/usr/bin/env bash
# Rebuild bin/harness and tag a unique local image under one exclusive lock.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

# shellcheck source=ci-utils.sh
source "${repo_root}/ci/scripts/ci-utils.sh"
acquire_local_image_lock
if [[ "${IMAGE_BUILD_FRESH:-0}" == "1" ]]; then
	clean_local_image_cache
fi
invalidate_previous_local_image_build
"${repo_root}/ci/scripts/build.sh"

command -v podman >/dev/null 2>&1 || {
	echo "error: podman is required" >&2
	exit 1
}
[[ -x bin/harness ]] || {
	echo "error: bin/harness is missing after build.sh" >&2
	exit 1
}

binary_version="$(binary_version)"
image_version="$(image_version)"
suffix="$(od -An -N4 -tx1 /dev/urandom | tr -d '[:space:]')"
image="storage-cert-harness:${image_version}-${suffix}"
arch="${GOARCH:-$(uname -m)}"
case "${arch}" in
	x86_64) arch=amd64 ;;
	aarch64) arch=arm64 ;;
esac
platform="linux/${arch}"

no_cache=()
if [[ "${IMAGE_BUILD_NO_CACHE:-0}" == "1" ]]; then
	echo "IMAGE_BUILD_NO_CACHE=1 (disabling layer cache)"
	no_cache=(--no-cache)
fi

podman build \
	"${no_cache[@]}" \
	--platform "${platform}" \
	--build-arg "BUILD_IMAGE=${BUILD_IMAGE}" \
	--build-arg "RUNTIME_IMAGE=${RUNTIME_IMAGE}" \
	--build-arg "TARGETARCH=${arch}" \
	--build-arg "HARNESS_VERSION=${binary_version}" \
	--build-arg "IMAGE_VERSION=${image_version}" \
	-t "${image}" \
	-f Containerfile .

mkdir -p work
printf '%s\n' "${image}" > work/local-image-ref
mkdir -p dist
printf '%s\n' "${image_version}" > dist/image-version.txt
printf '%s\n' "${binary_version}" > dist/binary-version.txt
echo "built ${image}"
echo "platform ${platform}"
echo "saved image reference to work/local-image-ref"
