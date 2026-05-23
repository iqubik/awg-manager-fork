package storage

import "strings"

const DefaultSingboxLogLevel = "trace"

var validSingboxLogLevels = map[string]struct{}{
	"trace": {},
	"debug": {},
	"info":  {},
	"warn":  {},
	"error": {},
	"fatal": {},
	"panic": {},
}

func NormalizeSingboxLogLevel(v string) string {
	normalized := strings.ToLower(strings.TrimSpace(v))
	if _, ok := validSingboxLogLevels[normalized]; ok {
		return normalized
	}
	return DefaultSingboxLogLevel
}

func IsValidSingboxLogLevel(v string) bool {
	normalized := strings.ToLower(strings.TrimSpace(v))
	_, ok := validSingboxLogLevels[normalized]
	return ok
}
