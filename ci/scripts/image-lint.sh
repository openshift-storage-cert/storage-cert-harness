#!/usr/bin/env bash
# Containerfile lint (independent of the built-image security scans).
set -euo pipefail
# shellcheck source=ci-utils.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/ci-utils.sh"
ci_log_init "image-lint"

eng="$(container_engine)"
echo "running Hadolint ${HADOLINT_IMAGE} against Containerfile"
echo "Containerfile privileged setup using USER 0:"
awk '
/^FROM / { stage = $0 }
/^[[:space:]]*USER[[:space:]]+0[[:space:]]*$/ {
	printf "  line %d: %s (%s)\n", NR, $0, stage
}' "${REPO_ROOT}/Containerfile"

"${eng}" run --rm \
	--security-opt label=disable \
	-v "${REPO_ROOT}:/workspace:ro" \
	"${HADOLINT_IMAGE}" \
	/bin/hadolint /workspace/Containerfile

echo "image-lint ok"