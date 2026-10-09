#!/usr/bin/env bash
# Remove build outputs under the same lock as image build and validation.
set -euo pipefail
# shellcheck source=ci-utils.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/ci-utils.sh"
acquire_local_image_lock
shopt -s nullglob
rm -rf bin dist coverage.out logs/*.log logs/*.json
echo "clean ok"
