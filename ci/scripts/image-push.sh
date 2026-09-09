#!/usr/bin/env bash
# Push the already-built image tarball to Quay. Call only after both scan jobs pass.
set -euo pipefail
# shellcheck source=ci-utils.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/ci-utils.sh"
ci_log_init "image-push"

[[ "${PUSH:-}" == "1" ]] || die "refusing to push (set PUSH=1)"
[[ -n "${QUAY_USER:-}" && -n "${QUAY_PASSWORD:-}" ]] || die "QUAY_USER and QUAY_PASSWORD are required"

arches=(amd64 arm64)
if [[ -n "${TARGETARCH:-}" ]]; then
	arches=("${TARGETARCH}")
fi
for arch in "${arches[@]}"; do
	case "${arch}" in
		amd64 | arm64) ;;
		*) die "unsupported TARGETARCH=${arch} (want amd64 or arm64)" ;;
	esac
done

eng="$(container_engine)"
img_ver="$(image_version)"
branch_tag="main"
if [[ "${CI_COMMIT_BRANCH:-}" == "test-ci" || "${GITHUB_REF:-}" == "refs/heads/test-ci" ]]; then
	branch_tag="test-ci"
fi
echo "${QUAY_PASSWORD}" | "${eng}" login -u "${QUAY_USER}" --password-stdin quay.io

refs=()
for arch in "${arches[@]}"; do
	tar="${IMAGE_TAR}"
	if [[ -z "${TARGETARCH:-}" ]]; then
		tar="${REPO_ROOT}/dist/${arch}/harness-image.tar"
	fi
	[[ -f "${tar}" ]] || die "missing ${tar}; run ci/scripts/image-build.sh for ${arch} first"
	ref="${QUAY_IMAGE}:${img_ver}-${arch}"
	refs+=("${ref}")
	if [[ "${eng}" == "buildah" ]]; then
		img_id="$(buildah pull "docker-archive:${tar}")"
		buildah tag "${img_id}" "${ref}"
		buildah push "${ref}" "docker://${ref}"
	else
		"${eng}" load -i "${tar}"
		"${eng}" tag "$(image_ref)" "${ref}" 2>/dev/null || true
		"${eng}" push "${ref}"
	fi
done

manifest="${QUAY_IMAGE}:${img_ver}"
branch_manifest="${QUAY_IMAGE}:${branch_tag}"
if [[ "${eng}" == "buildah" ]]; then
	buildah manifest rm "${manifest}" 2>/dev/null || true
	buildah manifest create "${manifest}" "${refs[@]}"
	buildah manifest push --all "${manifest}" "docker://${manifest}"
	buildah manifest rm "${branch_manifest}" 2>/dev/null || true
	buildah manifest create "${branch_manifest}" "${refs[@]}"
	buildah manifest push --all "${branch_manifest}" "docker://${branch_manifest}"
else
	"${eng}" manifest rm "${manifest}" 2>/dev/null || true
	"${eng}" manifest create "${manifest}" "${refs[@]}"
	"${eng}" manifest push "${manifest}"
	"${eng}" manifest rm "${branch_manifest}" 2>/dev/null || true
	"${eng}" manifest create "${branch_manifest}" "${refs[@]}"
	"${eng}" manifest push "${branch_manifest}"
fi
echo "pushed ${manifest} and ${branch_manifest} (${arches[*]})"
