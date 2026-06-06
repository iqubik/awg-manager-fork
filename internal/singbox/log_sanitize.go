package singbox

import "github.com/hoaxisr/awg-manager/internal/logging"

func sanitizeSingboxLogText(s string) string {
	// Compatibility wrapper for existing sing-box sanitizer tests. Runtime
	// masking is shared with /api/logs DTO projection in internal/logging.
	return logging.SanitizeLogText(s)
}
