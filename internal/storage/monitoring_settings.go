package storage

import "fmt"

const (
	DefaultMonitoringHistoryHours         = 24
	DefaultMonitoringSampleIntervalSec    = 60
	DefaultMonitoringMatrixRefreshSec     = 60
	MinMonitoringHistoryHours             = 1
	MaxMonitoringHistoryHours             = 168
	MinMonitoringSampleIntervalSec        = 10
	MaxMonitoringSampleIntervalSec        = 3600
	MinMonitoringMatrixRefreshIntervalSec = 10
	MaxMonitoringMatrixRefreshIntervalSec = 3600
	MaxMonitoringHistoryCapacity          = 10080
)

type MonitoringSettings struct {
	HistoryHours             int `json:"historyHours"`
	SampleIntervalSec        int `json:"sampleIntervalSec"`
	MatrixRefreshIntervalSec int `json:"matrixRefreshIntervalSec"`
}

func monitoringHistoryCapacityRaw(value MonitoringSettings) int {
	return (value.HistoryHours*3600 + value.SampleIntervalSec - 1) / value.SampleIntervalSec
}

func DefaultMonitoringSettings() MonitoringSettings {
	return MonitoringSettings{
		HistoryHours:             DefaultMonitoringHistoryHours,
		SampleIntervalSec:        DefaultMonitoringSampleIntervalSec,
		MatrixRefreshIntervalSec: DefaultMonitoringMatrixRefreshSec,
	}
}

func NormalizeMonitoringSettings(value MonitoringSettings) MonitoringSettings {
	normalized := value
	if normalized.HistoryHours < MinMonitoringHistoryHours || normalized.HistoryHours > MaxMonitoringHistoryHours {
		normalized.HistoryHours = DefaultMonitoringHistoryHours
	}
	if normalized.SampleIntervalSec < MinMonitoringSampleIntervalSec || normalized.SampleIntervalSec > MaxMonitoringSampleIntervalSec {
		normalized.SampleIntervalSec = DefaultMonitoringSampleIntervalSec
	}
	if normalized.MatrixRefreshIntervalSec < MinMonitoringMatrixRefreshIntervalSec || normalized.MatrixRefreshIntervalSec > MaxMonitoringMatrixRefreshIntervalSec {
		normalized.MatrixRefreshIntervalSec = DefaultMonitoringMatrixRefreshSec
	}
	if monitoringHistoryCapacityRaw(normalized) > MaxMonitoringHistoryCapacity {
		normalized = DefaultMonitoringSettings()
	}
	return normalized
}

func GetMonitoringHistoryCapacity(value MonitoringSettings) int {
	normalized := NormalizeMonitoringSettings(value)
	return monitoringHistoryCapacityRaw(normalized)
}

func ValidateMonitoringSettings(value MonitoringSettings) error {
	if value.HistoryHours < MinMonitoringHistoryHours || value.HistoryHours > MaxMonitoringHistoryHours {
		return fmt.Errorf("monitoring.historyHours must be an integer between %d and %d", MinMonitoringHistoryHours, MaxMonitoringHistoryHours)
	}
	if value.SampleIntervalSec < MinMonitoringSampleIntervalSec || value.SampleIntervalSec > MaxMonitoringSampleIntervalSec {
		return fmt.Errorf("monitoring.sampleIntervalSec must be an integer between %d and %d", MinMonitoringSampleIntervalSec, MaxMonitoringSampleIntervalSec)
	}
	if value.MatrixRefreshIntervalSec < MinMonitoringMatrixRefreshIntervalSec || value.MatrixRefreshIntervalSec > MaxMonitoringMatrixRefreshIntervalSec {
		return fmt.Errorf("monitoring.matrixRefreshIntervalSec must be an integer between %d and %d", MinMonitoringMatrixRefreshIntervalSec, MaxMonitoringMatrixRefreshIntervalSec)
	}
	if monitoringHistoryCapacityRaw(value) > MaxMonitoringHistoryCapacity {
		return fmt.Errorf("Слишком большой объём истории. Увеличьте шаг замера или уменьшите окно истории.")
	}
	return nil
}
