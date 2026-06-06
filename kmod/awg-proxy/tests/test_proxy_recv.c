/* SPDX-License-Identifier: GPL-2.0 */
/*
 * Host unit tests for awg_classify_recv() — the worker recv-loop
 * classifier in src/proxy_recv.h. The classifier is pure and has no
 * kernel-side dependencies, so we test it directly.
 *
 * Critical assertion for issue #234:
 *   awg_classify_recv(0, false) == AWG_RECV_RETRY_YIELD
 * i.e. an empty recv (UDP zero-length datagram, or kernel_sock_shutdown
 * before kthread_should_stop arms) does NOT exit the loop. Pre-fix
 * code treated n == 0 as EOF and broke out of the loop; that turned a
 * single empty datagram into a silent worker kill / local DoS.
 *
 * Build via tests/Makefile (TARGET=test_proxy_recv).
 */

/* proxy_recv.h is dual-mode (kernel/host); host build needs nothing
 * beyond stdbool + stdlib errno, which the header pulls itself. shim.h
 * is included for the test-framework conveniences (stdio, etc.). */
#include "shim.h"
#include "../src/proxy_recv.h"

/* Tiny assertion framework — same pattern as test_cps.c / test_transform.c. */
static int g_run, g_failed;
#define EXPECT(cond) do {                                                     \
		if (!(cond)) {                                                \
			fprintf(stderr, "  FAIL %s:%d  %s\n",                 \
				__FILE__, __LINE__, #cond);                   \
			g_failed++;                                           \
		}                                                             \
	} while (0)
#define TEST(name) static void name(void); \
	static void name##_run(void) { g_run++; name(); } \
	static void name(void)

/* ---- shutdown wins over recv result ---- */

TEST(test_should_stop_with_zero_recv_breaks)
{
	EXPECT(awg_classify_recv(0, true) == AWG_RECV_BREAK);
}

TEST(test_should_stop_with_full_packet_breaks)
{
	EXPECT(awg_classify_recv(148, true) == AWG_RECV_BREAK);
}

TEST(test_should_stop_with_transient_error_breaks)
{
	EXPECT(awg_classify_recv(-EAGAIN, true) == AWG_RECV_BREAK);
}

TEST(test_should_stop_with_shutdown_errno_breaks)
{
	EXPECT(awg_classify_recv(-EBADF, true) == AWG_RECV_BREAK);
}

/* ---- shutdown-class errnos exit the loop ---- */

TEST(test_erestartsys_breaks)
{
	EXPECT(awg_classify_recv(-ERESTARTSYS, false) == AWG_RECV_BREAK);
}

TEST(test_eintr_breaks)
{
	EXPECT(awg_classify_recv(-EINTR, false) == AWG_RECV_BREAK);
}

TEST(test_eshutdown_breaks)
{
	EXPECT(awg_classify_recv(-ESHUTDOWN, false) == AWG_RECV_BREAK);
}

TEST(test_ebadf_breaks)
{
	EXPECT(awg_classify_recv(-EBADF, false) == AWG_RECV_BREAK);
}

TEST(test_epipe_breaks)
{
	EXPECT(awg_classify_recv(-EPIPE, false) == AWG_RECV_BREAK);
}

/* ---- transient errnos retry with sleep ---- */

TEST(test_eagain_retries_with_sleep)
{
	EXPECT(awg_classify_recv(-EAGAIN, false) == AWG_RECV_RETRY_SLEEP);
}

TEST(test_enomem_retries_with_sleep)
{
	EXPECT(awg_classify_recv(-ENOMEM, false) == AWG_RECV_RETRY_SLEEP);
}

TEST(test_unknown_negative_retries_with_sleep)
{
	/* Any not-listed negative errno is treated as transient. */
	EXPECT(awg_classify_recv(-999, false) == AWG_RECV_RETRY_SLEEP);
}

/* ---- ISSUE #234: zero-length recv MUST NOT break ---- */

TEST(test_zero_length_recv_yields_not_breaks)
{
	/* This is THE assertion that documents the #234 fix.
	 * Pre-fix this returned AWG_RECV_BREAK, which let any local
	 * sender kill the worker with a single empty UDP datagram. */
	EXPECT(awg_classify_recv(0, false) == AWG_RECV_RETRY_YIELD);
}

/* ---- runt packets (n < 4) yield ---- */

TEST(test_one_byte_packet_yields)
{
	EXPECT(awg_classify_recv(1, false) == AWG_RECV_RETRY_YIELD);
}

TEST(test_three_byte_packet_yields)
{
	EXPECT(awg_classify_recv(3, false) == AWG_RECV_RETRY_YIELD);
}

/* ---- full packets are processed ---- */

TEST(test_four_byte_packet_processes)
{
	/* 4 bytes is the boundary — minimum size we'd consider real. */
	EXPECT(awg_classify_recv(4, false) == AWG_RECV_PROCESS);
}

TEST(test_handshake_init_size_processes)
{
	/* 148 = WireGuard handshake init. */
	EXPECT(awg_classify_recv(148, false) == AWG_RECV_PROCESS);
}

TEST(test_full_mtu_processes)
{
	EXPECT(awg_classify_recv(1500, false) == AWG_RECV_PROCESS);
}

TEST(test_large_packet_processes)
{
	EXPECT(awg_classify_recv(2048, false) == AWG_RECV_PROCESS);
}

int main(void)
{
	test_should_stop_with_zero_recv_breaks_run();
	test_should_stop_with_full_packet_breaks_run();
	test_should_stop_with_transient_error_breaks_run();
	test_should_stop_with_shutdown_errno_breaks_run();

	test_erestartsys_breaks_run();
	test_eintr_breaks_run();
	test_eshutdown_breaks_run();
	test_ebadf_breaks_run();
	test_epipe_breaks_run();

	test_eagain_retries_with_sleep_run();
	test_enomem_retries_with_sleep_run();
	test_unknown_negative_retries_with_sleep_run();

	test_zero_length_recv_yields_not_breaks_run();

	test_one_byte_packet_yields_run();
	test_three_byte_packet_yields_run();

	test_four_byte_packet_processes_run();
	test_handshake_init_size_processes_run();
	test_full_mtu_processes_run();
	test_large_packet_processes_run();

	fprintf(stderr, "\n=== %d run, %d failed ===\n", g_run, g_failed);
	return g_failed ? 1 : 0;
}
