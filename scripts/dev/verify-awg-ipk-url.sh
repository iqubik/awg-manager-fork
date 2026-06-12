#!/usr/bin/env bash
set -euo pipefail

usage() {
    echo "Usage: $0 <entware-arch> <release-base-url>" >&2
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

ENTWARE_ARCH="${1:-}"
RELEASE_BASE_URL_ARG="${2:-}"

if [[ $# -ne 2 || -z "$ENTWARE_ARCH" || -z "$RELEASE_BASE_URL_ARG" ]]; then
    usage
    exit 1
fi

cd "$PROJECT_ROOT"

ipk="$(ls -t dist/awg-manager_*_"$ENTWARE_ARCH"-kn.ipk 2>/dev/null | sed -n '1p')"
if [[ -z "$ipk" || ! -f "$ipk" ]]; then
    echo "ERROR: no awg-manager IPK found for $ENTWARE_ARCH" >&2
    exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

cp "$ipk" "$tmp/package.ipk"
cd "$tmp"
tar -xzf package.ipk
data_tar="$(ls data.tar.* | sed -n '1p')"
if [[ -z "$data_tar" ]]; then
    echo "ERROR: no data.tar.* found inside IPK" >&2
    exit 1
fi

mkdir data-root
tar -xzf "$data_tar" -C data-root
bin="data-root/opt/bin/awg-manager"
test -s "$bin"
grep -aF "$RELEASE_BASE_URL_ARG" "$bin" >/dev/null

echo "AWG release URL verified inside $ipk"
