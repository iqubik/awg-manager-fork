package subscription

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FetchOpts tunes the HTTP fetcher. Zero values produce defaults.
type FetchOpts struct {
	Timeout      time.Duration // default 20s
	MaxBodyBytes int64         // default 5 MiB
	UserAgent    string        // default "awg-manager"
}

// forbiddenHeaders are managed by Go's http client and cannot be set by users.
var forbiddenHeaders = map[string]bool{
	"host":              true,
	"content-length":    true,
	"connection":        true,
	"transfer-encoding": true,
	"upgrade":           true,
}

// Fetch GETs the URL with default + custom headers. Forbidden headers are
// silently skipped — they're managed by net/http. Body is capped at
// MaxBodyBytes (5 MiB default) to defend against runaway providers.
func Fetch(url string, headers []Header, opts FetchOpts) ([]byte, string, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 20 * time.Second
	}
	if opts.MaxBodyBytes == 0 {
		opts.MaxBodyBytes = 5 * 1024 * 1024
	}
	if opts.UserAgent == "" {
		opts.UserAgent = "awg-manager"
	}

	client := &http.Client{
		Timeout: opts.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", opts.UserAgent)
	for _, h := range headers {
		if forbiddenHeaders[strings.ToLower(h.Name)] {
			continue
		}
		req.Header.Set(h.Name, h.Value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("fetch: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, opts.MaxBodyBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(body)) > opts.MaxBodyBytes {
		return nil, "", fmt.Errorf("fetch: body exceeds %d bytes", opts.MaxBodyBytes)
	}
	return body, resp.Header.Get("Content-Type"), nil
}
