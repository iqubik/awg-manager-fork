package downloader

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

type Deps struct {
	Outbounds         OutboundsProvider
	TransportResolver TransportResolver
	RouteProvider     RouteProvider
}

type Service struct {
	depsMu    sync.RWMutex
	outbounds OutboundsProvider
	transport TransportResolver
	routeProv RouteProvider
}

func NewService(d Deps) *Service {
	return &Service{
		outbounds: d.Outbounds,
		transport: d.TransportResolver,
		routeProv: d.RouteProvider,
	}
}

func (s *Service) SetOutboundsProvider(p OutboundsProvider) {
	s.depsMu.Lock()
	defer s.depsMu.Unlock()
	s.outbounds = p
}

func (s *Service) SetTransportResolver(r TransportResolver) {
	s.depsMu.Lock()
	defer s.depsMu.Unlock()
	s.transport = r
}

func (s *Service) SetRouteProvider(p RouteProvider) {
	s.depsMu.Lock()
	defer s.depsMu.Unlock()
	s.routeProv = p
}

func (s *Service) snapshotDeps() (OutboundsProvider, TransportResolver, RouteProvider) {
	s.depsMu.RLock()
	defer s.depsMu.RUnlock()
	return s.outbounds, s.transport, s.routeProv
}

func (s *Service) ListOutbounds(ctx context.Context) []Outbound {
	outbounds, transport, _ := s.snapshotDeps()
	if outbounds == nil {
		return []Outbound{{
			Tag:       "direct",
			Kind:      "direct",
			Label:     "Direct (WAN)",
			Detail:    "без туннеля",
			Available: true,
		}}
	}

	items := outbounds.ListDownloadOutbounds(ctx)
	out := make([]Outbound, 0, len(items))
	for _, item := range items {
		if item.Tag == "direct" {
			item.Available = true
		} else if transport != nil {
			item.Available = transport.IsAvailable(ctx, item)
		}
		out = append(out, item)
	}
	return out
}

func (s *Service) DescribeRoute(ctx context.Context, route *Route) RouteInfo {
	outbounds, _, _ := s.snapshotDeps()
	return describeRouteWithProvider(ctx, outbounds, route)
}

func (s *Service) ValidateRoute(ctx context.Context, route *Route) (RouteInfo, error) {
	outbounds, _, _ := s.snapshotDeps()
	tag := "direct"
	if route != nil && strings.TrimSpace(route.Tag) != "" {
		tag = strings.TrimSpace(route.Tag)
	}
	if tag == "direct" {
		return RouteInfo{Tag: "direct", Kind: "direct", Label: "Direct (WAN)", Detail: "без туннеля"}, nil
	}
	if outbounds == nil {
		return RouteInfo{}, errors.New(unavailableRouteMessage(tag, routeKind(route)))
	}
	items := outbounds.ListDownloadOutbounds(ctx)
	if isRuntimeTagAmbiguous(items, tag) {
		return RouteInfo{}, fmt.Errorf("selected outbound tag %q is ambiguous for download transport: multiple outbounds share the same runtime tag", tag)
	}
	for _, ob := range items {
		if !routeMatches(ob, route, tag) {
			continue
		}
		return RouteInfo{Tag: ob.Tag, Kind: ob.Kind, Label: ob.Label, Detail: ob.Detail}, nil
	}
	return RouteInfo{}, errors.New(unavailableRouteMessage(tag, routeKind(route)))
}

func describeRouteWithProvider(ctx context.Context, outbounds OutboundsProvider, route *Route) RouteInfo {
	tag := "direct"
	if route != nil && strings.TrimSpace(route.Tag) != "" {
		tag = strings.TrimSpace(route.Tag)
	}
	if tag == "direct" {
		return RouteInfo{Tag: "direct", Kind: "direct", Label: "Direct (WAN)", Detail: "без туннеля"}
	}
	if outbounds != nil {
		for _, ob := range outbounds.ListDownloadOutbounds(ctx) {
			if routeMatches(ob, route, tag) {
				return RouteInfo{Tag: ob.Tag, Kind: ob.Kind, Label: ob.Label, Detail: ob.Detail}
			}
		}
	}
	kind := routeKind(route)
	return RouteInfo{Tag: tag, Kind: kind}
}

func (s *Service) ResolveClient(ctx context.Context, route *Route) (*Lease, error) {
	outbounds, transport, routeProvider := s.snapshotDeps()
	effectiveRoute := route
	if effectiveRoute == nil || strings.TrimSpace(effectiveRoute.Tag) == "" {
		if routeProvider != nil {
			providedRoute, err := routeProvider.GetDownloadRoute(ctx)
			if err != nil {
				return nil, fmt.Errorf("load download route settings: %w", err)
			}
			effectiveRoute = providedRoute
		}
	}
	info := describeRouteWithProvider(ctx, outbounds, effectiveRoute)
	if info.Tag == "direct" {
		client, err := newHTTPClientFromSpec(TransportSpec{Mode: TransportModeDirect})
		if err != nil {
			return nil, err
		}
		return &Lease{
			Client: client,
			Route:  info,
			cleanup: func() {
				client.CloseIdleConnections()
			},
		}, nil
	}

	tag := info.Tag
	if outbounds == nil {
		return nil, errors.New(unavailableRouteMessage(tag, routeKind(effectiveRoute)))
	}
	if transport == nil {
		return nil, fmt.Errorf("selected outbound %q is unavailable: download transport resolver is not configured", tag)
	}

	items := outbounds.ListDownloadOutbounds(ctx)
	if isRuntimeTagAmbiguous(items, tag) {
		return nil, fmt.Errorf("selected outbound tag %q is ambiguous for download transport: multiple outbounds share the same runtime tag", tag)
	}

	var ob Outbound
	found := false
	for _, candidate := range items {
		if routeMatches(candidate, effectiveRoute, tag) {
			ob = candidate
			found = true
			info = RouteInfo{Tag: ob.Tag, Kind: ob.Kind, Label: ob.Label, Detail: ob.Detail}
			break
		}
	}
	if !found {
		return nil, errors.New(unavailableRouteMessage(tag, routeKind(effectiveRoute)))
	}

	spec, err := transport.Resolve(ctx, ob)
	if err != nil {
		return nil, err
	}
	client, err := newHTTPClientFromSpec(spec)
	if err != nil {
		return nil, fmt.Errorf("build download client for %q: %w", tag, err)
	}

	return &Lease{
		Client: client,
		Route:  info,
		cleanup: func() {
			client.CloseIdleConnections()
		},
	}, nil
}

func routeKind(route *Route) string {
	if route == nil {
		return ""
	}
	return strings.TrimSpace(route.Kind)
}

func routeMatches(ob Outbound, route *Route, tag string) bool {
	if ob.Tag != tag {
		return false
	}
	kind := routeKind(route)
	if kind == "" {
		return true
	}
	return strings.TrimSpace(ob.Kind) == kind
}

func unavailableRouteMessage(tag, kind string) string {
	if strings.TrimSpace(kind) == "" {
		return fmt.Sprintf("selected outbound %q is unavailable for download transport", tag)
	}
	return fmt.Sprintf("selected outbound %q kind %q is unavailable for download transport", tag, strings.TrimSpace(kind))
}

func isRuntimeTagAmbiguous(items []Outbound, tag string) bool {
	if strings.TrimSpace(tag) == "" || strings.TrimSpace(tag) == "direct" {
		return false
	}
	count := 0
	for _, ob := range items {
		if strings.TrimSpace(ob.Tag) != tag {
			continue
		}
		count++
		if count > 1 {
			return true
		}
	}
	return false
}
