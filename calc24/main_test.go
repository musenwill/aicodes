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
	runCase(t, []int{8, 8, 8, 8}, []string{})
	runCase(t, []int{9, 9, 9, 9}, []string{})
}

func TestSingleSolution(t *testing.T) {
	runCase(t, []int{1, 2, 9, 11}, []string{
		"24 = (1 + 2) * 11 - 9",
	})

	runCase(t, []int{1, 3, 4, 6}, []string{
		"24 = 6 / (1 - 3 / 4)",
	})

	runCase(t, []int{1, 5, 5, 5}, []string{
		"24 = 5 * (5 - 1 / 5)",
	})

	runCase(t, []int{1, 5, 11, 11}, []string{
		"24 = (11 * 11 - 1) / 5",
	})

	runCase(t, []int{2, 5, 6, 9}, []string{
		"24 = 5 * 6 / 2 + 9",
	})

	runCase(t, []int{3, 3, 3, 3}, []string{
		"24 = 3 * 3 * 3 - 3",
	})

	runCase(t, []int{3, 3, 7, 7}, []string{
		"24 = (3 + 3 / 7) * 7",
	})

	runCase(t, []int{3, 3, 8, 8}, []string{
		"24 = 8 / (3 - 8 / 3)",
	})

	runCase(t, []int{3, 5, 8, 13}, []string{
		"24 = 5 * 8 - 13 - 3",
	})

	runCase(t, []int{3, 8, 8, 10}, []string{
		"24 = (10 * 8 - 8) / 3",
	})

	runCase(t, []int{4, 4, 4, 4}, []string{
		"24 = 4 + 4 + 4 * 4",
	})

	runCase(t, []int{4, 4, 7, 7}, []string{
		"24 = (4 - 4 / 7) * 7",
	})

	runCase(t, []int{4, 4, 10, 10}, []string{
		"24 = (10 * 10 - 4) / 4",
	})

	runCase(t, []int{4, 9, 11, 11}, []string{
		"24 = 11 * 4 - 11 - 9",
	})

	runCase(t, []int{5, 5, 5, 5}, []string{
		"24 = 5 * 5 - 5 / 5",
	})

	runCase(t, []int{5, 10, 10, 11}, []string{
		"24 = (10 + 10 * 11) / 5",
	})

	runCase(t, []int{6, 7, 8, 9}, []string{
		"24 = 6 * 8 / (9 - 7)",
	})
}

func TestMultiSolution(t *testing.T) {
	runCase(t, []int{1, 2, 3, 4}, []string{
		"24 = (1 + 2 + 3) * 4",
		"24 = (1 + 3) * (2 + 4)",
		"24 = 1 * 2 * 3 * 4",
		"24 = 2 * 3 * 4 / 1",
	})

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

	runCase(t, []int{2, 3, 4, 6}, []string{
		"24 = (2 * 6 - 4) * 3",
		"24 = (2 + 4) * 3 + 6",
		"24 = (3 + 6 / 2) * 4",
		"24 = (3 - 2) * 4 * 6",
		"24 = 2 * 4 * (6 - 3)",
		"24 = 2 * 6 + 3 * 4",
		"24 = 2 + 3 * 6 + 4",
		"24 = 3 * (4 + 6 - 2)",
		"24 = 3 * (4 / 2 + 6)",
		"24 = 4 * 6 / (3 - 2)",
	})

	runCase(t, []int{2, 3, 7, 9}, []string{
		"24 = 2 * (3 * 7 - 9)",
		"24 = 3 * (7 + 9) / 2",
		"24 = 3 * (7 - 2) + 9",
		"24 = 9 - (2 - 7) * 3",
	})

	runCase(t, []int{2, 4, 8, 10}, []string{
		"24 = (10 * 4 + 8) / 2",
		"24 = (10 + 2) * 8 / 4",
		"24 = (10 + 8 / 4) * 2",
		"24 = (10 - 2) * 4 - 8",
		"24 = (10 - 4) * 8 / 2",
		"24 = (10 - 8 / 2) * 4",
		"24 = (2 * 8 - 10) * 4",
		"24 = 10 * 2 + 8 - 4",
		"24 = 10 * 4 - 2 * 8",
		"24 = 10 + 2 + 4 + 8",
		"24 = 2 + 4 * 8 - 10",
	})

	runCase(t, []int{3, 3, 6, 10}, []string{
		"24 = (10 - 3 - 3) * 6",
		"24 = (10 - 6 / 3) * 3",
		"24 = (10 - 6) * (3 + 3)",
		"24 = 3 * (3 * 6 - 10)",
	})

	runCase(t, []int{6, 6, 6, 6}, []string{
		"24 = 6 * 6 - 6 - 6",
		"24 = 6 + 6 + 6 + 6",
	})

	runCase(t, []int{7, 8, 9, 12}, []string{
		"24 = (12 + 9) * 8 / 7",
		"24 = 12 * (7 + 9) / 8",
	})
}
