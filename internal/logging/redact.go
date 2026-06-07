package logging

import (
	"strings"
	"unicode/utf8"
)

func redactSensitiveToken(s string) string {
	n := utf8.RuneCountInString(s)
	if n <= 0 {
		return s
	}
	if n <= 4 {
		return strings.Repeat("*", n)
	}

	runes := []rune(s)
	return string(runes[:2]) + strings.Repeat("*", n-4) + string(runes[n-2:])
}
