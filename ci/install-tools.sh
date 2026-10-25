#!/usr/bin/env bash
# Wrapper for developer CI tooling: ./ci/install-tools.sh  (same as make install-tools)
exec "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/scripts/install-tools.sh" "$@"
