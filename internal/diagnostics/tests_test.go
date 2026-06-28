package diagnostics

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/singbox"
)

func TestRunOptions_RestartCycleOnlyWhenIncludeRestart(t *testing.T) {
	// Contract: restart_cycle running depends ONLY on opts.IncludeRestart.
	// Mode (Quick/Full) был выпилен; остаётся только это правило.
	cases := []struct {
		name               string
		opts               RunOptions
		wantIncludeRestart bool
	}{
		{"no-restart", RunOptions{IncludeRestart: false}, false},
		{"with-restart", RunOptions{IncludeRestart: true}, true},
	}

	for _, c := range cases {
		derived := c.opts.IncludeRestart
		if derived != c.wantIncludeRestart {
			t.Errorf("%s: derived includeRestart=%v, want %v", c.name, derived, c.wantIncludeRestart)
		}
	}
}

func TestBootHealth_GraceNotElapsed(t *testing.T) {
	// daemon только что стартовал — grace ещё не вышел, NotStartedOnBoot пусто.
	old := processStartedAt
	defer func() { processStartedAt = old }()
	processStartedAt = time.Now() // 0 секунд назад

	bh := computeBootHealth(
		[]bootHealthInput{
			{ID: "wg1", Name: "wg1", Backend: "kernel", Enabled: true, AutoStart: true,
				Status: "stopped", StoredStartedAt: ""},
		},
	)

	if bh.DaemonUptimeSec >= bh.GracePeriodSec {
		t.Fatalf("test setup invalid: uptime %d >= grace %d",
			bh.DaemonUptimeSec, bh.GracePeriodSec)
	}
	if len(bh.NotStartedOnBoot) != 0 {
		t.Errorf("expected empty NotStartedOnBoot during grace, got %v", bh.NotStartedOnBoot)
	}
}

func TestBootHealth_GraceElapsed_NeverStarted(t *testing.T) {
	// grace вышел, enabled+autoStart-туннель не running => never_started issue.
	old := processStartedAt
	defer func() { processStartedAt = old }()
	processStartedAt = time.Now().Add(-200 * time.Second) // > 120 grace

	bh := computeBootHealth(
		[]bootHealthInput{
			{ID: "wg1", Name: "wg1", Backend: "kernel", Enabled: true, AutoStart: true,
				Status: "stopped", StoredStartedAt: ""},
			{ID: "wg2", Name: "wg2", Backend: "nativewg", Enabled: true, AutoStart: true,
				Status: "running", StoredStartedAt: time.Now().Format(time.RFC3339)},
		},
	)

	if got := bh.GracePeriodSec; got != 120 {
		t.Errorf("GracePeriodSec=%d, want 120", got)
	}
	if got := len(bh.ExpectedRunning); got != 2 {
		t.Errorf("ExpectedRunning len=%d, want 2", got)
	}
	if got := len(bh.ActualRunning); got != 1 || bh.ActualRunning[0] != "wg2" {
		t.Errorf("ActualRunning=%v, want [wg2]", bh.ActualRunning)
	}
	if got := len(bh.NotStartedOnBoot); got != 1 {
		t.Fatalf("NotStartedOnBoot len=%d, want 1", got)
	}
	issue := bh.NotStartedOnBoot[0]
	if issue.TunnelID != "wg1" || issue.Reason != "never_started" {
		t.Errorf("issue=%+v, want id=wg1 reason=never_started", issue)
	}
}

func TestBootHealth_GraceElapsed_AllRunning(t *testing.T) {
	// grace вышел, всё что должно — running => NotStartedOnBoot пуст.
	old := processStartedAt
	defer func() { processStartedAt = old }()
	processStartedAt = time.Now().Add(-200 * time.Second)

	bh := computeBootHealth(
		[]bootHealthInput{
			{ID: "wg1", Name: "wg1", Backend: "kernel", Enabled: true, AutoStart: true, Status: "running"},
		},
	)
	if len(bh.NotStartedOnBoot) != 0 {
		t.Errorf("expected empty NotStartedOnBoot, got %v", bh.NotStartedOnBoot)
	}
}

