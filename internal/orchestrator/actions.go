package orchestrator

import "github.com/hoaxisr/awg-manager/internal/tunnel"

// ActionType identifies what the executor should do.
type ActionType int

const (
	// Tunnel lifecycle
	ActionColdStartKernel ActionType = iota
	ActionStartNativeWG
	ActionStopKernel
	ActionStopNativeWG
	ActionSuspendProxy
	ActionRestoreKmod
	ActionRestoreEndpointTracking
	ActionLinkToggle
	ActionReconcileKernel
	ActionSuspendKernel
	ActionResumeKernel
	ActionReconcileNativeWG

	// Monitoring
	ActionStartMonitoring
	ActionStopMonitoring
	ActionConfigurePingCheck
	ActionRemovePingCheck

	// Routing
	ActionApplyDNSRoutes
	ActionApplyStaticRoutes
	ActionRemoveStaticRoutes
	ActionApplyClientRoutes
	ActionRemoveClientRoutes
	ActionReconcileStaticRoutes
	ActionReconcileDNSRoutes
	ActionDeleteDNSRoutes
	ActionDeleteStaticRoutes
	ActionDeleteClientRoutes
	ActionHydraRoutePostStart

	// Persistence
	ActionPersistRunning
	ActionPersistStopped

	// CRUD
	ActionDeleteKernel
	ActionDeleteNativeWG
)

// Action is one step in the execution plan.
type Action struct {
	Type    ActionType
	Tunnel  string
	Config  *tunnel.Config
	WAN     string // resolved WAN interface
	Iface   string // kernel interface name
	NDMS    string // NDMS interface name
	Enabled *bool
	Reason  string
}
