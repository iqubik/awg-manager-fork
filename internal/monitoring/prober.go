package monitoring

import (
	"context"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/hoaxisr/awg-manager/internal/sys/exec"
	"github.com/hoaxisr/awg-manager/internal/sys/httpclient"
)

// Prober probes a single host through a specific interface and returns
// latency in milliseconds + success flag. Implementations must be safe for
// concurrent use.
type Prober interface {
	Probe(ctx context.Context, host, ifaceName string, timeout time.Duration) (latencyMs int, ok bool)
}

// TCPProber probes via a bare TCP connect to host:443 with SO_BINDTODEVICE
// and reports the dial duration. The matrix metric has always been TCP RTT:
// the previous HTTPS HEAD prober measured `time_connect - time_namelookup`
// and discarded the TLS exchange — but on softfloat MIPS each discarded
// TLS handshake costs seconds of CPU, so idle routers burned most of their
// awg-manager CPU on throwaway handshakes every matrix tick.
//
// "Reachable" is defined as: TCP connect succeeded before the timeout
// (was: any HTTP status code). For the bare-IP base targets these are
// equivalent in practice. Hostname targets resolve inside the measured
// window, matching the old time_total fallback behaviour.
type TCPProber struct {
	// port is dialed on every probed host; defaults to 443 (overridable
	// in tests, where no fixed port can be listened on).
	port string
}

// NewTCPProber builds a prober dialing the conventional HTTPS port.
func NewTCPProber() *TCPProber {
	return &TCPProber{port: "443"}
}

// defaultRunner is preserved for ICMPProber only.
type defaultRunner struct{}

func (defaultRunner) Run(ctx context.Context, name string, args ...string) (*exec.Result, error) {
	return exec.Run(ctx, name, args...)
}

// Probe opens and immediately closes one TCP connection through ifaceName.
// ok=false on context cancellation or any dial error.
func (p *TCPProber) Probe(ctx context.Context, host, ifaceName string, timeout time.Duration) (int, bool) {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout+1*time.Second)
	defer cancel()

	port := p.port
	if port == "" {
		port = "443"
	}
	start := time.Now()
	conn, err := httpclient.DialTCP(timeoutCtx, ifaceName, net.JoinHostPort(host, port), timeout)
	if err != nil {
		return 0, false
	}
	_ = conn.Close()

	latencyMs := int(time.Since(start).Milliseconds())
	if latencyMs < 1 {
		latencyMs = 1
	}
	return latencyMs, true
}

// ICMPProber sends a single ICMP echo via Entware ping bound to the tunnel
// interface. Used for matrix cells whose target is the tunnel's
// connectivity-check self host AND the tunnel's method is "ping".
type ICMPProber struct {
	Runner runner
}

// runner is the subset of the old Runner interface still used by ICMPProber.
type runner interface {
	Run(ctx context.Context, name string, args ...string) (*exec.Result, error)
}

// NewICMPProber builds an ICMP prober backed by the package-level exec.Run.
func NewICMPProber() *ICMPProber {
	return &ICMPProber{Runner: defaultRunner{}}
}

// Probe sends a single ICMP echo. ok=false on exec error, non-zero exit
// code, or unparseable timing.
func (p *ICMPProber) Probe(ctx context.Context, host, ifaceName string, timeout time.Duration) (int, bool) {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout+1*time.Second)
	defer cancel()

	timeoutSec := int(timeout.Seconds())
	if timeoutSec < 1 {
		timeoutSec = 1
	}
	res, err := p.Runner.Run(timeoutCtx, "/opt/bin/ping",
		"-I", ifaceName,
		"-c", "1",
		"-W", strconv.Itoa(timeoutSec),
		host,
	)
	if err != nil || res == nil || res.ExitCode != 0 {
		return 0, false
	}

	// busybox ping may report timing on either stdout or stderr.
	if ms, ok := parsePingTime(res.Stdout); ok {
		return ms, true
	}
	if ms, ok := parsePingTime(res.Stderr); ok {
		return ms, true
	}
	// Exit 0 without parseable timing — treat as success with floor latency.
	return 1, true
}

// parsePingTime extracts the round-trip time in milliseconds from
// `time=NN.N ms` in ping output.
func parsePingTime(output string) (int, bool) {
	idx := strings.Index(output, "time=")
	if idx < 0 {
		return 0, false
	}
	rest := output[idx+5:]
	end := strings.IndexAny(rest, " m")
	if end <= 0 {
		return 0, false
	}
	val, err := strconv.ParseFloat(rest[:end], 64)
	if err != nil {
		return 0, false
	}
	ms := int(val)
	if ms < 1 {
		ms = 1
	}
	return ms, true
}
