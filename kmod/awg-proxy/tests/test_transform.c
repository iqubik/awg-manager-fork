// SPDX-License-Identifier: GPL-2.0
/*
 * Userspace unit tests for kmod/awg-proxy/src/transform.c.
 *
 * Covers:
 *   1. S4 transport prefix filled with random bytes (not stale heap)
 *   2. S4 fill is deterministic given a seeded PRNG
 *   3. S1 handshake-init prefix filled with random bytes
 *   4. H4-noop identity passthrough (no prefix, no copy)
 *   5. MAC1 recomputed on outbound handshake init after H1 substitution
 *   6. MAC1 recomputed on inbound handshake init after header restore
 *
 * Run via `make test`.
 */

#include "shim.h"
#include "../src/transform.h"
#include "../src/blake2s.h"

#include <string.h>
#include <stdlib.h>
#include <stdio.h>
#include <stdarg.h>

/* ---- Tiny test harness (matches test_cps.c style) ---- */

static int tests_run, tests_failed;

static void test_fail(const char *test, const char *fmt, ...)
{
	va_list ap;

	fprintf(stderr, "FAIL %s: ", test);
	va_start(ap, fmt);
	vfprintf(stderr, fmt, ap);
	va_end(ap);
	fputc('\n', stderr);
	tests_failed++;
}

#define ASSERT_TRUE(test, cond, msg) do { \
	if (!(cond)) test_fail((test), "%s", (msg)); \
} while (0)

#define ASSERT_EQ(test, got, want, msg) do { \
	if ((got) != (want)) \
		test_fail((test), "%s: got %d, want %d", (msg), (int)(got), (int)(want)); \
} while (0)

#define ASSERT_PTR_EQ(test, got, want, msg) do { \
	if ((got) != (want)) \
		test_fail((test), "%s: got %p, want %p", (msg), (void *)(got), (void *)(want)); \
} while (0)

static int all_byte(const u8 *buf, int len, u8 val)
{
	int i;
	for (i = 0; i < len; i++)
		if (buf[i] != val)
			return 0;
	return 1;
}

static void write32_le_host(u8 *p, u32 v)
{
	__le32 le = cpu_to_le32(v);
	memcpy(p, &le, 4);
}

/* Build a minimal awg_config_t for tests.  Zero everything first. */
static void cfg_init(awg_config_t *cfg)
{
	memset(cfg, 0, sizeof(*cfg));
	/* Identity defaults — no transform */
	cfg->h1.min = WG_HANDSHAKE_INIT;    cfg->h1.max = WG_HANDSHAKE_INIT;
	cfg->h2.min = WG_HANDSHAKE_RESPONSE; cfg->h2.max = WG_HANDSHAKE_RESPONSE;
	cfg->h3.min = WG_COOKIE_REPLY;      cfg->h3.max = WG_COOKIE_REPLY;
	cfg->h4.min = WG_TRANSPORT_DATA;    cfg->h4.max = WG_TRANSPORT_DATA;
}

/* ---------- Tests ---------- */

/*
 * Test 1: S4 transport prefix is filled with random bytes.
 *
 * Fill the entire buffer with 0xAA sentinel, build a transport packet,
 * call transform_outbound. The S4 region must no longer be 0xAA.
 */
static void test_s4_random_fill(void)
{
	awg_config_t cfg;
	u8 buf[512];
	int dataoff = 256;
	int out_len, sendJunk;
	u32 msgType;
	u8 *out;

	tests_run++;
	cfg_init(&cfg);
	cfg.s4 = 64;
	cfg.h4.min = 100; cfg.h4.max = 200;
	config_compute(&cfg);

	memset(buf, 0xAA, sizeof(buf));
	/* Write a minimal WG transport packet at buf + dataoff */
	write32_le_host(buf + dataoff, WG_TRANSPORT_DATA);
	memset(buf + dataoff + 4, 0xBB, WG_TRANSPORT_MIN - 4);

	shim_set_random_seed(0xCAFE);
	out = transform_outbound(buf, dataoff, WG_TRANSPORT_MIN,
				 &cfg, 0x42ULL,
				 &out_len, &sendJunk, &msgType);

	ASSERT_PTR_EQ("s4_random_fill", out, buf + dataoff - 64, "returned pointer");
	ASSERT_EQ("s4_random_fill", out_len, 64 + WG_TRANSPORT_MIN, "out_len");
	ASSERT_TRUE("s4_random_fill",
		    !all_byte(buf + dataoff - 64, 64, 0xAA),
		    "S4 region should not be all-sentinel (was overwritten by random)");
	ASSERT_EQ("s4_random_fill", sendJunk, 0, "transport should not trigger junk");
}

