#!/usr/bin/env bash
# Lint GitHub Actions workflow files with actionlint.
# Used by GitHub Actions and local make/pre-commit workflows.
set -euo pipefail
# shellcheck source=ci-utils.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/ci-utils.sh"

if [[ -n "${PRE_COMMIT:-}" ]]; then
	ci_log_init "pre-commit-actions"
else
	ci_log_init "lint-actions"
fi

require_cmd actionlint
actionlint .github/workflows/*.yml
echo "actions lint ok"