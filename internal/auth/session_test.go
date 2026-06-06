package auth

import (
	"encoding/hex"
	"testing"
	"time"
)

func TestSessionStore_CreateAndGet(t *testing.T) {
	store := NewSessionStore()
	t.Cleanup(store.Stop)

	token, err := store.Create("admin")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}
	if len(token) != 64 {
		t.Fatalf("token len = %d, want 64", len(token))
	}
	if _, err := hex.DecodeString(token); err != nil {
		t.Fatalf("token is not valid hex: %v", err)
	}

	s := store.Get(token)
	if s == nil {
		t.Fatal("Get() returned nil")
	}
	if s.Login != "admin" {
		t.Fatalf("session.Login = %q, want %q", s.Login, "admin")
	}
	if s.Token != token {
		t.Fatalf("session.Token = %q, want %q", s.Token, token)
	}
}

func TestSessionStore_GetUpdatesLastSeen(t *testing.T) {
	store := NewSessionStore()
	t.Cleanup(store.Stop)

	token, err := store.Create("admin")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	store.mu.RLock()
	before := store.sessions[token].LastSeen
	store.mu.RUnlock()

	time.Sleep(time.Millisecond)
	got := store.Get(token)
	if got == nil {
		t.Fatal("Get() returned nil")
	}
	if !got.LastSeen.After(before) {
		t.Fatalf("LastSeen was not updated: before=%v after=%v", before, got.LastSeen)
	}
}

func TestSessionStore_GetMissingReturnsNil(t *testing.T) {
	store := NewSessionStore()
	t.Cleanup(store.Stop)

	if got := store.Get("missing"); got != nil {
		t.Fatalf("Get(missing) = %#v, want nil", got)
	}
}

func TestSessionStore_DeleteRemovesSession(t *testing.T) {
	store := NewSessionStore()
	t.Cleanup(store.Stop)

	token, err := store.Create("admin")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	store.Delete(token)
	if got := store.Get(token); got != nil {
		t.Fatalf("Get() after Delete = %#v, want nil", got)
	}
}

func TestSessionStore_ExpiredDeletedOnGet(t *testing.T) {
	store := NewSessionStore()
	t.Cleanup(store.Stop)

	store.mu.Lock()
	store.sessions["expired"] = &Session{
		Token:    "expired",
		Login:    "admin",
		LastSeen: time.Now().Add(-SessionTTL - time.Second),
	}
	store.mu.Unlock()

	if got := store.Get("expired"); got != nil {
		t.Fatalf("Get(expired) = %#v, want nil", got)
	}

	store.mu.RLock()
	_, ok := store.sessions["expired"]
	store.mu.RUnlock()
	if ok {
		t.Fatal("expired session was not removed from map")
	}
}

func TestSessionStore_CleanupRemovesExpiredKeepsActive(t *testing.T) {
	store := NewSessionStore()
	t.Cleanup(store.Stop)

	store.mu.Lock()
	store.sessions["expired"] = &Session{
		Token:    "expired",
		Login:    "admin",
		LastSeen: time.Now().Add(-SessionTTL - time.Second),
	}
	store.sessions["active"] = &Session{
		Token:    "active",
		Login:    "admin",
		LastSeen: time.Now(),
	}
	store.mu.Unlock()

	store.cleanup()

	store.mu.RLock()
	_, hasExpired := store.sessions["expired"]
	_, hasActive := store.sessions["active"]
	store.mu.RUnlock()
	if hasExpired {
		t.Fatal("expired session still present after cleanup")
	}
	if !hasActive {
		t.Fatal("active session removed by cleanup")
	}
}