func TestBootHealth_DisabledTunnelExcluded(t *testing.T) {
	// Disabled-туннели НЕ должны попадать в ExpectedRunning.
	old := processStartedAt
	defer func() { processStartedAt = old }()
	processStartedAt = time.Now().Add(-200 * time.Second)

	bh := computeBootHealth(
		[]bootHealthInput{
			{ID: "wg1", Name: "wg1", Backend: "kernel", Enabled: false, AutoStart: false, Status: "stopped"},
		},
	)
	if len(bh.ExpectedRunning) != 0 {
		t.Errorf("ExpectedRunning=%v, want []", bh.ExpectedRunning)
	}
	if len(bh.NotStartedOnBoot) != 0 {
		t.Errorf("NotStartedOnBoot=%v, want []", bh.NotStartedOnBoot)
	}
}

func TestAnonymize_AWGProxyModule_MasksRawListIPs(t *testing.T) {
	report := &Report{
		AWGProxyModule: AWGProxyModule{
			Loaded:        true,
			Version:       "1.2",
			EndpointCount: 1,
			RawList:       "203.0.113.42:51820 -> 127.0.0.1:7891 rx=1024 tx=512\n",
			DmesgLines: []string{
				"[12345.678] awg_proxy: client at 127.0.0.1:7891",
				"[12345.679] awg_proxy: send to 198.51.100.5:443 failed: -110",
			},
		},
	}
	anonymize(report)

	if strings.Contains(report.AWGProxyModule.RawList, "203.0.113.42") {
		t.Errorf("RawList still contains public IP: %q", report.AWGProxyModule.RawList)
	}
	for _, line := range report.AWGProxyModule.DmesgLines {
		if strings.Contains(line, "198.51.100.5") {
			t.Errorf("DmesgLines still contains public IP: %q", line)
		}
	}
	// Private IPs (127.0.0.1) MUST remain — see isPrivateIP() contract.
	hasPrivate := false
	for _, line := range report.AWGProxyModule.DmesgLines {
		if strings.Contains(line, "127.0.0.1") {
			hasPrivate = true
		}
	}
	if !hasPrivate {
		t.Errorf("expected 127.0.0.1 to remain in dmesg lines (private IPs must not be masked)")
	}
}

