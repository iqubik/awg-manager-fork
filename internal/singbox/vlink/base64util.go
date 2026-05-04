package vlink

import (
	"encoding/base64"
	"strings"
)

// DecodeBase64Url accepts standard, urlsafe, and unpadded base64 strings.
// Returns the decoded bytes, or an error if the string contains non-base64
// characters after normalization.
func DecodeBase64Url(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	// Normalize: urlsafe → standard
	norm := strings.NewReplacer("-", "+", "_", "/").Replace(s)
	// Pad to multiple of 4
	if rem := len(norm) % 4; rem != 0 {
		norm += strings.Repeat("=", 4-rem)
	}
	return base64.StdEncoding.DecodeString(norm)
}

// DoubleDecode tries base64 once. If the result looks like another base64
// string (printable ASCII + base64 alphabet), tries a second pass. Returns
// the deepest plain bytes plus ok=true if any decode succeeded; ok=false if
// the input was not base64 at all.
func DoubleDecode(b []byte) ([]byte, bool) {
	first, err := DecodeBase64Url(string(b))
	if err != nil || len(first) == 0 {
		return nil, false
	}
	if !looksLikeBase64(first) {
		return first, true
	}
	second, err := DecodeBase64Url(string(first))
	if err != nil || len(second) == 0 {
		return first, true
	}
	return second, true
}

// looksLikeBase64 returns true if every byte is in the base64 alphabet.
// Used by DoubleDecode to decide whether a second decode is worth trying.
func looksLikeBase64(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	for _, c := range b {
		switch {
		case c >= 'A' && c <= 'Z':
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '+' || c == '/' || c == '-' || c == '_' || c == '=':
		case c == '\r' || c == '\n':
		default:
			return false
		}
	}
	return true
}

// StripTrailing removes consecutive trailing '=' characters. Used when a
// subscription body has an obviously truncated base64 payload.
func StripTrailing(s string) string {
	return strings.TrimRight(s, "=")
}
