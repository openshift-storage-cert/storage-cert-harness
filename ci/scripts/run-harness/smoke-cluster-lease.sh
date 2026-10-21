#!/usr/bin/env bash
# Kubernetes Coordination Lease for harness smoke (edge-112 / shared lab).
# Source after ci-utils.sh; do not execute directly.
#
# Design: docs/design/storage-cert-harness-ci-smoke-test.md §2
set -euo pipefail

# Exit code when another run holds the lease (distinct from harness failure).
SMOKE_CLUSTER_LEASE_EXIT_BUSY="${SMOKE_CLUSTER_LEASE_EXIT_BUSY:-2}"

SMOKE_CLUSTER_LEASE_NAMESPACE="${SMOKE_CLUSTER_LEASE_NAMESPACE:-storage-cert-ci}"
SMOKE_CLUSTER_LEASE_NAME="${SMOKE_CLUSTER_LEASE_NAME:-storage-cert-harness-smoke}"
SMOKE_CLUSTER_LEASE_LABEL_KEY="${SMOKE_CLUSTER_LEASE_LABEL_KEY:-storage-cert-harness/ci-smoke}"
SMOKE_CLUSTER_LEASE_ANNOT_PREFIX="${SMOKE_CLUSTER_LEASE_ANNOT_PREFIX:-storage-cert-harness/}"

# Job timeout + 15m slack (default ≈ 240m + 15m).
SMOKE_CLUSTER_LEASE_DURATION_SECONDS="${SMOKE_CLUSTER_LEASE_DURATION_SECONDS:-15300}"
SMOKE_CLUSTER_LEASE_RENEW_INTERVAL_SECONDS="${SMOKE_CLUSTER_LEASE_RENEW_INTERVAL_SECONDS:-300}"

SMOKE_CLUSTER_LEASE_HELD="${SMOKE_CLUSTER_LEASE_HELD:-0}"
SMOKE_CLUSTER_LEASE_HOLDER_IDENTITY="${SMOKE_CLUSTER_LEASE_HOLDER_IDENTITY:-}"
_smoke_cluster_lease_renew_pid=""

smoke_cluster_lease_die() {
	echo "error: $*" >&2
	exit 1
}

smoke_cluster_lease_busy() {
	echo "error: cluster smoke lease held by run ${1:-<unknown>}" >&2
	smoke_cluster_lease_log_holder_annotations || true
	exit "${SMOKE_CLUSTER_LEASE_EXIT_BUSY}"
}

smoke_cluster_lease_oc_args() {
	local -n _out=$1
	_out=()
	if [[ -n "${SMOKE_CLUSTER_LEASE_KUBECONFIG:-}" ]]; then
		_out+=(--kubeconfig "${SMOKE_CLUSTER_LEASE_KUBECONFIG}")
	elif [[ -n "${KUBECONFIG:-}" ]]; then
		_out+=(--kubeconfig "${KUBECONFIG}")
	fi
	if [[ "${KUBECTL_INSECURE_SKIP_TLS_VERIFY:-false}" == "true" ]]; then
		_out+=(--insecure-skip-tls-verify=true)
	fi
}

smoke_cluster_lease_run_id() {
	if [[ -n "${GITHUB_RUN_ID:-}" ]]; then
		echo "${GITHUB_RUN_ID}"
		return
	fi
	if [[ -n "${CI_PIPELINE_ID:-}" ]]; then
		echo "${CI_PIPELINE_ID}"
		return
	fi
	if [[ -n "${HARNESS_SMOKE_LEASE_RUN_ID:-}" ]]; then
		echo "${HARNESS_SMOKE_LEASE_RUN_ID}"
		return
	fi
	echo "local-$(whoami)-$(date +%s)"
}

smoke_cluster_lease_rfc3339_now() {
	date -u +%Y-%m-%dT%H:%M:%SZ
}

# metav1.Time on the cluster API expects fractional seconds (…T%H:%M:%S.000000Z).
smoke_cluster_lease_api_time_now() {
	date -u +%Y-%m-%dT%H:%M:%S.000000Z
}

