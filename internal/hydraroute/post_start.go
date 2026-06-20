package hydraroute

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hoaxisr/awg-manager/internal/sys/exec"
)

type postStartStatus struct {
	NDMS   string
	Kernel string
	Ready  bool
	Detail string
}

// ScheduleRestartAfterInterfaceOnline waits for the managed tunnel interface to
// become fully usable and then performs one debounced HR Neo restart.
//
// The caller context is intentionally ignored for the background wait:
// request cancellation must not cancel lifecycle reconciliation.
// Short bounded contexts are created internally for rule lookup, readiness wait,
// and neo restart.
func (s *Service) ScheduleRestartAfterInterfaceOnline(_ context.Context, reason string, ndmsIface string, kernelIface string) {
	if strings.TrimSpace(ndmsIface) == "" || strings.TrimSpace(kernelIface) == "" {
		s.appLog.Debug("post-start-skip-invalid-interface", kernelIface, fmt.Sprintf("reason=%s ndms=%s", reason, ndmsIface))
		return
	}

	s.mu.Lock()
	if !s.status.Installed {
		s.mu.Unlock()
		s.appLog.Debug("post-start-skip-not-installed", kernelIface, fmt.Sprintf("reason=%s ndms=%s", reason, ndmsIface))
		return
	}
	s.mu.Unlock()

	ruleCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	relevant, err := s.hasRelevantPostStartRules(ruleCtx, ndmsIface, kernelIface)
	if err != nil {
		s.appLog.Warn("post-start-rules", kernelIface, err.Error())
		return
	}
	if !relevant {
		s.appLog.Info("post-start-skip-no-rules", kernelIface, fmt.Sprintf("reason=%s ndms=%s", reason, ndmsIface))
		return
	}

	s.mu.Lock()
	if s.postStartPending == nil {
		s.postStartPending = make(map[string]string)
	}
	s.postStartPending[ndmsIface] = kernelIface
	if !s.postStartLastRestartAt.IsZero() {
		if remainingCooldown := s.postStartCooldown - time.Since(s.postStartLastRestartAt); remainingCooldown > 0 {
			if !s.postStartRunning {
				s.schedulePostStartAfterLocked(remainingCooldown + s.postStartDebounce)
			}
			s.mu.Unlock()
			s.appLog.Debug("post-start-wait", kernelIface, fmt.Sprintf("queued during cooldown: remaining=%s", remainingCooldown.Round(time.Millisecond)))
			return
		}
	}
	if s.postStartRunning {
		s.mu.Unlock()
		s.appLog.Info("post-start-wait", kernelIface, fmt.Sprintf("joined active wait: reason=%s ndms=%s", reason, ndmsIface))
		return
	}
	s.scheduleNextPostStartLocked()
	s.mu.Unlock()

	s.appLog.Info("post-start-wait", kernelIface, fmt.Sprintf("scheduled: reason=%s ndms=%s", reason, ndmsIface))
}

func (s *Service) runPostStartCycle() {
	s.mu.Lock()
	if s.postStartRunning {
		s.mu.Unlock()
		return
	}
	s.postStartRunning = true
	s.postStartTimer = nil
	pollInterval := s.postStartPollInterval
	readyDelay := s.postStartReadyDelay
	timeout := s.postStartTimeout
	s.mu.Unlock()

	s.appLog.Debug("post-start-cycle-start", "", fmt.Sprintf("poll=%s readyDelay=%s timeout=%s", pollInterval, readyDelay, timeout))
	defer s.finishPostStartCycle()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var lastStatuses []postStartStatus
	for {
		statuses := s.collectPendingStatuses(ctx)
		lastStatuses = statuses

		allReady, anyReady := classifyPostStartStatuses(statuses)
		if len(statuses) > 0 && allReady {
			s.logPostStartReady(statuses)
			if !s.waitPostStartReadyDelay(ctx, readyDelay) {
				s.handlePostStartTimeout(statuses)
				return
			}
			s.executePostStartRestart(statuses)
			return
		}

		select {
		case <-ctx.Done():
			s.handlePostStartTimeout(lastStatuses)
			return
		case <-ticker.C:
			if anyReady {
				// Keep waiting until either all pending interfaces are ready or
				// the bounded timeout expires and we intentionally restart once.
			}
		}
	}
}

