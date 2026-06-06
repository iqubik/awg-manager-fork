#!/bin/sh
# Shared helpers for run-baseline.sh / run-fixed.sh.
# Sourced from host scripts. Not standalone.

# Resolve worktree paths relative to the script that sources this file.
QEMU_DIR="$(cd "$(dirname "$0")" && pwd)"
INITRAMFS_DIR="$QEMU_DIR/initramfs"
KERNEL="$QEMU_DIR/kernel/vmlinux"
LOG_DIR="$QEMU_DIR/logs"
BUILD_DIR="$QEMU_DIR/build"

mkdir -p "$LOG_DIR"

# Predicates (must match the strings emitted by Linux 4.9 watchdog/oops):
STALL_REGEX='BUG: soft lockup|watchdog: BUG: soft lockup|INFO: rcu_sched self-detected stall|RIP:|epc:|Kernel panic - not syncing|Oops\['

build_initramfs_with_ko() {
    KO_SRC="$1"
    OUT="$2"
    if [ ! -f "$KO_SRC" ]; then
        echo "ERR: .ko not found: $KO_SRC" >&2
        return 2
    fi
    cp "$KO_SRC" "$INITRAMFS_DIR/lib/modules/4.9.337/awg_proxy.ko" || return 3
    ( cd "$INITRAMFS_DIR" && find . | cpio -o -H newc 2>/dev/null | gzip -9 > "$OUT" ) || return 4
    echo "[runner] initramfs built: $OUT ($(du -h "$OUT" | cut -f1))"
    return 0
}

run_qemu() {
    CPIO="$1"
    OUTLOG="$2"
    TIMEOUT="${3:-90}"

    if [ ! -f "$KERNEL" ]; then
        echo "ERR: kernel not found at $KERNEL" >&2
        return 10
    fi

    echo "[runner] launching QEMU (timeout=${TIMEOUT}s)"
    # -no-reboot + panic=10 + oops=panic: turn any oops into kernel panic, halt instead of reboot.
    # softlockup_panic=1 explicitly forces panic on lockup.
    timeout --foreground -k 5 "$TIMEOUT" qemu-system-mipsel \
        -M malta \
        -cpu 24Kf \
        -m 128 \
        -kernel "$KERNEL" \
        -initrd "$CPIO" \
        -append "console=ttyS0 root=/dev/ram init=/init panic=5 oops=panic softlockup_panic=1 loglevel=8" \
        -nographic \
        -serial mon:stdio \
        -monitor none \
        -no-reboot \
        -smp 1 \
        -nodefaults \
        -vga none \
        > "$OUTLOG" 2>&1
    QRC=$?
    echo "[runner] qemu exit=$QRC, log=$OUTLOG"
    return $QRC
}

check_stall() {
    LOG="$1"
    if grep -E -q "$STALL_REGEX" "$LOG"; then
        return 0   # stall found
    fi
    return 1
}

check_repro_ran() {
    LOG="$1"
    # Did the userspace repro at least start the churn loop?
    grep -q "\[repro\] add/del churn 500x" "$LOG"
}
