#!/bin/sh
# Reproduce the v1.1.9 CPU0 stall under QEMU.
# Usage: ./run-baseline.sh /path/to/awg_proxy.ko
#
# Exit 0 = stall predicate matched in console log (bug reproduced).
# Exit 1 = no stall observed within timeout (bug NOT reproduced).
# Exit 2+ = setup/build/QEMU error before predicate could be evaluated.

set -e
. "$(cd "$(dirname "$0")" && pwd)/lib/runner-common.sh"
set +e

KO="${1:?usage: $0 /path/to/awg_proxy.ko}"
STAMP="$(date +%Y%m%d-%H%M%S)"
CPIO="$LOG_DIR/initramfs-baseline-$STAMP.cpio.gz"
CONSOLE="$LOG_DIR/console-baseline-$STAMP.log"

build_initramfs_with_ko "$KO" "$CPIO" || exit $?
run_qemu "$CPIO" "$CONSOLE" 90
# QEMU may exit 124 (timeout), 0 (poweroff), or non-zero (panic-no-reboot kill).
# We don't gate on qemu's exit code; we gate on console output.

echo
echo "=== baseline verdict ==="
if check_stall "$CONSOLE"; then
    echo "STALL DETECTED (baseline OK: bug reproduced)"
    grep -nE "$STALL_REGEX" "$CONSOLE" | head -5
    exit 0
fi
echo "no stall detected within 90s"
if ! check_repro_ran "$CONSOLE"; then
    echo "WARN: repro loop never executed (init/insmod may have failed)"
    tail -30 "$CONSOLE"
    exit 3
fi
exit 1
