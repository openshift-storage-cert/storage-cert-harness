#!/usr/bin/env bash
# Set the next binary or image version. Writes NEXT-VERSION / NEXT-IMAGE-VERSION.
#
# Usage:
#   ./ci/scripts/set-next-version.sh --init 0.1.0 0.1.0   # both: binary image
#   ./ci/scripts/set-next-version.sh --init 0.1.0          # both tracks same value
#   ./ci/scripts/set-next-version.sh --binary 1.0.0
#   ./ci/scripts/set-next-version.sh --image 2.0.0
#   ./ci/scripts/set-next-version.sh --self-test
#
# After running, commit with ALLOW_VERSION_CHANGE=1:
#   ALLOW_VERSION_CHANGE=1 git add NEXT-VERSION NEXT-IMAGE-VERSION
#   ALLOW_VERSION_CHANGE=1 git commit -m "chore: set next version"
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

die() {
	echo "error: $*" >&2
	exit 1
}

valid_semver() {
	[[ "$1" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]
}

bump_patch() {
	local v="$1"
	local major minor patch
	IFS='.' read -r major minor patch <<< "${v}"
	echo "${major}.${minor}.$((patch + 1))"
}

set_binary() {
	local ver="$1"
	valid_semver "${ver}" || die "invalid semver: ${ver}"
	echo "${ver}" > "${REPO_ROOT}/NEXT-VERSION"
	echo "NEXT-VERSION = ${ver}"
}

set_image() {
	local ver="$1"
	valid_semver "${ver}" || die "invalid semver: ${ver}"
	echo "${ver}" > "${REPO_ROOT}/NEXT-IMAGE-VERSION"
	echo "NEXT-IMAGE-VERSION = ${ver}"
}

do_init() {
	local binary_ver="$1"
	local image_ver="${2:-${binary_ver}}"
	valid_semver "${binary_ver}" || die "invalid semver: ${binary_ver}"
	valid_semver "${image_ver}" || die "invalid semver: ${image_ver}"
	echo "0.0.0" > "${REPO_ROOT}/VERSION"
	echo "${binary_ver}" > "${REPO_ROOT}/NEXT-VERSION"
	echo "0.0.0" > "${REPO_ROOT}/IMAGE-VERSION"
	echo "${image_ver}" > "${REPO_ROOT}/NEXT-IMAGE-VERSION"
	echo "VERSION = 0.0.0"
	echo "NEXT-VERSION = ${binary_ver}"
	echo "IMAGE-VERSION = 0.0.0"
	echo "NEXT-IMAGE-VERSION = ${image_ver}"
}

do_self_test() {
	local tmp
	tmp="$(mktemp -d)"
	# Clean up temp dir on exit from this function
	_st_cleanup() { rm -rf "${tmp}"; }
	trap '_st_cleanup' RETURN

	echo "0.0.0" > "${tmp}/VERSION"
	echo "0.1.0" > "${tmp}/NEXT-VERSION"
	echo "0.0.0" > "${tmp}/IMAGE-VERSION"
	echo "0.1.0" > "${tmp}/NEXT-IMAGE-VERSION"

	local errs=0
	# bump_patch
	[[ "$(bump_patch 0.1.0)" == "0.1.1" ]] || { echo "FAIL: bump_patch 0.1.0"; errs=1; }
	[[ "$(bump_patch 1.2.3)" == "1.2.4" ]] || { echo "FAIL: bump_patch 1.2.3"; errs=1; }
	# valid_semver
	valid_semver "1.2.3" || { echo "FAIL: valid_semver 1.2.3"; errs=1; }
	valid_semver "0.1.0" || { echo "FAIL: valid_semver 0.1.0"; errs=1; }
	! valid_semver "v1.2.3" || { echo "FAIL: valid_semver v1.2.3 should fail"; errs=1; }
	! valid_semver "1.2" || { echo "FAIL: valid_semver 1.2 should fail"; errs=1; }
	! valid_semver "abc" || { echo "FAIL: valid_semver abc should fail"; errs=1; }

	if [[ "${errs}" -ne 0 ]]; then
		echo "set-next-version --self-test FAILED"
		return 1
	fi
	echo "set-next-version --self-test ok"
}

case "${1:-}" in
--init)
	shift
	[[ "$#" -ge 1 ]] || die "usage: set-next-version.sh --init <binary-ver> [<image-ver>]"
	do_init "$@"
	;;
--binary)
	shift
	[[ "$#" -eq 1 ]] || die "usage: set-next-version.sh --binary <version>"
	set_binary "$1"
	;;
--image)
	shift
	[[ "$#" -eq 1 ]] || die "usage: set-next-version.sh --image <version>"
	set_image "$1"
	;;
--self-test)
	do_self_test
	;;
*)
	die "usage: set-next-version.sh {--init|--binary|--image|--self-test} <args>"
	;;
esac
