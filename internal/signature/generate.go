package signature

import (
	crand "crypto/rand"
	"encoding/binary"
	hexenc "encoding/hex"
	"errors"
	"fmt"
	mrand "math/rand"
	"regexp"
	"strconv"
	"strings"
)

// Port of the browser CPS generator that backs the ASC editor's
// "Протокол → Сгенерировать" button (frontend/src/lib/utils/awg-architect/
// generator.ts + signature.ts). The output is intentionally non-deterministic
// (same as the GUI, which yields a fresh chain on every click), so parity with
// the frontend is structural — identical tag layout, byte-size accounting and
// the 4096-byte ceiling — not byte-for-byte.
//
// This mirrors generateSignaturePackets() with its fixed baseGeneratorInput:
// version 2.0 (CPS enabled), intensity medium (iv=2), <c> off, <t>/<r>/<rc>/<rd>
// on, browser fingerprinting off (so every padding uses the entropy path), no
// router mode and no mimicAll. Composite profiles (tls_to_quic/quic_burst) are
// not exposed by the API, so only the single-profile path is ported.

// MaxSignatureBytes is the GUI ceiling for the summed I1–I5 payload
// (MAX_SIGNATURE_BYTES in ASCEditor.svelte).
const MaxSignatureBytes = 4096

// maxGenerateAttempts bounds the retry loop when a random chain overshoots
// MaxSignatureBytes. Without browser fingerprinting the padding stays modest,
// so an overshoot is rare and a handful of redraws is plenty.
const maxGenerateAttempts = 4

// defaultGenerateMTU matches generateSignaturePackets(protocol, mtu=1280).
const defaultGenerateMTU = 1280

// minGenerateMTU is the floor an explicit MTU is clamped up to. Below the classic
// 576-byte minimum the padding-range math (e.g. mkDNS rnd(64, mtu-20)) inverts,
// and such a tiny MTU is meaningless for a WireGuard tunnel anyway.
const minGenerateMTU = 576

// ivMedium is the intensity value for the fixed "medium" intensity (imap.medium).
const ivMedium = 2

// ErrUnknownProtocol is returned when the requested protocol key is not one of
// the supported mimicry profiles.
var ErrUnknownProtocol = errors.New("unknown protocol")

// ErrPacketsTooLarge is returned when every retry produced a chain exceeding
// MaxSignatureBytes.
var ErrPacketsTooLarge = errors.New("signature packets exceed size limit")

// GeneratedPackets holds the CPS I1–I5 signature chain.
type GeneratedPackets struct {
	I1 string
	I2 string
	I3 string
	I4 string
	I5 string
}

// SupportedProtocols lists the accepted protocol keys (as in the ASCEditor
// dropdown / issue #422). "tls" is accepted as an alias of "tls_client_hello".
var SupportedProtocols = []string{
	"quic_initial", "quic_0rtt", "tls_client_hello", "wireguard_noise",
	"dtls", "http3", "sip", "dns_query",
}

// CanonicalProtocol returns the canonical mimicry-profile key for an incoming
// protocol string (resolving the "tls" alias), or "" if unsupported.
func CanonicalProtocol(p string) string {
	return normalizeProtocol(p)
}

// normalizeProtocol maps an incoming key to its canonical mimicry profile,
// returning "" when unsupported. Mirrors LEGACY_TO_MIMIC for the "tls" alias.
func normalizeProtocol(p string) string {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "quic_initial":
		return "quic_initial"
	case "quic_0rtt":
		return "quic_0rtt"
	case "tls", "tls_client_hello":
		return "tls_client_hello"
	case "wireguard_noise":
		return "wireguard_noise"
	case "dtls":
		return "dtls"
	case "http3":
		return "http3"
	case "sip":
		return "sip"
	case "dns_query":
		return "dns_query"
	default:
		return ""
	}
}

// Generate produces an I1–I5 chain for the given protocol, retrying until the
// summed byte size fits MaxSignatureBytes. It returns the packets and that
// size. mtu <= 0 falls back to defaultGenerateMTU.
func Generate(protocol string, mtu int) (GeneratedPackets, int, error) {
	return generate(protocol, mtu, newCryptoSeededRand())
}

