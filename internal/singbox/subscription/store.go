package subscription

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Store persists subscriptions to disk as JSON, atomic-writes on every mutation.
type Store struct {
	path string
	mu   sync.RWMutex
	data map[string]*Subscription
}

func NewStore(path string) (*Store, error) {
	s := &Store{path: path, data: make(map[string]*Subscription)}
	if err := s.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	if len(b) == 0 {
		return nil
	}
	var list []*Subscription
	if err := json.Unmarshal(b, &list); err != nil {
		return fmt.Errorf("subscription store: parse %s: %w", s.path, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range list {
		s.data[item.ID] = item
	}
	return nil
}

func (s *Store) saveLocked() error {
	list := make([]*Subscription, 0, len(s.data))
	for _, item := range s.data {
		list = append(list, item)
	}
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func newID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand should never fail
	}
	return hex.EncodeToString(b[:])
}

func (s *Store) Create(in CreateInput) (*Subscription, error) {
	id := newID()
	short := id[:8]
	sub := &Subscription{
		ID:           id,
		Label:        in.Label,
		URL:          in.URL,
		Headers:      in.Headers,
		RefreshHours: in.RefreshHours,
		Enabled:      in.Enabled,
		SelectorTag:  "sub-" + short,
		InboundTag:   "sub-" + short + "-in",
		MemberTags:   []string{},
		OrphanTags:   []string{},
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[id] = sub
	if err := s.saveLocked(); err != nil {
		delete(s.data, id)
		return nil, err
	}
	return sub, nil
}

func (s *Store) Get(id string) (*Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sub, ok := s.data[id]
	if !ok {
		return nil, fmt.Errorf("subscription %q not found", id)
	}
	cp := *sub
	return &cp, nil
}

func (s *Store) List() []Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Subscription, 0, len(s.data))
	for _, sub := range s.data {
		out = append(out, *sub)
	}
	return out
}

func (s *Store) Update(id string, patch UpdatePatch) (*Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sub, ok := s.data[id]
	if !ok {
		return nil, fmt.Errorf("subscription %q not found", id)
	}
	if patch.Label != nil {
		sub.Label = *patch.Label
	}
	if patch.URL != nil {
		sub.URL = *patch.URL
	}
	if patch.Headers != nil {
		sub.Headers = *patch.Headers
	}
	if patch.RefreshHours != nil {
		sub.RefreshHours = *patch.RefreshHours
	}
	if patch.Enabled != nil {
		sub.Enabled = *patch.Enabled
	}
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	cp := *sub
	return &cp, nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[id]; !ok {
		return fmt.Errorf("subscription %q not found", id)
	}
	delete(s.data, id)
	return s.saveLocked()
}

func (s *Store) UpdateState(id string, res RefreshResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sub, ok := s.data[id]
	if !ok {
		return fmt.Errorf("subscription %q not found", id)
	}
	sub.LastFetched = res.When
	if res.Err != nil {
		sub.LastError = maskURL(res.Err.Error(), sub.URL)
	} else {
		sub.LastError = ""
	}
	return s.saveLocked()
}

// SetMembership replaces MemberTags + OrphanTags atomically. Used by Service.Refresh.
func (s *Store) SetMembership(id string, members, orphans []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sub, ok := s.data[id]
	if !ok {
		return fmt.Errorf("subscription %q not found", id)
	}
	sub.MemberTags = members
	sub.OrphanTags = orphans
	return s.saveLocked()
}

func (s *Store) SetListenPort(id string, port uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sub, ok := s.data[id]
	if !ok {
		return fmt.Errorf("subscription %q not found", id)
	}
	sub.ListenPort = port
	return s.saveLocked()
}

func (s *Store) SetIsDefaultRoute(id string, isDefault bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if isDefault {
		// Atomically: clear default-route from any other subscription.
		for _, sub := range s.data {
			if sub.ID != id {
				sub.IsDefaultRoute = false
			}
		}
	}
	sub, ok := s.data[id]
	if !ok {
		return fmt.Errorf("subscription %q not found", id)
	}
	sub.IsDefaultRoute = isDefault
	return s.saveLocked()
}

// maskURL replaces the subscription URL in an error message with a placeholder
// so logs never leak provider tokens / paths.
func maskURL(msg, url string) string {
	if url == "" || msg == "" {
		return msg
	}
	return strings.ReplaceAll(msg, url, "<subscription-url>")
}

// MaybeRefresh returns subscriptions whose RefreshHours interval has
// elapsed since LastFetched. Used by the scheduler to pick due items.
func (s *Store) MaybeRefresh(now time.Time) []Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Subscription{}
	for _, sub := range s.data {
		if !sub.Enabled || sub.RefreshHours <= 0 {
			continue
		}
		if sub.LastFetched.IsZero() ||
			now.Sub(sub.LastFetched) >= time.Duration(sub.RefreshHours)*time.Hour {
			out = append(out, *sub)
		}
	}
	return out
}
