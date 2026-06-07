package downloader

import (
	"context"
	"fmt"
	"net/http"
	"sync"
)

type Route struct {
	Tag  string
	Kind string
}

type Outbound struct {
	Tag       string
	Kind      string
	Label     string
	Detail    string
	Available bool
}

type RouteInfo struct {
	Tag    string
	Kind   string
	Label  string
	Detail string
}

func (r RouteInfo) DisplayName() string {
	if r.Tag == "" || r.Tag == "direct" {
		return "direct"
	}
	if r.Kind != "" {
		return fmt.Sprintf("%s (%s)", r.Tag, r.Kind)
	}
	return r.Tag
}

type Lease struct {
	Client  *http.Client
	Route   RouteInfo
	cleanup func()
	once    sync.Once
}

func (l *Lease) Close() {
	if l == nil || l.cleanup == nil {
		return
	}
	l.once.Do(l.cleanup)
}

type OutboundsProvider interface {
	ListDownloadOutbounds(ctx context.Context) []Outbound
}

type RouteProvider interface {
	GetDownloadRoute(ctx context.Context) (*Route, error)
}
