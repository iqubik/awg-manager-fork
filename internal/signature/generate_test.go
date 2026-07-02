package signature

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var cpsPacketRe = regexp.MustCompile(`^((<b 0x[0-9a-f]+>)|(<(r|rc|rd) \d+>)|(<[ct]>))+$`)
var hexTagRe = regexp.MustCompile(`<b 0x([0-9a-f]+)>`)

func TestGenerate_ValidProtocol(t *testing.T) {
	result, err := Generate("quic_initial", 1280)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if !result.OK {
		t.Fatal("result.OK = false, want true")
	}
	if result.Source != "generated" {
		t.Fatalf("Source = %q, want generated", result.Source)
	}
	if result.Protocol != "quic_initial" {
		t.Fatalf("Protocol = %q, want quic_initial", result.Protocol)
	}
	if result.Packets.I1 == "" {
		t.Fatal("I1 is empty")
	}
	assertPacketFormat(t, result.Packets.I1)
	assertPacketFormat(t, result.Packets.I2)
	assertPacketFormat(t, result.Packets.I3)
	assertPacketFormat(t, result.Packets.I4)
	assertPacketFormat(t, result.Packets.I5)
	if result.ByteSize != CalcTotalSignatureByteSize(result.Packets.I1, result.Packets.I2, result.Packets.I3, result.Packets.I4, result.Packets.I5) {
		t.Fatalf("ByteSize = %d, want exact CPS byte size", result.ByteSize)
	}
	if result.ByteSize > maxSignatureSize {
		t.Fatalf("ByteSize = %d, exceeds max", result.ByteSize)
	}
}

func TestCalcCPSByteSize(t *testing.T) {
	got := CalcCPSByteSize("<b 0x0102><t><r 10><rc 3><rd 4>")
	if got != 23 {
		t.Fatalf("CalcCPSByteSize() = %d, want 23", got)
	}
}

func TestGenerate_TLSAliases(t *testing.T) {
	for _, protocol := range []string{"tls", "tls_client_hello"} {
		result, err := Generate(protocol, 1280)
		if err != nil {
			t.Fatalf("Generate(%q) error = %v", protocol, err)
		}
		if result.Protocol != "tls_client_hello" {
			t.Fatalf("Generate(%q) protocol = %q, want tls_client_hello", protocol, result.Protocol)
		}
		if !strings.Contains(result.Packets.I1, "<b 0x16") {
			t.Fatalf("Generate(%q) I1 = %q, want TLS-like record", protocol, result.Packets.I1)
		}
	}
}

func TestGenerate_InvalidProtocol(t *testing.T) {
	if _, err := Generate("bad_proto", 1280); err == nil {
		t.Fatal("Generate() error = nil, want invalid protocol")
	}
}

func TestGenerate_DefaultMTUWhenMissingOrZero(t *testing.T) {
	result, err := Generate("dns_query", 0)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result.Packets.I1 == "" {
		t.Fatal("I1 is empty")
	}
}

func TestGenerate_DefaultsDoNotUseBrowserFPRanges(t *testing.T) {
	result, err := Generate("quic_initial", 1280)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got := CalcCPSByteSize(result.Packets.I1); got >= 900 {
		t.Fatalf("I1 byte size = %d, expected non-BFP default packet under 900 bytes", got)
	}
}

func TestGenerate_TLSDefaultsDoNotUseBrowserFPOrChromiumAlign(t *testing.T) {
	for i := 0; i < 20; i++ {
		result, err := Generate("tls", 1280)
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}

		recordLen := extractTLSRecordLength(t, result.Packets.I1)
		if recordLen < 300 || recordLen > 550 {
			t.Fatalf("TLS record length = %d, want 300..550 for default non-BFP mode", recordLen)
		}
	}
}

func assertPacketFormat(t *testing.T, packet string) {
	t.Helper()
	if packet == "" {
		return
	}
	if !cpsPacketRe.MatchString(packet) {
		t.Fatalf("packet %q does not match CPS tag format", packet)
	}
	matches := hexTagRe.FindAllStringSubmatch(packet, -1)
	for _, m := range matches {
		if len(m[1])%2 != 0 {
			t.Fatalf("hex payload %q has odd length", m[1])
		}
	}
}

func extractTLSRecordLength(t *testing.T, packet string) int {
	t.Helper()

	const prefix = "<b 0x160301"
	if !strings.HasPrefix(packet, prefix) {
		t.Fatalf("packet %q does not start with TLS record prefix", packet)
	}
	if len(packet) < len(prefix)+4 {
		t.Fatalf("packet %q is too short to contain TLS record length", packet)
	}

	value, err := strconv.ParseInt(packet[len(prefix):len(prefix)+4], 16, 32)
	if err != nil {
		t.Fatalf("parse TLS record length from %q: %v", packet, err)
	}
	return int(value)
}
