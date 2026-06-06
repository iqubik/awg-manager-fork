package storage

import "testing"

func TestNormalizeUsageLevelHelper(t *testing.T) {
	if NormalizeUsageLevel(UsageLevelBasic) != UsageLevelBasic {
		t.Fatal("basic should stay basic")
	}
	if NormalizeUsageLevel(UsageLevelAdvanced) != UsageLevelAdvanced {
		t.Fatal("advanced should stay advanced")
	}
	if NormalizeUsageLevel(UsageLevelExpert) != UsageLevelExpert {
		t.Fatal("expert should stay expert")
	}
	if NormalizeUsageLevel("unknown") != UsageLevelAdvanced {
		t.Fatal("unknown should fallback to advanced")
	}
}
