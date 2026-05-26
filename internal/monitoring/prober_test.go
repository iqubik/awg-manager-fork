package monitoring

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/sys/exec"
	"github.com/hoaxisr/awg-manager/internal/sys/httpclient"
)

type stubDoer struct {
	result *httpclient.Result
	err    error
}

func (s stubDoer) Do(_ context.Context, _ httpclient.CallConfig) (*httpclient.Result, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

// HTTPProber latency = (time_connect - time_namelookup) * 1000 ms.
// TimeConnect and TimeNameLookup are cumulative-from-start (matching curl
// semantics), so the subtraction yields pure TCP RTT.
func TestHTTPProber_ParseLatency(t *testing.T) {
	cases := []struct {
		name   string
		result *httpclient.Result
		err    error
		wantOK bool
		wantMs int
	}{
		{
			name: "ok 200, TCP RTT 12ms",
			result: &httpclient.Result{
				Metrics: httpclient.Metrics{HTTPCode: 200, TimeNameLookup: 0.001, TimeConnect: 0.013, TimeTotal: 0.020},
			},
			wantOK: true,
			wantMs: 12,
		},
		{
			name: "ok with 404 still reachable, TCP RTT 25ms",
			result: &httpclient.Result{
				Metrics: httpclient.Metrics{HTTPCode: 404, TimeNameLookup: 0.002, TimeConnect: 0.027, TimeTotal: 0.030},
			},
			wantOK: true,
			wantMs: 25,
		},
		{
			name: "no response — code 0",
			result: &httpclient.Result{
				Metrics: httpclient.Metrics{HTTPCode: 0, TimeNameLookup: 0, TimeConnect: 0, TimeTotal: 5.0},
			},
			wantOK: false,
		},
		{
			name: "DNS slow — cumulative timings preserve correct RTT",
			result: &httpclient.Result{
				Metrics: httpclient.Metrics{HTTPCode: 200, TimeNameLookup: 0.200, TimeConnect: 0.280, TimeTotal: 0.390},
			},
			wantOK: true,
			wantMs: 80,
		},
		{
			name: "fallback to time_total when timings invalid",
			result: &httpclient.Result{
				Metrics: httpclient.Metrics{HTTPCode: 200, TimeNameLookup: 0.020, TimeConnect: 0.010, TimeTotal: 0.030},
			},
			wantOK: true,
			wantMs: 30,
		},
		{
			name:   "exec error",
			err:    errors.New("boom"),
			wantOK: false,
		},
		{
			name:   "garbage output (nil result)",
			result: nil,
			wantOK: false,
		},
		{
			name: "non-numeric code (treated as 0)",
			result: &httpclient.Result{
				Metrics: httpclient.Metrics{HTTPCode: 0, TimeNameLookup: 0.001, TimeConnect: 0.013, TimeTotal: 0.020},
			},
			wantOK: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := &HTTPProber{Doer: stubDoer{result: c.result, err: c.err}}
			ms, ok := p.Probe(context.Background(), "1.1.1.1", "wg0", 5*time.Second)
			if ok != c.wantOK {
				t.Errorf("ok = %v, want %v", ok, c.wantOK)
			}
			if c.wantOK && ms != c.wantMs {
				t.Errorf("latency = %d, want %d", ms, c.wantMs)
			}
		})
	}
}

// runnerStub is retained for ICMPProber tests.
type runnerStub struct {
	stdout   string
	exitCode int
	err      error
}

func (s runnerStub) Run(_ context.Context, _ string, _ ...string) (*exec.Result, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &exec.Result{Stdout: s.stdout, ExitCode: s.exitCode}, nil
}

// ICMPProber parses `time=NN.N ms` from busybox ping output.
func TestICMPProber_ParseLatency(t *testing.T) {
	cases := []struct {
		name     string
		stdout   string
		exitCode int
		err      error
		wantOK   bool
		wantMs   int
	}{
		{
			name:     "stdout with time=14.2 ms",
			stdout:   "PING 1.1.1.1\n64 bytes from 1.1.1.1: time=14.2 ms",
			exitCode: 0,
			wantOK:   true,
			wantMs:   14,
		},
		{
			name:     "exit code != 0 means failure",
			stdout:   "request timeout",
			exitCode: 1,
			wantOK:   false,
		},
		{
			name:     "exit 0 without timing — floor latency 1ms",
			stdout:   "PING 8.8.8.8\n64 bytes from 8.8.8.8",
			exitCode: 0,
			wantOK:   true,
			wantMs:   1,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := &ICMPProber{Runner: runnerStub{stdout: c.stdout, exitCode: c.exitCode, err: c.err}}
			ms, ok := p.Probe(context.Background(), "1.1.1.1", "wg0", 5*time.Second)
			if ok != c.wantOK {
				t.Errorf("ok = %v, want %v", ok, c.wantOK)
			}
			if c.wantOK && ms != c.wantMs {
				t.Errorf("latency = %d, want %d", ms, c.wantMs)
			}
		})
	}
}
