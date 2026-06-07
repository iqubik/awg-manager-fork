#!/bin/sh
# Repro for awg_proxy v1.1.9 worker tight-loop after kernel_sock_shutdown.
# Stresses add/del slot churn under a UDP flood at listener port; on the
# real mt7628 single-core preempt-none kernel this triggers CPU0 stall in
# s2c_thread_fn / c2s_thread_fn within ~20-30s.

ADD=/proc/awg_proxy/add
DEL=/proc/awg_proxy/del
LIST=/proc/awg_proxy/list

# Slot points to local UDP echo (init.sh started nc -lu -p 5060)
SLOT='127.0.0.1:5060 H1=148-148 S1=0 S2=0 S3=0 S4=0 Jc=0'

echo "[repro] adding initial slot"
echo "$SLOT" > "$ADD" 2>&1

# Figure out the listen= port that awg_proxy chose for inbound.
# /proc/awg_proxy/list lines look like:
#   "REMOTE_IP:PORT listen=127.0.0.1:LPORT rx=N tx=N rx_pkt=N tx_pkt=N"
# We want just LPORT.
sleep 0.2
LPORT=$(sed -n 's/.*listen=[^:]*:\([0-9]\+\).*/\1/p' "$LIST" 2>/dev/null | head -1)

if [ -z "$LPORT" ]; then
    # Fallback: maybe format is different. Just dump it.
    echo "[repro] could not parse listen port; /proc/awg_proxy/list dump:"
    cat "$LIST" 2>/dev/null
    LPORT=51820
fi
echo "[repro] LPORT=$LPORT"

# Background UDP flood: a 4-byte handshake-init pattern at the listener.
# busybox sh supports /dev/udp/host/port (if compiled in). Fallback to nc.
( while :; do
    printf '\x04\x00\x00\x00' > /dev/udp/127.0.0.1/$LPORT 2>/dev/null || \
        echo -n 'X' | nc -u -w0 127.0.0.1 $LPORT 2>/dev/null
done ) &
FLOOD=$!
echo "[repro] flood pid=$FLOOD"

# Churn add/del 500x
echo "[repro] add/del churn 500x"
i=0
while [ $i -lt 500 ]; do
    echo "$SLOT" > "$ADD" 2>/dev/null
    # tiny pause; busybox sleep supports sub-second on most builds
    sleep 0.005 2>/dev/null || sleep 0
    echo "127.0.0.1:5060" > "$DEL" 2>/dev/null
    i=$((i+1))
done

echo "[repro] churn done; killing flood"
kill $FLOOD 2>/dev/null
wait 2>/dev/null

echo "[repro] OK"
exit 0
