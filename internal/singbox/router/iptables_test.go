package router

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeExec struct {
	calls    []fakeCall
	err      error
	runIPErr error
}

type fakeCall struct {
	kind  string
	args  []string
	stdin string
}

// errENOENT mimics the kernel's "rule not found" exit so the drain
// loops terminate after a single pass — without this, fakeExec.runIP
// returning nil for `ip rule del` causes the cap-bounded drain loop
// to record N entries (or, before the cap, to OOM the test process).
var errENOENT = errIPRule("RTNETLINK answers: No such file or directory")

type errIPRule string

func (e errIPRule) Error() string { return string(e) }

func (f *fakeExec) restoreNoflush(_ context.Context, input string) error {
	f.calls = append(f.calls, fakeCall{kind: "restore", stdin: input})
	return f.err
}

func (f *fakeExec) runIPTables(_ context.Context, args ...string) error {
	f.calls = append(f.calls, fakeCall{kind: "iptables", args: args})
	return f.err
}

func (f *fakeExec) runIP(_ context.Context, args ...string) error {
	f.calls = append(f.calls, fakeCall{kind: "ip", args: args})
	if f.runIPErr != nil {
		return f.runIPErr
	}
	if f.err != nil {
		return f.err
	}
	// Make `ip rule del fwmark ...` return ENOENT after the first call
	// so drain loops don't append forever.
	if len(args) >= 4 && args[0] == "rule" && args[1] == "del" {
		return errENOENT
	}
	return nil
}

func newFakeIPTables(fe *fakeExec) *IPTables {
	return &IPTables{
		restoreNoflush: fe.restoreNoflush,
		runIPTables:    fe.runIPTables,
		runIP:          fe.runIP,
	}
}

func newFakeExec() *fakeExec {
	return &fakeExec{}
}

