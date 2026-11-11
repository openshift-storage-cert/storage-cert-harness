#!/usr/bin/env bash
# Cluster-free checks for FAR host-file validation and the preflight mount.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=../partner-harness-common.sh
source "${repo_root}/ci/scripts/partner-harness-common.sh"

tmp="$(mktemp -d /tmp/opencode/partner-far-config.XXXXXXXX)"
trap 'rm -rf -- "${tmp}"' EXIT
cp "${repo_root}/examples/far-config.example.json" "${tmp}/far-config.json"
printf 'kubeconfig\n' > "${tmp}/kubeconfig"
cat > "${tmp}/plan.yaml" <<'YAML'
select:
  ids:
    - TR-VIRT-007
overrides:
  TR-VIRT-007:
    mode: far-operator
    node: worker-0.example.com
    far_config: /far-config.json
YAML

FAR_CONFIG="${tmp}/far-config.json"
partner_validate_far_config "${tmp}/plan.yaml" "${repo_root}"
[[ "${PARTNER_FAR_MOUNT}" == "${FAR_CONFIG}:/far-config.json:ro,Z" ]]

# Capture Podman arguments without pulling images or contacting a cluster.
podman() { printf '%s\n' "$@" > "${tmp}/podman-args"; }
REPO_ROOT="${repo_root}"
partner_run_harness_container image "${tmp}/kubeconfig" "${repo_root}/backends.example.yaml" example-array preflight --plan /plans/level3-customer-no-load-80.yaml
grep -Fxq "${PARTNER_FAR_MOUNT}" "${tmp}/podman-args"

# With no FAR execution selected, even a missing FAR_CONFIG must not be required.
sed 's/TR-VIRT-007/TR-STOR-002/g' "${tmp}/plan.yaml" > "${tmp}/no-far.yaml"
FAR_CONFIG="${tmp}/does-not-exist.json"
partner_validate_far_config "${tmp}/no-far.yaml" "${repo_root}"
[[ -z "${PARTNER_FAR_MOUNT}" ]]
partner_run_harness_container image "${tmp}/kubeconfig" "${repo_root}/backends.example.yaml" example-array preflight --plan /plans/level3-customer-load-80.yaml
! grep -Fq ':/far-config.json:ro,Z' "${tmp}/podman-args"

# Monitor-only mode does not need a FAR manifest.
sed 's/far-operator/monitor/' "${tmp}/plan.yaml" > "${tmp}/monitor.yaml"
partner_validate_far_config "${tmp}/monitor.yaml" "${repo_root}"
[[ -z "${PARTNER_FAR_MOUNT}" ]]

for bad in "${tmp}/does-not-exist.json" "${repo_root}/examples/far-config.example.json"; do
	FAR_CONFIG="${bad}"
	if partner_validate_far_config "${tmp}/plan.yaml" "${repo_root}" >"${tmp}/error" 2>&1; then
		echo "expected FAR_CONFIG rejection: ${bad}" >&2
		exit 1
	fi
	[[ -z "${PARTNER_FAR_MOUNT}" ]]
done

FAR_CONFIG="${tmp}/far-config.json"
sed 's@/far-config.json@/other-config.json@' "${tmp}/plan.yaml" > "${tmp}/wrong-path.yaml"
if partner_validate_far_config "${tmp}/wrong-path.yaml" "${repo_root}" >"${tmp}/error" 2>&1; then
	echo 'expected container-path mismatch rejection' >&2
	exit 1
fi
sed 's/worker-0.example.com/worker-1.example.com/' "${tmp}/plan.yaml" > "${tmp}/wrong-node.yaml"
if partner_validate_far_config "${tmp}/wrong-node.yaml" "${repo_root}" >"${tmp}/error" 2>&1; then
	echo 'expected FAR target mismatch rejection' >&2
	exit 1
fi

sed 's/"fence-agents-credentials-shared"/""/' "${tmp}/far-config.json" > "${tmp}/missing-secret.json"
FAR_CONFIG="${tmp}/missing-secret.json"
if partner_validate_far_config "${tmp}/plan.yaml" "${repo_root}" >"${tmp}/error" 2>&1; then
	echo 'expected missing Secret reference rejection' >&2
	exit 1
fi

sed 's/"worker-0.example.com": "192.0.2.10"/"worker-1.example.com": "192.0.2.10"/' "${tmp}/far-config.json" > "${tmp}/wrong-ip-key.json"
FAR_CONFIG="${tmp}/wrong-ip-key.json"
if partner_validate_far_config "${tmp}/plan.yaml" "${repo_root}" >"${tmp}/error" 2>&1; then
	echo 'expected missing target BMC address rejection' >&2
	exit 1
fi
echo 'partner FAR mount checks passed'
