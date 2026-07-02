package singboxwatchdog

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sync"

	"github.com/hoaxisr/awg-manager/internal/storage"
)

type StoreData struct {
	SchemaVersion int            `json:"schemaVersion"`
	Targets       []TargetConfig `json:"targets"`
}

type Store struct {
	path string
	mu   sync.RWMutex
	data StoreData
}

func NewStore(path string) *Store {
	return &Store{
		path: path,
		data: StoreData{
			SchemaVersion: 1,
			Targets:       []TargetConfig{},
		},
	}
}

func normalizeConfig(c TargetConfig) TargetConfig {
	if c.Interval <= 0 {
		c.Interval = 30
	}
	if c.Interval < 5 {
		c.Interval = 5
	}
	if c.Interval > 3600 {
		c.Interval = 3600
	}
	if c.FailThreshold <= 0 {
		c.FailThreshold = 3
	}
	if c.FailThreshold > 20 {
		c.FailThreshold = 20
	}
	if c.Timeout <= 0 {
		c.Timeout = 5
	}
	if c.Timeout > 30 {
		c.Timeout = 30
	}
	switch c.RecoveryMode {
	case RecoveryOff, RecoveryRestartSingbox, RecoverySwitchMember:
	default:
		c.RecoveryMode = ""
	}
	if c.Kind == TargetTunnel && c.RecoveryMode == RecoverySwitchMember {
		c.RecoveryMode = ""
	}
	if c.Kind == TargetSubscription && c.RecoveryMode == RecoveryRestartSingbox {
		c.RecoveryMode = ""
	}
	if c.RecoveryMode == "" {
		c.RecoveryMode = defaultRecoveryMode(c.Kind)
	}
	return c
}

func defaultRecoveryMode(kind TargetKind) RecoveryMode {
	if kind == TargetSubscription {
		return RecoverySwitchMember
	}
	return RecoveryOff
}

func (s *Store) Load() error {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(b) == 0 {
		return nil
	}
	var data StoreData
	if err := json.Unmarshal(b, &data); err != nil {
		return fmt.Errorf("parse singbox watchdog store: %w", err)
	}
	if data.SchemaVersion <= 0 {
		data.SchemaVersion = 1
	}
	for i := range data.Targets {
		data.Targets[i] = normalizeConfig(data.Targets[i])
	}
	slices.SortFunc(data.Targets, func(a, b TargetConfig) int { return compareIDs(a.ID, b.ID) })
	s.mu.Lock()
	s.data = data
	s.mu.Unlock()
	return nil
}

func compareIDs(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func (s *Store) saveLocked() error {
	s.data.SchemaVersion = 1
	slices.SortFunc(s.data.Targets, func(a, b TargetConfig) int { return compareIDs(a.ID, b.ID) })
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return storage.AtomicWrite(s.path, b)
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *Store) List() []TargetConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TargetConfig, len(s.data.Targets))
	copy(out, s.data.Targets)
	return out
}

func (s *Store) Get(id string) (TargetConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, cfg := range s.data.Targets {
		if cfg.ID == id {
			return cfg, true
		}
	}
	return TargetConfig{}, false
}

func (s *Store) Upsert(cfg TargetConfig) error {
	cfg = normalizeConfig(cfg)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, item := range s.data.Targets {
		if item.ID == cfg.ID {
			s.data.Targets[i] = cfg
			return s.saveLocked()
		}
	}
	s.data.Targets = append(s.data.Targets, cfg)
	return s.saveLocked()
}

func (s *Store) SetEnabled(id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, cfg := range s.data.Targets {
		if cfg.ID == id {
			cfg.Enabled = enabled
			s.data.Targets[i] = normalizeConfig(cfg)
			return s.saveLocked()
		}
	}
	return fmt.Errorf("%w: %s", ErrTargetNotFound, id)
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, cfg := range s.data.Targets {
		if cfg.ID == id {
			s.data.Targets = append(s.data.Targets[:i], s.data.Targets[i+1:]...)
			return s.saveLocked()
		}
	}
	return fmt.Errorf("%w: %s", ErrTargetNotFound, id)
}