func TestBuildTProxyModulePath(t *testing.T) {
	got := buildTProxyModulePath("5.15.0-mips")
	want := "/lib/modules/5.15.0-mips/xt_TPROXY.ko"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestKernelModuleName(t *testing.T) {
	if kernelModuleName() != "xt_TPROXY" {
		t.Errorf("got %q", kernelModuleName())
	}
}

// EnsureCommentModule is best-effort: NDMS on some OS 5.x EA builds
// doesn't auto-load xt_comment because it doesn't use `-m comment`
// itself, but our DNS-NOPOLICY rules do. We push the load ourselves
// — and if the .ko file is absent (module possibly built-in), we
// must NOT block Enable: the kernel either accepts comment match
// natively, or iptables-restore later surfaces a concrete error.
//
// Encountered on a Keenetic NC-1812 (MT7988 aarch64, OS 5.00.C.11.0-0
// EA): xt_comment.ko was present in /lib/modules but unloaded, and
// the AWGM router refused to install with "iptables-restore: line N
// failed" until xt_comment was manually insmod'd. See issue #130.
func TestEnsureCommentModule_MissingKoIsNotFatal(t *testing.T) {
	orig := ensureKernelModuleFn
	ensureKernelModuleFn = func(_ context.Context, _ string) error {
		return ErrNetfilterComponentMissing
	}
	t.Cleanup(func() { ensureKernelModuleFn = orig })

	if err := EnsureCommentModule(context.Background()); err != nil {
		t.Errorf("expected nil when .ko absent (built-in fallback), got %v", err)
	}
}

func TestEnsureCommentModule_PassesThroughInsmodErrors(t *testing.T) {
	orig := ensureKernelModuleFn
	insmodErr := errors.New("insmod xt_comment.ko: out of memory")
	ensureKernelModuleFn = func(_ context.Context, _ string) error {
		return insmodErr
	}
	t.Cleanup(func() { ensureKernelModuleFn = orig })

	err := EnsureCommentModule(context.Background())
	if err == nil {
		t.Fatal("expected error to surface, got nil")
	}
	if !errors.Is(err, insmodErr) {
		t.Errorf("expected wrapped insmod error, got %v", err)
	}
}

func TestEnsureCommentModule_LoadsSuccessfully(t *testing.T) {
	orig := ensureKernelModuleFn
	called := false
	ensureKernelModuleFn = func(_ context.Context, name string) error {
		called = true
		if name != "xt_comment" {
			t.Errorf("expected module name xt_comment, got %q", name)
		}
		return nil
	}
	t.Cleanup(func() { ensureKernelModuleFn = orig })

	if err := EnsureCommentModule(context.Background()); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if !called {
		t.Error("expected ensureKernelModuleFn to be invoked")
	}
}

func TestBuildRestoreInput_PolicyMark_JumpHasFilter(t *testing.T) {
	spec := RestoreInputSpec{PolicyMark: "0xffffaaa"}
	out := buildRestoreInput(spec)

	// Literal SKeen jump (set_prerouting_rules, skeen.sh:1383). No `-p`
	// on the jump — SKeen jumps unconditionally and per-proto filtering
	// happens inside the chain. `-j` (not `-g`) so RETURN bypasses unwind
	// cleanly. `-A PREROUTING` (append) so we run AFTER NDMS _NDM_*
	// chains set the connmark.
	wantMangle := "-A PREROUTING -m connmark --mark 0xffffaaa -m conntrack ! --ctstate INVALID -j " + ChainName
	if !strings.Contains(out, wantMangle) {
		t.Errorf("missing mangle PREROUTING jump\nwant: %s\ngot:\n%s", wantMangle, out)
	}
	wantNat := "-A PREROUTING -m connmark --mark 0xffffaaa -m conntrack ! --ctstate INVALID -j " + RedirectChain
	if !strings.Contains(out, wantNat) {
		t.Errorf("missing nat PREROUTING jump\nwant: %s\ngot:\n%s", wantNat, out)
	}
	// JUMP must NOT carry a `-p` matcher (this was our deviation from SKeen).
	for _, bad := range []string{
		"-m conntrack ! --ctstate INVALID -p udp -j " + ChainName,
		"-m conntrack ! --ctstate INVALID -p tcp -j " + RedirectChain,
	} {
		if strings.Contains(out, bad) {
			t.Errorf("PREROUTING jump must not carry `-p` matcher:\nfound: %s\nin:\n%s", bad, out)
		}
	}

	// Legacy/transitional forms MUST be gone:
	//   - `-g chain` (goto): replaced by `-j` for SKeen-style RETURN bypass
	//   - `-I PREROUTING N`: never in restore stdin
	//   - in-chain `-m connmark ! --mark POLICY -j ACCEPT`: filter moved to jump
	for _, bad := range []string{
		"-g " + ChainName,
		"-g " + RedirectChain,
		"-I PREROUTING",
		"-A " + ChainName + " -m connmark !",
		"-A " + RedirectChain + " -m connmark !",
		"-m conntrack --ctdir REPLY",
	} {
		if strings.Contains(out, bad) {
			t.Errorf("forbidden fragment %q must not appear:\n%s", bad, out)
		}
	}
}

func TestBuildRestoreInput_EmptyMark_NoPrerouting(t *testing.T) {
	spec := RestoreInputSpec{PolicyMark: ""}
	out := buildRestoreInput(spec)
	if strings.Contains(out, "-A PREROUTING") || strings.Contains(out, "-I PREROUTING") {
		t.Errorf("expected no PREROUTING entry for empty mark, got:\n%s", out)
	}
}

func TestBuildRestoreInput_NoDNSOffloadChain(t *testing.T) {
	// SKeen-style routing drops AWGM-DNS-OFFLOAD entirely: with policy
	// filter on the jump, non-policy DNS never reaches our chains. No
	// `-m addrtype --dst-type LOCAL` (xt_addrtype dependency), no
	// `-i br+`, no `-I PREROUTING 1`.
	out := buildRestoreInput(RestoreInputSpec{PolicyMark: "0xffffaaa"})
	for _, bad := range []string{
		"AWGM-DNS-OFFLOAD",
		"addrtype",
		"br+",
	} {
		if strings.Contains(out, bad) {
			t.Errorf("forbidden DNS-OFFLOAD fragment %q must not appear:\n%s", bad, out)
		}
	}
}

func TestBuildRestoreInput_BypassUsesReturn(t *testing.T) {
	// With `-j` jump (SKeen-style) bypass rules MUST use RETURN, not
	// ACCEPT — RETURN unwinds back to PREROUTING and lets NDMS rules
	// after our jump (if any) take their course. ACCEPT would terminate
	// the table prematurely.
	out := buildRestoreInput(RestoreInputSpec{PolicyMark: "0xffffaaa"})

	for _, want := range []string{
		"-A AWGM-TPROXY -d 127.0.0.0/8 -j RETURN",
		"-A AWGM-TPROXY -d 192.168.0.0/16 -j RETURN",
		"-A AWGM-REDIRECT -d 127.0.0.0/8 -j RETURN",
		"-A AWGM-REDIRECT -d 192.168.0.0/16 -j RETURN",
		"-A AWGM-REDIRECT -p tcp --dport 79 -j RETURN",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing RETURN bypass: %s\nin:\n%s", want, out)
		}
	}
	// Legacy ACCEPT bypasses (pre-SKeen) must be gone.
	for _, bad := range []string{
		"-A AWGM-TPROXY -d 127.0.0.0/8 -j ACCEPT",
		"-A AWGM-REDIRECT -d 127.0.0.0/8 -j ACCEPT",
		// `-m mark --mark 0xff` not in SKeen — must not appear at all.
		"-m mark --mark 0xff",
		// TCP DNS-specific REDIRECT not in SKeen — catch-all handles it.
		"-A AWGM-REDIRECT -p tcp --dport 53 -j REDIRECT",
	} {
		if strings.Contains(out, bad) {
			t.Errorf("non-SKeen fragment %q must not be present:\n%s", bad, out)
		}
	}
}

