package router

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	sysexec "github.com/hoaxisr/awg-manager/internal/sys/exec"
	sysiptables "github.com/hoaxisr/awg-manager/internal/sys/iptables"
	"github.com/hoaxisr/awg-manager/internal/sys/osdetect"
)

const (
	TPROXYPort = 51271
	// RedirectPort is sing-box's REDIRECT inbound for TCP. We split TCP
	// onto NAT REDIRECT (instead of mangle TPROXY) because TPROXY for
	// TCP requires a working `-m socket --transparent` bypass to deliver
	// ACK/data on already-established connections — Keenetic's 4.9-ndm-5
	// kernel evaluates that match as 0 hits, so ACKs would re-enter the
	// TPROXY rule, get redirected to 127.0.0.1:51271 with the listener
	// destination, and the kernel would emit RST because no socket
	// matches that 4-tuple. NAT REDIRECT sidesteps the issue entirely:
	// conntrack tracks the DNAT for established flows, ACKs are
	// auto-translated, and sing-box's accept()ed socket handles them
	// like any normal TCP connection. SKeen ships the same split.
	RedirectPort     = 51272
	Fwmark           = 0x1
	RoutingTable     = 100
	ChainName        = "AWGM-TPROXY"
	RedirectChain    = "AWGM-REDIRECT"
	// DNSNoPolicyChain re-marks DNS-port-53 packets that arrive with
	// mark=0 (no NDMS access policy assigned) up to the bridge's NDMS
	// catch-all mark, so NDMS's own _NDM_HOTSPOT_DNSREDIR fires and
	// REDIRECTs the DNS query to its per-policy ndnproxy. Without this,
	// sing-box's hijack-dns side-effect transparent listener at every
	// router LAN-IP:53 silently drops the query (no TPROXY ancillary
	// data → drop without log) and the client sees DNS timeout. The
	// chain itself is empty — rules are emitted directly under
	// PREROUTING per-bridge from LANBridges in buildRestoreInput.
	DNSNoPolicyChain = "AWGM-DNS-NOPOLICY"
	// IPRulePriority is the fixed `ip rule` priority for our fwmark rule.
	// Above NDMS policy rules (~100-200) and below system main/default
	// (32766/32767). Hard-coded so Install is fully idempotent and so
	// our rule never accidentally displaces the kernel's local-table
	// rule at priority 0.
	IPRulePriority = 30000
	// maxIPRuleDrainPasses caps the drain loop in Install/Uninstall.
	// Defensive bound — `ip rule del` should return ENOENT quickly when
	// nothing matches, but a buggy kernel returning success forever
	// would otherwise hang Install. 32 is well above any realistic
	// duplicate count (the worst observed was 10 in the wild).
	maxIPRuleDrainPasses = 32
)

// Mutable in tests via t.Cleanup so they can redirect into a tmp dir.
// Production code reads these at call time.
var (
	netfilterHookPath  = "/opt/etc/ndm/netfilter.d/50-awgm-tproxy.sh"
	netfilterRulesPath = "/opt/etc/awg-manager/singbox/router-netfilter.rules"
)

func kernelModuleName() string { return "xt_TPROXY" }

func buildTProxyModulePath(kernelVersion string) string {
	return filepath.Join("/lib/modules", kernelVersion, "xt_TPROXY.ko")
}