// generate is the testable core: it takes an explicit RNG so tests can pin the
// stream. r must be non-nil.
func generate(protocol string, mtu int, r *mrand.Rand) (GeneratedPackets, int, error) {
	profile := normalizeProtocol(protocol)
	if profile == "" {
		return GeneratedPackets{}, 0, ErrUnknownProtocol
	}
	if mtu <= 0 {
		mtu = defaultGenerateMTU
	} else if mtu < minGenerateMTU {
		mtu = minGenerateMTU
	}

	g := &generator{r: r, mtu: mtu}
	var lastSize int
	for attempt := 0; attempt < maxGenerateAttempts; attempt++ {
		packets := g.buildChain(profile)
		size := TotalByteSize(packets)
		if size <= MaxSignatureBytes {
			return packets, size, nil
		}
		lastSize = size
	}
	return GeneratedPackets{}, lastSize, fmt.Errorf(
		"%w: %d > %d bytes after %d attempts",
		ErrPacketsTooLarge, lastSize, MaxSignatureBytes, maxGenerateAttempts)
}

// newCryptoSeededRand builds a math/rand generator seeded from crypto/rand, so
// callers need not manage a global seed. The randomness here is obfuscation
// padding, not key material, so math/rand is appropriate.
func newCryptoSeededRand() *mrand.Rand {
	var seed [8]byte
	if _, err := crand.Read(seed[:]); err != nil {
		// crypto/rand should not fail; fall back to a fixed seed rather than panic.
		return mrand.New(mrand.NewSource(1))
	}
	return mrand.New(mrand.NewSource(int64(binary.LittleEndian.Uint64(seed[:]))))
}

// generator carries the RNG and MTU through one build. iv is fixed at ivMedium.
type generator struct {
	r   *mrand.Rand
	mtu int
}

// buildChain assembles I1–I5: I1 is the protocol-specific packet, I2–I5 are
// entropy packets (mimicAll is off in the fixed input).
func (g *generator) buildChain(profile string) GeneratedPackets {
	return GeneratedPackets{
		I1: g.genI1(profile),
		I2: g.mkEntropy(1),
		I3: g.mkEntropy(2),
		I4: g.mkEntropy(3),
		I5: g.mkEntropy(4),
	}
}

func (g *generator) genI1(profile string) string {
	switch profile {
	case "quic_initial":
		return g.mkQUICi()
	case "quic_0rtt":
		return g.mkQUIC0()
	case "tls_client_hello":
		return g.mkTLS()
	case "wireguard_noise":
		return g.mkNoise()
	case "dtls":
		return g.mkDTLS()
	case "http3":
		return g.mkHTTP3()
	case "sip":
		return g.mkSIP()
	case "dns_query":
		return g.mkDNS()
	default:
		return g.mkQUICi()
	}
}

// ── RNG / hex helpers (port of rnd, rh, hexPad, assertEvenHex, splitPad) ──────

// rnd returns a random int in [a, b] inclusive.
func (g *generator) rnd(a, b int) int {
	if b <= a {
		return a
	}
	return g.r.Intn(b-a+1) + a
}

// rh returns n bytes of random lowercase hex (2*n chars). One Intn(256) draw
// per byte, matching the TS rh() RNG consumption order.
func (g *generator) rh(n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = byte(g.r.Intn(256))
	}
	return hexenc.EncodeToString(buf)
}

// hexPad renders value as hex padded/truncated to exactly byteLen bytes.
func hexPad(value, byteLen int) string {
	if value < 0 {
		value = 0
	}
	h := strconv.FormatInt(int64(value), 16)
	width := byteLen * 2
	for len(h) < width {
		h = "0" + h
	}
	return h[len(h)-width:]
}

// assertEvenHex pads an odd-length hex string; a no-op on well-formed input.
func assertEvenHex(hex string) string {
	if len(hex)%2 != 0 {
		return hex + "0"
	}
	return hex
}

