package httpclient

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestEncodeDNSQuery(t *testing.T) {
	q, err := encodeDNSQuery("raw.githubusercontent.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(q) < 20 {
		t.Fatalf("query too short: %d", len(q))
	}
}

func TestParseDNSARecord_directA(t *testing.T) {
	pkt := buildDNSResponse(1, net.IPv4(1, 2, 3, 4).To4())
	ip, err := parseDNSARecord(t.Context(), pkt, "1.1.1.1", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if ip != "1.2.3.4" {
		t.Fatalf("ip = %q, want 1.2.3.4", ip)
	}
}

func buildDNSResponse(rrType uint16, rdata []byte) []byte {
	q := []byte{7, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 3, 'c', 'o', 'm', 0}
	ansName := []byte{0xc0, 0x0c}
	ansTail := make([]byte, 10+len(rdata))
	binary.BigEndian.PutUint16(ansTail[0:2], rrType)
	binary.BigEndian.PutUint16(ansTail[2:4], 1)
	binary.BigEndian.PutUint32(ansTail[4:8], 60)
	binary.BigEndian.PutUint16(ansTail[8:10], uint16(len(rdata)))
	copy(ansTail[10:], rdata)

	out := make([]byte, 12+len(q)+4+len(ansName)+len(ansTail))
	out[2] = 0x81
	binary.BigEndian.PutUint16(out[6:8], 1)
	copy(out[12:], q)
	off := 12 + len(q)
	binary.BigEndian.PutUint16(out[off:], 1)
	binary.BigEndian.PutUint16(out[off+2:], 1)
	off += 4
	copy(out[off:], ansName)
	off += len(ansName)
	copy(out[off:], ansTail)
	return out
}