func isModuleLoaded(name string) bool {
	data, err := os.ReadFile("/proc/modules")
	if err != nil {
		return false
	}
	prefix := name + " "
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

var (
	netfilterOnce      sync.Once
	netfilterAvailable bool
)

func IsNetfilterAvailable() bool {
	netfilterOnce.Do(func() {
		if isModuleLoaded(kernelModuleName()) {
			netfilterAvailable = true
			return
		}
		kernel := osdetect.KernelRelease()
		if kernel == "" {
			return
		}
		_, err := os.Stat(buildTProxyModulePath(kernel))
		netfilterAvailable = err == nil
	})
	return netfilterAvailable
}

func EnsureTProxyModule(ctx context.Context) error {
	return ensureKernelModuleFn(ctx, kernelModuleName())
}

// EnsureCommentModule loads xt_comment when available as a .ko module
// so DNS-NOPOLICY rules (which use `-m comment --comment "..."` as
// their scrub identifier in netfilter.d) can be accepted by the kernel
// at iptables-restore COMMIT time.
//
// Soft-fail by design: if the .ko file is absent we return nil and let
// iptables-restore surface a concrete error later. Reason — the module
// may be built into the kernel (no .ko on disk), in which case the
// rules apply normally and a hard "missing component" error here would
// be a false positive. The harder failure path (genuinely missing both
// as module and built-in) shows up as "iptables-restore: line N failed"
// at Install time, which is the same error path the user already
// observes today.
//
// Why this is needed: NDMS on some Keenetic OS 5.x EA firmwares
// (observed on NC-1812 / MT7988 / OS 5.00.C.11.0-0 EA) does not use
// `-m comment` anywhere, so xt_comment isn't auto-loaded at boot.
// Without an explicit insmod our DNS-NOPOLICY rules (added by commit
// ad5ad113) are rejected at COMMIT and the whole mangle install fails.
// See issue #130.
func EnsureCommentModule(ctx context.Context) error {
	err := ensureKernelModuleFn(ctx, "xt_comment")
	if errors.Is(err, ErrNetfilterComponentMissing) {
		return nil
	}
	return err
}

// ensureKernelModuleFn is the indirection point that tests redirect to
// inject deterministic outcomes for EnsureCommentModule. Production
// callers use the real ensureKernelModule below.
var ensureKernelModuleFn = ensureKernelModule

func ensureKernelModule(ctx context.Context, name string) error {
	if isModuleLoaded(name) {
		return nil
	}
	kernel := osdetect.KernelRelease()
	if kernel == "" {
		return ErrNetfilterComponentMissing
	}
	path := filepath.Join("/lib/modules", kernel, name+".ko")
	if _, err := os.Stat(path); err != nil {
		return ErrNetfilterComponentMissing
	}
	if _, err := sysexec.Run(ctx, "insmod", path); err != nil {
		return fmt.Errorf("insmod %s: %w", path, err)
	}
	return nil
}

var (
	tproxyTargetOnce   sync.Once
	tproxyTargetResult bool
)

func IsTProxyTargetAvailable(ctx context.Context) bool {
	tproxyTargetOnce.Do(func() {
		res, err := sysexec.Run(ctx, sysiptables.Binary, "-j", "TPROXY", "--help")
		if err != nil || res == nil {
			return
		}
		tproxyTargetResult = strings.Contains(res.Stdout+res.Stderr, "TPROXY")
	})
	return tproxyTargetResult
}

type RestoreInputSpec struct {
	// PolicyMark is the NDMS-assigned mark (hex, e.g. "0xffffaaa") that
	// NDMS sets on connections from devices bound to the chosen access
	// policy. Empty means no PREROUTING jump (defensive — caller should
	// never reach Install with empty mark, but iptables doesn't panic).
	PolicyMark string

	// WANIPs is a list of router-owned IP addresses (in "X.X.X.X/32" form)
	// that must NOT be TPROXY'd: traffic from LAN to the router's own
	// public-WAN or tunnel-endpoint IPs would otherwise loop back into
	// sing-box. Collected dynamically by WANIPCollector before Install.
	// Empty list = no extra exclusions (router still works, just exposes
	// the WAN-IP loop edge case).
	WANIPs []string

	// LANBridges enumerates (bridge, NDMS catch-all mark) pairs for the
	// DNS-NOPOLICY mangle rules. Discovered by DiscoverLANBridges() — see
	// lanbridges.go for the discovery algorithm and the why. Empty list
	// = skip DNS-NOPOLICY entirely (no bridges where the fall-through is
	// applicable on this router).
	LANBridges []LANBridgeMark
}

var bypassCIDRs = []string{
	"127.0.0.0/8",
	"169.254.0.0/16",
	"100.64.0.0/10", // CGNAT (RFC 6598)
	"0.0.0.0/8",     // this network (RFC 1122)
	"192.0.0.0/24",  // IETF Protocol Assignments (NAT64 well-known)
	"224.0.0.0/4",
	"255.255.255.255/32",
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
}

func buildRestoreInput(spec RestoreInputSpec) string {
	var b strings.Builder

	// SKeen-style layout (`reference/SKeen/skeen.sh`, set_chain_rules /
	// set_prerouting_rules / add_tproxy_rules / add_redirect_rules):
	//
	//   - one chain per table (mangle: AWGM-TPROXY, nat: AWGM-REDIRECT)
	//   - policy connmark filter lives ON THE PREROUTING JUMP, not inside
	//     the chain — non-policy traffic never enters the chain at all
	//   - jump uses `-j` (not `-g`) so bypasses can `-j RETURN` cleanly
	//     back into PREROUTING; the catch-all TPROXY/REDIRECT at the end
	//     of the chain handles everything else
	//   - jump is `-A PREROUTING` (append) so it runs AFTER NDMS
	//     _NDM_*_PREROUTING_* chains have a chance to set the connmark;
	//     this also keeps the fast_nat-cache issue (FORWARD MASQUERADE
	//     poisoning conntrack with WAN-IP source) from coming back
	//   - NO AWGM-DNS-OFFLOAD chain: with policy filter on the jump,
	//     non-policy DNS never reaches sing-box via these rules. (If the
	//     `hijack-dns` side-effect listener turns out to still grab
	//     non-policy DNS at the kernel socket-lookup level, that's a
	//     sing-box inbound/dns config matter — see Уровень Б discussion
	//     in commit log)
	//   - no `-m addrtype` or `-i br+` matchers anywhere: zero kernel
	//     module surface beyond xt_TPROXY

	// ---- *mangle table: UDP via TPROXY ----
	// Literal port of `add_tproxy_rules` from reference/SKeen/skeen.sh
	// (hybrid mode, mangle table) plus the DNS interception rule from
	// set_chain_rules (`INTERCEPT_DNS_ENABLE=1` branch). No extras.
	b.WriteString("*mangle\n")
	fmt.Fprintf(&b, ":%s - [0:0]\n", ChainName)

	// set_chain_rules: DNS first (when INTERCEPT_DNS_ENABLE=1)
	fmt.Fprintf(&b, "-A %s -p udp --dport 53 -j TPROXY --on-port %d --on-ip 127.0.0.1 --tproxy-mark 0x%x\n",
		ChainName, TPROXYPort, Fwmark)

	// set_chain_rules: bypass set. SKeen uses one ipset rule; we render
	// the same destinations as discrete CIDR rules (semantically equal).
	for _, cidr := range bypassCIDRs {
		fmt.Fprintf(&b, "-A %s -d %s -j RETURN\n", ChainName, cidr)
	}
	for _, ip := range spec.WANIPs {
		fmt.Fprintf(&b, "-A %s -d %s -j RETURN\n", ChainName, ip)
	}

	// add_tproxy_rules: catch-all TPROXY for UDP.
	fmt.Fprintf(&b, "-A %s -p udp -j TPROXY --on-port %d --on-ip 127.0.0.1 --tproxy-mark 0x%x\n",
		ChainName, TPROXYPort, Fwmark)

	// set_prerouting_rules: policy connmark filter ON THE JUMP, no `-p`
	// matcher (SKeen jumps unconditionally; per-proto matching happens
	// inside the chain).
	if spec.PolicyMark != "" {
		fmt.Fprintf(&b, "-A PREROUTING -m connmark --mark %s -m conntrack ! --ctstate INVALID -j %s\n",
			spec.PolicyMark, ChainName)
	}

	// ---- DNS-NOPOLICY: re-mark mark=0 DNS up to bridge's NDMS catch-all ----
	// For each (bridge, mark) discovered in mangle _NDM_HOTSPOT_PREROUTING_MANGL,
	// emit a packet-MARK rule on PREROUTING that fires only when:
	//   - packet arrived on that bridge (-i bridge),
	//   - current MARK is 0 (i.e., a MAC-RETURN-override in NDMS's
	//     hotspot chain skipped this device, so NDMS catch-all never
	//     ran for it),
	//   - destination port is 53 (DNS only — other no-policy traffic
	//     stays at mark=0, keeping the "no-policy" semantics intact),
	//   - packet type is unicast (don't touch mDNS / link-local multicast).
	// Re-marking ONLY this single packet (no CONNMARK save) means
	// subsequent flow packets retain mark=0; only the DNS query gets
	// elevated. NDMS's existing _NDM_HOTSPOT_DNSREDIR rule for that
	// mark then REDIRECTs the query to its per-policy ndnproxy port.
	for _, bm := range spec.LANBridges {
		fmt.Fprintf(&b, "-A PREROUTING -i %s -m mark --mark 0x0 -m pkttype --pkt-type unicast -p udp --dport 53 -m comment --comment %q -j MARK --set-mark %s\n",
			bm.Bridge, DNSNoPolicyChain, bm.Mark)
		fmt.Fprintf(&b, "-A PREROUTING -i %s -m mark --mark 0x0 -m pkttype --pkt-type unicast -p tcp --dport 53 -m comment --comment %q -j MARK --set-mark %s\n",
			bm.Bridge, DNSNoPolicyChain, bm.Mark)
	}

	b.WriteString("COMMIT\n")

	// ---- *nat table: TCP via REDIRECT ----
	// Literal port of `add_redirect_rules` from reference/SKeen/skeen.sh
	// (hybrid mode, nat table). SKeen's nat chain has ONLY the bypass set
	// + catch-all `-p tcp -j REDIRECT`; there is no DNS-specific TCP rule
	// because the catch-all already covers TCP/53.
	b.WriteString("*nat\n")
	fmt.Fprintf(&b, ":%s - [0:0]\n", RedirectChain)

	for _, cidr := range bypassCIDRs {
		fmt.Fprintf(&b, "-A %s -d %s -j RETURN\n", RedirectChain, cidr)
	}
	for _, ip := range spec.WANIPs {
		fmt.Fprintf(&b, "-A %s -d %s -j RETURN\n", RedirectChain, ip)
	}
	// Bypass router admin port so we don't redirect our own UI traffic.
	// (SKeen has equivalent dynamic admin-port discovery — same intent.)
	fmt.Fprintf(&b, "-A %s -p tcp --dport 79 -j RETURN\n", RedirectChain)

	// add_redirect_rules: catch-all REDIRECT for TCP.
	fmt.Fprintf(&b, "-A %s -p tcp -j REDIRECT --to-ports %d\n", RedirectChain, RedirectPort)

	if spec.PolicyMark != "" {
		fmt.Fprintf(&b, "-A PREROUTING -m connmark --mark %s -m conntrack ! --ctstate INVALID -j %s\n",
			spec.PolicyMark, RedirectChain)
	}
	b.WriteString("COMMIT\n")
	return b.String()
}

type restoreNoflushFn func(ctx context.Context, input string) error
type runFn func(ctx context.Context, args ...string) error

type persistFn func(input string) error

type IPTables struct {
	restoreNoflush restoreNoflushFn
	runIPTables    runFn
	runIP          runFn
	persistRules   persistFn
	persistHook    func() error
	cleanupHook    func()
}

func NewIPTables() *IPTables {
	return &IPTables{
		restoreNoflush: sysiptables.RestoreNoflush,
		runIPTables:    sysiptables.Run,
		runIP: func(ctx context.Context, args ...string) error {
			result, err := sysexec.Run(ctx, "ip", args...)
			return sysexec.FormatError(result, err)
		},
		persistRules: writeNetfilterRulesFile,
		persistHook:  writeNetfilterHook,
		cleanupHook:  removeNetfilterRulesFile,
	}
}

func (it *IPTables) Install(ctx context.Context, spec RestoreInputSpec) error {
	// Scrub any existing PREROUTING jumps to AWGM-TPROXY before inserting
	// the new one. iptables-restore --noflush + -I PREROUTING 1 would
	// otherwise stack a duplicate jump on every restart / mark-change /
	// re-Enable: the stale rule from the previous policy/mark survives
	// because mangle isn't flushed, and the new rule lands in front of it.
	// Idempotent: a no-op when no prior jumps exist.
	it.removeSourceHooks(ctx)

	input := buildRestoreInput(spec)
	if it.persistRules != nil {
		if err := it.persistRules(input); err != nil {
			return fmt.Errorf("write netfilter rules: %w", err)
		}
	}
	if it.persistHook != nil {
		if err := it.persistHook(); err != nil {
			return fmt.Errorf("write netfilter hook: %w", err)
		}
	}
	if err := it.restoreNoflush(ctx, input); err != nil {
		return fmt.Errorf("iptables-restore: %w", err)
	}
	// Drain ALL existing fwmark rules pointing at our table before
	// adding a fresh one. Without this, every Install (Reconcile,
	// daemon restart, mark-change, re-Enable) leaves a duplicate
	// because `ip rule add` without explicit priority lands at
	// previous_priority + 1 instead of being deduped — and a stack of
	// rules at priorities 0-N displaces the kernel's `from all lookup
	// local` rule (normally at prio 0), breaking router-local routing
	// (sing-box outbounds to direct silently fail).
	for i := 0; i < maxIPRuleDrainPasses; i++ {
		if err := it.runIP(ctx, "rule", "del", "fwmark", fmt.Sprintf("0x%x", Fwmark),
			"table", fmt.Sprintf("%d", RoutingTable)); err != nil {
			break
		}
	}
	// Use an explicit priority well above NDMS policy rules (100-200)
	// and well below the system main/default tables (32766/32767), so
	// our rule is identifiable and idempotent.
	if err := it.runIP(ctx, "rule", "add", "fwmark", fmt.Sprintf("0x%x", Fwmark),
		"table", fmt.Sprintf("%d", RoutingTable),
		"priority", fmt.Sprintf("%d", IPRulePriority)); err != nil {
		if !strings.Contains(err.Error(), "File exists") {
			return fmt.Errorf("ip rule add: %w", err)
		}
	}
	if err := it.runIP(ctx, "route", "add", "local", "0.0.0.0/0", "dev", "lo",
		"table", fmt.Sprintf("%d", RoutingTable)); err != nil {
		if !strings.Contains(err.Error(), "File exists") {
			return fmt.Errorf("ip route add: %w", err)
		}
	}
	return nil
}

func writeNetfilterRulesFile(input string) error {
	if err := os.MkdirAll(filepath.Dir(netfilterRulesPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(netfilterRulesPath, []byte(input), 0644)
}

func writeNetfilterHook() error {
	if err := os.MkdirAll(filepath.Dir(netfilterHookPath), 0755); err != nil {
		return err
	}
	// Scrub block before restore: when NDMS reloads only one table (e.g.
	// nat) but leaves mangle intact, restoring --noflush would append a
	// SECOND PREROUTING jump to AWGM-TPROXY on top of the surviving one.
	// `-[jg]` regex covers both legacy `-j` and current `-g` syntax so
	// the upgrade path doesn't leave duplicates around.
	script := fmt.Sprintf(`#!/bin/sh
[ "$type" = "ip6tables" ] && exit 0
case "$table" in mangle|nat) ;; *) exit 0 ;; esac
[ -f %[1]q ] || exit 0
pidof sing-box >/dev/null 2>&1 || exit 0
mangle_ok=0; nat_ok=0
/opt/sbin/iptables -w -t mangle -nL %[2]s >/dev/null 2>&1 && mangle_ok=1
/opt/sbin/iptables -w -t nat    -nL %[6]s >/dev/null 2>&1 && nat_ok=1
if [ "$mangle_ok" -eq 0 ] || [ "$nat_ok" -eq 0 ]; then
  /opt/sbin/iptables -w -t mangle -S PREROUTING 2>/dev/null \
    | grep -E -- '-[jg] %[2]s($| )' \
    | sed 's/-A PREROUTING/-D PREROUTING/' \
    | while IFS= read -r line; do /opt/sbin/iptables -w -t mangle $line 2>/dev/null; done
  /opt/sbin/iptables -w -t nat -S PREROUTING 2>/dev/null \
    | grep -E -- '-[jg] %[6]s($| )' \
    | sed 's/-A PREROUTING/-D PREROUTING/' \
    | while IFS= read -r line; do /opt/sbin/iptables -w -t nat $line 2>/dev/null; done
  # Scrub DNS-NOPOLICY direct PREROUTING rules (identified by comment
  # tag, not by jump target — these are -j MARK rules, not chain
  # jumps). Same drop-and-restore approach as above, just matched
  # via the iptables-save comment serialisation.
  /opt/sbin/iptables -w -t mangle -S PREROUTING 2>/dev/null \
    | grep -F -- '--comment "%[7]s"' \
    | sed 's/-A PREROUTING/-D PREROUTING/' \
    | while IFS= read -r line; do /opt/sbin/iptables -w -t mangle $line 2>/dev/null; done
  /opt/sbin/iptables-restore --noflush < %[1]q
  /opt/sbin/ip rule add fwmark 0x%[3]x table %[4]d priority %[5]d 2>/dev/null || true
  /opt/sbin/ip route add local 0.0.0.0/0 dev lo table %[4]d 2>/dev/null || true
  logger -t awgm-tproxy "netfilter.d: restored AWGM chains after NDMS reload"
fi
`, netfilterRulesPath, ChainName, Fwmark, RoutingTable, IPRulePriority, RedirectChain, DNSNoPolicyChain)
	return os.WriteFile(netfilterHookPath, []byte(script), 0755)
}

// removeNetfilterRulesFile deletes the persisted rules file so the
// netfilter.d hook becomes a no-op on the next NDMS reload. Called on
// engine Disable. Idempotent.
func removeNetfilterRulesFile() {
	_ = os.Remove(netfilterRulesPath)
}

// refreshNetfilterHookIfPresent rewrites the netfilter.d hook script
// when one is already installed, so older versions get the current
// pidof guard on daemon startup. No-op when the file is absent —
// Install creates it on first Enable.
func refreshNetfilterHookIfPresent() {
	if _, err := os.Stat(netfilterHookPath); err != nil {
		return
	}
	_ = writeNetfilterHook()
}

func (it *IPTables) Uninstall(ctx context.Context) error {
	if it.cleanupHook != nil {
		it.cleanupHook()
	}
	it.removeSourceHooks(ctx)
	_ = it.runIPTables(ctx, "-t", "mangle", "-F", ChainName)
	_ = it.runIPTables(ctx, "-t", "mangle", "-X", ChainName)
	_ = it.runIPTables(ctx, "-t", "nat", "-F", RedirectChain)
	_ = it.runIPTables(ctx, "-t", "nat", "-X", RedirectChain)
	// Drain ALL fwmark rules — historically Install accumulated
	// duplicates at priorities 0-N (auto-assigned), so a single `del`
	// would leave the rest. Loop until ENOENT, capped defensively.
	for i := 0; i < maxIPRuleDrainPasses; i++ {
		if err := it.runIP(ctx, "rule", "del", "fwmark", fmt.Sprintf("0x%x", Fwmark),
			"table", fmt.Sprintf("%d", RoutingTable)); err != nil {
			break
		}
	}
	_ = it.runIP(ctx, "route", "flush", "table", fmt.Sprintf("%d", RoutingTable))
	return nil
}

func (it *IPTables) removeSourceHooks(ctx context.Context) {
	it.removeSourceHooksFromTable(ctx, "mangle", ChainName)
	it.removeSourceHooksFromTable(ctx, "nat", RedirectChain)
	// DNS-NOPOLICY uses direct PREROUTING rules (no intermediate chain
	// jump), identified by their `-m comment --comment AWGM-DNS-NOPOLICY`
	// tag. Same pattern XKeen uses for its own tagged rules.
	it.removeCommentTaggedRulesFromTable(ctx, "mangle", "PREROUTING", DNSNoPolicyChain)
}

// removeCommentTaggedRulesFromTable scrubs every rule in `chain` whose
// iptables-save output contains the given comment tag. Used for
// DNS-NOPOLICY where rules are direct PREROUTING entries (not jumps
// to a custom chain). The grep+sed pattern is the same approach as
// XKeen's removal logic — robust to rule ordering and matcher changes
// as long as the `-m comment --comment <tag>` survives serialisation.
func (it *IPTables) removeCommentTaggedRulesFromTable(ctx context.Context, table, chain, tag string) {
	result, err := sysexec.Run(ctx, sysiptables.Binary, "-w", "-t", table, "-S", chain)
	if err != nil || result == nil {
		return
	}
	for _, line := range strings.Split(result.Stdout, "\n") {
		if !strings.Contains(line, `--comment "`+tag+`"`) && !strings.Contains(line, `--comment `+tag) {
			continue
		}
		if !strings.HasPrefix(line, "-A "+chain+" ") {
			continue
		}
		delLine := "-D " + strings.TrimPrefix(line, "-A ")
		args := append([]string{"-w", "-t", table}, strings.Fields(delLine)...)
		_, _ = sysexec.Run(ctx, sysiptables.Binary, args...)
	}
}

func (it *IPTables) removeSourceHooksFromTable(ctx context.Context, table, chain string) {
	result, err := sysexec.Run(ctx, sysiptables.Binary, "-w", "-t", table, "-S", "PREROUTING")
	if err != nil || result == nil {
		return
	}
	// Match both `-j chain` (old jump syntax pre-fastnat-fix) and
	// `-g chain` (current goto syntax) so upgrading installs scrub
	// stale jumps from previous versions before we re-append the new one.
	jumpJ := "-j " + chain
	gotoG := "-g " + chain
	for _, line := range strings.Split(result.Stdout, "\n") {
		if !strings.Contains(line, jumpJ) && !strings.Contains(line, gotoG) {
			continue
		}
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "-A PREROUTING") {
			continue
		}
		deleteLine := strings.Replace(line, "-A PREROUTING", "-D PREROUTING", 1)
		args := append([]string{"-t", table}, strings.Fields(deleteLine)...)
		_ = it.runIPTables(ctx, args...)
	}
}

func (it *IPTables) IsInstalled(ctx context.Context) bool {
	return it.runIPTables(ctx, "-t", "mangle", "-nL", ChainName) == nil
}
