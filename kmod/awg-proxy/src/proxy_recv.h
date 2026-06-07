/* SPDX-License-Identifier: GPL-2.0 */
/*
 * Worker recv-loop classifier — pure-C, host-testable.
 *
 * Maps a kernel_recvmsg return value (and the cooperative-stop signal)
 * to one of four next-step actions. Lives in its own header (no
 * <linux includes> includes) so the same logic can be unit-tested on a regular
 * dev box via kmod/awg-proxy/tests/test_proxy_recv.c — no kernel
 * headers, no shim, just plain C and the standard errno macros.
 *
 * Issue #234 invariant: n == 0 alone MUST NOT trigger AWG_RECV_BREAK.
 * UDP recv returns 0 both on legitimate zero-length datagrams and on
 * kernel_sock_shutdown(); the only safe discriminator is should_stop,
 * which proxy_stop() arms via kthread_stop() before tearing the slot
 * down. Pre-fix code treated n == 0 as EOF and exited the loop, which
 * (a) let any local sender kill the worker with one empty datagram,
 * and (b) silently broke the tunnel with no log line.
 */
#ifndef _AWG_PROXY_PROXY_RECV_H
#define _AWG_PROXY_PROXY_RECV_H

/*
 * Dual-mode include: kernel build pulls real <linux includes>, host build
 * (kmod/awg-proxy/tests/) uses stdbool + stdlib errno. ERESTARTSYS and
 * ESHUTDOWN are kernel-internal on Linux, so the host build stubs them
 * to their canonical numeric values (512 and 108 respectively).
 */
#ifdef __KERNEL__
# include <linux/errno.h>
# include <linux/types.h>
#else
# include <errno.h>
# include <stdbool.h>
# ifndef ERESTARTSYS
#  define ERESTARTSYS 512
# endif
# ifndef ESHUTDOWN
#  define ESHUTDOWN  108
# endif
#endif

enum awg_recv_action {
	AWG_RECV_BREAK,         /* caller must exit the loop */
	AWG_RECV_RETRY_SLEEP,   /* caller should msleep(10) then continue */
	AWG_RECV_RETRY_YIELD,   /* caller should cond_resched() then continue */
	AWG_RECV_PROCESS,       /* n is a full packet (>= 4); caller processes */
};

static inline enum awg_recv_action
awg_classify_recv(int n, bool should_stop)
{
	/* Cooperative stop wins regardless of recv result. */
	if (should_stop)
		return AWG_RECV_BREAK;

	if (n < 0) {
		/* Shutdown-class errnos: socket is gone, no point retrying. */
		switch (n) {
		case -ERESTARTSYS:
		case -EINTR:
		case -ESHUTDOWN:
		case -EBADF:
		case -EPIPE:
			return AWG_RECV_BREAK;
		default:
			/* Transient (e.g. -EAGAIN, -ENOMEM): back off and retry. */
			return AWG_RECV_RETRY_SLEEP;
		}
	}

	/* n >= 0. Empty (n == 0) or runt (n < 4) packets: yield to keep
	 * non-preempt single-core MIPS from stalling, then continue.
	 *
	 * Critical: n == 0 falls here, NOT into AWG_RECV_BREAK. See file
	 * banner above for the issue #234 invariant.
	 */
	if (n < 4)
		return AWG_RECV_RETRY_YIELD;

	return AWG_RECV_PROCESS;
}

#endif /* _AWG_PROXY_PROXY_RECV_H */