/*
 * Test 2: S4 fill is deterministic with seeded PRNG.
 *
 * Same seed → same S4 bytes.
 */
static void test_s4_deterministic(void)
{
	awg_config_t cfg;
	u8 buf1[512], buf2[512];
	int dataoff = 256;
	int out_len, sendJunk;
	u32 msgType;

	tests_run++;
	cfg_init(&cfg);
	cfg.s4 = 64;
	cfg.h4.min = 100; cfg.h4.max = 100; /* fixed H4 for reproducibility */
	config_compute(&cfg);

	/* First call */
	memset(buf1, 0, sizeof(buf1));
	write32_le_host(buf1 + dataoff, WG_TRANSPORT_DATA);
	memset(buf1 + dataoff + 4, 0xBB, WG_TRANSPORT_MIN - 4);
	shim_set_random_seed(0x12345);
	transform_outbound(buf1, dataoff, WG_TRANSPORT_MIN,
			   &cfg, 0x42ULL, &out_len, &sendJunk, &msgType);

	/* Second call, same seed */
	memset(buf2, 0, sizeof(buf2));
	write32_le_host(buf2 + dataoff, WG_TRANSPORT_DATA);
	memset(buf2 + dataoff + 4, 0xBB, WG_TRANSPORT_MIN - 4);
	shim_set_random_seed(0x12345);
	transform_outbound(buf2, dataoff, WG_TRANSPORT_MIN,
			   &cfg, 0x42ULL, &out_len, &sendJunk, &msgType);

	ASSERT_TRUE("s4_deterministic",
		    memcmp(buf1 + dataoff - 64, buf2 + dataoff - 64, 64) == 0,
		    "same seed should produce identical S4 prefix bytes");
}

/*
 * Test 3: S1 handshake-init prefix filled with random bytes.
 */
static void test_s1_random_fill(void)
{
	awg_config_t cfg;
	u8 buf[512];
	int dataoff = 256;
	int out_len, sendJunk;
	u32 msgType;
	u8 *out;

	tests_run++;
	cfg_init(&cfg);
	cfg.s1 = 32;
	cfg.h1.min = 1000; cfg.h1.max = 2000;
	config_compute(&cfg);

	memset(buf, 0xAA, sizeof(buf));
	/* Write a 148-byte WG handshake init at buf + dataoff */
	write32_le_host(buf + dataoff, WG_HANDSHAKE_INIT);
	memset(buf + dataoff + 4, 0xCC, WG_INIT_SIZE - 4);

	shim_set_random_seed(0xBEEF);
	out = transform_outbound(buf, dataoff, WG_INIT_SIZE,
				 &cfg, 0x99ULL,
				 &out_len, &sendJunk, &msgType);

	ASSERT_PTR_EQ("s1_random_fill", out, buf + dataoff - 32, "returned pointer");
	ASSERT_EQ("s1_random_fill", out_len, 32 + WG_INIT_SIZE, "out_len");
	ASSERT_TRUE("s1_random_fill",
		    !all_byte(buf + dataoff - 32, 32, 0xAA),
		    "S1 region should not be all-sentinel");
	ASSERT_EQ("s1_random_fill", sendJunk, 0,
		  "sendJunk=0 because jc=0");
}

/*
 * Test 4: H4-noop identity passthrough — no prefix, pointer unchanged.
 */
