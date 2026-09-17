#!/usr/bin/env bash
# Containerfile lint (independent of the built-image security scans).
set -euo pipefail
# shellcheck source=ci-utils.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/ci-utils.sh"
ci_log_init "image-lint"

echo "Containerfile privileged setup using USER 0:"
awk '
/^FROM / { stage = $0 }
/^[[:space:]]*USER[[:space:]]+0[[:space:]]*$/ {
	printf "  line %d: %s (%s)\n", NR, $0, stage
}' "${REPO_ROOT}/Containerfile"

if [[ "${CI:-}" == "true" || "${GITHUB_ACTIONS:-}" == "true" ]]; then
	eng="$(container_engine)"
	echo "running Hadolint ${HADOLINT_IMAGE} in CI against Containerfile"
	"${eng}" run --rm \
		--security-opt label=disable \
		-v "${REPO_ROOT}:/workspace:ro" \
		"${HADOLINT_IMAGE}" \
		/bin/hadolint /workspace/Containerfile
elif command -v hadolint >/dev/null 2>&1; then
	echo "running native Hadolint $(hadolint --version) against Containerfile"
	hadolint "${REPO_ROOT}/Containerfile"
else
	die "hadolint not found; run ./ci/scripts/install-tools.sh or set CI=true to use the pinned image"
fi

echo "image-lint ok"