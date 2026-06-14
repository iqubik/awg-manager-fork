package monitoring

import "time"

const (
	// MonitoringSampleInterval is the scheduler/history step used by the
	// monitoring matrix.
	MonitoringSampleInterval = time.Minute
	// MonitoringRetentionHours is the history window kept per matrix cell.
	MonitoringRetentionHours = 24
	// MonitoringHistoryCapacity is the number of retained samples per
	// (target, tunnel) pair at the current scheduler interval.
	MonitoringHistoryCapacity = MonitoringRetentionHours * int(time.Hour/MonitoringSampleInterval)
)
