package subscription

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hoaxisr/awg-manager/internal/singbox/vlink"
)

// ConfigMutator is the narrow contract for committing subscription state to
// sing-box config. The real implementation lives in singbox.Operator.
type ConfigMutator interface {
	AllocListenPort() (uint16, error)
	AllocProxyIndex(ctx context.Context) (int, error)
	AddOutbound(tag string, jsonBody []byte) error
	UpdateOutbound(tag string, jsonBody []byte) error
	RemoveOutbound(tag string) error
	AddInbound(tag string, jsonBody []byte) error
	RemoveInbound(tag string) error
	AddRouteRule(jsonBody []byte) error
	RemoveRouteRule(inboundTag, outboundTag string) error
	EnsureProxy(ctx context.Context, idx, port int, description string) error
	RemoveProxy(ctx context.Context, idx int) error
	Reload(ctx context.Context) error
}

// Service is the subscription business-logic facade.
type Service struct {
	store     *Store
	mutator   ConfigMutator
	muById    sync.Map // map[string]*sync.Mutex
	fetchOpts FetchOpts
}

func NewService(store *Store, mutator ConfigMutator) *Service {
	return &Service{store: store, mutator: mutator}
}

func (s *Service) lockSub(id string) *sync.Mutex {
	if v, ok := s.muById.Load(id); ok {
		return v.(*sync.Mutex)
	}
	m := &sync.Mutex{}
	actual, _ := s.muById.LoadOrStore(id, m)
	return actual.(*sync.Mutex)
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Subscription, error) {
	if in.URL == "" {
		return nil, errors.New("subscription: URL is required")
	}
	sub, err := s.store.Create(in)
	if err != nil {
		return nil, err
	}
	mu := s.lockSub(sub.ID)
	mu.Lock()
	defer mu.Unlock()

	port, err := s.mutator.AllocListenPort()
	if err != nil {
		s.store.Delete(sub.ID)
		return nil, fmt.Errorf("subscription: alloc listen port: %w", err)
	}
	if err := s.store.SetListenPort(sub.ID, port); err != nil {
		s.store.Delete(sub.ID)
		return nil, err
	}

	proxyIdx, err := s.mutator.AllocProxyIndex(ctx)
	if err != nil {
		s.store.Delete(sub.ID)
		return nil, fmt.Errorf("subscription: alloc proxy index: %w", err)
	}
	if err := s.store.SetProxyIndex(sub.ID, proxyIdx); err != nil {
		s.store.Delete(sub.ID)
		return nil, err
	}
	if err := s.mutator.EnsureProxy(ctx, proxyIdx, int(port), sub.Label); err != nil {
		s.store.Delete(sub.ID)
		return nil, fmt.Errorf("subscription: register NDMS proxy: %w", err)
	}

	if _, err := s.refreshLocked(ctx, sub.ID); err != nil {
		s.store.Delete(sub.ID)
		return nil, fmt.Errorf("subscription: initial fetch failed: %w", err)
	}

	final, _ := s.store.Get(sub.ID)
	return final, nil
}

func (s *Service) Refresh(ctx context.Context, id string) (*RefreshResult, error) {
	mu := s.lockSub(id)
	mu.Lock()
	defer mu.Unlock()
	return s.refreshLocked(ctx, id)
}

func (s *Service) refreshLocked(ctx context.Context, id string) (*RefreshResult, error) {
	sub, err := s.store.Get(id)
	if err != nil {
		return nil, err
	}
	body, ct, err := Fetch(sub.URL, sub.Headers, s.fetchOpts)
	if err != nil {
		masked := fmt.Errorf("%s", MaskURL(err.Error(), sub.URL))
		s.store.UpdateState(id, RefreshResult{When: time.Now(), Err: masked})
		return nil, masked
	}
	lines := NormalizeBody(body, ct)
	parseRes := vlink.ParseBatch(lines)
	diff := ApplyDiff(id, sub.MemberTags, parseRes.Outbounds)

	if err := s.applyDiff(ctx, sub, diff); err != nil {
		s.store.UpdateState(id, RefreshResult{When: time.Now(), Err: err})
		return nil, err
	}

	newMembers := make([]MemberInfo, 0, len(diff.New)+len(diff.Existing))
	for _, n := range diff.New {
		newMembers = append(newMembers, toMemberInfo(n.Tag, n.Out))
	}
	for _, e := range diff.Existing {
		newMembers = append(newMembers, toMemberInfo(e.Tag, e.Out))
	}
	if err := s.store.SetMembers(id, newMembers, diff.Orphan); err != nil {
		return nil, err
	}

	res := &RefreshResult{
		When:         time.Now(),
		Added:        len(diff.New),
		Updated:      len(diff.Existing),
		Orphaned:     len(diff.Orphan),
		SkippedVmess: parseRes.SkippedVmess,
		SkippedOther: parseRes.SkippedUnsupp,
	}
	for _, e := range parseRes.Errors {
		res.ParseErrors = append(res.ParseErrors, e.Error())
	}
	if err := s.store.UpdateState(id, *res); err != nil {
		return nil, err
	}
	return res, nil
}

