#!/usr/bin/env bash
# Dive-only layer-efficiency scan (parallel with ci/image-scan-trivy.sh).
set -euo pipefail
# shellcheck source=ci-utils.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/ci-utils.sh"
ci_log_init "image-scan-dive"
require_cmd dive

# Dive CI thresholds — document in docs/ci.md. Override via env.
export CI=true
export DIVE_CI_HighestUserWastedBytes="${DIVE_CI_HighestUserWastedBytes:-20971520}" # 20 MiB
export DIVE_CI_HighestWastedBytes="${DIVE_CI_HighestWastedBytes:-41943040}"         # 40 MiB

dive_report="${REPO_ROOT}/dist/dive-report.txt"
mkdir -p "${REPO_ROOT}/dist"

if [[ -f "${IMAGE_TAR}" ]]; then
	set +e
	dive --ci --source docker-archive "${IMAGE_TAR}" | tee "${dive_report}"
	dive_exit=${PIPESTATUS[0]}
	set -e
else
	set +e
	dive --ci "$(image_ref)" | tee "${dive_report}"
	dive_exit=${PIPESTATUS[0]}
	set -e
fi

echo "wrote ${dive_report}"
[[ "${dive_exit}" -eq 0 ]] || exit "${dive_exit}"

echo "image-scan-dive ok"
