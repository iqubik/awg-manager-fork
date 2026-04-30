package storage

// UsageLevel values controlling UI section visibility. The backend stores
// the value but never enforces it — filtering is frontend-only. Validation
// lives in NormalizeUsageLevel: unknown values fall back to advanced
// rather than errors, so a bad settings.json never bricks the daemon.
const (
	UsageLevelBasic    = "basic"
	UsageLevelAdvanced = "advanced"
	UsageLevelExpert   = "expert"
)

// NormalizeUsageLevel returns v if it is one of the known levels, otherwise
// UsageLevelAdvanced. Used as the safe fallback for empty or corrupt
// values read from disk.
func NormalizeUsageLevel(v string) string {
	switch v {
	case UsageLevelBasic, UsageLevelAdvanced, UsageLevelExpert:
		return v
	default:
		return UsageLevelAdvanced
	}
}
