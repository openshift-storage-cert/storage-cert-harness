#!/usr/bin/env bash
# Load ci/images.env and derive Trivy/Dive refs. Source this file; do not execute it.
# install-tools.sh sources this instead of lib.sh (lib.sh sets GOPROXY=off).

_ci_images_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../config" && pwd)"

_ci_images_trim() {
	local s="$1"
	s="${s#"${s%%[![:space:]]*}"}"
	s="${s%"${s##*[![:space:]]}"}"
	printf '%s' "${s}"
}

# Fill unset keys from images.env. Does not override an already-exported pin.
_ci_images_load_if_unset() {
	local f="${_ci_images_dir}/images.env"
	local line key val
	[[ -f "${f}" ]] || {
		echo "error: missing ${f}" >&2
		return 1
	}
	while IFS= read -r line || [[ -n "${line}" ]]; do
		line="${line%%#*}"
		line="$(_ci_images_trim "${line}")"
		[[ -z "${line}" ]] && continue
		key="${line%%=*}"
		val="${line#*=}"
		key="$(_ci_images_trim "${key}")"
		val="$(_ci_images_trim "${val}")"
		[[ -z "${key}" ]] && continue
		if [[ -n "${!key+x}" ]]; then
			continue
		fi
		export "${key}=${val}"
	done <"${f}"
}

_ci_images_derive() {
	export TRIVY_TAG="${TRIVY_TAG:-trivy-${TRIVY_VERSION#v}}"
	export DIVE_TAG="${DIVE_TAG:-dive-${DIVE_VERSION}}"
	export TRIVY_SRC="${TRIVY_SRC:-docker.io/aquasec/trivy:${TRIVY_VERSION#v}}"
	export DIVE_SRC="${DIVE_SRC:-docker.io/wagoodman/dive:${DIVE_VERSION}}"
	export TRIVY_IMAGE="${TRIVY_IMAGE:-${CI_TOOLS_IMAGE}:${TRIVY_TAG}}"
	export DIVE_IMAGE="${DIVE_IMAGE:-${CI_TOOLS_IMAGE}:${DIVE_TAG}}"
}

if [[ "${CI_IMAGES_FORCE_FILE:-0}" == "1" ]]; then
	set -a
	# shellcheck disable=SC1091
	source "${_ci_images_dir}/images.env"
	set +a
else
	_ci_images_load_if_unset
fi
_ci_images_derive
