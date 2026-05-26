package dnsrewrite

import (
	"encoding/json"
	"fmt"
)

// SlotName — имя слота оркестратора (= orchestrator.SlotDNSRewrites как строка).
const SlotName = "dns-rewrites"

type Store interface {
	List() ([]DNSRewrite, error)
	Add(DNSRewrite) error
	Update(int, DNSRewrite) error
	Delete(int) error
	Move(from, to int) error
}

type Orchestrator interface {
	Save(slot string, data []byte) error
	SetEnabled(slot string, on bool) error
}

type EventBus interface {
	Publish(eventType string, data any)
}

type Service struct {
	store Store
	orch  Orchestrator
	bus   EventBus
}

func NewService(store Store, orch Orchestrator, bus EventBus) *Service {
	return &Service{store: store, orch: orch, bus: bus}
}

func (s *Service) List() ([]DNSRewrite, error) { return s.store.List() }

func (s *Service) Add(r DNSRewrite) error {
	if _, err := compileRewrite(r); err != nil {
		return err
	}
	if err := s.store.Add(r); err != nil {
		return err
	}
	return s.flush()
}

func (s *Service) Update(index int, r DNSRewrite) error {
	if _, err := compileRewrite(r); err != nil {
		return err
	}
	if err := s.store.Update(index, r); err != nil {
		return err
	}
	return s.flush()
}

func (s *Service) Delete(index int) error {
	if err := s.store.Delete(index); err != nil {
		return err
	}
	return s.flush()
}

func (s *Service) Move(from, to int) error {
	if err := s.store.Move(from, to); err != nil {
		return err
	}
	return s.flush()
}

// Resync пересобирает слот из текущего содержимого стора.
func (s *Service) Resync() error { return s.flush() }

type slotConfig struct {
	DNS slotDNS `json:"dns"`
}
type slotDNS struct {
	Rules []map[string]any `json:"rules"`
}

func (s *Service) flush() error {
	items, err := s.store.List()
	if err != nil {
		return err
	}
	rules := make([]map[string]any, 0, len(items))
	for _, r := range items {
		compiled, err := compileRewrite(r)
		if err != nil {
			return fmt.Errorf("compile %q: %w", r.Pattern, err)
		}
		rules = append(rules, compiled...)
	}
	data, err := json.MarshalIndent(slotConfig{DNS: slotDNS{Rules: rules}}, "", "  ")
	if err != nil {
		return err
	}
	if err := s.orch.Save(SlotName, data); err != nil {
		return err
	}
	if err := s.orch.SetEnabled(SlotName, len(rules) > 0); err != nil {
		return err
	}
	if s.bus != nil {
		s.bus.Publish("resource:invalidated", map[string]any{"resource": "singbox.dns-rewrites"})
	}
	return nil
}