// splitPad renders n padding bytes as one or more <tag N> tokens, each ≤ 1000
// bytes — the AmneziaWG kernel per-tag limit.
func splitPad(n int, tag string) string {
	if n <= 0 {
		return ""
	}
	var sb strings.Builder
	for n > 1000 {
		fmt.Fprintf(&sb, "<%s 1000>", tag)
		n -= 1000
	}
	fmt.Fprintf(&sb, "<%s %d>", tag, n)
	return sb.String()
}

// tagOverhead is the fixed weight of <c>/<t>. In the fixed input <c> is off and
// <t> is on, so this is always 4, but keep it explicit for parity.
const tagOverhead = 4 // useTagT on (4), useTagC off (0)

// calcPaddingEntropy is calcPadding on the fp=null path (browser fingerprinting
// is off in the fixed input): min(rnd(20,80)*iv, 500, maxPad).
func (g *generator) calcPaddingEntropy(headerB, extraB int) int {
	maxPad := g.mtu - headerB - extraB
	if maxPad < 0 {
		maxPad = 0
	}
	return min(g.rnd(20, 80)*ivMedium, 500, maxPad)
}

func (g *generator) host(poolKey string) string {
	key := poolKey
	if key == "dns_query" {
		key = "dns"
	}
	pool, ok := hostPools[key]
	if !ok || len(pool) == 0 {
		pool = hostPools["tls_client_hello"]
	}
	return pool[g.rnd(0, len(pool)-1)]
}

// ── Protocol generators (I1) ─────────────────────────────────────────────────

func (g *generator) mkQUICi() string {
	host := g.host("quic_initial")
	dcid := g.rnd(8, 20)
	scid := g.rnd(0, 20)
	tokenLen := 0
	if g.rnd(0, 1) != 0 {
		tokenLen = g.rnd(8, 32)
	}
	sniRc := min(len(host)+g.rnd(0, 6), 64)

	hex := assertEvenHex(
		hexPad(0xc0|g.rnd(0, 3), 1) +
			"00000001" +
			hexPad(dcid, 1) + g.rh(dcid) +
			hexPad(scid, 1) + g.rh(scid) +
			hexPad(tokenLen, 1) + g.rh(tokenLen) +
			g.rh(4))

	headerB := len(hex) / 2
	extraB := sniRc + tagOverhead
	pad := g.calcPaddingEntropy(headerB, extraB)

	return "<b 0x" + hex + ">" +
		fmt.Sprintf("<rc %d>", sniRc) +
		"<t>" +
		splitPad(pad, "r")
}

func (g *generator) mkQUIC0() string {
	host := g.host("quic_0rtt")
	dcid := g.rnd(8, 20)
	scid := g.rnd(0, 20)
	ticketHint := min(len(host)+g.rnd(4, 16), 48)

	hex := assertEvenHex(
		hexPad(0xd0|g.rnd(0, 3), 1) +
			"00000001" +
			hexPad(dcid, 1) + g.rh(dcid) +
			hexPad(scid, 1) + g.rh(scid) +
			g.rh(4))

	headerB := len(hex) / 2
	extraB := ticketHint + tagOverhead
	pad := g.calcPaddingEntropy(headerB, extraB)

	return "<b 0x" + hex + ">" +
		"<t>" +
		splitPad(pad, "r") +
		fmt.Sprintf("<rc %d>", ticketHint)
}

func (g *generator) mkTLS() string {
	host := g.host("tls_client_hello")
	sniExt := 2 + 2 + 2 + 1 + 2 + len(host)
	sniRc := min(sniExt, 64)

	baseLen := g.rnd(300, 550)
	recLen := baseLen // non-Chromium profile: no 128-byte alignment
	hsLen := recLen - g.rnd(4, 9)

	rLen := min(
		g.rnd(20, 60)*ivMedium,
		300,
		max(0, g.mtu-44-sniRc-tagOverhead))

	hex := assertEvenHex(
		"160301" +
			hexPad(recLen, 2) +
			"01" +
			hexPad(hsLen, 3) +
			"0303" +
			g.rh(32))

	return "<b 0x" + hex + ">" +
		fmt.Sprintf("<rc %d>", sniRc) +
		splitPad(rLen, "r") +
		"<t>"
}