func (s *Service) collectPendingStatuses(ctx context.Context) []postStartStatus {
	s.mu.Lock()
	pending := make(map[string]string, len(s.postStartPending))
	for ndmsIface, kernelIface := range s.postStartPending {
		pending[ndmsIface] = kernelIface
	}
	probe := s.postStartReadyProbe
	s.mu.Unlock()

	if probe == nil {
		probe = s.isInterfaceReady
	}

	statuses := make([]postStartStatus, 0, len(pending))
	for ndmsIface, kernelIface := range pending {
		ready, detail, err := probe(ctx, ndmsIface)
		if err != nil {
			detail = err.Error()
			s.appLog.Debug("post-start-wait", kernelIface, fmt.Sprintf("ndms=%s %v", ndmsIface, err))
		}
		statuses = append(statuses, postStartStatus{
			NDMS:   ndmsIface,
			Kernel: kernelIface,
			Ready:  ready,
			Detail: detail,
		})
	}
	return statuses
}

func classifyPostStartStatuses(statuses []postStartStatus) (allReady bool, anyReady bool) {
	if len(statuses) == 0 {
		return false, false
	}
	allReady = true
	for _, st := range statuses {
		if st.Ready {
			anyReady = true
			continue
		}
		allReady = false
	}
	return allReady, anyReady
}

func splitReadyPostStartStatuses(statuses []postStartStatus) (ready []postStartStatus, waiting []postStartStatus) {
	for _, st := range statuses {
		if st.Ready {
			ready = append(ready, st)
			continue
		}
		waiting = append(waiting, st)
	}
	return ready, waiting
}

func (s *Service) logPostStartReady(statuses []postStartStatus) {
	for _, st := range statuses {
		s.appLog.Info("post-start-ready", st.Kernel, fmt.Sprintf("ndms=%s %s", st.NDMS, st.Detail))
	}
}

func (s *Service) waitPostStartReadyDelay(ctx context.Context, readyDelay time.Duration) bool {
	if readyDelay <= 0 {
		return true
	}
	timer := time.NewTimer(readyDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (s *Service) executePostStartRestart(statuses []postStartStatus) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	reason := "unknown"
	if len(statuses) > 0 {
		reason = statuses[0].Kernel
	}
	for _, st := range statuses {
		s.appLog.Info("post-start-restart", st.Kernel, fmt.Sprintf("ndms=%s", st.NDMS))
	}
	if err := s.postStartRestart(ctx); err != nil {
		s.appLog.Warn("post-start-restart", reason, err.Error())
	} else {
		s.appLog.Info("post-start-restart-completed", reason, fmt.Sprintf("restarted for %d interface(s)", len(statuses)))
	}

	s.mu.Lock()
	for _, st := range statuses {
		delete(s.postStartPending, st.NDMS)
	}
	s.postStartLastRestartAt = time.Now()
	s.mu.Unlock()
}

func (s *Service) handlePostStartTimeout(statuses []postStartStatus) {
	ready, waiting := splitReadyPostStartStatuses(statuses)
	s.appLog.Warn("post-start-timeout", "", s.pendingSummary(statuses))
	if len(ready) > 0 {
		if len(waiting) > 0 {
			s.appLog.Info("post-start-timeout-partial", "", s.pendingSummary(waiting))
		}
		s.executePostStartRestart(ready)
		return
	}

	s.mu.Lock()
	s.postStartPending = nil
	s.mu.Unlock()
	s.appLog.Warn("post-start-timeout-drop", "", "dropping pending interfaces after timeout without ready candidates")
}

func (s *Service) isInterfaceReady(ctx context.Context, ndmsIface string) (bool, string, error) {
	s.mu.Lock()
	queries := s.queries
	s.mu.Unlock()
	if queries == nil || queries.Interfaces == nil {
		return false, "", fmt.Errorf("NDMS interface queries are not configured")
	}

	iface, err := queries.Interfaces.Get(ctx, ndmsIface)
	if err != nil {
		return false, "", err
	}
	if iface == nil {
		return false, "", fmt.Errorf("interface %s not found", ndmsIface)
	}

	details, err := queries.Interfaces.FetchSummary(ctx, ndmsIface)
	if err != nil {
		return false, "", err
	}
	if details == nil {
		return false, "", fmt.Errorf("interface %s details unavailable", ndmsIface)
	}

	detail := fmt.Sprintf(
		"connected=%s summaryConnected=%t online=%t conf=%s link=%s state=%s",
		iface.Connected,
		details.Connected,
		details.Online,
		details.ConfLayer,
		details.Link,
		details.State,
	)
	ready := details.Connected &&
		details.Online &&
		details.ConfLayer == "running" &&
		details.Link == "up" &&
		details.State == "up"
	return ready, detail, nil
}

func (s *Service) hasRelevantPostStartRules(ctx context.Context, ndmsIface string, kernelIface string) (bool, error) {
	rules, _, err := s.ListRules()
	if err != nil {
		return false, err
	}

	hasPolicyTargets := false
	for _, rule := range rules {
		if rule.Disabled {
			continue
		}
		target := strings.TrimSpace(rule.Target)
		if target == "" {
			continue
		}
		if target == kernelIface {
			return true, nil
		}
		if target != kernelIface {
			hasPolicyTargets = true
		}
	}
	if !hasPolicyTargets {
		return false, nil
	}

	s.mu.Lock()
	queries := s.queries
	s.mu.Unlock()
	if queries == nil || queries.Policies == nil {
		return true, nil
	}

	policies, err := queries.Policies.List(ctx)
	if err != nil {
		s.appLog.Warn("post-start-policy-check", kernelIface, err.Error())
		return true, nil
	}
	permitted := make(map[string]map[string]struct{}, len(policies))
	for _, policy := range policies {
		if permitted[policy.Name] == nil {
			permitted[policy.Name] = make(map[string]struct{})
		}
		for _, iface := range policy.Interfaces {
			if iface.Denied {
				continue
			}
			permitted[policy.Name][iface.Name] = struct{}{}
		}
	}

	for _, rule := range rules {
		if rule.Disabled {
			continue
		}
		target := strings.TrimSpace(rule.Target)
		if target == "" || target == kernelIface {
			continue
		}
		if allowed, ok := permitted[target]; ok {
			if _, match := allowed[ndmsIface]; match {
				return true, nil
			}
		}
	}

	return false, nil
}

func (s *Service) scheduleNextPostStartLocked() {
	s.schedulePostStartAfterLocked(s.postStartDebounce)
}

func (s *Service) finishPostStartCycle() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.postStartRunning = false
	defer s.appLog.Debug("post-start-cycle-finish", "", fmt.Sprintf("pending=%d timerArmed=%t", len(s.postStartPending), s.postStartTimer != nil))
	if len(s.postStartPending) == 0 || s.postStartTimer != nil {
		return
	}

	delay := s.postStartDebounce
	if !s.postStartLastRestartAt.IsZero() {
		if remaining := s.postStartCooldown - time.Since(s.postStartLastRestartAt); remaining > 0 {
			delay += remaining
		}
	}
	s.appLog.Debug("post-start-reschedule", "", fmt.Sprintf("pending=%d delay=%s", len(s.postStartPending), delay))
	s.schedulePostStartAfterLocked(delay)
}

