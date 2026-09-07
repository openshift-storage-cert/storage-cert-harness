#!/usr/bin/env bash
# Build the harness container and save dist/harness-image.tar for parallel scan jobs.
# Always builds linux/amd64 (x86, Intel Mac native; Apple Silicon via Rosetta 2).
# Does not push. Use ci/scripts/image-push.sh after image-scan-trivy and image-scan-dive.
set -euo pipefail
# shellcheck source=ci-utils.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/ci-utils.sh"
ci_log_init "image-build"

eng="$(container_engine)"
bin_ver="$(binary_version)"
img_ver="$(image_version)"
ref="${QUAY_IMAGE}:${img_ver}"
mkdir -p "${REPO_ROOT}/dist"

echo "building ${ref} with ${eng} (linux/amd64)"
echo "  binary_version=${bin_ver}  image_version=${img_ver}"

build_args=(
	--build-arg "HARNESS_VERSION=${bin_ver}"
	--build-arg "IMAGE_VERSION=${img_ver}"
	--build-arg "BUILD_IMAGE=${BUILD_IMAGE}"
	--build-arg "RUNTIME_IMAGE=${RUNTIME_IMAGE}"
	--build-arg "KUBE_BURNER_OCP_VERSION=${KUBE_BURNER_OCP_VERSION}"
	--build-arg "OPENSHIFT_CLIENT_VERSION=${OPENSHIFT_CLIENT_VERSION}"
	--build-arg "VIRTBENCH_VERSION=${VIRTBENCH_VERSION}"
	--platform linux/amd64
	-t "${ref}"
	-t "${QUAY_IMAGE}:local"
	-f "${REPO_ROOT}/Containerfile"
	"${REPO_ROOT}"
)

if [[ "${eng}" == "buildah" ]]; then
	buildah bud "${build_args[@]}"
	buildah push "${ref}" "docker-archive:${IMAGE_TAR}"
else
	"${eng}" build "${build_args[@]}"
	"${eng}" save -o "${IMAGE_TAR}" "${ref}"
fi

echo "${img_ver}" > "${REPO_ROOT}/dist/image-version.txt"
echo "${bin_ver}" > "${REPO_ROOT}/dist/binary-version.txt"

if is_main_push; then
	echo "main push: also tagging ${QUAY_IMAGE}:main"
	if [[ "${eng}" == "buildah" ]]; then
		buildah tag "${ref}" "${QUAY_IMAGE}:main"
	else
		"${eng}" tag "${ref}" "${QUAY_IMAGE}:main"
	fi
fi

echo "saved ${IMAGE_TAR} (image=${img_ver} binary=${bin_ver})"