func TestBuildRestoreInput_TablesAndRulesPresent(t *testing.T) {
	input := buildRestoreInput(RestoreInputSpec{PolicyMark: "0xffffaaa"})

	expected := []string{
		// mangle table — literal SKeen hybrid mode
		"*mangle",
		":AWGM-TPROXY - [0:0]",
		"-A AWGM-TPROXY -p udp --dport 53 -j TPROXY --on-port 51271 --on-ip 127.0.0.1 --tproxy-mark 0x1",
		"-A AWGM-TPROXY -d 127.0.0.0/8 -j RETURN",
		"-A AWGM-TPROXY -d 192.168.0.0/16 -j RETURN",
		"-A AWGM-TPROXY -p udp -j TPROXY --on-port 51271 --on-ip 127.0.0.1 --tproxy-mark 0x1",
		// nat table — literal SKeen hybrid mode
		"*nat",
		":AWGM-REDIRECT - [0:0]",
		"-A AWGM-REDIRECT -d 127.0.0.0/8 -j RETURN",
		"-A AWGM-REDIRECT -d 192.168.0.0/16 -j RETURN",
		"-A AWGM-REDIRECT -p tcp --dport 79 -j RETURN",
		"-A AWGM-REDIRECT -p tcp -j REDIRECT --to-ports 51272",
		"COMMIT",
	}
	for _, line := range expected {
		if !strings.Contains(input, line) {
			t.Errorf("missing line: %q\nin:\n%s", line, input)
		}
	}
	// TCP TPROXY MUST NOT appear in mangle (we moved TCP to nat REDIRECT).
	if strings.Contains(input, "-A AWGM-TPROXY -p tcp -j TPROXY") {
		t.Errorf("legacy TCP TPROXY rule must not be present:\n%s", input)
	}
}

func TestIPTablesInstallSequence(t *testing.T) {
	fe := &fakeExec{}
	it := newFakeIPTables(fe)
	if err := it.Install(context.Background(), RestoreInputSpec{PolicyMark: "0xffffaaa"}); err != nil {
		t.Fatal(err)
	}
	// removeSourceHooks scans mangle+nat PREROUTING, then iptables-restore,
	// then `ip rule del` drain, `ip rule add`, `ip route add`. After the
	// SKeen-style port there is NO separate `iptables -t nat -I PREROUTING`
	// call — the only PREROUTING jumps are emitted by iptables-restore.
	var (
		restoreSeen   bool
		ruleAddSeen   bool
		ruleAddArgs   string
		routeAddSeen  bool
		ruleDrainSeen bool
	)
	for _, c := range fe.calls {
		switch c.kind {
		case "restore":
			restoreSeen = true
			if !strings.Contains(c.stdin, "AWGM-TPROXY") {
				t.Errorf("restore stdin missing AWGM-TPROXY:\n%s", c.stdin)
			}
			if !strings.Contains(c.stdin, "AWGM-REDIRECT") {
				t.Errorf("restore stdin missing AWGM-REDIRECT:\n%s", c.stdin)
			}
			if strings.Contains(c.stdin, "AWGM-DNS-OFFLOAD") {
				t.Errorf("DNS-OFFLOAD chain must not appear in restore stdin:\n%s", c.stdin)
			}
		case "iptables":
			args := strings.Join(c.args, " ")
			if strings.Contains(args, "AWGM-DNS-OFFLOAD") {
				t.Errorf("no DNS-OFFLOAD iptables calls expected, got: %q", args)
			}
		case "ip":
			args := strings.Join(c.args, " ")
			if strings.Contains(args, "rule del fwmark") {
				ruleDrainSeen = true
			}
			if strings.Contains(args, "rule add fwmark") {
				ruleAddSeen = true
				ruleAddArgs = args
			}
			if strings.Contains(args, "route add local") {
				routeAddSeen = true
			}
		}
	}
	if !restoreSeen {
		t.Errorf("expected iptables-restore call")
	}
	if !ruleDrainSeen {
		t.Errorf("expected ip rule del drain pass")
	}
	if !ruleAddSeen || !strings.Contains(ruleAddArgs, "priority 30000") {
		t.Errorf("expected ip rule add with priority 30000, got %q", ruleAddArgs)
	}
	if !routeAddSeen {
		t.Errorf("expected ip route add local")
	}
}

