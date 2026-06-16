#!/usr/bin/env bash
set -euo pipefail

usage() {
    echo "Usage: $0 <ipk-file-name> <release-base-url>" >&2
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

IPK_NAME="${1:-}"
RELEASE_BASE_URL_ARG="${2:-}"

if [[ $# -ne 2 || -z "$IPK_NAME" || -z "$RELEASE_BASE_URL_ARG" ]]; then
    usage
    exit 1
fi

IPK_PATH="$PROJECT_ROOT/dist/$IPK_NAME"
if [[ ! -f "$IPK_PATH" ]]; then
    echo "ERROR: missing IPK: $IPK_PATH" >&2
    exit 1
fi

echo "Uploading $IPK_NAME to rax1..."
cat "$IPK_PATH" | ssh rax1 "cat > /opt/tmp/$IPK_NAME"
ssh rax1 "ls -l /opt/tmp/$IPK_NAME"

echo "Installing $IPK_NAME on rax1..."
ssh rax1 "opkg install /opt/tmp/$IPK_NAME"

echo "Restarting awg-manager on rax1..."
ssh rax1 "/opt/etc/init.d/S99awg-manager restart || true"

echo "Verifying installed /opt/bin/awg-manager on rax1..."
ssh rax1 "grep -aF '$RELEASE_BASE_URL_ARG' /opt/bin/awg-manager >/dev/null" || {
    echo "ERROR: installed /opt/bin/awg-manager does not contain fork release URL" >&2
    exit 1
}
