package managed

import (
	"context"
	"fmt"
	"net"

	"github.com/hoaxisr/awg-manager/internal/storage"
)

// RestoreOptions controls per-batch behaviour of Service.Restore.
type RestoreOptions struct {
	// AllowRenumber: when true and the target slot Wireguard<N> is taken
	// by a DIFFERENT server (matched by public-key derived from the
	// supplied private key), Restore picks the next free Wireguard<M>
	// slot and creates the server there. When false, such a server is
	// returned with action="conflict".
	AllowRenumber bool `json:"allowRenumber"`
}

// RestoreOutcome is the per-server report from Service.Restore.
type RestoreOutcome struct {
	Name       string   `json:"name"`                 // original Wireguard<N> from the input
	NewName    string   `json:"newName,omitempty"`    // populated when action == "renamed"
	Action     string   `json:"action"`               // created|merged|renamed|conflict|failed
	AddedPeers int      `json:"addedPeers,omitempty"` // for merged
	Conflicts  []string `json:"conflicts,omitempty"`  // human-readable reasons
	Error      string   `json:"error,omitempty"`      // only for failed
}

// Restore reconciles the supplied list of managed-server snapshots against
// NDMS, per-server atomic with rollback on transient RCI errors. See
// docs/superpowers/specs/2026-05-14-managed-server-export-import-design.md
// §Per-server atomicity for the contract.
func (s *Service) Restore(ctx context.Context, in []ManagedServerExport, opts RestoreOptions) []RestoreOutcome {
	out := make([]RestoreOutcome, 0, len(in))
	for _, sv := range in {
		out = append(out, s.restoreOne(ctx, sv, opts))
	}
	return out
}

func (s *Service) restoreOne(ctx context.Context, sv ManagedServerExport, opts RestoreOptions) RestoreOutcome {
	outcome := RestoreOutcome{Name: sv.InterfaceName}

	if sv.PrivateKey == "" {
		outcome.Action = "failed"
		outcome.Error = "PrivateKey is empty in input; cannot restore without server key"
		return outcome
	}

	// Same-server identity match → merge missing peers.
	if existing, ok := s.findOccupant(ctx, sv); ok && samePubKey(existing, sv) {
		added, err := s.mergePeers(ctx, existing, sv)
		if err != nil {
			outcome.Action = "failed"
			outcome.Error = err.Error()
			return outcome
		}
		outcome.Action = "merged"
		outcome.AddedPeers = added
		return outcome
	}

	// Pre-flight all other paths.
	conflicts := s.preflight(ctx, sv, opts)
	if len(conflicts) > 0 {
		outcome.Action = "conflict"
		outcome.Conflicts = conflicts
		return outcome
	}

	// Resolve target slot (renumber if requested and original is taken).
	target := sv.InterfaceName
	renamed := false
	if existing, ok := s.findOccupant(ctx, sv); ok && !samePubKey(existing, sv) && opts.AllowRenumber {
		if s.queries == nil || s.queries.WGServers == nil {
			outcome.Action = "failed"
			outcome.Error = "renumber requested but WGServers queries layer unavailable"
			return outcome
		}
		idx, err := s.queries.WGServers.FindFreeIndex(ctx)
		if err != nil {
			outcome.Action = "failed"
			outcome.Error = fmt.Sprintf("find free index: %v", err)
			return outcome
		}
		target = fmt.Sprintf("Wireguard%d", idx)
		renamed = true
	}

	if err := s.applyOne(ctx, target, sv); err != nil {
		s.cleanupInterface(ctx, target)
		outcome.Action = "failed"
		outcome.Error = err.Error()
		return outcome
	}

	if renamed {
		outcome.Action = "renamed"
		outcome.NewName = target
	} else {
		outcome.Action = "created"
	}
	return outcome
}

// preflight runs read-only conflict checks for one server. Returns a slice
// of human-readable conflict reasons; empty means OK to apply.
func (s *Service) preflight(ctx context.Context, sv ManagedServerExport, opts RestoreOptions) []string {
	var reasons []string

	// Basic param sanity.
	if net.ParseIP(sv.Address) == nil {
		reasons = append(reasons, fmt.Sprintf("address %q is not a valid IP", sv.Address))
	}
	if sv.ListenPort < 1 || sv.ListenPort > 65535 {
		reasons = append(reasons, fmt.Sprintf("listen-port %d out of range", sv.ListenPort))
	}

	// Subnet overlap with other interfaces.
	cidr, err := parseManagedSubnet(sv.Address, sv.Mask)
	if err != nil {
		reasons = append(reasons, fmt.Sprintf("subnet %s/%s: %v", sv.Address, sv.Mask, err))
	} else {
		if used, err := s.listUsedSubnets(ctx, sv.InterfaceName); err == nil {
			if conflict := findConflict(cidr, used); conflict != nil {
				reasons = append(reasons, fmt.Sprintf("subnet %s overlaps with interface %q (%s)",
					cidr.String(), conflict.label, conflict.cidr.String()))
			}
		}
	}

	// Listen-port collision with other managed servers.
	if portConflict := findPortConflict(sv.ListenPort, s.listUsedListenPorts(sv.InterfaceName)); portConflict != nil {
		reasons = append(reasons, fmt.Sprintf("listen-port %d already used by managed server %q",
			sv.ListenPort, portConflict.iface))
	}

	// Slot occupancy (foreign server).
	if other, ok := s.findOccupant(ctx, sv); ok && !samePubKey(other, sv) {
		if !opts.AllowRenumber {
			reasons = append(reasons, fmt.Sprintf("slot %s is occupied by a different server; enable AllowRenumber to relocate",
				sv.InterfaceName))
		}
		// If AllowRenumber is true, this is not a conflict — Task 9 will
		// pick a free slot in the apply phase.
	}

	return reasons
}