func TestIPTablesUninstallSequence(t *testing.T) {
	fe := &fakeExec{err: nil}
	it := newFakeIPTables(fe)
	if err := it.Uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fe.calls) < 3 {
		t.Errorf("expected >=3 calls, got %d", len(fe.calls))
	}
	// Uninstall must not touch AWGM-DNS-OFFLOAD (it's gone).
	for _, c := range fe.calls {
		if c.kind == "iptables" {
			for _, a := range c.args {
				if strings.Contains(a, "AWGM-DNS-OFFLOAD") {
					t.Errorf("Uninstall referenced removed chain AWGM-DNS-OFFLOAD: %v", c.args)
				}
			}
		}
	}
}

func TestWriteNetfilterHookContainsPidofGuard(t *testing.T) {
	tmp := t.TempDir()
	orig := netfilterHookPath
	netfilterHookPath = filepath.Join(tmp, "50-awgm-tproxy.sh")
	t.Cleanup(func() { netfilterHookPath = orig })

	if err := writeNetfilterHook(); err != nil {
		t.Fatalf("writeNetfilterHook: %v", err)
	}
	data, err := os.ReadFile(netfilterHookPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "pidof sing-box >/dev/null 2>&1 || exit 0") {
		t.Errorf("hook missing pidof guard:\n%s", body)
	}
	if !strings.Contains(body, "iptables-restore --noflush") {
		t.Errorf("hook missing restore line:\n%s", body)
	}
}

func TestWriteNetfilterHookHasScrub(t *testing.T) {
	tmp := t.TempDir()
	orig := netfilterHookPath
	netfilterHookPath = filepath.Join(tmp, "50-awgm-tproxy.sh")
	t.Cleanup(func() { netfilterHookPath = orig })

	if err := writeNetfilterHook(); err != nil {
		t.Fatalf("writeNetfilterHook: %v", err)
	}
	data, _ := os.ReadFile(netfilterHookPath)
	body := string(data)

	// Scrub block: NDMS reloads can flush one table but not the other.
	// Without scrubbing existing PREROUTING jumps before iptables-restore,
	// --noflush would append a duplicate jump on top of the surviving one.
	wants := []string{
		"-[jg] AWGM-TPROXY",
		"-[jg] AWGM-REDIRECT",
		"-D PREROUTING",
	}
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("hook missing scrub fragment %q:\n%s", w, body)
		}
	}
	// DNS-OFFLOAD references must be gone from the hook.
	if strings.Contains(body, "AWGM-DNS-OFFLOAD") {
		t.Errorf("hook still references removed AWGM-DNS-OFFLOAD chain:\n%s", body)
	}
	// Scrub must come BEFORE the restore.
	scrubIdx := strings.Index(body, "-D PREROUTING")
	restoreIdx := strings.Index(body, "iptables-restore --noflush")
	if scrubIdx < 0 || restoreIdx < 0 || scrubIdx > restoreIdx {
		t.Errorf("scrub must precede restore: scrub=%d restore=%d", scrubIdx, restoreIdx)
	}
}

