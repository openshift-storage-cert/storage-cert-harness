#!/usr/bin/env bash
# Trivy-only image scan (parallel with ci/image-scan-dive.sh).
# Vulns, secrets in layers, misconfig, and CycloneDX SBOM.
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

trivy_status=0
trivy "${trivy_input[@]}" \
	--exit-code 1 \
	--severity HIGH,CRITICAL \
	--ignore-unfixed \
	--ignorefile "${CI_CONFIG_DIR}/trivyignore" \
	--format table || trivy_status=$?
if (( trivy_status == 1 )); then
	echo "warning: HIGH/CRITICAL vulnerabilities found; continuing while remediation is tracked"
elif (( trivy_status != 0 )); then
	exit "${trivy_status}"
fi

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
echo "image-scan-trivy ok"
