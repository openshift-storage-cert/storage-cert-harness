#!/usr/bin/env bash
# Unit tests only: go test ./... (package tests and golden-file parser tests).
# No cluster, no e2e, no live smoke. Offline replay-smoke is ci/replay-smoke.sh.
set -euo pipefail
# shellcheck source=ci-utils.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/ci-utils.sh"
ci_log_init "unittest"
require_cmd go
go test ./...
echo "unit tests ok"