func (s *Service) schedulePostStartAfterLocked(delay time.Duration) {
	if s.postStartTimer != nil {
		s.postStartTimer.Stop()
	}
	if delay < 0 {
		delay = 0
	}
	s.postStartTimer = time.AfterFunc(delay, func() {
		s.runPostStartCycle()
	})
}

func (s *Service) restartAfterPostStart(ctx context.Context) error {
	controlPath := activeControlPath()
	if controlPath == "" {
		err := fmt.Errorf("neo restart: control command not found")
		s.mu.Lock()
		s.lastError = err.Error()
		s.status = Detect()
		s.status.Version = s.getVersionCachedLocked()
		s.status = s.enrichStatusLocked(s.status)
		if s.status.Running {
			s.status.LastError = ""
		} else {
			s.status.LastError = s.lastError
		}
		s.mu.Unlock()
		return err
	}
	result, err := exec.Run(ctx, controlPath, "restart")
	if err != nil {
		formatted := fmt.Errorf("neo restart: %w", exec.FormatError(result, err))
		s.mu.Lock()
		s.lastError = formatted.Error()
		s.status = Detect()
		s.status.Version = s.getVersionCachedLocked()
		s.status = s.enrichStatusLocked(s.status)
		if s.status.Running {
			s.status.LastError = ""
		} else {
			s.status.LastError = s.lastError
		}
		s.mu.Unlock()
		return formatted
	}

	s.mu.Lock()
	s.lastError = ""
	s.status = Detect()
	s.status.Version = s.getVersionCachedLocked()
	s.status = s.enrichStatusLocked(s.status)
	if s.status.Running {
		s.status.LastError = ""
	} else {
		s.status.LastError = s.lastError
	}
	s.mu.Unlock()
	return nil
}

func (s *Service) pendingSummary(statuses []postStartStatus) string {
	if len(statuses) == 0 {
		s.mu.Lock()
		defer s.mu.Unlock()
		if len(s.postStartPending) == 0 {
			return "no pending interfaces"
		}
		statuses = make([]postStartStatus, 0, len(s.postStartPending))
		for ndmsIface, kernelIface := range s.postStartPending {
			statuses = append(statuses, postStartStatus{NDMS: ndmsIface, Kernel: kernelIface})
		}
	}
	parts := make([]string, 0, len(statuses))
	for _, st := range statuses {
		suffix := ""
		if st.Detail != "" {
			suffix = ": " + st.Detail
		}
		parts = append(parts, fmt.Sprintf("%s(%s)%s", st.NDMS, st.Kernel, suffix))
	}
	return "pending: " + strings.Join(parts, ", ")
}