func (g *generator) mkNoise() string {
	rcLen := g.rnd(4, 12)
	headerB := 148 // Noise_IK fixed size

	extraB := rcLen + tagOverhead
	pad := min(g.rnd(10, 40)*ivMedium, 200, max(0, g.mtu-headerB-extraB))

	return "<b 0x01000000" + g.rh(4) + ">" +
		"<b 0x" + g.rh(32) + ">" +
		"<b 0x" + g.rh(48) + ">" +
		"<b 0x" + g.rh(28) + ">" +
		"<b 0x" + g.rh(32) + ">" +
		splitPad(pad, "r") +
		"<t>" +
		fmt.Sprintf("<rc %d>", rcLen)
}

func (g *generator) mkDTLS() string {
	host := g.host("dtls")
	fragLen := g.rnd(100, 300)
	sniRc := min(len(host)+g.rnd(2, 8), 60)
	epoch := g.rnd(0, 255)

	hex := assertEvenHex(
		"16" +
			"fefd" +
			hexPad(epoch, 2) +
			g.rh(6) +
			hexPad(fragLen, 2) +
			"01" +
			g.rh(6) +
			"fefd0000" +
			g.rh(4) +
			g.rh(32))

	headerB := len(hex) / 2
	extraB := sniRc + tagOverhead
	pad := g.calcPaddingEntropy(headerB, extraB)

	return "<b 0x" + hex + ">" +
		fmt.Sprintf("<rc %d>", sniRc) +
		"<t>" +
		splitPad(pad, "r")
}

func (g *generator) mkHTTP3() string {
	host := g.host("quic_initial")
	ptypes := []int{0xc0, 0xc1, 0xc2, 0xc3, 0xe0, 0xe1, 0xe2}
	dcid := g.rnd(8, 20)
	scid := g.rnd(0, 20)
	sniLen := min(len(host)+9+g.rnd(0, 6), 64)

	hex := assertEvenHex(
		hexPad(ptypes[g.rnd(0, len(ptypes)-1)], 1) +
			"00000001" +
			hexPad(dcid, 1) + g.rh(dcid) +
			hexPad(scid, 1) + g.rh(scid) +
			g.rh(4))

	headerB := len(hex) / 2
	extraB := sniLen + tagOverhead
	pad := g.calcPaddingEntropy(headerB, extraB)

	return "<b 0x" + hex + ">" +
		fmt.Sprintf("<rc %d>", sniLen) +
		splitPad(pad, "r") +
		"<t>"
}

func (g *generator) mkSIP() string {
	host := g.host("sip")
	hostHex := hexenc.EncodeToString([]byte(host))

	packet := assertEvenHex(
		"524547495354455220736970" + // "REGISTER sip"
			"3a" + // ":"
			hostHex +
			"20" + // " "
			g.rh(4))

	headerB := len(packet) / 2
	rcVal := min(len(host)+g.rnd(8, 24)*ivMedium, 150)
	rLen := min(
		g.rnd(5, 30)*ivMedium,
		120,
		max(0, g.mtu-headerB-rcVal-tagOverhead))

	return "<b 0x" + packet + ">" +
		fmt.Sprintf("<rc %d>", rcVal) +
		"<t>" +
		splitPad(rLen, "r")
}

