#!/usr/bin/env bash
# Remove harness smoke artifacts, staged smoke logs, and optional staged config copies.
set -euo pipefail

_run_harness_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../ci-utils.sh
source "${_run_harness_dir}/../ci-utils.sh"

echo "==> clean-smoke-env"
clean_harness_smoke_artifacts
clean_smoke_run_logs

work="${REPO_ROOT}/work"
for f in harness-run-thresholds.json backends.local.yaml thresholds.json catalog.json; do
	path="${work}/${f}"
	if [[ -f "${path}" ]]; then
		echo "clean-smoke-env: removing staged ${path}"
		rm -f -- "${path}"
	fi
done

# Stale lock file only (not held by a live process — do not run clean during an active smoke).
if [[ -f "${work}/smoke-run.lock" ]]; then
	echo "clean-smoke-env: removing stale ${work}/smoke-run.lock (ignore if smoke is running)"
	rm -f -- "${work}/smoke-run.lock"
fi

echo "clean-smoke-env: ok (kubeconfig and local-image-ref preserved)"
