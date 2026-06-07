/* SPDX-License-Identifier: GPL-2.0 */
/*
 * Host loop-convergence tests for the worker recv loop in src/proxy.c.
 *
 * proxy.c's s2c_thread_fn / c2s_thread_fn live behind kernel-only types
 * (struct socket, struct task_struct, etc.) and so cannot be compiled
 * on the host directly. Instead we replicate the loop's CONTROL FLOW
 * here, swap in a scripted recv source and a settable should-stop flag,
 * and assert two convergence invariants on top of awg_classify_recv:
 *
 *   1. Liveness under stop: setting should_stop=true MUST cause the
 *      loop to exit within a bounded number of iterations, regardless
 *      of what recv returns next.
 *   2. No-stall under zero-flood: while should_stop=false and recv
 *      keeps returning 0 (the issue #234 trigger), the loop MUST keep
 *      spinning instead of bailing out — i.e. it neither breaks early
 *      nor diverges into a tight non-yielding loop. We can't actually
 *      observe "yield" on the host (cond_resched is a no-op here), so
 *      we instead assert that the loop body sees a YIELD/SLEEP action,
 *      never a BREAK.
 *
 * What this DOESN'T prove: that cond_resched() actually releases CPU0
 * on a non-preempt single-core MIPS target. Only a real-hardware or
 * QEMU run can show that — see kmod/awg-proxy/tests/qemu/.
 *
 * Build via tests/Makefile (TARGET=test_proxy_loop).
 */

#include "shim.h"
#include "../src/proxy_recv.h"

#include <limits.h>

/* Tiny assertion framework — same pattern as test_proxy_recv.c. */
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

/* ---- Scripted recv + cooperative-stop fixture ---- */

struct loop_fixture {
	int recv_seq[1024];      /* scripted return values from "recvmsg" */
	int recv_seq_len;        /* honoured length; rest are zero-filled */
	int stop_at_iter;        /* iteration after which should_stop becomes true */
	int max_iter;            /* fail-safe ceiling — exceeding == divergence */
	int iter;                /* current iteration (read after run) */
	int last_action;         /* last awg_recv_action seen in the body */
	int yield_count;         /* how many YIELD actions fired */
	int sleep_count;         /* how many SLEEP actions fired */
	int process_count;       /* how many PROCESS actions fired */
};

static int fixture_recv(struct loop_fixture *f)
{
	if (f->iter < f->recv_seq_len)
		return f->recv_seq[f->iter];
	return 0;  /* default fill */
}

static bool fixture_should_stop(struct loop_fixture *f)
{
	return f->iter >= f->stop_at_iter;
}

/*
 * Replicates the control-flow skeleton of c2s_thread_fn / s2c_thread_fn:
 *
 *   while (!kthread_should_stop()) {
 *       n = kernel_recvmsg(...);
 *       switch (awg_classify_recv(n, kthread_should_stop())) {
 *       case AWG_RECV_BREAK:        goto out;
 *       case AWG_RECV_RETRY_SLEEP:  msleep(10); continue;
 *       case AWG_RECV_RETRY_YIELD:  cond_resched(); continue;
 *       case AWG_RECV_PROCESS:      <process packet>; break;
 *       }
 *   }
 *
 * Returns the iteration count at which the loop exited. Returns -1 if
 * the max_iter ceiling was hit (i.e. the loop didn't converge — bug).
 */
static int run_loop(struct loop_fixture *f)
{
	while (!fixture_should_stop(f)) {
		int n;
		enum awg_recv_action act;

		if (f->iter >= f->max_iter)
			return -1;

		n = fixture_recv(f);
		act = awg_classify_recv(n, fixture_should_stop(f));
		f->last_action = act;

		switch (act) {
		case AWG_RECV_BREAK:
			return f->iter;
		case AWG_RECV_RETRY_SLEEP:
			f->sleep_count++;
			break;
		case AWG_RECV_RETRY_YIELD:
			f->yield_count++;
			break;
		case AWG_RECV_PROCESS:
			f->process_count++;
			break;
		}
		f->iter++;
	}
	return f->iter;
}

/* ---- Invariant 1: liveness under stop ---- */

TEST(test_stop_at_zero_exits_immediately)
{
	struct loop_fixture f = {
		.recv_seq_len = 0,
		.stop_at_iter = 0,
		.max_iter     = 100,
	};
	int exit_at = run_loop(&f);
	EXPECT(exit_at == 0);                /* zero iterations */
	EXPECT(f.process_count == 0);
}

TEST(test_stop_after_n_iters_with_steady_packets)
{
	struct loop_fixture f = {
		.recv_seq_len = 0,            /* fill: 0 → YIELD */
		.stop_at_iter = 10,
		.max_iter     = 100,
	};
	/* Override fill: pretend recvmsg always returns 148 (handshake init) */
	for (int i = 0; i < 1024; i++) f.recv_seq[i] = 148;
	f.recv_seq_len = 1024;

	int exit_at = run_loop(&f);
	EXPECT(exit_at == 10);
	EXPECT(f.process_count == 10);
}

