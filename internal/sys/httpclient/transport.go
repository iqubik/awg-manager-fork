package httpclient

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// TransportConfig configures a reusable *http.Transport for http.Client.
type TransportConfig struct {
	// Interface binds outgoing sockets to a kernel device (SO_BINDTODEVICE).
	Interface string

	// ProxyURL routes through an HTTP proxy, e.g. "http://127.0.0.1:1080".
	ProxyURL string

	// DNSServers are used for hostname resolution when Interface is set.
	DNSServers []string
}

// NewTransport returns an http.Transport with optional interface binding
// and/or HTTP proxy. Caller owns the returned value; clone per-client if
// needed.
func NewTransport(cfg TransportConfig) (*http.Transport, error) {
	var parsedProxy *url.URL
	if cfg.ProxyURL != "" {
		u, err := url.Parse(cfg.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("httpclient: invalid proxy URL %q: %w", cfg.ProxyURL, err)
		}
		parsedProxy = u
	}

	base := &http.Transport{
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		DisableKeepAlives:     true,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     false,
	}

	c := &Client{baseTransport: base}
	return c.buildTransport(CallConfig{
		Interface:      cfg.Interface,
		DNSServers:     cfg.DNSServers,
		ConnectTimeout: 10 * time.Second,
	}, parsedProxy), nil
}
