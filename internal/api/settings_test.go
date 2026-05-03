package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/storage"
)

// newSettingsHandlerForTest returns a SettingsHandler backed by an
// isolated SettingsStore in a temp directory. The store is preloaded so
// defaults (including the v16 UsageLevel) are populated.
//
// nil AppLogger is intentionally passed — see internal/logging/applogger.go:
// "Safe to use with nil appLogger — all methods become no-ops."
func newSettingsHandlerForTest(t *testing.T) (*SettingsHandler, *storage.SettingsStore) {
	t.Helper()
	tmp := t.TempDir()
	store := storage.NewSettingsStore(tmp)
	if _, err := store.Load(); err != nil {
		t.Fatalf("seed Load: %v", err)
	}
	h := NewSettingsHandler(store, nil)
	return h, store
}

// TestUpdateUsageLevelAccepted verifies that a valid usageLevel value
// (here: "expert") is accepted and persisted by the Update handler.
func TestUpdateUsageLevelAccepted(t *testing.T) {
	h, store := newSettingsHandlerForTest(t)
	current, _ := store.Get()
	// store.Get() returns a pointer into the cache; copy by value so
	// payload mutations do not retroactively alias the cached state read
	// back as oldSettings inside the handler.
	payload := *current
	payload.UsageLevel = storage.UsageLevelExpert
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/settings/update", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	got, _ := store.Get()
	if got.UsageLevel != storage.UsageLevelExpert {
		t.Errorf("UsageLevel after update = %q, want expert", got.UsageLevel)
	}
}

