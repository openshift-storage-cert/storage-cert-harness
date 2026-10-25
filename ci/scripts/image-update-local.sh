#!/usr/bin/env bash
# Build the local harness image and push it to Quay as :local.
set -euo pipefail
# shellcheck source=ci-utils.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/ci-utils.sh"

acquire_local_image_lock
eng="$(container_engine)"
require_cmd go
target_arch="$(go env GOARCH)"
case "${target_arch}" in
	amd64 | arm64) ;;
	*) die "unsupported host architecture ${target_arch} (want amd64 or arm64)" ;;
esac

echo "building local harness image for linux/${target_arch}"
GOOS=linux TARGETARCH="${target_arch}" "${CI_SCRIPTS_DIR}/build.sh"
TARGETARCH="${target_arch}" "${CI_SCRIPTS_DIR}/image-build.sh"

ref="${QUAY_IMAGE}:local"
echo "pushing ${ref} to Quay using the existing ${eng} login"
if [[ "${eng}" == "buildah" ]]; then
	buildah push "${ref}" "docker://${ref}"
else
	"${eng}" push "${ref}"
fi
echo "pushed ${ref}"
