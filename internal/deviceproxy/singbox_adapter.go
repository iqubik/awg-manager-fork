package deviceproxy

import (
	"context"
	"fmt"

	"github.com/hoaxisr/awg-manager/internal/singbox"
	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

// SingboxAdapter bridges deviceproxy.Service (which speaks
// ExternalSpec) to singbox.Operator (which speaks DeviceProxySpec).
// Keeping the adapter here — rather than inside internal/singbox —
// preserves the one-way dependency: singbox knows nothing about
// deviceproxy, deviceproxy depends on singbox.
//
// Production wiring also injects the orchestrator: ApplyDeviceProxy
// then writes its standalone fragment (30-deviceproxy.json) through
// SlotDeviceProxy and toggles the slot via SetEnabled. When orch is
// nil (legacy / tests), it falls back to embedding the device-proxy
// blocks into 10-tunnels.json via Operator.ApplyConfig.
type SingboxAdapter struct {
	op   *singbox.Operator
	orch *orchestrator.Orchestrator
}

func NewSingboxAdapter(op *singbox.Operator) *SingboxAdapter {
	return &SingboxAdapter{op: op}
}

// SetOrch wires the config.d orchestrator after construction. The
// orchestrator is built post-Operator (it needs Operator.Process) so
// we can't supply it in the constructor without a cycle.
func (a *SingboxAdapter) SetOrch(orch *orchestrator.Orchestrator) {
	a.orch = orch
}

// ApplyDeviceProxy persists the device-proxy slot via the orchestrator
// (production) or falls back to the legacy embedded-in-tunnels path.
func (a *SingboxAdapter) ApplyDeviceProxy(ctx context.Context, spec ExternalSpec) error {
	if a.orch != nil {
		sbSpec := toSingboxSpec(spec)
		data, err := singbox.BuildDeviceProxyConfig(sbSpec)
		if err != nil {
			return fmt.Errorf("build deviceproxy config: %w", err)
		}
		if err := a.orch.Save(orchestrator.SlotDeviceProxy, data); err != nil {
			return err
		}
		if err := a.orch.SetEnabled(orchestrator.SlotDeviceProxy, sbSpec.Enabled); err != nil {
			return err
		}
		return nil
	}

	cfg, err := a.op.LoadCurrentConfig()
	if err != nil {
		return err
	}
	if err := cfg.EnsureDeviceProxy(toSingboxSpec(spec)); err != nil {
		return err
	}
	return a.op.ApplyConfig(ctx, cfg)
}

// ApplyDeviceProxyNoReload is the no-SIGHUP twin of ApplyDeviceProxy.
// Used by Service.SaveConfig when the diff is SelectedOutbound-only,
// so writing the new selector.default does not disturb the live
// selector.now that a hot-switch may have set.
//
// On the orchestrator path this maps to SaveSilent — the slot file is
// updated but no reload is scheduled. The slot's enabled flag is also
// pushed through SetEnabled in case the spec.Enabled has changed; that
// IS allowed to schedule a reload (toggling enabled is by definition
// a config-content change, not a no-op selector-default tweak).
func (a *SingboxAdapter) ApplyDeviceProxyNoReload(ctx context.Context, spec ExternalSpec) error {
	if a.orch != nil {
		sbSpec := toSingboxSpec(spec)
		data, err := singbox.BuildDeviceProxyConfig(sbSpec)
		if err != nil {
			return fmt.Errorf("build deviceproxy config: %w", err)
		}
		if err := a.orch.SaveSilent(orchestrator.SlotDeviceProxy, data); err != nil {
			return err
		}
		// SetEnabled is a no-op when the desired state already matches
		// (no rename, no reload arming). Safe to call unconditionally.
		if err := a.orch.SetEnabled(orchestrator.SlotDeviceProxy, sbSpec.Enabled); err != nil {
			return err
		}
		return nil
	}

	cfg, err := a.op.LoadCurrentConfig()
	if err != nil {
		return err
	}
	if err := cfg.EnsureDeviceProxy(toSingboxSpec(spec)); err != nil {
		return err
	}
	return a.op.ApplyConfigNoReload(ctx, cfg)
}

// GetSelectorActive returns the currently-active member of the named
// selector. Thin pass-through — see singbox.Operator for the contract.
func (a *SingboxAdapter) GetSelectorActive(ctx context.Context, selectorTag string) (string, error) {
	return a.op.GetSelectorActive(ctx, selectorTag)
}

func (a *SingboxAdapter) SetSelectorDefault(ctx context.Context, selectorTag, memberTag string) error {
	return a.op.SetSelectorDefault(ctx, selectorTag, memberTag)
}

func (a *SingboxAdapter) TunnelTags() []string {
	tunnels, err := a.op.ListTunnels(context.Background())
	if err != nil {
		return nil
	}
	tags := make([]string, 0, len(tunnels))
	for _, t := range tunnels {
		tags = append(tags, t.Tag)
	}
	return tags
}

func (a *SingboxAdapter) IsRunning() bool {
	running, _ := a.op.IsRunningPublic()
	return running
}

func toSingboxSpec(s ExternalSpec) singbox.DeviceProxySpec {
	out := singbox.DeviceProxySpec{
		Enabled:     s.Enabled,
		ListenAddr:  s.ListenAddr,
		Port:        s.Port,
		SelectedTag: s.SelectedTag,
		SBTags:      s.SBTags,
	}
	if s.Auth.Enabled {
		out.Auth = singbox.DeviceProxyAuth{
			Enabled:  true,
			Username: s.Auth.Username,
			Password: s.Auth.Password,
		}
	}
	out.AWGTags = append([]string(nil), s.AWGTags...)
	return out
}
