#!/usr/bin/env bash
# Archive full Level-3 work/ and report/ trees plus helper logs for Red Hat submission.
set -euo pipefail

usage() {
	cat <<'EOF'
Usage:
  ./ci/scripts/run-partner-package-results.sh [OUTPUT_TAR]

Creates a gzip tarball with every file under:
  certification-results/load-80/work/
  certification-results/load-80/report/
  certification-results/no-load-80/work/
  certification-results/no-load-80/report/
  logs/run-partner-*.log (most recent helper logs, if present)

Default OUTPUT_TAR: certification-results-full.tar.gz in the repository root.

Set CERT_RESULTS_ROOT to use a different results base directory.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
	usage
	exit 0
fi
if [[ "$#" -gt 1 ]]; then
	usage >&2
	exit 2
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"
# shellcheck source=ci-utils.sh
source "${repo_root}/ci/scripts/ci-utils.sh"

results_root="${CERT_RESULTS_ROOT:-certification-results}"
out_tar="${1:-certification-results-full.tar.gz}"
[[ "${out_tar}" = /* ]] || out_tar="${repo_root}/${out_tar}"

paths=()
for profile in load-80 no-load-80; do
	for sub in work report; do
		dir="${results_root}/${profile}/${sub}"
		if [[ -d "${dir}" ]]; then
			paths+=("${results_root}/${profile}/${sub}")
		else
			echo "warning: missing ${dir} (skipped)" >&2
		fi
	done
done

partner_logs=()
if compgen -G "${LOG_DIR}/run-partner-"*.log >/dev/null 2>&1; then
	while IFS= read -r log; do
		partner_logs+=("${log}")
	done < <(find "${LOG_DIR}" -maxdepth 1 -name 'run-partner-*.log' -type f | sort)
fi
for log in "${partner_logs[@]}"; do
	rel="${log#"${repo_root}/"}"
	paths+=("${rel}")
done

if [[ "${#paths[@]}" -eq 0 ]]; then
	echo "error: nothing to package under ${results_root}/" >&2
	exit 1
fi

echo "==> packaging ${#paths[@]} path(s) -> ${out_tar}"
tar -czf "${out_tar}" -C "${repo_root}" "${paths[@]}"
echo "==> done: ${out_tar}"
echo "    For failures, ensure recipients also have each profile's report/run.log (included when report/ exists)."
