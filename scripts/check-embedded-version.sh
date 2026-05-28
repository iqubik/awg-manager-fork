#!/usr/bin/env bash
# Verifies internal/singbox/installer/embedded.go RequiredVersion matches
# the expected version passed as arg.
# Usage: ./scripts/check-embedded-version.sh <expected-version>
set -euo pipefail

EXPECTED="${1:?expected version arg required}"
FILE="${EMBEDDED_GO:-internal/singbox/installer/embedded.go}"

actual="$(sed -n 's/^const RequiredVersion = "\(.*\)"/\1/p' "$FILE")"

if [[ "$actual" != "$EXPECTED" ]]; then
    echo "embedded.go RequiredVersion is '$actual', want '$EXPECTED'" >&2
    exit 1
fi
