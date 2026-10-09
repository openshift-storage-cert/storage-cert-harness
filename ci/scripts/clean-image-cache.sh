#!/usr/bin/env bash
# Remove local harness image artifacts and prune the active container engine cache.
# Use before image-build when Containerfile cleanup steps appear to have no effect
# (stale cached layers). Pair with IMAGE_BUILD_NO_CACHE=1 or make image-build-fresh.
set -euo pipefail
# shellcheck source=ci-utils.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/ci-utils.sh"
acquire_local_image_lock
clean_local_image_cache
echo "clean-image-cache ok"
