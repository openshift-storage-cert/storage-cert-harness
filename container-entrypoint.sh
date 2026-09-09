#!/bin/sh
set -eu

runtime_dir="$(mktemp -d "${TMPDIR:-/tmp}/storage-cert-harness.XXXXXX")"
home_dir="${runtime_dir}/home"
mkdir -p "${home_dir}/.config" "${home_dir}/.local/share/containers"
trap 'rm -rf "${runtime_dir}"' EXIT HUP INT TERM
cp -R /opt/virtbench-runtime/. "${runtime_dir}/"
chmod -R u+rwX "${runtime_dir}"
export VIRTBENCH_REPO="${runtime_dir}"
export HOME="${home_dir}"
export XDG_CONFIG_HOME="${home_dir}/.config"
export XDG_DATA_HOME="${home_dir}/.local/share"

exec /usr/bin/harness "$@"