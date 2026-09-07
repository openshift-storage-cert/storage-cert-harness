#!/usr/bin/env bash
# Trivy-only image scan (parallel with ci/image-scan-dive.sh).
# Vulns, secrets in layers, misconfig, CycloneDX SBOM, hadolint on Containerfile.
set -euo pipefail
# shellcheck source=ci-utils.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/ci-utils.sh"
ci_log_init "image-scan-trivy"
require_cmd trivy

target="${IMAGE_TAR}"
trivy_input=(image)
if [[ -f "${target}" ]]; then
	trivy_input=(image --input "${target}")
else
	trivy_input=(image "$(image_ref)")
fi

mkdir -p "${REPO_ROOT}/dist"
trivy_report_json="${REPO_ROOT}/dist/trivy-report.json"
trivy_report_txt="${REPO_ROOT}/dist/trivy-report.txt"

# Always write reports for CI artifacts (gate runs separately below).
trivy "${trivy_input[@]}" \
	--format json \
	--output "${trivy_report_json}" \
	--ignore-unfixed \
	--ignorefile "${CI_CONFIG_DIR}/trivyignore"

trivy "${trivy_input[@]}" \
	--format table \
	--output "${trivy_report_txt}" \
	--severity HIGH,CRITICAL \
	--ignore-unfixed \
	--ignorefile "${CI_CONFIG_DIR}/trivyignore"

echo "wrote ${trivy_report_json} and ${trivy_report_txt}"

trivy "${trivy_input[@]}" \
	--exit-code 1 \
	--severity HIGH,CRITICAL \
	--ignore-unfixed \
	--ignorefile "${CI_CONFIG_DIR}/trivyignore" \
	--format table

trivy "${trivy_input[@]}" \
	--scanners secret \
	--exit-code 1

trivy "${trivy_input[@]}" \
	--scanners misconfig \
	--exit-code 1 \
	--severity HIGH,CRITICAL

trivy "${trivy_input[@]}" \
	--format cyclonedx \
	--output "${LOG_DIR}/sbom.cdx.json"

if command -v hadolint >/dev/null 2>&1; then
	hadolint "${REPO_ROOT}/Containerfile"
else
	echo "hadolint not installed; skipped Containerfile lint"
fi
echo "image-scan-trivy ok"