TEST(test_stop_after_zero_flood_does_converge)
{
	/* Issue #234 regression sentinel: 100 empty datagrams, then stop.
	 * Before fix the very first 0 would break the loop. */
	struct loop_fixture f = {
		.recv_seq_len = 0,            /* fill: 0 */
		.stop_at_iter = 100,
		.max_iter     = 200,
	};
	int exit_at = run_loop(&f);
	EXPECT(exit_at == 100);
	EXPECT(f.process_count == 0);
	EXPECT(f.yield_count == 100);
	EXPECT(f.sleep_count == 0);
}

TEST(test_shutdown_errno_exits_first_iter)
{
	struct loop_fixture f = {
		.recv_seq     = { -ESHUTDOWN, 148, 148, 148 },
		.recv_seq_len = 4,
		.stop_at_iter = INT_MAX,
		.max_iter     = 100,
	};
	int exit_at = run_loop(&f);
	EXPECT(exit_at == 0);
	EXPECT(f.last_action == AWG_RECV_BREAK);
}

TEST(test_signal_errno_exits_after_partial_run)
{
	struct loop_fixture f = {
		.recv_seq     = { 148, 148, 148, -ERESTARTSYS },
		.recv_seq_len = 4,
		.stop_at_iter = INT_MAX,
		.max_iter     = 100,
	};
	int exit_at = run_loop(&f);
	EXPECT(exit_at == 3);
	EXPECT(f.process_count == 3);
	EXPECT(f.last_action == AWG_RECV_BREAK);
}

/* ---- Invariant 2: no-stall under sustained zero-flood ---- */

TEST(test_zero_flood_without_stop_does_not_break_early)
{
	/* CRITICAL regression test for issue #234:
	 * with should_stop = false forever and recv always = 0, the loop
	 * must NOT exit via AWG_RECV_BREAK. We model "forever" as max_iter
	 * = 1000 with stop_at_iter set higher: if the classifier broke
	 * on n == 0 we'd see exit_at < max_iter, here we expect to be
	 * stopped by the safety ceiling.
	 *
	 * Pre-fix this test FAILS: classifier broke on n == 0, loop
	 * exited at iter 0, exit_at = 0.
	 */
	struct loop_fixture f = {
		.recv_seq_len = 0,
		.stop_at_iter = INT_MAX,
		.max_iter     = 1000,
	};
	int exit_at = run_loop(&f);
	EXPECT(exit_at == -1);                /* hit ceiling, didn't break */
	EXPECT(f.yield_count == 1000);
	EXPECT(f.process_count == 0);
}

TEST(test_runt_flood_without_stop_does_not_break_early)
{
	/* 1-3 byte packets must also keep the loop alive, not break it. */
	struct loop_fixture f = {
		.recv_seq_len = 0,
		.stop_at_iter = INT_MAX,
		.max_iter     = 500,
	};
	for (int i = 0; i < 1024; i++) f.recv_seq[i] = (i % 3) + 1;
	f.recv_seq_len = 1024;

	int exit_at = run_loop(&f);
	EXPECT(exit_at == -1);
	EXPECT(f.yield_count == 500);
	EXPECT(f.process_count == 0);
}

TEST(test_transient_error_flood_does_not_break_early)
{
	/* -EAGAIN / -ENOMEM are retry-with-sleep, not break. */
	struct loop_fixture f = {
		.recv_seq_len = 0,
		.stop_at_iter = INT_MAX,
		.max_iter     = 500,
	};
	for (int i = 0; i < 1024; i++) f.recv_seq[i] = -EAGAIN;
	f.recv_seq_len = 1024;

	int exit_at = run_loop(&f);
	EXPECT(exit_at == -1);
	EXPECT(f.sleep_count == 500);
}

/* ---- Mixed-traffic convergence ---- */

TEST(test_mixed_traffic_converges_on_stop)
{
	/* Realistic alternating real / empty packets. */
	struct loop_fixture f = {
		.recv_seq_len = 0,
		.stop_at_iter = 50,
		.max_iter     = 200,
	};
	for (int i = 0; i < 1024; i++)
		f.recv_seq[i] = (i % 2 == 0) ? 148 : 0;
	f.recv_seq_len = 1024;

	int exit_at = run_loop(&f);
	EXPECT(exit_at == 50);
	EXPECT(f.process_count == 25);
	EXPECT(f.yield_count == 25);
}

int main(void)
{
	test_stop_at_zero_exits_immediately_run();
	test_stop_after_n_iters_with_steady_packets_run();
	test_stop_after_zero_flood_does_converge_run();
	test_shutdown_errno_exits_first_iter_run();
	test_signal_errno_exits_after_partial_run_run();

	test_zero_flood_without_stop_does_not_break_early_run();
	test_runt_flood_without_stop_does_not_break_early_run();
	test_transient_error_flood_does_not_break_early_run();

	test_mixed_traffic_converges_on_stop_run();

	fprintf(stderr, "\n=== %d run, %d failed ===\n", g_run, g_failed);
	return g_failed ? 1 : 0;
}