static void test_s4_noop_passthrough(void)
{
	awg_config_t cfg;
	u8 buf[256];
	int dataoff = 64;
	int out_len, sendJunk;
	u32 msgType;
	u8 *out;

	tests_run++;
	cfg_init(&cfg);
	/* identity: H4={4,4}, S4=0 → h4_noop */
	config_compute(&cfg);

	ASSERT_TRUE("s4_noop_passthrough", cfg.h4_noop,
		    "config_compute should set h4_noop for identity H4");

	write32_le_host(buf + dataoff, WG_TRANSPORT_DATA);
	memset(buf + dataoff + 4, 0xDD, WG_TRANSPORT_MIN - 4);

	out = transform_outbound(buf, dataoff, WG_TRANSPORT_MIN,
				 &cfg, 0ULL,
				 &out_len, &sendJunk, &msgType);

	ASSERT_PTR_EQ("s4_noop_passthrough", out, buf + dataoff, "pointer unchanged");
	ASSERT_EQ("s4_noop_passthrough", out_len, WG_TRANSPORT_MIN, "length unchanged");
}

/*
 * Test 5: MAC1 is recomputed on outbound handshake init after H1 substitution.
 * Guards the fix from PR #138.
 */
static void test_mac1_recompute_outbound_init(void)
{
	awg_config_t cfg;
	u8 buf[512];
	int dataoff = 256;
	int out_len, sendJunk;
	u32 msgType;
	u8 mac1_before[16];

	tests_run++;
	cfg_init(&cfg);
	cfg.h1.min = 999; cfg.h1.max = 999;
	/* Set a non-zero server public key so MAC1 recompute triggers */
	memset(cfg.server_pub, 0x42, 32);
	config_compute(&cfg);

	ASSERT_TRUE("mac1_outbound_init", cfg.has_server_pub,
		    "server_pub should be detected as non-zero");

	/* Build a 148-byte init packet with known MAC1 area */
	memset(buf + dataoff, 0xEE, WG_INIT_SIZE);
	write32_le_host(buf + dataoff, WG_HANDSHAKE_INIT);

	/* Save the MAC1 region before transform (bytes 116..132) */
	memcpy(mac1_before, buf + dataoff + 116, 16);

	transform_outbound(buf, dataoff, WG_INIT_SIZE,
			   &cfg, 0x55ULL,
			   &out_len, &sendJunk, &msgType);

	ASSERT_TRUE("mac1_outbound_init",
		    memcmp(mac1_before, buf + dataoff + 116, 16) != 0,
		    "MAC1 should differ after H1 substitution + recompute");
}

/*
 * Test 6: MAC1 is recomputed on inbound handshake init after header restore.
 * Guards the fix from PR #138.
 */
static void test_mac1_recompute_inbound_init(void)
{
	awg_config_t cfg;
	u8 buf[512];
	int s1 = 16;
	int out_len;
	u8 mac1_before[16];
	u8 *out;

	tests_run++;
	cfg_init(&cfg);
	cfg.s1 = s1;
	cfg.h1.min = 999; cfg.h1.max = 999;
	/* Set a non-zero client public key so inbound MAC1 recompute triggers */
	memset(cfg.client_pub, 0x37, 32);
	config_compute(&cfg);

	ASSERT_TRUE("mac1_inbound_init", cfg.has_client_pub,
		    "client_pub should be detected as non-zero");

	/* Build a packet that looks like it came from AWG server:
	 * [S1 random prefix][H1-substituted header][148-byte body] */
	memset(buf, 0, sizeof(buf));
	memset(buf, 0x11, s1);  /* S1 prefix (random in reality) */
	write32_le_host(buf + s1, 999);  /* H1-substituted header */
	memset(buf + s1 + 4, 0x77, WG_INIT_SIZE - 4);  /* body */

	/* Save MAC1 before (bytes 116..132 within the init portion) */
	memcpy(mac1_before, buf + s1 + 116, 16);

	out = transform_inbound(buf, s1 + WG_INIT_SIZE, &cfg, &out_len);

	ASSERT_TRUE("mac1_inbound_init", out != NULL, "should not be NULL (valid packet)");
	ASSERT_EQ("mac1_inbound_init", out_len, WG_INIT_SIZE, "stripped size");
	ASSERT_TRUE("mac1_inbound_init",
		    memcmp(mac1_before, out + 116, 16) != 0,
		    "MAC1 should differ after header restore + recompute");
}

