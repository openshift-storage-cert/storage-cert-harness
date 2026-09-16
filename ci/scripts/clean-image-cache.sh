#!/usr/bin/env bash
# Remove local harness image artifacts and prune the active container engine cache.
# Use before image-build when Containerfile cleanup steps appear to have no effect
# (stale cached layers). Pair with IMAGE_BUILD_NO_CACHE=1 or make image-build-fresh.
set -euo pipefail
# shellcheck source=ci-utils.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/ci-utils.sh"

eng="$(container_engine)"
img_ver="$(image_version)"
ref="${QUAY_IMAGE}:${img_ver}"

echo "==> removing dist image tarball"
rm -f -- "${IMAGE_TAR}"

echo "==> removing local harness image tags (${eng})"
for tag in "${ref}" "${QUAY_IMAGE}:local" "${QUAY_IMAGE}:main"; do
	if "${eng}" image exists "${tag}" >/dev/null 2>&1; then
		"${eng}" rmi -f "${tag}" >/dev/null 2>&1 || true
	fi
done

echo "==> pruning ${eng} build cache"
case "${eng}" in
	podman)
		podman builder prune -af >/dev/null 2>&1 || true
		podman image prune -af >/dev/null 2>&1 || true
		;;
	buildah)
		buildah rm -af >/dev/null 2>&1 || true
		buildah rmi -af >/dev/null 2>&1 || true
		;;
	docker)
		docker builder prune -af >/dev/null 2>&1 || true
		docker image prune -af >/dev/null 2>&1 || true
		;;
esac

echo "clean-image-cache ok"