smoke_cluster_lease_template_path() {
	echo "${REPO_ROOT}/ci/config/smoke/cluster-smoke-lease.yaml.template"
}

smoke_cluster_lease_render_template() {
	local run_id="$1"
	local template now
	template="$(smoke_cluster_lease_template_path)"
	[[ -f "${template}" ]] || smoke_cluster_lease_die "smoke lease: missing template ${template}"
	now="$(smoke_cluster_lease_api_time_now)"
	sed \
		-e "s|@LEASE_NAME@|${SMOKE_CLUSTER_LEASE_NAME}|g" \
		-e "s|@LEASE_NAMESPACE@|${SMOKE_CLUSTER_LEASE_NAMESPACE}|g" \
		-e "s|@HOLDER_IDENTITY@|${run_id}|g" \
		-e "s|@LEASE_DURATION@|${SMOKE_CLUSTER_LEASE_DURATION_SECONDS}|g" \
		-e "s|@ACQUIRE_TIME@|${now}|g" \
		-e "s|@RENEW_TIME@|${now}|g" \
		"${template}"
}

smoke_cluster_lease_api_server_host() {
	local oc_args=()
	smoke_cluster_lease_oc_args oc_args
	local server
	server="$(oc "${oc_args[@]}" whoami --show-server 2>/dev/null || true)"
	server="${server#https://}"
	server="${server#http://}"
	server="${server%%/*}"
	echo "${server}"
}

smoke_cluster_lease_workflow_url() {
	if [[ -n "${GITHUB_SERVER_URL:-}" && -n "${GITHUB_REPOSITORY:-}" && -n "${GITHUB_RUN_ID:-}" ]]; then
		echo "${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}"
		return
	fi
	if [[ -n "${CI_PIPELINE_URL:-}" ]]; then
		echo "${CI_PIPELINE_URL}"
		return
	fi
	echo ""
}

smoke_cluster_lease_runner_host() {
	local host=""
	host="$(hostname -f 2>/dev/null || true)"
	if [[ -z "${host}" ]]; then
		host="$(hostname 2>/dev/null || true)"
	fi
	echo "${host}"
}

# Metadata keys match design §2; bootstrap-host mirrors virtbench ssh-pod annotations.
smoke_cluster_lease_collect_annotations() {
	local run_id="$1"
	local plan_name="${2:-harness-smoke}"
	local plan_file="${3:-plans/harness-smoke-plan.yaml}"
	local -n _anns=$4
	_anns=()
	_anns+=("${SMOKE_CLUSTER_LEASE_ANNOT_PREFIX}run-id=${run_id}")
	_anns+=("${SMOKE_CLUSTER_LEASE_ANNOT_PREFIX}plan=${plan_name}")
	_anns+=("${SMOKE_CLUSTER_LEASE_ANNOT_PREFIX}plan-file=${plan_file}")
	_anns+=("${SMOKE_CLUSTER_LEASE_ANNOT_PREFIX}git-sha=$(git_sha)")
	_anns+=("${SMOKE_CLUSTER_LEASE_ANNOT_PREFIX}started-at=$(smoke_cluster_lease_rfc3339_now)")

	local pr="${LIVE_SMOKE_PR_NUMBER:-${GITHUB_PR_NUMBER:-${CI_MERGE_REQUEST_IID:-}}}"
	if [[ -n "${pr}" ]]; then
		_anns+=("${SMOKE_CLUSTER_LEASE_ANNOT_PREFIX}pr-number=${pr}")
	fi

	local wf_url
	wf_url="$(smoke_cluster_lease_workflow_url)"
	if [[ -n "${wf_url}" ]]; then
		_anns+=("${SMOKE_CLUSTER_LEASE_ANNOT_PREFIX}workflow-url=${wf_url}")
	fi

	local runner
	runner="$(smoke_cluster_lease_runner_host)"
	if [[ -n "${runner}" ]]; then
		_anns+=("${SMOKE_CLUSTER_LEASE_ANNOT_PREFIX}runner=${runner}")
		_anns+=("${SMOKE_CLUSTER_LEASE_ANNOT_PREFIX}bootstrap-host=${runner}")
	fi

	local api_host
	api_host="$(smoke_cluster_lease_api_server_host)"
	if [[ -n "${api_host}" ]]; then
		_anns+=("${SMOKE_CLUSTER_LEASE_ANNOT_PREFIX}api-server=${api_host}")
	fi
}

