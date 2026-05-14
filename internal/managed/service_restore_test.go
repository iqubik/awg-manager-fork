package managed

import (
	"context"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/storage"
)

func TestRestore_RejectsEmptyPrivateKey(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewSettingsStore(dir)
	_, _ = store.Load()

	s := &Service{settings: store}
	outcomes := s.Restore(context.Background(), []ManagedServerExport{{
		InterfaceName: "Wireguard0",
		Address:       "10.0.0.1",
		Mask:          "255.255.255.0",
		ListenPort:    51820,
		// PrivateKey deliberately empty
	}}, RestoreOptions{})

	if len(outcomes) != 1 || outcomes[0].Action != "failed" {
		t.Fatalf("outcomes: %+v", outcomes)
	}
}

func TestRestore_PreflightDetectsInvalidAddress(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewSettingsStore(dir)
	_, _ = store.Load()

	s := &Service{settings: store}
	outcomes := s.Restore(context.Background(), []ManagedServerExport{{
		InterfaceName: "Wireguard0",
		Address:       "not-an-ip",
		Mask:          "255.255.255.0",
		ListenPort:    51820,
		PrivateKey:    "k0",
	}}, RestoreOptions{})

	if outcomes[0].Action != "conflict" {
		t.Fatalf("action: %q, conflicts: %v", outcomes[0].Action, outcomes[0].Conflicts)
	}
	found := false
	for _, c := range outcomes[0].Conflicts {
		if strings.Contains(c, "not a valid IP") {
			found = true
		}
	}
	if !found {
		t.Errorf("conflicts: %v (expected invalid-IP reason)", outcomes[0].Conflicts)
	}
}
