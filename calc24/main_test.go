package main

import (
	"sort"
	"testing"
)

// runCase is a test helper that runs Solve with the given input numbers
// and compares the results against the expected output list.
// Comparison is order-independent; it reports any missing or unexpected results.
func runCase(t *testing.T, input []int, expected []string) {
	t.Helper()

	got := Solve(input)

	gotSet := make(map[string]bool)
	for _, s := range got {
		gotSet[s] = true
	}

	expectedSet := make(map[string]bool)
	for _, s := range expected {
		expectedSet[s] = true
	}

	var missing, unexpected []string
	for _, s := range expected {
		if !gotSet[s] {
			missing = append(missing, s)
		}
	}
	for _, s := range got {
		if !expectedSet[s] {
			unexpected = append(unexpected, s)
		}
	}

	if len(missing) > 0 || len(unexpected) > 0 {
		sort.Strings(missing)
		sort.Strings(unexpected)
		t.Errorf("Mismatch for input %v:\n  Missing (%d): %v\n  Unexpected (%d): %v",
			input, len(missing), missing, len(unexpected), unexpected)
	}
}

func TestNoSolution(t *testing.T) {
	runCase(t, []int{7, 7, 7, 7}, []string{})
}

func TestSingleSolution(t *testing.T) {
	runCase(t, []int{3, 3, 7, 7}, []string{
		"24 = (3 + 3 / 7) * 7",
	})
}

func TestMultiSolution(t *testing.T) {
	runCase(t, []int{1, 3, 6, 9}, []string{
		"24 = (1 + 9 / 3) * 6",
		"24 = (1 + 9) * 3 - 6",
		"24 = (1 - 9) * (3 - 6)",
		"24 = (3 - 1) * 9 + 6",
		"24 = (6 - 3) * (9 - 1)",
		"24 = 3 * (6 - 1) + 9",
		"24 = 6 - (1 - 3) * 9",
		"24 = 9 - (1 - 6) * 3",
	})

	runCase(t, []int{2, 3, 7, 9}, []string{
		"24 = (7 + 9) / (2 / 3)",
		"24 = (7 + 9) / 2 * 3",
		"24 = 2 * (3 * 7 - 9)",
		"24 = 3 * (7 + 9) / 2",
		"24 = 3 * (7 - 2) + 9",
		"24 = 3 / 2 * (7 + 9)",
		"24 = 9 - (2 - 7) * 3",
	})
}