// TestUpdateUsageLevelInvalidRejected verifies that an unknown
// usageLevel value is rejected with 400 and the INVALID_USAGE_LEVEL
// error code, instead of being silently coerced to a default.
func TestUpdateUsageLevelInvalidRejected(t *testing.T) {
	h, _ := newSettingsHandlerForTest(t)
	body := []byte(`{
		"schemaVersion": 16,
		"authEnabled": false,
		"server": {"port": 2222, "interface": "br0"},
		"pingCheck": {"enabled": false, "defaults": {"method":"http","target":"8.8.8.8","interval":45,"deadInterval":120,"failThreshold":3}},
		"logging": {"enabled": true, "maxAge": 2},
		"disableMemorySaving": false,
		"updates": {"checkEnabled": true},
		"dnsRoute": {"autoRefreshEnabled": false},
		"usageLevel": "garbage",
		"singboxRouter": {"enabled": false, "policyName": "", "refreshMode": "interval", "refreshIntervalHours": 24}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/settings/update", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "INVALID_USAGE_LEVEL") {
		t.Errorf("body missing error code:\n%s", rec.Body.String())
	}
}

// TestUpdateUsageLevelEmptyPreserves verifies that a payload that
// OMITS usageLevel preserves the previously stored value. This is
// the partial-update defense after the SettingsPatch refactor:
// nil pointer in the patch DTO means "field absent in payload, keep
// existing value." An EXPLICIT empty string is now correctly rejected
// as INVALID_USAGE_LEVEL (separate test).
func TestUpdateUsageLevelEmptyPreserves(t *testing.T) {
	h, store := newSettingsHandlerForTest(t)

	// Pre-seed with expert.
	current, _ := store.Get()
	seed := *current
	seed.UsageLevel = storage.UsageLevelExpert
	if err := store.Save(&seed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Build a payload that OMITS usageLevel entirely. Marshalling the
	// struct with UsageLevel="" would still serialize the field (no
	// omitempty), so we hand-craft the raw JSON instead.
	body := []byte(`{
		"schemaVersion": 16,
		"authEnabled": false,
		"server": {"port": 2222, "interface": "br0"},
		"pingCheck": {"enabled": false, "defaults": {"method":"http","target":"8.8.8.8","interval":45,"deadInterval":120,"failThreshold":3}},
		"logging": {"enabled": true, "maxAge": 2},
		"disableMemorySaving": false,
		"updates": {"checkEnabled": true},
		"dnsRoute": {"autoRefreshEnabled": false},
		"singboxRouter": {"enabled": false, "policyName": "", "refreshMode": "interval", "refreshIntervalHours": 24}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/settings/update", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	got, _ := store.Get()
	if got.UsageLevel != storage.UsageLevelExpert {
		t.Errorf("UsageLevel = %q after omitted update, want expert (preserved)", got.UsageLevel)
	}
}

// TestUpdate_PartialPayload_PreservesOmittedFields verifies the
// post-refactor defense: a PATCH body containing only one sub-struct
// must leave every other field of the saved settings untouched.
func TestUpdate_PartialPayload_PreservesOmittedFields(t *testing.T) {
	h, store := newSettingsHandlerForTest(t)

	// Pre-seed with non-default values across multiple fields so we can
	// detect any silent revert.
	current, _ := store.Get()
	seed := *current
	seed.AuthEnabled = true
	seed.ApiKey = "seeded-key"
	seed.Server.Port = 3333
	seed.UsageLevel = storage.UsageLevelExpert
	if err := store.Save(&seed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Patch only logging.
	body := []byte(`{"logging":{"enabled":false,"maxAge":4,"logLevel":"warn"}}`)
	req := httptest.NewRequest(http.MethodPost, "/settings/update", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	got, _ := store.Get()
	if got.AuthEnabled != true {
		t.Errorf("AuthEnabled = %v, want true (preserved)", got.AuthEnabled)
	}
	if got.ApiKey != "seeded-key" {
		t.Errorf("ApiKey = %q, want seeded-key (preserved)", got.ApiKey)
	}
	if got.Server.Port != 3333 {
		t.Errorf("Server.Port = %d, want 3333 (preserved)", got.Server.Port)
	}
	if got.UsageLevel != storage.UsageLevelExpert {
		t.Errorf("UsageLevel = %q, want expert (preserved)", got.UsageLevel)
	}
	if got.Logging.Enabled != false || got.Logging.MaxAge != 4 || got.Logging.LogLevel != "warn" {
		t.Errorf("Logging not applied as patched: %+v", got.Logging)
	}
}

// TestUpdate_AuthEnabledFalse_NotReverted is the headline security
// regression test. Before the refactor, top-level bool fields like
// AuthEnabled could not be defended against partial payloads — an
// explicit `false` was indistinguishable from "not sent" in a value-
// typed DTO and would be silently reverted by the zero-value-restore
// defense. After the refactor, an explicit false propagates correctly.
func TestUpdate_AuthEnabledFalse_NotReverted(t *testing.T) {
	h, store := newSettingsHandlerForTest(t)

	// Pre-seed with AuthEnabled=true.
	current, _ := store.Get()
	seed := *current
	seed.AuthEnabled = true
	if err := store.Save(&seed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Send patch with only authEnabled:false.
	body := []byte(`{"authEnabled":false}`)
	req := httptest.NewRequest(http.MethodPost, "/settings/update", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	got, _ := store.Get()
	if got.AuthEnabled != false {
		t.Errorf("AuthEnabled = %v after explicit false patch, want false", got.AuthEnabled)
	}
}

// TestUpdate_FullPayload_BehavesIdenticallyToBefore verifies backward
// compatibility with the existing frontend payload pattern: every
// field present in the request, ApplyPatch sets every field, save
// matches the request payload (modulo computed/derived fields).
func TestUpdate_FullPayload_BehavesIdenticallyToBefore(t *testing.T) {
	h, store := newSettingsHandlerForTest(t)
	current, _ := store.Get()
	payload := *current
	payload.AuthEnabled = true
	payload.UsageLevel = storage.UsageLevelExpert
	payload.Logging.Enabled = false
	payload.Logging.MaxAge = 7
	payload.Server.Port = 4444
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/settings/update", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	got, _ := store.Get()
	if got.AuthEnabled != true || got.UsageLevel != storage.UsageLevelExpert ||
		got.Logging.Enabled != false || got.Logging.MaxAge != 7 ||
		got.Server.Port != 4444 {
		t.Errorf("full-payload identity merge failed: %+v", got)
	}
}

// TestUpdate_ApiKeyExplicitEmpty_Preserved verifies the defense-in-depth
// guard against a buggy/stale client accidentally revoking its own
// Bearer-auth key. An explicit empty apiKey in the payload is treated as
// "absent" so the saved key survives. The supported rotation path is
// /settings/regenerate-api-key — explicit-clear is intentionally NOT a
// public contract.
func TestUpdate_ApiKeyExplicitEmpty_Preserved(t *testing.T) {
	h, store := newSettingsHandlerForTest(t)

	current, _ := store.Get()
	seed := *current
	seed.ApiKey = "preexisting-key"
	if err := store.Save(&seed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	body := []byte(`{"apiKey":""}`)
	req := httptest.NewRequest(http.MethodPost, "/settings/update", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	got, _ := store.Get()
	if got.ApiKey != "preexisting-key" {
		t.Errorf("ApiKey = %q after explicit empty patch, want preexisting-key", got.ApiKey)
	}
}
