package diagnostics

import (
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/logging"
)

func TestAnonymize_MACinWANIPAddr(t *testing.T) {
	report := Report{
		WAN: WANInfo{
			IPAddr: "11: eth3: <BROADCAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default qlen 1000\n    link/ether a0:21:aa:be:40:58 brd ff:ff:ff:ff:ff:ff\n",
		},
	}

	anonymize(&report)

	if strings.Contains(report.WAN.IPAddr, "a0:21:aa:be:40:58") {
		t.Fatalf("real MAC still present in IPAddr:\n%s", report.WAN.IPAddr)
	}
	if !strings.Contains(report.WAN.IPAddr, "a0:21:**:**:**:58") {
		t.Fatalf("masked MAC not found in IPAddr:\n%s", report.WAN.IPAddr)
	}
	if strings.Contains(report.WAN.IPAddr, "ff:ff:ff:ff:ff:ff") {
		t.Fatalf("broadcast MAC still present in IPAddr:\n%s", report.WAN.IPAddr)
	}
	if !strings.Contains(report.WAN.IPAddr, "ff:ff:**:**:**:ff") {
		t.Fatalf("masked broadcast MAC not found in IPAddr:\n%s", report.WAN.IPAddr)
	}
}

func TestAnonymize_WGPublicKeyInNDMSRawOutput(t *testing.T) {
	raw := `{"wireguard":{"public-key":"f43p1w9IIpUvqe9m1IrUcLbVrMfgrXRgShPEu7X78Ec="}}`
	report := Report{
		Tunnels: []TunnelInfo{
			{
				Interface: IfaceInfo{
					NDMSState: "",
				},
				Connection: ConnectionInfo{
					RawOutput: raw,
				},
			},
		},
	}

	anonymize(&report)

	result := report.Tunnels[0].Connection.RawOutput
	if strings.Contains(result, "f43p1w9IIpUvqe9m1IrUcLbVrMfgrXRgShPEu7X78Ec=") {
		t.Fatalf("real WG public key still present:\n%s", result)
	}
	// maskWGKey keeps first 6 and last 6 chars, with 4 * in-between.
	// key[:6]="f43p1w", key[-6:]="X78Ec=", so expected masked = "f43p1w****X78Ec=".
	if !strings.Contains(result, "f43p1w****") {
		t.Fatalf("masked WG key prefix not found:\n%s", result)
	}
	if !strings.Contains(result, "X78Ec=") {
		t.Fatalf("masked WG key suffix not found:\n%s", result)
	}
}

func TestAnonymize_MACDedupAcrossFields(t *testing.T) {
	expectedMask := "a0:21:**:**:**:58" // maskMAC("a0:21:aa:be:40:58")

	report := Report{
		WAN: WANInfo{
			IPAddr: "link/ether a0:21:aa:be:40:58\n",
		},
		Logs: []logging.LogEntry{
			{
				Target:  "system",
				Message: "detected mac a0:21:aa:be:40:58 on eth3",
			},
		},
	}

	anonymize(&report)

	if !strings.Contains(report.WAN.IPAddr, expectedMask) {
		t.Fatalf("expected masked MAC %q not found in WAN.IPAddr:\n%s", expectedMask, report.WAN.IPAddr)
	}
	if !strings.Contains(report.Logs[0].Message, expectedMask) {
		t.Fatalf("expected masked MAC %q not found in Logs[0].Message:\n%s", expectedMask, report.Logs[0].Message)
	}
	// Both occurrences must carry the identical mask string.
	wanHasMask := strings.Contains(report.WAN.IPAddr, expectedMask)
	logHasMask := strings.Contains(report.Logs[0].Message, expectedMask)
	if !wanHasMask || !logHasMask {
		t.Fatalf("same MAC not masked identically: wan=%v log=%v", wanHasMask, logHasMask)
	}
}

func TestAnonymize_PublicIPInTestDetail(t *testing.T) {
	report := Report{
		Tests: []TestResult{
			{
				Name:   "tunnel_connectivity",
				Detail: "IP: 95.25.93.179 (via https://ifconfig.me)",
			},
		},
	}

	anonymize(&report)

	if strings.Contains(report.Tests[0].Detail, "95.25.93.179") {
		t.Fatalf("public IP still present: %s", report.Tests[0].Detail)
	}
	if !strings.Contains(report.Tests[0].Detail, "PUBLIC-IP-") {
		t.Fatalf("public IP alias not found: %s", report.Tests[0].Detail)
	}
}

func TestAnonymize_DoesNotAliasDefaultRouteMarkers(t *testing.T) {
	report := Report{
		Tunnels: []TunnelInfo{
			{
				ConfigFile: "AllowedIPs = 0.0.0.0/0, ::/0\n",
			},
		},
		WAN: WANInfo{
			IPAddr: "inet6 fe80::a221:aaff:febe:4058/64 scope link\n",
		},
	}

	anonymize(&report)

	if !strings.Contains(report.Tunnels[0].ConfigFile, "0.0.0.0/0") {
		t.Fatalf("0.0.0.0/0 was unexpectedly anonymized: %s", report.Tunnels[0].ConfigFile)
	}
	if !strings.Contains(report.Tunnels[0].ConfigFile, "::/0") {
		t.Fatalf("::/0 was unexpectedly anonymized: %s", report.Tunnels[0].ConfigFile)
	}
	if strings.Contains(report.WAN.IPAddr, "PUBLIC-IP") {
		t.Fatalf("link-local IPv6 was corrupted by anonymizer: %s", report.WAN.IPAddr)
	}
	if !strings.Contains(report.WAN.IPAddr, "fe80::a221:aaff:febe:4058/64") {
		t.Fatalf("link-local IPv6 changed unexpectedly: %s", report.WAN.IPAddr)
	}
}

func TestAnonymize_PublicIPsInStructuredTunnelSettings(t *testing.T) {
	report := Report{
		Tunnels: []TunnelInfo{
			{
				Settings: TunnelSettings{
					DNS: "172.29.172.254, 1.0.0.1",
					PingCheckConfig: &PingCheckConfig{
						Enabled: true,
						Method:  "icmp",
						Target:  "8.8.8.8",
					},
				},
				Routes: RouteInfo{
					EndpointRoute: "95.25.93.179 via 192.168.1.1 dev eth3",
				},
			},
		},
	}

	anonymize(&report)

	out := report.Tunnels[0].Settings.DNS + " " +
		report.Tunnels[0].Settings.PingCheckConfig.Target + " " +
		report.Tunnels[0].Routes.EndpointRoute

	if strings.Contains(out, "1.0.0.1") {
		t.Fatalf("public DNS IP still present: %s", out)
	}
	if strings.Contains(out, "8.8.8.8") {
		t.Fatalf("ping target public IP still present: %s", out)
	}
	if strings.Contains(out, "95.25.93.179") {
		t.Fatalf("route public IP still present: %s", out)
	}
	if !strings.Contains(out, "PUBLIC-IP-") {
		t.Fatalf("public IP aliases not found: %s", out)
	}
	if !strings.Contains(out, "172.29.172.254") {
		t.Fatalf("private IP should be preserved: %s", out)
	}
}
