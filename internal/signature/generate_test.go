package signature

import (
	"encoding/hex"
	mrand "math/rand"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// deterministic RNG so the suite is stable.
func testRand() *mrand.Rand { return mrand.New(mrand.NewSource(42)) }

var bTagRe = regexp.MustCompile(`<b 0x([0-9a-fA-F]*)>`)
var padTagRe = regexp.MustCompile(`<(r|rc|rd) (\d+)>`)
var tlsRecordLenRe = regexp.MustCompile(`<b 0x160301([0-9a-fA-F]{4})`)

func allPackets(p GeneratedPackets) []string {
	return []string{p.I1, p.I2, p.I3, p.I4, p.I5}
}

func TestGenerate_AllProtocols(t *testing.T) {
	for _, proto := range SupportedProtocols {
		t.Run(proto, func(t *testing.T) {
			p, size, err := generate(proto, 1280, testRand())
			if err != nil {
				t.Fatalf("generate(%q): %v", proto, err)
			}
			if p.I1 == "" {
				t.Errorf("%q: I1 must be non-empty (required field)", proto)
			}
			if size <= 0 || size > MaxSignatureBytes {
				t.Errorf("%q: size=%d out of (0, %d]", proto, size, MaxSignatureBytes)
			}
			if got := TotalByteSize(p); got != size {
				t.Errorf("%q: returned size %d ≠ recomputed %d", proto, size, got)
			}
		})
	}
}

// Every <b> payload must be even-length hex (whole bytes) — the kernel rejects
// odd nibbles.
func TestGenerate_EvenHexInBTags(t *testing.T) {
	r := testRand()
	for _, proto := range SupportedProtocols {
		for i := 0; i < 50; i++ {
			p, _, err := generate(proto, 1280, r)
			if err != nil {
				t.Fatalf("%q: %v", proto, err)
			}
			for _, pkt := range allPackets(p) {
				for _, m := range bTagRe.FindAllStringSubmatch(pkt, -1) {
					if len(m[1])%2 != 0 {
						t.Fatalf("%q: odd hex in <b>: %q (len %d)", proto, m[1], len(m[1]))
					}
				}
			}
		}
	}
}

// No single <r>/<rc>/<rd> token may exceed 1000 bytes (AmneziaWG per-tag limit).
func TestGenerate_PadTagsWithinKernelLimit(t *testing.T) {
	r := testRand()
	for _, proto := range SupportedProtocols {
		for i := 0; i < 50; i++ {
			p, _, err := generate(proto, 9000, r) // large MTU to stress padding
			if err != nil {
				t.Fatalf("%q: %v", proto, err)
			}
			for _, pkt := range allPackets(p) {
				for _, m := range padTagRe.FindAllStringSubmatch(pkt, -1) {
					n, _ := strconv.Atoi(m[2])
					if n > 1000 {
						t.Fatalf("%q: pad tag <%s %d> exceeds 1000", proto, m[1], n)
					}
				}
			}
		}
	}
}

// Fixed input keeps <c> off — the tag breaks older clients (ErrorCode 1000).
func TestGenerate_NeverEmitsCTag(t *testing.T) {
	r := testRand()
	for _, proto := range SupportedProtocols {
		for i := 0; i < 30; i++ {
			p, _, err := generate(proto, 1280, r)
			if err != nil {
				t.Fatalf("%q: %v", proto, err)
			}
			for _, pkt := range allPackets(p) {
				if strings.Contains(pkt, "<c>") {
					t.Fatalf("%q: unexpected <c> tag in %q", proto, pkt)
				}
			}
		}
	}
}

func TestGenerate_UnknownProtocol(t *testing.T) {
	if _, _, err := generate("nope", 1280, testRand()); err != ErrUnknownProtocol {
		t.Fatalf("want ErrUnknownProtocol, got %v", err)
	}
}

func TestGenerate_TLSAlias(t *testing.T) {
	if CanonicalProtocol("tls") != "tls_client_hello" {
		t.Fatalf("tls must canonicalize to tls_client_hello, got %q", CanonicalProtocol("tls"))
	}
	if _, _, err := generate("tls", 1280, testRand()); err != nil {
		t.Fatalf("tls alias must generate: %v", err)
	}
	if CanonicalProtocol("QUIC_Initial") != "quic_initial" {
		t.Fatalf("case-insensitive normalization failed")
	}
	if CanonicalProtocol("bogus") != "" {
		t.Fatalf("unsupported must return empty")
	}
}

func TestGenerate_DefaultMTU(t *testing.T) {
	// mtu <= 0 must fall back to the default without error.
	if _, _, err := generate("quic_initial", 0, testRand()); err != nil {
		t.Fatalf("default MTU path: %v", err)
	}
	if _, _, err := generate("quic_initial", -5, testRand()); err != nil {
		t.Fatalf("negative MTU path: %v", err)
	}
}

// A pathologically small MTU must be clamped to minGenerateMTU so no padding
// range inverts (mkDNS rnd(64, mtu-20) would otherwise flip). Every protocol
// must still yield a valid, non-empty, in-limit chain.
func TestGenerate_TinyMTUClamped(t *testing.T) {
	r := testRand()
	for _, mtu := range []int{1, 20, 84, 200, 575} {
		for _, proto := range SupportedProtocols {
			p, size, err := generate(proto, mtu, r)
			if err != nil {
				t.Fatalf("mtu=%d %q: %v", mtu, proto, err)
			}
			if p.I1 == "" || size <= 0 || size > MaxSignatureBytes {
				t.Fatalf("mtu=%d %q: i1=%q size=%d", mtu, proto, p.I1, size)
			}
		}
	}
}

// The public Generate seeds its own RNG; smoke-test that it succeeds for all
// protocols and stays within the ceiling.
func TestGenerate_PublicEntrypoint(t *testing.T) {
	for _, proto := range SupportedProtocols {
		p, size, err := Generate(proto, 1280)
		if err != nil {
			t.Fatalf("Generate(%q): %v", proto, err)
		}
		if p.I1 == "" || size > MaxSignatureBytes {
			t.Fatalf("Generate(%q): i1=%q size=%d", proto, p.I1, size)
		}
	}
}

func TestByteSize(t *testing.T) {
	cases := []struct {
		pattern string
		want    int
	}{
		{"", 0},
		{"<b 0x0a1b2c3d>", 4}, // 8 hex nibbles → 4 bytes
		{"<r 100>", 100},      // padding count
		{"<rc 12><rd 8>", 20}, // rc + rd
		{"<t>", 4},            // fixed 4 bytes
		{"<c>", 4},            // fixed 4 bytes
		{"<b 0xffff><r 10><t>", 2 + 10 + 4},
		{"<r 0>", 0},  // non-positive ignored
		{"<r -5>", 0}, // negative ignored
		{"<b 0x>", 0}, // empty hex
	}
	for _, c := range cases {
		if got := ByteSize(c.pattern); got != c.want {
			t.Errorf("ByteSize(%q)=%d want %d", c.pattern, got, c.want)
		}
	}
}

func TestCalcCPSByteSize(t *testing.T) {
	if got := CalcCPSByteSize("<b 0x0102><t><r 10><rc 3><rd 4>"); got != 23 {
		t.Fatalf("CalcCPSByteSize() = %d, want 23", got)
	}
}

func TestGenerate_TLSDefaultsDoNotUseBrowserFPOrChromiumAlign(t *testing.T) {
	for i := 0; i < 20; i++ {
		p, _, err := Generate("tls", 1280)
		if err != nil {
			t.Fatalf("Generate(tls): %v", err)
		}
		m := tlsRecordLenRe.FindStringSubmatch(p.I1)
		if len(m) != 2 {
			t.Fatalf("I1 = %q, want TLS record length prefix", p.I1)
		}
		raw, err := hex.DecodeString(m[1])
		if err != nil || len(raw) != 2 {
			t.Fatalf("decode record length %q: %v", m[1], err)
		}
		recLen := int(raw[0])<<8 | int(raw[1])
		if recLen < 300 || recLen > 550 {
			t.Fatalf("record length = %d, want default non-BFP range 300..550", recLen)
		}
	}
}

// splitPad must chunk into ≤1000-byte tokens whose N sums to the input.
func TestSplitPad(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, ""},
		{-3, ""},
		{200, "<r 200>"},
		{1000, "<r 1000>"},
		{1200, "<r 1000><r 200>"},
		{2500, "<r 1000><r 1000><r 500>"},
	}
	for _, c := range cases {
		if got := splitPad(c.n, "r"); got != c.want {
			t.Errorf("splitPad(%d)=%q want %q", c.n, got, c.want)
		}
	}
}

func TestHexPad(t *testing.T) {
	cases := []struct {
		value, byteLen int
		want           string
	}{
		{0xc0, 1, "c0"},
		{1, 4, "00000001"},
		{0, 1, "00"},
		{0x1234, 1, "34"}, // truncates to last byte
	}
	for _, c := range cases {
		if got := hexPad(c.value, c.byteLen); got != c.want {
			t.Errorf("hexPad(%#x,%d)=%q want %q", c.value, c.byteLen, got, c.want)
		}
	}
}
