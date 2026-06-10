package monitoring

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/sys/exec"
)

// TCPProber: ok=true + positive latency on successful connect, ok=false on
// refused/unreachable. No interface binding in tests (empty ifaceName).
func TestTCPProber_ConnectSuccess(t *testing.T) {
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

	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	p := &TCPProber{port: port}
	ms, ok := p.Probe(context.Background(), "127.0.0.1", "", 5*time.Second)
	if !ok {
		t.Fatal("Probe() ok = false, want true for listening port")
	}
	if ms < 1 {
		t.Errorf("latency = %d, want >= 1", ms)
	}
}

func TestTCPProber_ConnectRefused(t *testing.T) {
	// Grab a free port and close the listener so the connect is refused.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	ln.Close()

	p := &TCPProber{port: port}
	if _, ok := p.Probe(context.Background(), "127.0.0.1", "", 2*time.Second); ok {
		t.Fatal("Probe() ok = true, want false for closed port")
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
