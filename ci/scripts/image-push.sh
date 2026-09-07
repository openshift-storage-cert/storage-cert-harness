#!/usr/bin/env bash
# Push the already-built image tarball to Quay. Call only after both scan jobs pass.
set -euo pipefail
# shellcheck source=ci-utils.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/ci-utils.sh"
ci_log_init "image-push"

[[ "${PUSH:-}" == "1" ]] || die "refusing to push (set PUSH=1)"
[[ -n "${QUAY_USER:-}" && -n "${QUAY_PASSWORD:-}" ]] || die "QUAY_USER and QUAY_PASSWORD are required"
[[ -f "${IMAGE_TAR}" ]] || die "missing ${IMAGE_TAR}; run ci/scripts/image-build.sh first"

eng="$(container_engine)"
img_ver="$(image_version)"
ref="${QUAY_IMAGE}:${img_ver}"
branch_tag="main"
if [[ "${CI_COMMIT_BRANCH:-}" == "test-ci" || "${GITHUB_REF:-}" == "refs/heads/test-ci" ]]; then
	branch_tag="test-ci"
fi
echo "${QUAY_PASSWORD}" | "${eng}" login -u "${QUAY_USER}" --password-stdin quay.io

if [[ "${eng}" == "buildah" ]]; then
	img_id="$(buildah pull "docker-archive:${IMAGE_TAR}")"
	buildah tag "${img_id}" "${ref}"
	buildah tag "${img_id}" "${QUAY_IMAGE}:${branch_tag}"
	buildah push "${ref}" "docker://${ref}"
	buildah push "${QUAY_IMAGE}:${branch_tag}" "docker://${QUAY_IMAGE}:${branch_tag}"
else
	"${eng}" load -i "${IMAGE_TAR}"
	"${eng}" tag "$(image_ref)" "${ref}" 2>/dev/null || true
	"${eng}" tag "${ref}" "${QUAY_IMAGE}:${branch_tag}"
	"${eng}" push "${ref}"
	"${eng}" push "${QUAY_IMAGE}:${branch_tag}"
fi
echo "pushed ${ref} and ${QUAY_IMAGE}:${branch_tag}"