smoke_cluster_lease_ensure_namespace() {
	local oc_args=()
	smoke_cluster_lease_oc_args oc_args
	oc "${oc_args[@]}" get namespace "${SMOKE_CLUSTER_LEASE_NAMESPACE}" >/dev/null 2>&1 ||
		oc "${oc_args[@]}" create namespace "${SMOKE_CLUSTER_LEASE_NAMESPACE}" >/dev/null
}

smoke_cluster_lease_exists() {
	local oc_args=()
	smoke_cluster_lease_oc_args oc_args
	oc "${oc_args[@]}" get lease "${SMOKE_CLUSTER_LEASE_NAME}" \
		-n "${SMOKE_CLUSTER_LEASE_NAMESPACE}" >/dev/null 2>&1
}

smoke_cluster_lease_holder() {
	local oc_args=()
	smoke_cluster_lease_oc_args oc_args
	oc "${oc_args[@]}" get lease "${SMOKE_CLUSTER_LEASE_NAME}" \
		-n "${SMOKE_CLUSTER_LEASE_NAMESPACE}" \
		-o jsonpath='{.spec.holderIdentity}' 2>/dev/null || true
}

smoke_cluster_lease_mark_held_and_refresh() {
	local run_id="$1"
	local plan_name="${2:-harness-smoke}"
	local plan_file="${3:-plans/harness-smoke-plan.yaml}"
	export SMOKE_CLUSTER_LEASE_HELD=1
	smoke_cluster_lease_renew_heartbeat
	smoke_cluster_lease_apply_metadata "${run_id}" "${plan_name}" "${plan_file}"
}

smoke_cluster_lease_log_holder_annotations() {
	local oc_args=()
	smoke_cluster_lease_oc_args oc_args
	if ! oc "${oc_args[@]}" get lease "${SMOKE_CLUSTER_LEASE_NAME}" \
		-n "${SMOKE_CLUSTER_LEASE_NAMESPACE}" >/dev/null 2>&1; then
		return 0
	fi
	echo "smoke lease: current lease object annotations:" >&2
	oc "${oc_args[@]}" get lease "${SMOKE_CLUSTER_LEASE_NAME}" \
		-n "${SMOKE_CLUSTER_LEASE_NAMESPACE}" \
		-o 'go-template={{range $k, $v := .metadata.annotations}}{{$k}}={{$v}}{{"\n"}}{{end}}' 2>/dev/null |
		while IFS= read -r line; do
			[[ -n "${line}" ]] && echo "  ${line}" >&2
		done || true
}

smoke_cluster_lease_apply_metadata() {
	local run_id="$1"
	local plan_name="${2:-harness-smoke}"
	local plan_file="${3:-plans/harness-smoke-plan.yaml}"
	local oc_args=()
	smoke_cluster_lease_oc_args oc_args
	local anns=()
	smoke_cluster_lease_collect_annotations "${run_id}" "${plan_name}" "${plan_file}" anns

	oc "${oc_args[@]}" label lease "${SMOKE_CLUSTER_LEASE_NAME}" \
		-n "${SMOKE_CLUSTER_LEASE_NAMESPACE}" \
		"app.kubernetes.io/managed-by=storage-cert-harness" \
		"${SMOKE_CLUSTER_LEASE_LABEL_KEY}=${run_id}" \
		--overwrite >/dev/null

	# shellcheck disable=SC2068
	oc "${oc_args[@]}" annotate lease "${SMOKE_CLUSTER_LEASE_NAME}" \
		-n "${SMOKE_CLUSTER_LEASE_NAMESPACE}" \
		--overwrite "${anns[@]}" >/dev/null
}

