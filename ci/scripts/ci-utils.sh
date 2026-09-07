#!/usr/bin/env bash
# Shared helpers for ci/scripts/*.sh. Source this file; do not execute it.
#
# Offline Go contract: GOPROXY=off and -mod=vendor (ECOPROJECT-5274 / ADR-0010).

set -euo pipefail

CI_SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CI_CONFIG_DIR="$(cd "${CI_SCRIPTS_DIR}/../config" && pwd)"
REPO_ROOT="$(cd "${CI_SCRIPTS_DIR}/../.." && pwd)"
export CI_SCRIPTS_DIR CI_CONFIG_DIR REPO_ROOT
cd "${REPO_ROOT}"

# shellcheck source=images.sh
source "${CI_SCRIPTS_DIR}/images.sh"

export GOPROXY="${GOPROXY:-off}"
export GOFLAGS="${GOFLAGS:--mod=vendor}"

IMAGE_TAR="${IMAGE_TAR:-${REPO_ROOT}/dist/harness-image.tar}"
LOG_DIR="${REPO_ROOT}/logs"
mkdir -p "${LOG_DIR}"

die() {
	echo "error: $*" >&2
	exit 1
}

require_cmd() {
	command -v "$1" >/dev/null 2>&1 || die "required command not found: $1 (see docs/ci.md)"
}

git_sha() {
	local sha="${CI_COMMIT_SHA:-${GITHUB_SHA:-}}"
	if [[ -n "${sha}" ]]; then
		echo "${sha:0:12}"
		return
	fi
	git rev-parse --short=12 HEAD 2>/dev/null || echo "dev"
}

image_ref() {
	echo "${QUAY_IMAGE}:$(image_version)"
}

is_main_push() {
	if [[ -n "${CI_COMMIT_BRANCH:-}" && "${CI_COMMIT_BRANCH}" == "main" && "${CI_PIPELINE_SOURCE:-}" == "push" ]]; then
		return 0
	fi
	if [[ "${GITHUB_REF:-}" == "refs/heads/main" && "${GITHUB_EVENT_NAME:-}" == "push" ]]; then
		return 0
	fi
	return 1
}

is_version_bump_push() {
	if is_main_push; then
		return 0
	fi
	if [[ "${CI_COMMIT_BRANCH:-}" == "test-ci" && "${CI_PIPELINE_SOURCE:-}" == "push" ]]; then
		return 0
	fi
	if [[ "${GITHUB_REF:-}" == "refs/heads/test-ci" && "${GITHUB_EVENT_NAME:-}" == "push" ]]; then
		return 0
	fi
	if [[ "${GITHUB_REF:-}" == "refs/heads/main" || "${GITHUB_REF:-}" == "refs/heads/test-ci" ]] &&
		[[ "${GITHUB_EVENT_NAME:-}" == "workflow_dispatch" ]]; then
		return 0
	fi
	return 1
}

binary_version() {
	if [[ -n "${HARNESS_VERSION:-}" ]]; then
		echo "${HARNESS_VERSION}"
		return
	fi
	local next
	next="$(tr -d '[:space:]' < "${REPO_ROOT}/NEXT-VERSION")"
	if is_main_push; then
		echo "${next}"
	else
		echo "${next}-$(git_sha)"
	fi
}

image_version() {
	if [[ -n "${HARNESS_IMAGE_VERSION:-}" ]]; then
		echo "${HARNESS_IMAGE_VERSION}"
		return
	fi
	local next
	next="$(tr -d '[:space:]' < "${REPO_ROOT}/NEXT-IMAGE-VERSION")"
	if is_main_push; then
		echo "${next}"
	else
		echo "${next}-$(git_sha)"
	fi
}

# Tee stdout/stderr to logs/<name>.log for the rest of the process.
_ci_log_tee=0
ci_log_init() {
	local name="$1"
	local log="${LOG_DIR}/${name}.log"
	if [[ -n "${LOG_TS:-}" ]]; then
		log="${LOG_DIR}/${name}-$(date -u +%Y%m%dT%H%M%SZ).log"
	fi
	: >"${log}"
	# GitLab k8s executor packs artifacts as soon as the script exits and does
	# not wait for process-substitution tee. Close FDs on EXIT so the log is complete.
	exec > >(tee -a "${log}") 2>&1
	_ci_log_tee=1
	echo "==> ${name}  $(date -u +%Y-%m-%dT%H:%M:%SZ)  sha=$(git_sha)"
}

_ci_log_flush() {
	if [[ "${_ci_log_tee:-0}" -eq 1 ]]; then
		exec 1>&- 2>&- || true
		sleep 0.2 || true
		_ci_log_tee=0
	fi
}
trap '_ci_log_flush' EXIT

merge_base() {
	if [[ -n "${CI_MERGE_REQUEST_DIFF_BASE_SHA:-}" ]]; then
		echo "${CI_MERGE_REQUEST_DIFF_BASE_SHA}"
		return
	fi
	if git rev-parse --verify origin/main >/dev/null 2>&1; then
		if base="$(git merge-base HEAD origin/main 2>/dev/null)"; then
			echo "${base}"
			return
		fi
	fi
	if git rev-parse --verify main >/dev/null 2>&1; then
		if base="$(git merge-base HEAD main 2>/dev/null)"; then
			echo "${base}"
			return
		fi
	fi
	# No origin/main (initial commit, shallow clone): parent of HEAD, or HEAD itself.
	if git rev-parse --verify HEAD~1 >/dev/null 2>&1; then
		git rev-parse HEAD~1
		return
	fi
	git rev-parse HEAD
}

# Default container build/run tool is podman (local). Wrappers override:
# GitLab image jobs set CONTAINER_ENGINE=buildah; GitHub sets docker.
container_engine() {
	local eng="${CONTAINER_ENGINE:-podman}"
	case "${eng}" in
	podman | buildah | docker) ;;
	*) die "unsupported CONTAINER_ENGINE=${eng} (want podman, buildah, or docker)" ;;
	esac
	command -v "${eng}" >/dev/null 2>&1 || die "required command not found: ${eng} (default is podman; see docs/ci.md / ./ci/install-tools.sh)"
	echo "${eng}"
}
