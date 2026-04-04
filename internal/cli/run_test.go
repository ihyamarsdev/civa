package cli

import "testing"

func TestNormalizeLegacyCompatibleArgsSupportsPlanNameListPattern(t *testing.T) {
	got := normalizeLegacyCompatibleArgs([]string{"plan", "hyane", "list"})
	if len(got) != 3 || got[0] != "plan" || got[1] != "list" || got[2] != "hyane" {
		t.Fatalf("unexpected normalized args: %#v", got)
	}
}

func TestNormalizeLegacyCompatibleArgsLeavesOtherArgsUntouched(t *testing.T) {
	input := []string{"plan", "start", "--server", "203.0.113.10"}
	got := normalizeLegacyCompatibleArgs(input)
	if len(got) != len(input) {
		t.Fatalf("expected same arg length, got %#v", got)
	}
	for i := range input {
		if got[i] != input[i] {
			t.Fatalf("expected args unchanged, got %#v", got)
		}
	}
}
