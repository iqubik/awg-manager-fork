package signature

import (
	"regexp"
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
	if result.ByteSize != len(result.Packets.I1)+len(result.Packets.I2)+len(result.Packets.I3)+len(result.Packets.I4)+len(result.Packets.I5) {
		t.Fatalf("ByteSize = %d, want exact CPS string sum", result.ByteSize)
	}
	if result.ByteSize > maxSignatureSize {
		t.Fatalf("ByteSize = %d, exceeds max", result.ByteSize)
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
