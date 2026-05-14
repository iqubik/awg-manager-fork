package managed

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/storage"
)

func TestMigratePrivateKeys_FillsEmptyEntries(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewSettingsStore(filepath.Join(dir))
	if _, err := store.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	_ = store.AddManagedServer(storage.ManagedServer{
		InterfaceName: "Wireguard0", Address: "10.0.0.1", Mask: "255.255.255.0",
		ListenPort: 51820, Peers: []storage.ManagedPeer{},
		// PrivateKey deliberately empty — legacy entry
	})

	s := &Service{settings: store}
	stub := func(ctx context.Context, name string, args ...string) (string, error) {
		if args[1] == "nwg0" {
			return "yKOZJtI2nQbSWzo8zvmBSjjnSkn89AkLXbekWTgKQ08=\n", nil
		}
		return "", errors.New("unknown")
	}
	s.migratePrivateKeysWith(context.Background(),
		func(_ context.Context, ndms string) string {
			if ndms == "Wireguard0" {
				return "nwg0"
			}
			return ""
		},
		stub,
	)
	got, _ := store.GetManagedServerByID("Wireguard0")
	if got.PrivateKey != "yKOZJtI2nQbSWzo8zvmBSjjnSkn89AkLXbekWTgKQ08=" {
		t.Errorf("PrivateKey: got %q", got.PrivateKey)
	}
}

func TestMigratePrivateKeys_SkipsPopulated(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewSettingsStore(filepath.Join(dir))
	if _, err := store.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	_ = store.AddManagedServer(storage.ManagedServer{
		InterfaceName: "Wireguard0", Address: "10.0.0.1", Mask: "255.255.255.0",
		ListenPort: 51820, PrivateKey: "EXISTING_KEY", Peers: []storage.ManagedPeer{},
	})

	s := &Service{settings: store}
	callCount := 0
	stub := func(ctx context.Context, name string, args ...string) (string, error) {
		callCount++
		return "should-not-be-called", nil
	}
	s.migratePrivateKeysWith(context.Background(),
		func(_ context.Context, ndms string) string { return "nwg0" },
		stub,
	)
	if callCount != 0 {
		t.Errorf("wg-tools called %d times for already-populated entry", callCount)
	}
	got, _ := store.GetManagedServerByID("Wireguard0")
	if got.PrivateKey != "EXISTING_KEY" {
		t.Errorf("PrivateKey overwritten: got %q", got.PrivateKey)
	}
}

func TestMigratePrivateKeys_LogsAndContinuesOnError(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewSettingsStore(filepath.Join(dir))
	if _, err := store.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	_ = store.AddManagedServer(storage.ManagedServer{
		InterfaceName: "Wireguard0", Address: "10.0.0.1", Mask: "255.255.255.0",
		ListenPort: 51820, Peers: []storage.ManagedPeer{},
	})
	_ = store.AddManagedServer(storage.ManagedServer{
		InterfaceName: "Wireguard1", Address: "10.1.0.1", Mask: "255.255.255.0",
		ListenPort: 51821, Peers: []storage.ManagedPeer{},
	})

	s := &Service{settings: store}
	stub := func(ctx context.Context, name string, args ...string) (string, error) {
		if args[1] == "nwg0" {
			return "", errors.New("Unable to access interface: No such device")
		}
		return "VALID_KEY=\n", nil
	}
	s.migratePrivateKeysWith(context.Background(),
		func(_ context.Context, ndms string) string {
			switch ndms {
			case "Wireguard0":
				return "nwg0"
			case "Wireguard1":
				return "nwg1"
			}
			return ""
		},
		stub,
	)
	g0, _ := store.GetManagedServerByID("Wireguard0")
	g1, _ := store.GetManagedServerByID("Wireguard1")
	if g0.PrivateKey != "" {
		t.Errorf("Wireguard0 PrivateKey should remain empty on error; got %q", g0.PrivateKey)
	}
	if g1.PrivateKey != "VALID_KEY=" {
		t.Errorf("Wireguard1 PrivateKey: got %q", g1.PrivateKey)
	}
}
