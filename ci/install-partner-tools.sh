#!/usr/bin/env bash
# Wrapper so partners can run: ./ci/install-partner-tools.sh
exec "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/scripts/install-partner-tools.sh" "$@"
