package storage

import "testing"

func TestSettingsPatchZeroValuesAreOptional(t *testing.T) {
	var p SettingsPatch
	if p.AuthEnabled != nil || p.ApiKey != nil || p.Server != nil || p.Logging != nil {
		t.Fatalf("zero SettingsPatch should keep all optional fields nil: %#v", p)
	}
}

func TestLoggingSettingsPatchZeroValuesAreOptional(t *testing.T) {
	var p LoggingSettingsPatch
	if p.Enabled != nil || p.MaxAge != nil || p.LogLevel != nil || p.SingboxLogLevel != nil {
		t.Fatalf("zero LoggingSettingsPatch should keep optional fields nil: %#v", p)
	}
}

