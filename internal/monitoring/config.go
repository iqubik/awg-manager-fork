package monitoring

import (
	"time"

	"github.com/hoaxisr/awg-manager/internal/storage"
)

const (
	DefaultMonitoringSampleInterval  = time.Duration(storage.DefaultMonitoringSampleIntervalSec) * time.Second
	DefaultMonitoringHistoryHours    = storage.DefaultMonitoringHistoryHours
	DefaultMonitoringHistoryCapacity = storage.DefaultMonitoringHistoryHours * 3600 / storage.DefaultMonitoringSampleIntervalSec
)

func effectiveMonitoringSettings(store *storage.SettingsStore) storage.MonitoringSettings {
	if store == nil {
		return storage.DefaultMonitoringSettings()
	}
	settings, err := store.Get()
	if err != nil || settings == nil {
		return storage.DefaultMonitoringSettings()
	}
	return storage.NormalizeMonitoringSettings(settings.Monitoring)
}

func monitoringSampleInterval(store *storage.SettingsStore) time.Duration {
	settings := effectiveMonitoringSettings(store)
	return time.Duration(settings.SampleIntervalSec) * time.Second
}

func monitoringHistoryCapacity(store *storage.SettingsStore) int {
	return storage.GetMonitoringHistoryCapacity(effectiveMonitoringSettings(store))
}