func TestRouteDevFromIPRouteGet(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "via gateway",
			in:   "1.1.1.1 via 192.168.1.1 dev eth0 src 192.168.1.10 uid 0\n    cache",
			want: "eth0",
		},
		{
			name: "direct dev",
			in:   "10.8.0.1 dev wg0 src 10.8.0.2 uid 0\n    cache",
			want: "wg0",
		},
		{
			name: "local dev",
			in:   "local 127.0.0.1 dev lo src 127.0.0.1 uid 0",
			want: "lo",
		},
		{
			name: "no dev",
			in:   "unreachable 10.0.0.1",
			want: "",
		},
		{
			name: "empty",
			in:   "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := routeDevFromIPRouteGet(tt.in)
			if got != tt.want {
				t.Errorf("routeDevFromIPRouteGet(%q) = %q; want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestClassifyProxyFailure(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"timeout", errors.New("context deadline exceeded"), "timeout"},
		{"reset", errors.New("Recv failure: Connection reset by peer"), "connection reset"},
		{"proxy502", errors.New("proxy returned 502 Bad Gateway"), "proxy returned 502"},
		{"noRoute", errors.New("dial tcp 1.2.3.4:443: connect: no route to host"), "no route to host"},
		{"fallback", errors.New("some other error"), "request failed"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyProxyFailure(tc.err); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDNSDetoursForOutbound(t *testing.T) {
	r := NewRunner(Deps{
		SingboxConfigPreview: func() (string, error) {
			return `{
				"dns": {
					"servers": [
						{"tag":"dns-bootstrap","type":"udp","server":"1.1.1.1"},
						{"tag":"wizard-upstream","type":"tls","server":"9.9.9.9","detour":"sub-1a656a35"}
					]
				}
			}`, nil
		},
	})

	got := r.dnsDetoursForOutbound("sub-1a656a35")
	if len(got) != 1 || got[0] != "wizard-upstream" {
		t.Fatalf("got %#v, want [wizard-upstream]", got)
	}
}

func TestTestSingboxTunnelConnectivity_UrlTestGroupRowCreatedWithoutActiveKnown(t *testing.T) {
	ctx := context.Background()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port

	fakeSingbox := &fakeSingboxForDiag{
		status:  singbox.Status{Installed: true, Running: true, TunnelCount: 0},
		tunnels: []singbox.TunnelInfo{},
	}

	r := NewRunner(Deps{
		Singbox: fakeSingbox,
		SingboxSubMembers: func() []SingboxSubMember {
			return []SingboxSubMember{
				{
					Tag:         "member-de",
					GroupTag:    "sub-1a656a35",
					Mode:        "urltest",
					ListenPort:  port,
					Enabled:     true,
					ActiveKnown: false,
					Active:      false,
				},
				{
					Tag:         "member-fr",
					GroupTag:    "sub-1a656a35",
					Mode:        "urltest",
					ListenPort:  port,
					Enabled:     true,
					ActiveKnown: true,
					Active:      true,
				},
			}
		},
		SingboxConfigPreview: func() (string, error) {
			return "{}", nil
		},
	})

	results := r.testSingboxTunnelConnectivity(ctx)

	var stateRes *TestResult
	for i := range results {
		if results[i].Name == "singbox_tunnel_state" && results[i].TunnelID == "singbox:sub-1a656a35" {
			stateRes = &results[i]
			break
		}
	}

	if stateRes == nil {
		t.Fatalf("expected state result for group row sub-1a656a35, got results: %v", results)
	}

	if stateRes.Status == StatusSkip {
		t.Fatalf("group row must not be skipped due to ActiveKnown=false, got status=%v detail=%s", stateRes.Status, stateRes.Detail)
	}
	if stateRes.Status == StatusWarn && strings.Contains(stateRes.Detail, "активный member") {
		t.Fatalf("group row must not warn about active member, got: %s", stateRes.Detail)
	}

	for _, tr := range results {
		if tr.TunnelID == "singbox:member-fr" {
			t.Fatalf("urltest member-fr must not produce its own diagnostics row when Mode=urltest, got: %+v", tr)
		}
		if tr.TunnelID == "singbox:member-de" {
			t.Fatalf("urltest member-de must not produce its own diagnostics row when Mode=urltest, got: %+v", tr)
		}
	}
}

func TestTestSingboxTunnelConnectivity_GroupTargetUsesSubscriptionListenPort(t *testing.T) {
	ctx := context.Background()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port

	r := NewRunner(Deps{
		Singbox: &fakeSingboxForDiag{
			status: singbox.Status{Installed: true, Running: true, TunnelCount: 1},
			tunnels: []singbox.TunnelInfo{
				{Tag: "iq0", ListenPort: 0, Running: true},
			},
		},
		SingboxSubMembers: func() []SingboxSubMember {
			return []SingboxSubMember{
				{
					Tag:         "member-1",
					GroupTag:    "iq0",
					Mode:        "selector",
					ListenPort:  port,
					Enabled:     true,
					ActiveKnown: true,
					Active:      true,
				},
			}
		},
		SingboxConfigPreview: func() (string, error) {
			return "{}", nil
		},
	})

	results := r.testSingboxTunnelConnectivity(ctx)

	var stateRes *TestResult
	for i := range results {
		if results[i].Name == "singbox_tunnel_state" && results[i].TunnelID == "singbox:iq0" {
			stateRes = &results[i]
			break
		}
	}

	if stateRes == nil {
		t.Fatalf("expected state result for singbox:iq0, got results: %v", results)
	}

	if stateRes.Status != StatusPass {
		t.Fatalf("expected state result to pass via subscription group mapping, got status=%v detail=%s", stateRes.Status, stateRes.Detail)
	}
	if strings.Contains(stateRes.Detail, "Не задан listenPort") {
		t.Fatalf("expected subscription listenPort to replace raw group listenPort, got detail=%s", stateRes.Detail)
	}
	if !strings.Contains(stateRes.Detail, "127.0.0.1:") || !strings.Contains(stateRes.Detail, "local proxy") {
		t.Fatalf("expected mapped proxy port detail, got %s", stateRes.Detail)
	}
}

type fakeSingboxForDiag struct {
	status  singbox.Status
	tunnels []singbox.TunnelInfo
	err     error
}

func (f *fakeSingboxForDiag) GetStatus(ctx context.Context) singbox.Status {
	return f.status
}

func (f *fakeSingboxForDiag) ListTunnels(ctx context.Context) ([]singbox.TunnelInfo, error) {
	return f.tunnels, f.err
}
