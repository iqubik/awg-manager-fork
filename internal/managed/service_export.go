package managed

import (
	"context"
	"fmt"

	"github.com/hoaxisr/awg-manager/internal/storage"
)

// ManagedServerExport is the wire DTO used by Export/Import/Drift. Same
// shape as storage.ManagedServer; the type alias exists so the API and
// frontend can refer to a stable name even if we ever decide to extend
// the export format (e.g. for backup metadata) without breaking the
// storage schema.
type ManagedServerExport = storage.ManagedServer

// ExportAll returns every managed server from settings.json, ready for
// inclusion in a backup file. The slice is a fresh copy — callers may
// mutate it freely without affecting persisted state.
func (s *Service) ExportAll(ctx context.Context) ([]ManagedServerExport, error) {
	servers := s.settings.GetManagedServers()
	out := make([]ManagedServerExport, len(servers))
	copy(out, servers)
	s.sysLog().Info("managed export prepared", "servers", len(out))
	s.appLog.Info("export-managed-backup", fmt.Sprintf("%d servers", len(out)), "Prepared managed server backup payload")
	return out, nil
}

// presenceFn is the NDMS-interface-presence indirection used by tests.
type presenceFn func(ndmsName string) bool

// Drift returns ManagedServer entries present in settings.json but missing
// from the NDMS interface cache. Used by the boot-time drift banner and
// the restore-drift HTTP endpoint to recover settings that survived an
// NDMS reset.
func (s *Service) Drift(ctx context.Context) ([]ManagedServerExport, error) {
	return s.driftWith(ctx, func(name string) bool {
		if s.queries == nil || s.queries.Interfaces == nil {
			return false
		}
		iface, err := s.queries.Interfaces.Get(ctx, name)
		return err == nil && iface != nil
	})
}

func (s *Service) driftWith(ctx context.Context, present presenceFn) ([]ManagedServerExport, error) {
	servers := s.settings.GetManagedServers()
	out := make([]ManagedServerExport, 0, len(servers))
	for _, sv := range servers {
		if !present(sv.InterfaceName) {
			out = append(out, sv)
		}
	}
	s.sysLog().Info("managed drift scan finished", "total", len(servers), "drifted", len(out))
	if len(out) > 0 {
		s.appLog.Warn("managed-drift-detected", fmt.Sprintf("%d servers", len(out)), "Managed servers exist in storage but are missing live interfaces")
	}
	return out, nil
}
