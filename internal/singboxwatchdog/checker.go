package singboxwatchdog

import (
	"context"
	"fmt"
	"time"

	"github.com/hoaxisr/awg-manager/internal/sys/httpclient"
)

const defaultTestURL = "http://www.gstatic.com/generate_204"
const defaultHTTPURL = "https://www.gstatic.com/generate_204"

type Clash interface {
	TestDelay(name, url string, timeout time.Duration) (int, error)
	SetSelector(selectorTag, memberTag string) error
	SelectorActive(selectorTag string) (string, error)
}

func CheckTarget(ctx context.Context, clash Clash, target RuntimeTarget, timeout time.Duration) CheckResult {
	if !target.Running {
		return CheckResult{Success: false, Error: "target stopped"}
	}

	if target.CheckTag != "" && clash != nil {
		if delay, err := clash.TestDelay(target.CheckTag, defaultTestURL, timeout); err == nil && delay > 0 {
			return CheckResult{Success: true, Latency: delay}
		}
	}

	if target.ListenPort <= 0 {
		return CheckResult{Success: false, Error: "listenPort missing"}
	}

	proxyURL := fmt.Sprintf("http://127.0.0.1:%d", target.ListenPort)
	res, err := httpclient.DefaultClient.Do(ctx, httpclient.CallConfig{
		URL:         defaultHTTPURL,
		ProxyURL:    proxyURL,
		MaxTime:     timeout,
		DiscardBody: true,
	})
	if err != nil {
		return CheckResult{Success: false, Error: err.Error()}
	}
	if res.Metrics.HTTPCode == 200 || res.Metrics.HTTPCode == 204 {
		return CheckResult{
			Success: true,
			Latency: int(res.Metrics.TimeTotal * 1000),
		}
	}
	return CheckResult{
		Success: false,
		Latency: int(res.Metrics.TimeTotal * 1000),
		Error:   fmt.Sprintf("unexpected HTTP code: %d", res.Metrics.HTTPCode),
	}
}
