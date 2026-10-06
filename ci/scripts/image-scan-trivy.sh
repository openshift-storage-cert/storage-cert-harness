#!/usr/bin/env bash
# Trivy-only image scan (parallel with ci/image-scan-dive.sh).
# Vulns, secrets in layers, misconfig, and CycloneDX SBOM.
set -euo pipefail
# shellcheck source=ci-utils.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/ci-utils.sh"
acquire_local_image_lock
ci_log_init "image-scan-trivy"
require_cmd trivy

target="${IMAGE_TAR}"
trivy_input=(image)
if [[ -f "${target}" ]]; then
	trivy_input=(image --input "${target}")
else
	trivy_input=(image "$(image_ref)")
fi

# Common flags to prevent third-party SBOM warnings and standardize scanning
vuln_flags=(
    --scanners vuln
    --pkg-types os,library
    --ignore-unfixed
    --ignorefile "${CI_CONFIG_DIR}/trivyignore"
)

mkdir -p "${REPO_ROOT}/dist"
trivy_report_json="${REPO_ROOT}/dist/trivy-report.json"
trivy_report_txt="${REPO_ROOT}/dist/trivy-report.txt"
trivy_sbom_json="${REPO_ROOT}/dist/sbom.cdx.json"
trivy_secret_json="${REPO_ROOT}/dist/trivy-secret-report.json"
trivy_misconfig_txt="${REPO_ROOT}/dist/trivy-misconfig-report.txt"

# SBOM first: secret/misconfig gates use --exit-code 1 and set -e would skip SBOM if they ran first.
trivy "${trivy_input[@]}" \
	--format cyclonedx \
	--output "${trivy_sbom_json}"
echo "wrote ${trivy_sbom_json}"

# Always write vuln reports for CI artifacts (gate runs separately below).
trivy "${trivy_input[@]}" \
    "${vuln_flags[@]}" \
	--format json \
	--output "${trivy_report_json}"

trivy "${trivy_input[@]}" \
	"${vuln_flags[@]}" \
	--format table \
	--output "${trivy_report_txt}" \
	--severity HIGH,CRITICAL
	
echo "wrote ${trivy_report_json} and ${trivy_report_txt}"

trivy_status=0
trivy "${trivy_input[@]}" \
	"${vuln_flags[@]}" \
	--exit-code 1 \
	--severity HIGH,CRITICAL \
	--format table || trivy_status=$?

if (( trivy_status == 1 )); then
	echo "warning: HIGH/CRITICAL vulnerabilities found; continuing while remediation is tracked"
elif (( trivy_status != 0 )); then
	exit "${trivy_status}"
fi

# Secret scan: write report for CI artifacts, then gate (stdout only is not uploaded).
trivy "${trivy_input[@]}" \
	--scanners secret \
	--format json \
	--output "${trivy_secret_json}"
trivy "${trivy_input[@]}" \
	--scanners secret \
	--exit-code 1

# Misconfiguration scan: report file + gate on HIGH/CRITICAL.
trivy "${trivy_input[@]}" \
	--scanners misconfig \
	--format table \
	--output "${trivy_misconfig_txt}" \
	--severity HIGH,CRITICAL
trivy "${trivy_input[@]}" \
	--scanners misconfig \
	--exit-code 1 \
	--severity HIGH,CRITICAL

echo "wrote ${trivy_secret_json} and ${trivy_misconfig_txt}"
echo "image-scan-trivy ok"
