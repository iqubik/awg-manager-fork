package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/auth"
	"github.com/hoaxisr/awg-manager/internal/logging"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

type silentAppLogger struct{}

func (silentAppLogger) AppLog(level logging.Level, group, subgroup, action, target, message string) {}

func TestAuthStatus_WrongMethod(t *testing.T) {
	sessions := auth.NewSessionStore()
	t.Cleanup(sessions.Stop)
	settings := storage.NewSettingsStore(t.TempDir())
	if _, err := settings.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	h := NewAuthHandler(nil, sessions, settings, silentAppLogger{})

	req := httptest.NewRequest(http.MethodPost, "/auth/status", nil)
	rr := httptest.NewRecorder()
	h.Status(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestAuthStatus_AuthDisabled(t *testing.T) {
	sessions := auth.NewSessionStore()
	t.Cleanup(sessions.Stop)
	settings := storage.NewSettingsStore(t.TempDir())
	if _, err := settings.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	h := NewAuthHandler(nil, sessions, settings, silentAppLogger{})

	req := httptest.NewRequest(http.MethodGet, "/auth/status", nil)
	rr := httptest.NewRecorder()
	h.Status(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body["authenticated"] != true {
		t.Fatalf("authenticated = %#v, want true", body["authenticated"])
	}
	if body["authDisabled"] != true {
		t.Fatalf("authDisabled = %#v, want true", body["authDisabled"])
	}
}

func TestAuthStatus_AuthEnabled_NoCookie(t *testing.T) {
	sessions := auth.NewSessionStore()
	t.Cleanup(sessions.Stop)
	settings := storage.NewSettingsStore(t.TempDir())
	s, err := settings.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	s.AuthEnabled = true
	if err := settings.Save(s); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	h := NewAuthHandler(nil, sessions, settings, silentAppLogger{})

	req := httptest.NewRequest(http.MethodGet, "/auth/status", nil)
	rr := httptest.NewRecorder()
	h.Status(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body["authenticated"] != false {
		t.Fatalf("authenticated = %#v, want false", body["authenticated"])
	}
}

func TestAuthStatus_ValidSession(t *testing.T) {
	sessions := auth.NewSessionStore()
	t.Cleanup(sessions.Stop)
	settings := storage.NewSettingsStore(t.TempDir())
	s, err := settings.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	s.AuthEnabled = true
	if err := settings.Save(s); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	h := NewAuthHandler(nil, sessions, settings, silentAppLogger{})

	token, err := sessions.Create("admin")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/status", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: token})
	rr := httptest.NewRecorder()
	h.Status(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body["authenticated"] != true {
		t.Fatalf("authenticated = %#v, want true", body["authenticated"])
	}
	if body["login"] != "admin" {
		t.Fatalf("login = %#v, want admin", body["login"])
	}
	expires, ok := body["expiresIn"].(float64)
	if !ok || expires <= 0 {
		t.Fatalf("expiresIn = %#v, want > 0", body["expiresIn"])
	}
}

func TestAuthLogout_WrongMethod(t *testing.T) {
	sessions := auth.NewSessionStore()
	t.Cleanup(sessions.Stop)
	settings := storage.NewSettingsStore(t.TempDir())
	if _, err := settings.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	h := NewAuthHandler(nil, sessions, settings, silentAppLogger{})

	req := httptest.NewRequest(http.MethodGet, "/auth/logout", nil)
	rr := httptest.NewRecorder()
	h.Logout(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestAuthLogout_DeletesSessionAndClearsCookie(t *testing.T) {
	sessions := auth.NewSessionStore()
	t.Cleanup(sessions.Stop)
	settings := storage.NewSettingsStore(t.TempDir())
	if _, err := settings.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	h := NewAuthHandler(nil, sessions, settings, silentAppLogger{})

	token, err := sessions.Create("admin")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: token})
	rr := httptest.NewRecorder()
	h.Logout(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if sessions.Get(token) != nil {
		t.Fatal("session was not deleted")
	}
	found := false
	for _, c := range rr.Result().Cookies() {
		if c.Name == auth.SessionCookie {
			found = true
			if c.MaxAge != -1 {
				t.Fatalf("MaxAge = %d, want -1", c.MaxAge)
			}
		}
	}
	if !found {
		t.Fatal("clearing awg_session cookie not set")
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body["success"] != true {
		t.Fatalf("success = %#v, want true", body["success"])
	}
}