func TestRemoveNetfilterRulesFile(t *testing.T) {
	tmp := t.TempDir()
	orig := netfilterRulesPath
	netfilterRulesPath = filepath.Join(tmp, "router-netfilter.rules")
	t.Cleanup(func() { netfilterRulesPath = orig })

	if err := os.WriteFile(netfilterRulesPath, []byte("dummy"), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	removeNetfilterRulesFile()
	if _, err := os.Stat(netfilterRulesPath); !os.IsNotExist(err) {
		t.Errorf("expected file to be gone, got err=%v", err)
	}
	// Idempotent — second call must not panic.
	removeNetfilterRulesFile()
}

func TestRefreshNetfilterHookIfPresent(t *testing.T) {
	tmp := t.TempDir()
	orig := netfilterHookPath
	netfilterHookPath = filepath.Join(tmp, "50-awgm-tproxy.sh")
	t.Cleanup(func() { netfilterHookPath = orig })

	// No file → no-op (does not create one).
	refreshNetfilterHookIfPresent()
	if _, err := os.Stat(netfilterHookPath); !os.IsNotExist(err) {
		t.Errorf("expected no file, got err=%v", err)
	}

	// File present → rewrite with current content (and our pidof guard).
	if err := os.WriteFile(netfilterHookPath, []byte("# stale old version\n"), 0755); err != nil {
		t.Fatalf("seed: %v", err)
	}
	refreshNetfilterHookIfPresent()
	data, _ := os.ReadFile(netfilterHookPath)
	if !strings.Contains(string(data), "pidof sing-box") {
		t.Errorf("expected refreshed hook with pidof, got:\n%s", data)
	}
}

func TestInstall_IdempotentOnFileExists(t *testing.T) {
	// After the runIP fix (Task 1 of wizard cleanup), stderr from `ip` is
	// appended to err.Error() via sysexec.FormatError. The substring guards
	// in Install() catch "File exists" and silently swallow the error so a
	// re-Install on already-installed routes/rules is a no-op.
	rec := newFakeExec()
	it := &IPTables{
		restoreNoflush: rec.restoreNoflush,
		runIPTables:    rec.runIPTables,
		runIP:          rec.runIP,
		persistRules:   func(string) error { return nil },
		persistHook:    func() error { return nil },
		cleanupHook:    func() {},
	}
	if err := it.Install(context.Background(), RestoreInputSpec{PolicyMark: "0xff"}); err != nil {
		t.Fatalf("first Install: %v", err)
	}

	// Simulate "File exists" failure on subsequent ip-rule/ip-route add.
	rec.runIPErr = errors.New("exit status 2 (exit 2, stderr: RTNETLINK answers: File exists)")
	if err := it.Install(context.Background(), RestoreInputSpec{PolicyMark: "0xff"}); err != nil {
		t.Fatalf("second Install (idempotent): %v", err)
	}
}

func TestBuildRestoreInput_ExpandedBypassCIDRs(t *testing.T) {
	input := buildRestoreInput(RestoreInputSpec{PolicyMark: "0xffffaaa"})

	// New CIDRs that close edge cases SKeen covered:
	// - CGNAT (RFC 6598) — ISPs deploying carrier-grade NAT
	// - 0.0.0.0/8 "this network" (RFC 1122) — never routable
	// - 192.0.0.0/24 IETF Protocol Assignments — includes NAT64 well-known
	expected := []string{
		"-A AWGM-TPROXY -d 100.64.0.0/10 -j RETURN",
		"-A AWGM-TPROXY -d 0.0.0.0/8 -j RETURN",
		"-A AWGM-TPROXY -d 192.0.0.0/24 -j RETURN",
		"-A AWGM-REDIRECT -d 100.64.0.0/10 -j RETURN",
		"-A AWGM-REDIRECT -d 0.0.0.0/8 -j RETURN",
		"-A AWGM-REDIRECT -d 192.0.0.0/24 -j RETURN",
	}
	for _, line := range expected {
		if !strings.Contains(input, line) {
			t.Errorf("missing expanded-bypass line: %q\nin:\n%s", line, input)
		}
	}
}

func TestBuildRestoreInput_DNSInterceptUDP(t *testing.T) {
	input := buildRestoreInput(RestoreInputSpec{PolicyMark: "0xffffaaa"})

	// DNS rule MUST exist in AWGM-TPROXY: -p udp --dport 53 -j TPROXY ...
	wantDNS := "-A AWGM-TPROXY -p udp --dport 53 -j TPROXY --on-port 51271 --on-ip 127.0.0.1 --tproxy-mark 0x1"
	if !strings.Contains(input, wantDNS) {
		t.Errorf("missing DNS UDP TPROXY rule\nwant: %s\ngot:\n%s", wantDNS, input)
	}

	// CRITICAL ORDERING: DNS rule MUST precede the 192.168.0.0/16 bypass.
	// Otherwise DNS-to-router-LAN-IP gets bypassed before the DNS rule fires.
	dnsIdx := strings.Index(input, wantDNS)
	bypassIdx := strings.Index(input, "-A AWGM-TPROXY -d 192.168.0.0/16 -j RETURN")
	if dnsIdx < 0 || bypassIdx < 0 {
		t.Fatalf("DNS or bypass rule not found")
	}
	if dnsIdx > bypassIdx {
		t.Errorf("DNS rule at offset %d must precede 192.168/16 bypass at offset %d", dnsIdx, bypassIdx)
	}
}

func TestBuildRestoreInput_TCPCatchAllHandlesDNS(t *testing.T) {
	input := buildRestoreInput(RestoreInputSpec{PolicyMark: "0xffffaaa"})

	// SKeen's nat chain (`add_redirect_rules`) has NO dport-53-specific
	// rule; the catch-all `-p tcp -j REDIRECT` covers TCP DNS too. Verify
	// (a) the explicit DNS rule is absent and (b) the catch-all is present
	// and lands AFTER the bypasses (so private/router IPs still RETURN).
	if strings.Contains(input, "-A AWGM-REDIRECT -p tcp --dport 53") {
		t.Errorf("explicit TCP DNS rule must not appear (SKeen handles via catch-all):\n%s", input)
	}
	wantCatch := "-A AWGM-REDIRECT -p tcp -j REDIRECT --to-ports 51272"
	if !strings.Contains(input, wantCatch) {
		t.Errorf("missing TCP catch-all REDIRECT:\n%s", input)
	}
	catchIdx := strings.Index(input, wantCatch)
	bypassIdx := strings.Index(input, "-A AWGM-REDIRECT -d 192.168.0.0/16 -j RETURN")
	if catchIdx < bypassIdx {
		t.Errorf("TCP catch-all (%d) must come after bypasses (%d)", catchIdx, bypassIdx)
	}
}

func TestBuildRestoreInput_WANIPsRendered(t *testing.T) {
	// Synthetic RFC 5737 TEST-NET-3 + RFC 1918 — mirrors a real multi-WAN
	// router with public WAN + tunnel addresses.
	spec := RestoreInputSpec{
		PolicyMark: "0xffffaaa",
		WANIPs:     []string{"203.0.113.207/32", "10.8.1.3/32"},
	}
	input := buildRestoreInput(spec)

	// WAN-IP rules MUST appear in BOTH chains as RETURN bypasses.
	expected := []string{
		"-A AWGM-TPROXY -d 203.0.113.207/32 -j RETURN",
		"-A AWGM-TPROXY -d 10.8.1.3/32 -j RETURN",
		"-A AWGM-REDIRECT -d 203.0.113.207/32 -j RETURN",
		"-A AWGM-REDIRECT -d 10.8.1.3/32 -j RETURN",
	}
	for _, line := range expected {
		if !strings.Contains(input, line) {
			t.Errorf("missing WAN-IP line: %q\nin:\n%s", line, input)
		}
	}
}

func TestBuildRestoreInput_EmptyWANIPs_NoExclusions(t *testing.T) {
	spec := RestoreInputSpec{PolicyMark: "0xffffaaa", WANIPs: nil}
	input := buildRestoreInput(spec)

	// No /32 host-routes should appear other than 255.255.255.255/32.
	for _, line := range strings.Split(input, "\n") {
		if strings.Contains(line, "/32 -j RETURN") && !strings.Contains(line, "255.255.255.255") {
			t.Errorf("unexpected /32 exclusion when WANIPs empty: %s", line)
		}
	}
}

func TestBuildRestoreInput_LANBridges_DNSNoPolicyRules(t *testing.T) {
	// LAN bridges with matching NDMS catch-all marks → DNS-NOPOLICY
	// rules in mangle PREROUTING that re-mark mark=0 DNS to the
	// bridge's catch-all mark.
	spec := RestoreInputSpec{
		PolicyMark: "0xffffaad",
		LANBridges: []LANBridgeMark{
			{Bridge: "br0", Mark: "0xffffaab"},
			{Bridge: "br1", Mark: "0xffffaab"},
		},
	}
	input := buildRestoreInput(spec)

	expected := []string{
		`-A PREROUTING -i br0 -m mark --mark 0x0 -m pkttype --pkt-type unicast -p udp --dport 53 -m comment --comment "AWGM-DNS-NOPOLICY" -j MARK --set-mark 0xffffaab`,
		`-A PREROUTING -i br0 -m mark --mark 0x0 -m pkttype --pkt-type unicast -p tcp --dport 53 -m comment --comment "AWGM-DNS-NOPOLICY" -j MARK --set-mark 0xffffaab`,
		`-A PREROUTING -i br1 -m mark --mark 0x0 -m pkttype --pkt-type unicast -p udp --dport 53 -m comment --comment "AWGM-DNS-NOPOLICY" -j MARK --set-mark 0xffffaab`,
		`-A PREROUTING -i br1 -m mark --mark 0x0 -m pkttype --pkt-type unicast -p tcp --dport 53 -m comment --comment "AWGM-DNS-NOPOLICY" -j MARK --set-mark 0xffffaab`,
	}
	for _, line := range expected {
		if !strings.Contains(input, line) {
			t.Errorf("missing DNS-NOPOLICY line: %q\nin:\n%s", line, input)
		}
	}
}

func TestBuildRestoreInput_LANBridges_DifferentMarksPerBridge(t *testing.T) {
	// Sanity: per-bridge mark wired through correctly when bridges
	// have different NDMS catch-all marks (e.g. different LAN policies
	// per bridge). Each bridge gets its OWN re-mark target.
	spec := RestoreInputSpec{
		PolicyMark: "0xffffaad",
		LANBridges: []LANBridgeMark{
			{Bridge: "br0", Mark: "0xffffaab"},
			{Bridge: "br1", Mark: "0xffffab0"},
		},
	}
	input := buildRestoreInput(spec)

	if !strings.Contains(input, "-i br0 -m mark --mark 0x0 -m pkttype --pkt-type unicast -p udp --dport 53 -m comment --comment \"AWGM-DNS-NOPOLICY\" -j MARK --set-mark 0xffffaab") {
		t.Errorf("br0 should use mark 0xffffaab")
	}
	if !strings.Contains(input, "-i br1 -m mark --mark 0x0 -m pkttype --pkt-type unicast -p udp --dport 53 -m comment --comment \"AWGM-DNS-NOPOLICY\" -j MARK --set-mark 0xffffab0") {
		t.Errorf("br1 should use mark 0xffffab0")
	}
}

func TestBuildRestoreInput_NoLANBridges_NoDNSNoPolicyRules(t *testing.T) {
	// Empty LANBridges → no DNS-NOPOLICY rules emitted at all.
	// Caller (Service.Enable) skips DNS fallback entirely on routers
	// without hotspot LAN configuration.
	spec := RestoreInputSpec{
		PolicyMark: "0xffffaad",
		LANBridges: nil,
	}
	input := buildRestoreInput(spec)

	for _, marker := range []string{"AWGM-DNS-NOPOLICY", "--set-mark"} {
		if strings.Contains(input, marker) {
			t.Errorf("DNS-NOPOLICY artifact %q leaked into output when LANBridges empty:\n%s", marker, input)
		}
	}
}

func TestEqualLANBridges(t *testing.T) {
	a := []LANBridgeMark{{Bridge: "br0", Mark: "0xffffaab"}, {Bridge: "br1", Mark: "0xffffaab"}}
	b := []LANBridgeMark{{Bridge: "br0", Mark: "0xffffaab"}, {Bridge: "br1", Mark: "0xffffaab"}}
	c := []LANBridgeMark{{Bridge: "br0", Mark: "0xffffaab"}, {Bridge: "br1", Mark: "0xffffab0"}}  // different mark
	d := []LANBridgeMark{{Bridge: "br0", Mark: "0xffffaab"}}                                       // shorter
	e := []LANBridgeMark{{Bridge: "br1", Mark: "0xffffaab"}, {Bridge: "br0", Mark: "0xffffaab"}}  // different order

	if !equalLANBridges(a, b) {
		t.Error("identical slices must compare equal")
	}
	if equalLANBridges(a, c) {
		t.Error("differing mark must not compare equal")
	}
	if equalLANBridges(a, d) {
		t.Error("differing length must not compare equal")
	}
	if equalLANBridges(a, e) {
		t.Error("differing order must not compare equal (caller relies on stable order)")
	}
	if !equalLANBridges(nil, nil) {
		t.Error("nil/nil must compare equal")
	}
	if !equalLANBridges([]LANBridgeMark{}, nil) {
		t.Error("empty and nil must compare equal")
	}
}

func TestHotspotCatchAllRegexp_RealisticParse(t *testing.T) {
	// Realistic snapshot of `iptables -t mangle -S _NDM_HOTSPOT_PREROUTING_MANGL`
	// from a router with three bridges in NDMS hotspot (br0/br1) plus a
	// WireGuard "bridge" (nwg2) and a MAC-RETURN entry. Only the catch-all
	// `-i X -j MARK --set-xmark Y/...` lines should match — MAC-specific
	// and CONNMARK/RETURN lines must NOT.
	snapshot := `-N _NDM_HOTSPOT_PREROUTING_MANGL
-A _NDM_HOTSPOT_PREROUTING_MANGL -m mac --mac-source 50:FF:20:B4:F4:37 -j RETURN
-A _NDM_HOTSPOT_PREROUTING_MANGL -m mac --mac-source B0:4A:B4:74:80:F8 -j RETURN
-A _NDM_HOTSPOT_PREROUTING_MANGL -m mac --mac-source 80:B6:55:60:28:FE -j MARK --set-xmark 0xffffaab/0xffffffff
-A _NDM_HOTSPOT_PREROUTING_MANGL -m mac --mac-source 80:B6:55:60:28:FE -j CONNMARK --save-mark --nfmask 0xffffffff --ctmask 0xffffffff
-A _NDM_HOTSPOT_PREROUTING_MANGL -m mac --mac-source 80:B6:55:60:28:FE -j RETURN
-A _NDM_HOTSPOT_PREROUTING_MANGL -i br0 -j MARK --set-xmark 0xffffaab/0xffffffff
-A _NDM_HOTSPOT_PREROUTING_MANGL -i br0 -j CONNMARK --save-mark --nfmask 0xffffffff --ctmask 0xffffffff
-A _NDM_HOTSPOT_PREROUTING_MANGL -i br0 -j RETURN
-A _NDM_HOTSPOT_PREROUTING_MANGL -i nwg2 -j MARK --set-xmark 0xffffaab/0xffffffff
-A _NDM_HOTSPOT_PREROUTING_MANGL -i nwg2 -j CONNMARK --save-mark --nfmask 0xffffffff --ctmask 0xffffffff
-A _NDM_HOTSPOT_PREROUTING_MANGL -i nwg2 -j RETURN
-A _NDM_HOTSPOT_PREROUTING_MANGL -i br1 -j MARK --set-xmark 0xffffaab/0xffffffff`

	matched := make(map[string]string)
	for _, line := range strings.Split(snapshot, "\n") {
		m := hotspotCatchAllRegexp.FindStringSubmatch(line)
		if m != nil {
			matched[m[1]] = m[2]
		}
	}

	want := map[string]string{
		"br0":  "0xffffaab",
		"nwg2": "0xffffaab",
		"br1":  "0xffffaab",
	}
	if len(matched) != len(want) {
		t.Errorf("matched %d ifaces, want %d: %+v", len(matched), len(want), matched)
	}
	for iface, mark := range want {
		if matched[iface] != mark {
			t.Errorf("iface %s: got mark %q, want %q", iface, matched[iface], mark)
		}
	}
	// CONNMARK and MAC-source rules must NOT match.
	for _, line := range strings.Split(snapshot, "\n") {
		if strings.Contains(line, "CONNMARK") || strings.Contains(line, "RETURN") || strings.Contains(line, "mac-source") {
			if hotspotCatchAllRegexp.FindStringSubmatch(line) != nil {
				t.Errorf("false positive on: %s", line)
			}
		}
	}
}

func TestSplitLines(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a\nb\nc", []string{"a", "b", "c"}},
		{"a\nb\n", []string{"a", "b"}}, // trailing \n produces no empty entry
		{"\na", []string{"a"}},         // leading \n produces no empty entry
	}
	for _, c := range cases {
		got := splitLines(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitLines(%q): got %d lines, want %d (%+v vs %+v)", c.in, len(got), len(c.want), got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitLines(%q)[%d]: got %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}
