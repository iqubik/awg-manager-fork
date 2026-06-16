package api

import (
	"errors"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/logging"
)

type recordingAppLoggerForUpdate struct {
	messages []string
}

func (r *recordingAppLoggerForUpdate) AppLog(level logging.Level, group, subgroup, action, target, message string) {
	r.messages = append(r.messages, message)
}

func TestChangelogFetchUnavailableMessage_IsSafeForUsers(t *testing.T) {
	msg := changelogFetchUnavailableMessage
	if msg != "Список изменений временно недоступен. Повторите попытку позже." {
		t.Fatalf("unexpected message: %q", msg)
	}
	for _, forbidden := range []string{"502", "Bad Gateway", "Unexpected token", "<!DOCTYPE", "http://", "https://"} {
		if strings.Contains(msg, forbidden) {
			t.Fatalf("message leaks raw technical details: %q", msg)
		}
	}
}

func TestUpdateHandlerChangelog_LogsTechnicalDetails(t *testing.T) {
	rec := &recordingAppLoggerForUpdate{}
	h := &UpdateHandler{
		log: logging.NewScopedLogger(rec, logging.GroupSystem, logging.SubUpdate),
	}

	h.log.Warn("changelog", "", "Changelog fetch failed: "+errors.New("download via http://repo status 502").Error())

	if len(rec.messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(rec.messages))
	}
	if !strings.Contains(rec.messages[0], "status 502") {
		t.Fatalf("log should keep technical details, got %q", rec.messages[0])
	}
}