// applyDiff commits the diff to sing-box config. Selector + mixed inbound +
// route rule are recreated each refresh (they may not exist yet on first
// run). Per-member outbounds are added/updated/left alone — orphans are NOT
// removed (the UI offers explicit deletion).
func (s *Service) applyDiff(ctx context.Context, sub *Subscription, diff DiffResult) error {
	for _, n := range diff.New {
		jsonWithTag := replaceTag(n.Out.Outbound, n.Tag)
		if err := s.mutator.AddOutbound(n.Tag, jsonWithTag); err != nil {
			return err
		}
	}
	for _, e := range diff.Existing {
		jsonWithTag := replaceTag(e.Out.Outbound, e.Tag)
		if err := s.mutator.UpdateOutbound(e.Tag, jsonWithTag); err != nil {
			return err
		}
	}

	memberTags := make([]string, 0, len(diff.New)+len(diff.Existing))
	for _, n := range diff.New {
		memberTags = append(memberTags, n.Tag)
	}
	for _, e := range diff.Existing {
		memberTags = append(memberTags, e.Tag)
	}

	// Selector — remove old (idempotent) then add fresh.
	s.mutator.RemoveOutbound(sub.SelectorTag)
	if err := s.mutator.AddOutbound(sub.SelectorTag, BuildSelector(sub.SelectorTag, memberTags, "")); err != nil {
		return err
	}

	// Mixed inbound — add only if not present (first time).
	if sub.ListenPort != 0 {
		s.mutator.AddInbound(sub.InboundTag, BuildMixedInbound(sub.InboundTag, sub.ListenPort))
		s.mutator.AddRouteRule(BuildRouteRule(sub.InboundTag, sub.SelectorTag))
	}

	return s.mutator.Reload(ctx)
}

func (s *Service) Delete(ctx context.Context, id string, cascade bool) error {
	mu := s.lockSub(id)
	mu.Lock()
	defer mu.Unlock()

	sub, err := s.store.Get(id)
	if err != nil {
		return err
	}
	if cascade {
		s.mutator.RemoveRouteRule(sub.InboundTag, sub.SelectorTag)
		s.mutator.RemoveInbound(sub.InboundTag)
		s.mutator.RemoveOutbound(sub.SelectorTag)
		for _, m := range sub.MemberTags {
			s.mutator.RemoveOutbound(m)
		}
		if sub.ProxyIndex >= 0 {
			if err := s.mutator.RemoveProxy(ctx, sub.ProxyIndex); err != nil {
				// Log but don't block delete — orphan ProxyN is recoverable.
				s.store.UpdateState(id, RefreshResult{When: time.Now(), Err: err})
			}
		}
		if err := s.mutator.Reload(ctx); err != nil {
			return fmt.Errorf("subscription: cascade delete reload: %w", err)
		}
	}
	return s.store.Delete(id)
}

// replaceTag patches the "tag" field of a JSON-encoded outbound to match the
// stable tag we're committing under.
func replaceTag(raw []byte, tag string) []byte {
	var ob map[string]any
	_ = json.Unmarshal(raw, &ob)
	ob["tag"] = tag
	out, _ := json.Marshal(ob)
	return out
}

// toMemberInfo extracts user-facing metadata from a parsed outbound so the
// UI can render protocol, server:port, transport, and security badges without
// re-parsing the raw JSON on every render.
func toMemberInfo(tag string, p vlink.ParsedOutbound) MemberInfo {
	mi := MemberInfo{
		Tag:      tag,
		Protocol: p.Protocol,
		Server:   p.Server,
		Port:     p.Port,
	}
	var ob map[string]any
	if json.Unmarshal(p.Outbound, &ob) != nil {
		return mi
	}
	if tr, ok := ob["transport"].(map[string]any); ok {
		if t, ok := tr["type"].(string); ok {
			mi.Transport = t
		}
	}
	if tls, ok := ob["tls"].(map[string]any); ok {
		if _, hasReality := tls["reality"]; hasReality {
			mi.Security = "reality"
		} else if enabled, _ := tls["enabled"].(bool); enabled {
			mi.Security = "tls"
		}
	}
	return mi
}

// === Helpers used by REST handlers (B-Task 5) ===

func (s *Service) List() []Subscription                 { return s.store.List() }
func (s *Service) Get(id string) (*Subscription, error) { return s.store.Get(id) }
func (s *Service) Update(id string, patch UpdatePatch) (*Subscription, error) {
	mu := s.lockSub(id)
	mu.Lock()
	defer mu.Unlock()
	return s.store.Update(id, patch)
}

// SetActiveMember updates the selector's "default" pointer to memberTag.
func (s *Service) SetActiveMember(ctx context.Context, id, memberTag string) error {
	mu := s.lockSub(id)
	mu.Lock()
	defer mu.Unlock()

	sub, err := s.store.Get(id)
	if err != nil {
		return err
	}
	found := false
	for _, m := range sub.MemberTags {
		if m == memberTag {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("member %q not in subscription", memberTag)
	}
	s.mutator.RemoveOutbound(sub.SelectorTag)
	if err := s.mutator.AddOutbound(sub.SelectorTag, BuildSelector(sub.SelectorTag, sub.MemberTags, memberTag)); err != nil {
		return err
	}
	if err := s.store.SetActiveMember(id, memberTag); err != nil {
		return err
	}
	return s.mutator.Reload(ctx)
}

// SetDefaultRoute marks one subscription as defaultRoute (atomically clears
// the flag from all others).
func (s *Service) SetDefaultRoute(ctx context.Context, id string, enabled bool) error {
	if err := s.store.SetIsDefaultRoute(id, enabled); err != nil {
		return err
	}
	return s.mutator.Reload(ctx)
}

// DeleteOrphans removes orphan-flagged outbounds from sing-box config and
// clears the OrphanTags slice in the store.
func (s *Service) DeleteOrphans(ctx context.Context, id string) error {
	mu := s.lockSub(id)
	mu.Lock()
	defer mu.Unlock()
	sub, err := s.store.Get(id)
	if err != nil {
		return err
	}
	for _, t := range sub.OrphanTags {
		s.mutator.RemoveOutbound(t)
	}
	if err := s.store.SetMembership(id, sub.MemberTags, nil); err != nil {
		return err
	}
	return s.mutator.Reload(ctx)
}
