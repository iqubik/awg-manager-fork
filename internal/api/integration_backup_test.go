package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/hydraroute"
	"github.com/hoaxisr/awg-manager/internal/integrationbackup"
	"github.com/hoaxisr/awg-manager/internal/response"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

type testIntegrationSingbox struct {
	dir        string
	restartErr error
}

func (s *testIntegrationSingbox) ConfigDir() string { return s.dir }
func (s *testIntegrationSingbox) ValidateConfigDir(context.Context) error {
	return nil
}
func (s *testIntegrationSingbox) ValidateConfigPath(context.Context, string) error {
	return nil
}
func (s *testIntegrationSingbox) Control(_ context.Context, action string) error {
	if action == "restart" && s.restartErr != nil {
		return s.restartErr
	}
	return nil
}
func (s *testIntegrationSingbox) IsRunning() (bool, int) { return true, 123 }

type testIntegrationHydra struct{}

func (h *testIntegrationHydra) ReadConfig() (*hydraroute.Config, error) {
	return &hydraroute.Config{}, nil
}
func (h *testIntegrationHydra) ListRules() ([]hydraroute.HRRule, []string, error) {
	return nil, nil, nil
}
func (h *testIntegrationHydra) Control(string) error { return nil }
func (h *testIntegrationHydra) GetStatus() hydraroute.Status {
	return hydraroute.Status{Installed: true}
}

func TestSingboxRestoreReturnsRollbackEnvelopeOnRollback(t *testing.T) {
	root := t.TempDir()
	store := storage.NewSettingsStore(root)
	if _, err := store.Load(); err != nil {
		t.Fatalf("settings load: %v", err)
	}
	sb := &testIntegrationSingbox{dir: filepath.Join(root, "singbox", "config.d")}
	if err := os.MkdirAll(sb.dir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	writeAPIBackupTestFile(t, filepath.Join(sb.dir, "00-base.json"), `{"log":{"level":"info"}}`)
	svc := integrationbackup.NewService(root, store, sb, &testIntegrationHydra{})

	_, payload, err := svc.CreateBackup(context.Background(), integrationbackup.ComponentSingbox)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	writeAPIBackupTestFile(t, filepath.Join(sb.dir, "99-old.json"), `{"route":{"final":"old"}}`)
	sb.restartErr = errors.New("restart failed")

	req := httptest.NewRequest(http.MethodPost, "/api/singbox/restore", strings.NewReader(string(payload)))
	rec := httptest.NewRecorder()

	handler := NewIntegrationBackupHandler(svc)
	handler.SingboxRestore(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 rollback envelope, got %d body=%s", rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                              `json:"success"`
		Data    integrationbackup.RestoreResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success {
		t.Fatalf("expected success envelope, got body=%s", rec.Body.String())
	}
	if len(envelope.Data.Warnings) == 0 {
		t.Fatalf("expected rollback warning, got %+v", envelope.Data)
	}
	foundRollback := false
	for _, outcome := range envelope.Data.Outcomes {
		if outcome.Action == "rollback" {
			foundRollback = true
			break
		}
	}
	if !foundRollback {
		t.Fatalf("expected rollback outcome, got %+v", envelope.Data.Outcomes)
	}

	var apiResp response.APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &apiResp); err != nil {
		t.Fatalf("decode envelope via response type: %v", err)
	}
	if !apiResp.Success || apiResp.Error {
		t.Fatalf("unexpected API envelope flags: %+v", apiResp)
	}
}

func writeAPIBackupTestFile(t *testing.T, target string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", target, err)
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", target, err)
	}
}