// findOccupant returns the storage entry currently at sv.InterfaceName, if any.
func (s *Service) findOccupant(_ context.Context, sv ManagedServerExport) (storage.ManagedServer, bool) {
	existing, ok := s.settings.GetManagedServerByID(sv.InterfaceName)
	if !ok || existing == nil {
		return storage.ManagedServer{}, false
	}
	return *existing, true
}

// samePubKey reports whether the existing server and the input describe
// the same server identity. Both sides come from the same kernel at some
// point, so identical private keys imply identical public keys without a
// derivation call.
func samePubKey(existing storage.ManagedServer, input ManagedServerExport) bool {
	if existing.PrivateKey == "" || input.PrivateKey == "" {
		return false
	}
	return existing.PrivateKey == input.PrivateKey
}

// applyOne is the per-server transaction: RCI create, configure, set
// private key, set NAT, add peers, persist to settings.json. The caller
// (restoreOne) wraps with cleanupInterface on error so any partial NDMS
// state is rolled back.
func (s *Service) applyOne(ctx context.Context, target string, sv ManagedServerExport) error {
	if err := s.rciCreateInterface(ctx, target); err != nil {
		return fmt.Errorf("create interface: %w", err)
	}
	if err := s.rciConfigureServer(ctx, target, sv.Description, sv.Address, sv.Mask, sv.ListenPort); err != nil {
		return fmt.Errorf("configure interface: %w", err)
	}
	if err := s.rciSetPrivateKey(ctx, target, sv.PrivateKey); err != nil {
		return fmt.Errorf("set private key: %w", err)
	}
	if err := s.rciSetNAT(ctx, target, sv.NATEnabled); err != nil {
		return fmt.Errorf("set NAT: %w", err)
	}
	for _, peer := range sv.Peers {
		ip, _, err := net.ParseCIDR(peer.TunnelIP)
		if err != nil {
			return fmt.Errorf("peer tunnel IP %q: %w", peer.TunnelIP, err)
		}
		if err := s.rciAddPeer(ctx, target, peer.PublicKey, peer.PresharedKey, peer.Description, ip.String()); err != nil {
			return fmt.Errorf("add peer %s: %w", peer.PublicKey, err)
		}
	}
	// Persist to settings.json under the (possibly renamed) target.
	saved := sv
	saved.InterfaceName = target
	if err := s.settings.AddManagedServer(saved); err != nil {
		return fmt.Errorf("save to storage: %w", err)
	}
	if s.queries != nil && s.queries.Interfaces != nil {
		s.queries.Interfaces.InvalidateAll()
	}
	return nil
}

// mergePeers adds peers from sv that are not already present (by public
// key) on the live existing server. Returns the count actually added.
func (s *Service) mergePeers(ctx context.Context, existing storage.ManagedServer, sv ManagedServerExport) (int, error) {
	have := make(map[string]struct{}, len(existing.Peers))
	for _, p := range existing.Peers {
		have[p.PublicKey] = struct{}{}
	}
	added := 0
	for _, peer := range sv.Peers {
		if _, ok := have[peer.PublicKey]; ok {
			continue
		}
		ip, _, err := net.ParseCIDR(peer.TunnelIP)
		if err != nil {
			return added, fmt.Errorf("peer tunnel IP %q: %w", peer.TunnelIP, err)
		}
		if err := s.rciAddPeer(ctx, existing.InterfaceName, peer.PublicKey, peer.PresharedKey, peer.Description, ip.String()); err != nil {
			return added, fmt.Errorf("add peer %s: %w", peer.PublicKey, err)
		}
		if err := s.settings.UpdateManagedServer(existing.InterfaceName, func(target *storage.ManagedServer) error {
			target.Peers = append(target.Peers, peer)
			return nil
		}); err != nil {
			return added, fmt.Errorf("persist merged peer: %w", err)
		}
		added++
	}
	return added, nil
}