smoke_cluster_lease_renew_heartbeat() {
	[[ "${SMOKE_CLUSTER_LEASE_HELD:-0}" == "1" ]] || return 0
	local oc_args=()
	smoke_cluster_lease_oc_args oc_args
	local now holder
	now="$(smoke_cluster_lease_api_time_now)"
	holder="${SMOKE_CLUSTER_LEASE_HOLDER_IDENTITY}"
	[[ -n "${holder}" ]] || smoke_cluster_lease_die "smoke lease renew: holder identity is unset"

	oc "${oc_args[@]}" patch lease "${SMOKE_CLUSTER_LEASE_NAME}" \
		-n "${SMOKE_CLUSTER_LEASE_NAMESPACE}" \
		--type=merge \
		-p "{\"spec\":{\"holderIdentity\":\"${holder}\",\"leaseDurationSeconds\":${SMOKE_CLUSTER_LEASE_DURATION_SECONDS},\"renewTime\":\"${now}\"}}" \
		>/dev/null
}

smoke_cluster_lease_create() {
	local run_id="$1"
	local plan_name="${2:-harness-smoke}"
	local plan_file="${3:-plans/harness-smoke-plan.yaml}"
	local oc_args=()
	smoke_cluster_lease_oc_args oc_args

	# ci-smoke label (key contains "/") is applied via oc label in apply_metadata.
	local manifest create_err holder
	manifest="$(smoke_cluster_lease_render_template "${run_id}")"
	local create_status=0
	set +e
	create_err="$(printf '%s\n' "${manifest}" | oc "${oc_args[@]}" create -f - 2>&1)"
	create_status=$?
	set -e
	if [[ "${create_status}" -eq 0 ]]; then
		smoke_cluster_lease_mark_held_and_refresh "${run_id}" "${plan_name}" "${plan_file}"
		return 0
	fi

	if smoke_cluster_lease_exists; then
		holder="$(smoke_cluster_lease_holder)"
		if [[ -z "${holder}" ]]; then
			smoke_cluster_lease_die "smoke lease: lease exists but holderIdentity is empty"
		fi
		if [[ "${holder}" != "${run_id}" ]]; then
			smoke_cluster_lease_busy "${holder}"
		fi
		smoke_cluster_lease_mark_held_and_refresh "${run_id}" "${plan_name}" "${plan_file}"
		return 0
	fi

	if [[ -n "${create_err}" ]]; then
		smoke_cluster_lease_die "smoke lease: create failed: ${create_err}"
	fi
	smoke_cluster_lease_die "smoke lease: create failed (unknown error)"
}

smoke_cluster_lease_acquire() {
	local plan_name="${1:-harness-smoke}"
	local plan_file="${2:-plans/harness-smoke-plan.yaml}"

	if [[ "${HARNESS_SMOKE_SKIP_CLUSTER_LEASE:-false}" == "true" ]]; then
		echo "smoke lease: skipped (HARNESS_SMOKE_SKIP_CLUSTER_LEASE=true)"
		return 0
	fi

	command -v oc >/dev/null 2>&1 || smoke_cluster_lease_die "oc is required for cluster smoke lease"

	local run_id
	run_id="$(smoke_cluster_lease_run_id)"
	export SMOKE_CLUSTER_LEASE_HOLDER_IDENTITY="${run_id}"

	echo "smoke lease: acquiring ${SMOKE_CLUSTER_LEASE_NAME} in ${SMOKE_CLUSTER_LEASE_NAMESPACE} (holder=${run_id})"
	smoke_cluster_lease_ensure_namespace

	if smoke_cluster_lease_exists; then
		local holder=""
		holder="$(smoke_cluster_lease_holder)"
		if [[ -n "${holder}" && "${holder}" != "${run_id}" ]]; then
			smoke_cluster_lease_busy "${holder}"
		fi
		smoke_cluster_lease_mark_held_and_refresh "${run_id}" "${plan_name}" "${plan_file}"
	else
		smoke_cluster_lease_create "${run_id}" "${plan_name}" "${plan_file}"
	fi

	echo "smoke lease: acquired (renew every ${SMOKE_CLUSTER_LEASE_RENEW_INTERVAL_SECONDS}s)"
}