/*
 * Test 7: recompute_mac2_if_present rewrites MAC2 for init with non-zero MAC2,
 * value matches an independent compute_mac2() over the same bytes.
 */
static void test_mac2_recompute_outbound_init(void)
{
	u8 buf[WG_INIT_SIZE];
	u8 cookie[16];
	u8 expected[16];

	tests_run++;
	/* Build init body with non-zero MAC2 (simulates client with cookie) */
	memset(buf, 0x33, WG_INIT_SIZE);
	write32_le_host(buf, WG_HANDSHAKE_INIT);
	memset(cookie, 0xAB, 16);

	/* Reference: MAC2 = blake2s(cookie, buf[0..132], 16) */
	compute_mac2(cookie, buf, 132, expected);

	recompute_mac2_if_present(buf, WG_INIT_SIZE,
				  WG_HANDSHAKE_INIT, cookie);

	ASSERT_TRUE("mac2_outbound_init",
		    memcmp(buf + 132, expected, 16) == 0,
		    "MAC2 must match independent compute_mac2 result");
}

/*
 * Test 8: same for response packet (mac1_end=60, mac2_off=76).
 */
static void test_mac2_recompute_outbound_response(void)
{
	u8 buf[WG_RESP_SIZE];
	u8 cookie[16];
	u8 expected[16];

	tests_run++;
	memset(buf, 0x55, WG_RESP_SIZE);
	write32_le_host(buf, WG_HANDSHAKE_RESPONSE);
	memset(cookie, 0xCD, 16);

	compute_mac2(cookie, buf, 76, expected);

	recompute_mac2_if_present(buf, WG_RESP_SIZE,
				  WG_HANDSHAKE_RESPONSE, cookie);

	ASSERT_TRUE("mac2_outbound_response",
		    memcmp(buf + 76, expected, 16) == 0,
		    "MAC2 must match independent compute_mac2 result");
}

/*
 * Test 9: zero MAC2 stays zero (client without cookie — must not lie to server).
 */
static void test_mac2_zero_passthrough(void)
{
	u8 buf[WG_INIT_SIZE];
	u8 cookie[16];
	u8 zeros[16] = {0};

	tests_run++;
	memset(buf, 0x77, WG_INIT_SIZE);
	write32_le_host(buf, WG_HANDSHAKE_INIT);
	memset(buf + 132, 0, 16);  /* MAC2 = zeros */
	memset(cookie, 0xEF, 16);

	recompute_mac2_if_present(buf, WG_INIT_SIZE,
				  WG_HANDSHAKE_INIT, cookie);

	ASSERT_TRUE("mac2_zero_passthrough",
		    memcmp(buf + 132, zeros, 16) == 0,
		    "zero MAC2 must remain zero");
}

/*
 * Test 10: non-handshake msgType is no-op (transport / cookie / unknown).
 */
static void test_mac2_non_handshake_noop(void)
{
	u8 buf[WG_INIT_SIZE];
	u8 buf_before[WG_INIT_SIZE];
	u8 cookie[16];

	tests_run++;
	memset(buf, 0x99, WG_INIT_SIZE);
	memset(cookie, 0x11, 16);
	memcpy(buf_before, buf, WG_INIT_SIZE);

	recompute_mac2_if_present(buf, WG_INIT_SIZE,
				  WG_TRANSPORT_DATA, cookie);

	ASSERT_TRUE("mac2_non_handshake_noop",
		    memcmp(buf, buf_before, WG_INIT_SIZE) == 0,
		    "non-handshake msgType must leave buffer untouched");
}

/* ---------- Main ---------- */

int main(void)
{
	test_s4_random_fill();
	test_s4_deterministic();
	test_s1_random_fill();
	test_s4_noop_passthrough();
	test_mac1_recompute_outbound_init();
	test_mac1_recompute_inbound_init();
	test_mac2_recompute_outbound_init();
	test_mac2_recompute_outbound_response();
	test_mac2_zero_passthrough();
	test_mac2_non_handshake_noop();

	printf("\n=== %d run, %d failed ===\n", tests_run, tests_failed);
	return tests_failed == 0 ? 0 : 1;
}
