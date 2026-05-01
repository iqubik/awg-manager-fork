package router

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeExec struct {
	calls []fakeCall
	err   error
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

func TestBuildRestoreInput_PolicyMark_EmitsConnmarkRule(t *testing.T) {
	spec := RestoreInputSpec{PolicyMark: "0xffffaaa"}
	out := buildRestoreInput(spec)
	want := "-I PREROUTING 1 -m connmark --mark 0xffffaaa -j " + ChainName
	if !strings.Contains(out, want) {
		t.Errorf("output missing PREROUTING rule\nwant substring: %s\ngot:\n%s", want, out)
	}
}

func TestBuildRestoreInput_EmptyMark_NoPrerouting(t *testing.T) {
	spec := RestoreInputSpec{PolicyMark: ""}
	out := buildRestoreInput(spec)
	if strings.Contains(out, "-I PREROUTING") {
		t.Errorf("expected no PREROUTING jump for empty mark, got:\n%s", out)
	}
}

func TestBuildRestoreInput_BaseRules_AlwaysPresent(t *testing.T) {
	input := buildRestoreInput(RestoreInputSpec{PolicyMark: "0xffffaaa"})

	expected := []string{
		"*mangle",
		":AWGM-TPROXY - [0:0]",
		"-A AWGM-TPROXY -d 127.0.0.0/8 -j RETURN",
		"-A AWGM-TPROXY -d 192.168.0.0/16 -j RETURN",
		"-A AWGM-TPROXY -p tcp --dport 79 -j RETURN",
		"-A AWGM-TPROXY -m mark --mark 0xff -j RETURN",
		"-A AWGM-TPROXY -p tcp -j TPROXY --on-port 51271 --on-ip 127.0.0.1 --tproxy-mark 0x1",
		"-A AWGM-TPROXY -p udp -j TPROXY --on-port 51271 --on-ip 127.0.0.1 --tproxy-mark 0x1",
		"COMMIT",
	}
	for _, line := range expected {
		if !strings.Contains(input, line) {
			t.Errorf("missing line: %q\nin:\n%s", line, input)
		}
	}
	// Socket-bypass MUST NOT appear when feature flag is off (xt_socket
	// missing → loading the rules would fail with "Couldn't load match").
	if strings.Contains(input, "-m socket --transparent") {
		t.Errorf("socket bypass present without SocketBypass=true:\n%s", input)
	}
}

func TestBuildRestoreInput_SocketBypass_AppearsBeforeTPROXY(t *testing.T) {
	input := buildRestoreInput(RestoreInputSpec{
		PolicyMark:   "0xffffaaa",
		SocketBypass: true,
	})
	bypass := "-A AWGM-TPROXY -p tcp -m socket --transparent -j RETURN"
	tproxy := "-A AWGM-TPROXY -p tcp -j TPROXY"
	bi := strings.Index(input, bypass)
	ti := strings.Index(input, tproxy)
	if bi < 0 {
		t.Fatalf("missing socket bypass line:\n%s", input)
	}
	if ti < 0 {
		t.Fatalf("missing TPROXY line:\n%s", input)
	}
	if bi >= ti {
		t.Errorf("socket bypass must precede TPROXY rule (bi=%d, ti=%d):\n%s", bi, ti, input)
	}
}

func TestIPTablesInstallSequence(t *testing.T) {
	fe := &fakeExec{}
	it := newFakeIPTables(fe)
	if err := it.Install(context.Background(), "0xffffaaa"); err != nil {
		t.Fatal(err)
	}
	// Expected order: iptables-restore, then `ip rule del` drain (one
	// pass — fake returns ENOENT immediately), then `ip rule add` with
	// our fixed priority, then `ip route add local 0.0.0.0/0`.
	if len(fe.calls) != 4 {
		t.Fatalf("expected 4 calls, got %d: %+v", len(fe.calls), fe.calls)
	}
	if fe.calls[0].kind != "restore" || !strings.Contains(fe.calls[0].stdin, "AWGM-TPROXY") {
		t.Errorf("call 0: %+v", fe.calls[0])
	}
	if fe.calls[1].kind != "ip" || !strings.Contains(strings.Join(fe.calls[1].args, " "), "rule del fwmark") {
		t.Errorf("call 1 (drain): %+v", fe.calls[1])
	}
	addArgs := strings.Join(fe.calls[2].args, " ")
	if fe.calls[2].kind != "ip" || !strings.Contains(addArgs, "rule add fwmark") ||
		!strings.Contains(addArgs, "priority 30000") {
		t.Errorf("call 2 (rule add): %+v", fe.calls[2])
	}
	if fe.calls[3].kind != "ip" || !strings.Contains(strings.Join(fe.calls[3].args, " "), "route add local") {
		t.Errorf("call 3 (route add): %+v", fe.calls[3])
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