smoke_cluster_lease_release() {
	if [[ "${SMOKE_CLUSTER_LEASE_HELD:-0}" != "1" ]]; then
		return 0
	fi

	local oc_args=()
	smoke_cluster_lease_oc_args oc_args
	local holder run_id
	run_id="${SMOKE_CLUSTER_LEASE_HOLDER_IDENTITY}"
	holder="$(oc "${oc_args[@]}" get lease "${SMOKE_CLUSTER_LEASE_NAME}" \
		-n "${SMOKE_CLUSTER_LEASE_NAMESPACE}" \
		-o jsonpath='{.spec.holderIdentity}' 2>/dev/null || true)"

	if [[ -n "${holder}" && "${holder}" != "${run_id}" ]]; then
		echo "smoke lease: not releasing ${SMOKE_CLUSTER_LEASE_NAME}; holder is ${holder} (this run=${run_id})" >&2
		export SMOKE_CLUSTER_LEASE_HELD=0
		return 0
	fi

	echo "smoke lease: releasing ${SMOKE_CLUSTER_LEASE_NAME}"
	oc "${oc_args[@]}" delete lease "${SMOKE_CLUSTER_LEASE_NAME}" \
		-n "${SMOKE_CLUSTER_LEASE_NAMESPACE}" \
		--ignore-not-found >/dev/null || true
	export SMOKE_CLUSTER_LEASE_HELD=0
}

smoke_cluster_lease_start_renewal_loop() {
	if [[ "${SMOKE_CLUSTER_LEASE_HELD:-0}" != "1" ]]; then
		return 0
	fi
	if [[ -n "${_smoke_cluster_lease_renew_pid}" ]] && kill -0 "${_smoke_cluster_lease_renew_pid}" 2>/dev/null; then
		return 0
	fi

	local guard="${REPO_ROOT:-.}/work/.smoke-cluster-lease-renew-$$"
	mkdir -p "$(dirname "${guard}")"
	: >"${guard}"

	(
		trap 'rm -f "${guard}"' EXIT
		while [[ -f "${guard}" ]]; do
			sleep "${SMOKE_CLUSTER_LEASE_RENEW_INTERVAL_SECONDS}" || exit 0
			if [[ ! -f "${guard}" ]]; then
				exit 0
			fi
			if ! smoke_cluster_lease_renew_heartbeat; then
				echo "smoke lease: heartbeat renew failed (will retry on next interval)" >&2
			fi
		done
	) &
	_smoke_cluster_lease_renew_pid=$!
	echo "smoke lease: background renewal pid=${_smoke_cluster_lease_renew_pid}"
}

smoke_cluster_lease_stop_renewal_loop() {
	local guard="${REPO_ROOT:-.}/work/.smoke-cluster-lease-renew-$$"
	rm -f "${guard}" 2>/dev/null || true
	if [[ -n "${_smoke_cluster_lease_renew_pid}" ]]; then
		kill "${_smoke_cluster_lease_renew_pid}" 2>/dev/null || true
		wait "${_smoke_cluster_lease_renew_pid}" 2>/dev/null || true
		_smoke_cluster_lease_renew_pid=""
	fi
}

smoke_cluster_lease_on_exit() {
	smoke_cluster_lease_stop_renewal_loop
	smoke_cluster_lease_release
}

# Install trap handlers that release the lease on normal exit, cancel, or kill.
# Optional first argument: extra shell snippet to run after lease cleanup (e.g. _ci_log_flush).
smoke_cluster_lease_install_traps() {
	local after="${1:-}"
	if [[ -n "${after}" ]]; then
		# shellcheck disable=SC2064
		trap "smoke_cluster_lease_on_exit; ${after}" EXIT
		# shellcheck disable=SC2064
		trap "smoke_cluster_lease_on_exit; exit 130" INT
		# shellcheck disable=SC2064
		trap "smoke_cluster_lease_on_exit; exit 143" TERM
	else
		trap 'smoke_cluster_lease_on_exit' EXIT
		trap 'smoke_cluster_lease_on_exit; exit 130' INT
		trap 'smoke_cluster_lease_on_exit; exit 143' TERM
	fi
}
