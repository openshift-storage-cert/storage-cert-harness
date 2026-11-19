#!/usr/bin/env bash
# Dive-only layer-efficiency scan (parallel with ci/image-scan-trivy.sh).
set -euo pipefail
# shellcheck source=ci-utils.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/ci-utils.sh"
acquire_local_image_lock
ci_log_init "image-scan-dive"
require_cmd dive

# Dive CI thresholds — document in docs/ci.md. Override via env.
export CI=true
highest_user_wasted_percent="${DIVE_HIGHEST_USER_WASTED_PERCENT:-0.1}"
highest_wasted_bytes="${DIVE_HIGHEST_WASTED_BYTES:-41943040}" # 40 MiB
lowest_efficiency="${DIVE_LOWEST_EFFICIENCY:-0.9}"

dive_ci_args=(
	--ci
	--highestUserWastedPercent "${highest_user_wasted_percent}"
	--highestWastedBytes "${highest_wasted_bytes}"
	--lowestEfficiency "${lowest_efficiency}"
)

dive_report="${REPO_ROOT}/dist/dive-report.txt"
mkdir -p "${REPO_ROOT}/dist"

if [[ -f "${IMAGE_TAR}" ]]; then
	set +e
	dive "${dive_ci_args[@]}" --source docker-archive "${IMAGE_TAR}" | tee "${dive_report}"
	dive_exit=${PIPESTATUS[0]}
	set -e
else
	set +e
	dive "${dive_ci_args[@]}" "$(image_ref)" | tee "${dive_report}"
	dive_exit=${PIPESTATUS[0]}
	set -e
fi

echo "wrote ${dive_report}"
[[ "${dive_exit}" -eq 0 ]] || exit "${dive_exit}"

echo "image-scan-dive ok"