func (g *generator) mkDNS() string {
	host := g.host("dns_query")

	var queryName strings.Builder
	for _, label := range strings.Split(host, ".") {
		fmt.Fprintf(&queryName, "%02x", len(label))
		queryName.WriteString(hexenc.EncodeToString([]byte(label)))
	}
	queryName.WriteString("00")

	// TS picks AAAA (001c) for odd iv, A (0001) for even; iv is fixed at medium (2).
	qtype := "0001"

	dnsQueryHex := g.rh(2) + // transaction ID
		"0100" + // flags: standard query, recursion desired
		"0001" + // 1 question
		"0000" + // 0 answers
		"0000" + // 0 authority
		"0000" + // 0 additional
		queryName.String() +
		qtype +
		"0001" // IN class

	hex := assertEvenHex(dnsQueryHex)
	headerB := len(hex) / 2
	targetSize := g.rnd(64, min(512, g.mtu-20))
	rLen := max(0, targetSize-headerB)

	out := "<b 0x" + hex + ">"
	if rLen > 0 {
		out += splitPad(min(rLen, 200), "r")
	}
	out += "<t>"
	return out
}

// mkEntropy is the port of mkEntropy for the fixed input (<c> off, <t>/<r>/<rc>/
// <rd> on, iv=2 so the first <b> block is present and the second is not).
func (g *generator) mkEntropy(idx int) string {
	isBig := g.rnd(1, 10) > 6
	baseLen := g.rnd(4, 20)
	capLimit := 60
	if isBig {
		baseLen = g.rnd(200, 500)
		capLimit = 500
	}
	rLen := min(baseLen*ivMedium, capLimit, max(0, g.mtu-20-tagOverhead))

	rcLen := g.rnd(4, 12)
	rdLen := g.rnd(4, 8)

	t := "<t>"
	r := splitPad(rLen, "r")
	rc := fmt.Sprintf("<rc %d>", rcLen)
	rd := fmt.Sprintf("<rd %d>", rdLen)
	b := "<b 0x" + g.rh(g.rnd(4, 8*ivMedium)) + ">"
	// c and b2 are empty in the fixed input (<c> off, iv < 3).

	patterns := []string{
		b + r + t + rc + rd,
		t + b + r + rc + rd,
		rc + b + r + t + rd,
		t + r + rc + b + rd,
		r + rc + b + t + rd,
		t + r + b + rc + rd,
		rd + b + rc + r + t,
		b + t + rc + r + rd,
	}

	result := patterns[(idx+g.rnd(0, len(patterns)-1))%len(patterns)]
	if result == "" {
		return "<r 10>"
	}
	return result
}

// ── byte-size accounting (port of calcByteSize / calcTotalSize) ──────────────

var cpsTagRe = regexp.MustCompile(`<(\w+)(?:\s+([^>]*))?>`)
var hexArgRe = regexp.MustCompile(`0x([0-9a-fA-F]*)`)

// ByteSize computes the payload byte size of a single CPS pattern, counting the
// hex inside <b>, the N of <r>/<rc>/<rd>, and 4 bytes each for <c>/<t>.
func ByteSize(pattern string) int {
	if pattern == "" {
		return 0
	}
	total := 0
	for _, m := range cpsTagRe.FindAllStringSubmatch(pattern, -1) {
		tag := strings.ToLower(m[1])
		arg := strings.TrimSpace(m[2])
		switch tag {
		case "b":
			if hm := hexArgRe.FindStringSubmatch(arg); hm != nil {
				total += len(hm[1]) / 2
			}
		case "r", "rc", "rd":
			if n, err := strconv.Atoi(arg); err == nil && n > 0 {
				total += n
			}
		case "c", "t":
			total += 4
		}
	}
	return total
}

// TotalByteSize sums ByteSize across I1–I5.
func TotalByteSize(p GeneratedPackets) int {
	return ByteSize(p.I1) + ByteSize(p.I2) + ByteSize(p.I3) + ByteSize(p.I4) + ByteSize(p.I5)
}

// CalcCPSByteSize is the public/helper name used by tests and API parity notes.
// It is an alias of ByteSize for one CPS packet pattern string.
func CalcCPSByteSize(pattern string) int {
	return ByteSize(pattern)
}

// CalcTotalSignatureByteSize sums the true byte sizes of arbitrary CPS packet
// strings. It mirrors the frontend helper naming used in issue #422.
func CalcTotalSignatureByteSize(packets ...string) int {
	total := 0
	for _, packet := range packets {
		total += CalcCPSByteSize(packet)
	}
	return total
}
