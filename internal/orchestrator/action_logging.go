package orchestrator

import (
	"fmt"
	"strconv"
	"strings"
)

func actionName(typ ActionType) string {
	switch typ {
	case ActionColdStartKernel:
		return "ActionColdStartKernel"
	case ActionStartNativeWG:
		return "ActionStartNativeWG"
	case ActionStopKernel:
		return "ActionStopKernel"
	case ActionStopNativeWG:
		return "ActionStopNativeWG"
	case ActionSuspendProxy:
		return "ActionSuspendProxy"
	case ActionRestoreKmod:
		return "ActionRestoreKmod"
	case ActionRestoreEndpointTracking:
		return "ActionRestoreEndpointTracking"
	case ActionLinkToggle:
		return "ActionLinkToggle"
	case ActionReconcileKernel:
		return "ActionReconcileKernel"
	case ActionSuspendKernel:
		return "ActionSuspendKernel"
	case ActionResumeKernel:
		return "ActionResumeKernel"
	case ActionReconcileNativeWG:
		return "ActionReconcileNativeWG"
	case ActionApplyConfig:
		return "ActionApplyConfig"
	case ActionSetMTU:
		return "ActionSetMTU"
	case ActionSetDefaultRoute:
		return "ActionSetDefaultRoute"
	case ActionRemoveDefaultRoute:
		return "ActionRemoveDefaultRoute"
	case ActionStartMonitoring:
		return "ActionStartMonitoring"
	case ActionStopMonitoring:
		return "ActionStopMonitoring"
	case ActionConfigurePingCheck:
		return "ActionConfigurePingCheck"
	case ActionRemovePingCheck:
		return "ActionRemovePingCheck"
	case ActionApplyDNSRoutes:
		return "ActionApplyDNSRoutes"
	case ActionApplyStaticRoutes:
		return "ActionApplyStaticRoutes"
	case ActionRemoveStaticRoutes:
		return "ActionRemoveStaticRoutes"
	case ActionApplyClientRoutes:
		return "ActionApplyClientRoutes"
	case ActionRemoveClientRoutes:
		return "ActionRemoveClientRoutes"
	case ActionApplySystemClientRoutes:
		return "ActionApplySystemClientRoutes"
	case ActionRemoveSystemClientRoutes:
		return "ActionRemoveSystemClientRoutes"
	case ActionReconcileStaticRoutes:
		return "ActionReconcileStaticRoutes"
	case ActionReconcileDNSRoutes:
		return "ActionReconcileDNSRoutes"
	case ActionDeleteDNSRoutes:
		return "ActionDeleteDNSRoutes"
	case ActionDeleteStaticRoutes:
		return "ActionDeleteStaticRoutes"
	case ActionDeleteClientRoutes:
		return "ActionDeleteClientRoutes"
	case ActionHydraRoutePostStart:
		return "ActionHydraRoutePostStart"
	case ActionPersistRunning:
		return "ActionPersistRunning"
	case ActionPersistStopped:
		return "ActionPersistStopped"
	case ActionPersistEnabled:
		return "ActionPersistEnabled"
	case ActionCreateKernel:
		return "ActionCreateKernel"
	case ActionCreateNativeWG:
		return "ActionCreateNativeWG"
	case ActionDeleteKernel:
		return "ActionDeleteKernel"
	case ActionDeleteNativeWG:
		return "ActionDeleteNativeWG"
	default:
		return "ActionType(" + strconv.Itoa(int(typ)) + ")"
	}
}

func actionTarget(action Action) string {
	switch {
	case action.Tunnel != "":
		return action.Tunnel
	case action.WAN != "":
		return action.WAN
	case action.NDMS != "":
		return action.NDMS
	default:
		return "orchestrator"
	}
}

func actionMessage(action Action) string {
	parts := []string{actionName(action.Type)}
	if action.Type == ActionHydraRoutePostStart {
		if action.Reason != "" {
			parts = append(parts, "reason="+action.Reason)
		}
		if action.NDMS != "" {
			parts = append(parts, "ndms="+action.NDMS)
		}
		if action.Iface != "" {
			parts = append(parts, "iface="+action.Iface)
		}
		return strings.Join(parts, " ")
	}
	if action.WAN != "" {
		parts = append(parts, "wan="+action.WAN)
	}
	if action.Enabled != nil {
		parts = append(parts, "enabled="+strconv.FormatBool(*action.Enabled))
	}
	return strings.Join(parts, " ")
}

func (o *Orchestrator) logActionStart(action Action) {
	if o == nil || o.appLog == nil {
		return
	}
	o.appLog.Full("action-start", actionTarget(action), actionMessage(action))
}

func (o *Orchestrator) logActionFinish(action Action, err error) {
	if o == nil || o.appLog == nil {
		return
	}
	if err != nil {
		o.appLog.Warn("action-error", actionTarget(action), fmt.Sprintf("%s error=%v", actionMessage(action), err))
		return
	}
	o.appLog.Full("action-done", actionTarget(action), actionMessage(action))
}

func (o *Orchestrator) logActionSkip(action Action, reason string) {
	if o == nil || o.appLog == nil {
		return
	}
	msg := actionMessage(action)
	if reason != "" {
		msg += " skip=" + reason
	}
	o.appLog.Debug("action-skip", actionTarget(action), msg)
}
