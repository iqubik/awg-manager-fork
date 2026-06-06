#!/bin/sh
# Verify v1.1.10 fix: same workload, no stall.
# Usage: ./run-fixed.sh /path/to/awg_proxy.ko
#
# Exit 0 = no stall (fix verified).
# Exit 1 = stall still present (fix did NOT take).
# Exit 2+ = setup/build/QEMU error.

set -e
. "$(cd "$(dirname "$0")" && pwd)/lib/runner-common.sh"
set +e

KO="${1:?usage: $0 /path/to/awg_proxy.ko}"
STAMP="$(date +%Y%m%d-%H%M%S)"
CPIO="$LOG_DIR/initramfs-fixed-$STAMP.cpio.gz"
CONSOLE="$LOG_DIR/console-fixed-$STAMP.log"

build_initramfs_with_ko "$KO" "$CPIO" || exit $?
run_qemu "$CPIO" "$CONSOLE" 90

echo
echo "=== fixed verdict ==="
if ! check_repro_ran "$CONSOLE"; then
    echo "ERR: repro never ran; cannot validate fix"
    tail -30 "$CONSOLE"
    exit 3
fi
if check_stall "$CONSOLE"; then
    echo "STALL STILL DETECTED (fix did NOT take)"
    grep -nE "$STALL_REGEX" "$CONSOLE" | head -5
    exit 1
fi
if grep -q "NO_STALL_DETECTED" "$CONSOLE"; then
    echo "no stall observed; init reached clean shutdown (fix verified)"
    exit 0
fi
echo "no stall observed but init did not reach clean exit; review log:"
tail -30 "$CONSOLE"
exit 1
