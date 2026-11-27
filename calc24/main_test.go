package main

import (
	"bytes"
	"sort"
	"strings"
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
	runCase(t, []int{1, 1, 1, 1}, []string{})
	runCase(t, []int{2, 2, 2, 2}, []string{})
	runCase(t, []int{7, 7, 7, 7}, []string{})
	runCase(t, []int{8, 8, 8, 8}, []string{})
	runCase(t, []int{9, 9, 9, 9}, []string{})
	runCase(t, []int{13, 13, 13, 13}, []string{})
}

func TestSingleSolution(t *testing.T) {
	runCase(t, []int{1, 1, 1, 8}, []string{
		"24 = (1 + 1 + 1) * 8",
	})

	runCase(t, []int{1, 2, 7, 7}, []string{
		"24 = (7 * 7 - 1) / 2",
	})

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

	runCase(t, []int{1, 7, 13, 13}, []string{
		"24 = (13 * 13 - 1) / 7",
	})

	runCase(t, []int{2, 5, 6, 9}, []string{
		"24 = 5 * 6 / 2 + 9",
	})

	runCase(t, []int{2, 7, 7, 10}, []string{
		"24 = (10 / 7 + 2) * 7",
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

// --- Rational ---

func TestIs24(t *testing.T) {
	tests := []struct {
		r    Rational
		want bool
	}{
		{Rational{24, 1}, true},
		{Rational{48, 2}, true},
		{Rational{-24, -1}, true},
		{Rational{0, 0}, false}, // division-by-zero sentinel
		{Rational{25, 1}, false},
		{Rational{6, -1}, false},
	}
	for _, tc := range tests {
		if got := tc.r.Is24(); got != tc.want {
			t.Errorf("Rational{%d, %d}.Is24() = %v, want %v", tc.r.Num, tc.r.Den, got, tc.want)
		}
	}
}

func TestReduce(t *testing.T) {
	tests := []struct {
		in, want Rational
	}{
		{Rational{2, 4}, Rational{1, 2}},
		{Rational{-2, 4}, Rational{-1, 2}},
		{Rational{2, -4}, Rational{1, -2}},
		{Rational{0, 5}, Rational{0, 1}},
		{Rational{7, 1}, Rational{7, 1}},
		// The zero-denominator sentinel passes through unchanged.
		{Rational{0, 0}, Rational{0, 0}},
	}
	for _, tc := range tests {
		if got := tc.in.reduce(); got != tc.want {
			t.Errorf("Rational{%d, %d}.reduce() = Rational{%d, %d}, want Rational{%d, %d}",
				tc.in.Num, tc.in.Den, got.Num, got.Den, tc.want.Num, tc.want.Den)
		}
	}
}

// TestRationalOps covers the four arithmetic methods directly, including
// reduction, sign handling, and sentinel behavior at the method level.
func TestRationalOps(t *testing.T) {
	sentinel := Rational{0, 0}
	tests := []struct {
		name      string
		got, want Rational
	}{
		{"add", Rational{1, 2}.Add(Rational{1, 3}), Rational{5, 6}},
		{"add reduces", Rational{1, 2}.Add(Rational{1, 2}), Rational{1, 1}},
		{"sub", Rational{1, 2}.Sub(Rational{1, 3}), Rational{1, 6}},
		{"sub negative", Rational{1, 3}.Sub(Rational{1, 2}), Rational{-1, 6}},
		{"mul", Rational{2, 3}.Mul(Rational{3, 4}), Rational{1, 2}},
		{"div", Rational{1, 2}.Div(Rational{3, 4}), Rational{2, 3}},
		{"div by zero", Rational{1, 2}.Div(Rational{0, 1}), sentinel},
		{"sentinel absorbs", sentinel.Add(Rational{24, 1}), sentinel},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s: got Rational{%d, %d}, want Rational{%d, %d}",
				tc.name, tc.got.Num, tc.got.Den, tc.want.Num, tc.want.Den)
		}
	}
}

// divByZero builds 1 / (n - n), which evaluates to the undefined sentinel.
func divByZero(n int64) Expr {
	return BinaryOp{
		Op:    "/",
		Left:  Literal(1),
		Right: BinaryOp{Op: "-", Left: Literal(n), Right: Literal(n)},
	}
}

// TestEvalSentinelAbsorbing verifies that an undefined subexpression poisons
// the whole result, regardless of the operator that consumes it.
func TestEvalSentinelAbsorbing(t *testing.T) {
	sentinel := Rational{0, 0}
	cases := []Expr{
		divByZero(5), // 1 / (5 - 5)
		BinaryOp{Op: "+", Left: divByZero(5), Right: Literal(24)}, // sentinel + 24
		BinaryOp{Op: "-", Left: Literal(24), Right: divByZero(5)}, // 24 - sentinel
		BinaryOp{Op: "*", Left: divByZero(5), Right: Literal(24)}, // sentinel * 24
		BinaryOp{Op: "/", Left: Literal(24), Right: divByZero(5)}, // 24 / sentinel
		BinaryOp{Op: "/", Left: divByZero(5), Right: Literal(24)}, // sentinel / 24
	}
	for _, expr := range cases {
		if got := expr.Eval(); got != sentinel || got.Is24() {
			t.Errorf("%s: Eval() = Rational{%d, %d}, want absorbing sentinel {0, 0}", expr, got.Num, got.Den)
		}
	}
}

// --- toString / Normalize ---

func TestToStringParenthesization(t *testing.T) {
	tests := []struct {
		expr Expr
		want string
	}{
		// Same-precedence left nesting needs no parens.
		{BinaryOp{Op: "-", Left: BinaryOp{Op: "-", Left: Literal(8), Right: Literal(3)}, Right: Literal(3)}, "8 - 3 - 3"},
		{BinaryOp{Op: "/", Left: BinaryOp{Op: "/", Left: Literal(8), Right: Literal(3)}, Right: Literal(3)}, "8 / 3 / 3"},
		// Associative ops: same-precedence right nesting is also safe without parens.
		{BinaryOp{Op: "+", Left: Literal(8), Right: BinaryOp{Op: "+", Left: Literal(3), Right: Literal(3)}}, "8 + 3 + 3"},
		{BinaryOp{Op: "*", Left: Literal(8), Right: BinaryOp{Op: "*", Left: Literal(3), Right: Literal(3)}}, "8 * 3 * 3"},
		// Non-associative ops: right nesting of the same precedence needs parens.
		{BinaryOp{Op: "-", Left: Literal(8), Right: BinaryOp{Op: "-", Left: Literal(3), Right: Literal(3)}}, "8 - (3 - 3)"},
		{BinaryOp{Op: "/", Left: Literal(8), Right: BinaryOp{Op: "/", Left: Literal(3), Right: Literal(3)}}, "8 / (3 / 3)"},
		// Mixed precedence.
		{BinaryOp{Op: "*", Left: BinaryOp{Op: "+", Left: Literal(1), Right: Literal(2)}, Right: Literal(3)}, "(1 + 2) * 3"},
		{BinaryOp{Op: "+", Left: Literal(1), Right: BinaryOp{Op: "*", Left: Literal(2), Right: Literal(3)}}, "1 + 2 * 3"},
		{BinaryOp{Op: "-", Left: Literal(6), Right: BinaryOp{Op: "*", Left: BinaryOp{Op: "-", Left: Literal(1), Right: Literal(3)}, Right: Literal(9)}}, "6 - (1 - 3) * 9"},
	}
	for _, tc := range tests {
		if got := tc.expr.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}

// TestNormalizeEquivalence verifies that algebraically equivalent trees
// normalize to the same canonical string.
func TestNormalizeEquivalence(t *testing.T) {
	sixTimesSix := BinaryOp{Op: "*", Left: Literal(6), Right: Literal(6)}
	pairs := []struct {
		a, b Expr
		want string
	}{
		{
			// 6*6 - (6+6) ≡ (6*6 - 6) - 6
			BinaryOp{Op: "-", Left: sixTimesSix, Right: BinaryOp{Op: "+", Left: Literal(6), Right: Literal(6)}},
			BinaryOp{Op: "-", Left: BinaryOp{Op: "-", Left: sixTimesSix, Right: Literal(6)}, Right: Literal(6)},
			"6 * 6 - 6 - 6",
		},
		{
			// (2*3/1)*4 ≡ (2/1*3)*4
			BinaryOp{Op: "*", Left: BinaryOp{Op: "/", Left: BinaryOp{Op: "*", Left: Literal(2), Right: Literal(3)}, Right: Literal(1)}, Right: Literal(4)},
			BinaryOp{Op: "*", Left: BinaryOp{Op: "*", Left: BinaryOp{Op: "/", Left: Literal(2), Right: Literal(1)}, Right: Literal(3)}, Right: Literal(4)},
			"2 * 3 * 4 / 1",
		},
		{
			// (1+2)*3 ≡ 3*(1+2) (commutativity across operands)
			BinaryOp{Op: "*", Left: BinaryOp{Op: "+", Left: Literal(1), Right: Literal(2)}, Right: Literal(3)},
			BinaryOp{Op: "*", Left: Literal(3), Right: BinaryOp{Op: "+", Left: Literal(1), Right: Literal(2)}},
			"(1 + 2) * 3",
		},
		{
			// 8/(3/3) ≡ (8/3)*3
			BinaryOp{Op: "/", Left: Literal(8), Right: BinaryOp{Op: "/", Left: Literal(3), Right: Literal(3)}},
			BinaryOp{Op: "*", Left: BinaryOp{Op: "/", Left: Literal(8), Right: Literal(3)}, Right: Literal(3)},
			"3 * 8 / 3",
		},
	}
	for _, p := range pairs {
		na, nb := p.a.Normalize(), p.b.Normalize()
		if na.String() != p.want || nb.String() != p.want {
			t.Errorf("Normalize() = %q / %q, want both %q", na, nb, p.want)
		}
	}
}

// allExprs yields every raw candidate expression for the given numbers
// (unique permutations × 64 operator combos × 5 templates).
func allExprs(nums []int) []Expr {
	var exprs []Expr
	for _, perm := range generatePermutations(nums) {
		a := Literal(perm[0])
		b := Literal(perm[1])
		c := Literal(perm[2])
		d := Literal(perm[3])
		for _, combo := range opCombos {
			for _, tmpl := range templates {
				exprs = append(exprs, tmpl(a, b, c, d, combo[0], combo[1], combo[2]))
			}
		}
	}
	return exprs
}

var propertyInputs = [][]int{
	{1, 2, 3, 4}, {3, 3, 8, 8}, {1, 5, 5, 5}, {13, 13, 13, 1},
}

// TestNormalizePreservesValue checks Eval before and after normalization.
// Undefined expressions (division by zero somewhere) are skipped: the
// solver never normalizes them — the sentinel is absorbing, so they cannot
// evaluate to 24 — and the flatten transforms may legitimately turn an
// undefined expression into a defined one (e.g. a / (b / 0) → a * 0 / b).
func TestNormalizePreservesValue(t *testing.T) {
	for _, nums := range propertyInputs {
		for _, expr := range allExprs(nums) {
			want := expr.Eval()
			if want.Den == 0 {
				continue
			}
			if got := expr.Normalize().Eval(); got != want {
				t.Errorf("%s: Eval = {%d,%d}, but Normalize().Eval() = {%d,%d}",
					expr, want.Num, want.Den, got.Num, got.Den)
			}
		}
	}
}

// TestNormalizeIdempotent checks that normalizing twice changes nothing.
func TestNormalizeIdempotent(t *testing.T) {
	for _, nums := range propertyInputs {
		for _, expr := range allExprs(nums) {
			once := expr.Normalize()
			if twice := once.Normalize(); once.String() != twice.String() {
				t.Errorf("Normalize not idempotent: %q → %q", once, twice)
			}
		}
	}
}

// --- CLI ---

// TestRun checks argument parsing, validation, exit codes, and output
// routing (results and help to stdout, errors to stderr).
func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		exitCode int
		stdout   string // required substring; "" requires empty stdout
		stderr   string // required substring; "" requires empty stderr
	}{
		{"no args", nil, 1, "", "exactly 4 numbers"},
		{"too few", []string{"1", "2", "3"}, 1, "", "exactly 4 numbers"},
		{"too many", []string{"1", "2", "3", "4", "5"}, 1, "", "exactly 4 numbers"},
		{"zero", []string{"0", "1", "2", "3"}, 1, "", "not a valid number"},
		{"too big", []string{"1", "2", "3", "14"}, 1, "", "not a valid number"},
		{"not a number", []string{"1", "2", "3", "x"}, 1, "", "not a valid number"},
		{"negative", []string{"1", "2", "3", "-4"}, 1, "", "not a valid number"},
		{"bad flag", []string{"--bogus", "1", "2", "3", "4"}, 2, "", "not defined"},
		{"help", []string{"-h"}, 0, "Usage:", ""},
		{"help long", []string{"-help"}, 0, "Usage:", ""},
		{"version", []string{"-V"}, 0, "calc24 version", ""},
		{"version long", []string{"-version"}, 0, "calc24 version", ""},
		{"solve", []string{"1", "2", "3", "4"}, 0, "24 = ", ""},
		{"no solution", []string{"7", "7", "7", "7"}, 0, "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tc.args, &stdout, &stderr)
			if code != tc.exitCode {
				t.Errorf("exit code = %d, want %d (stdout=%q stderr=%q)", code, tc.exitCode, stdout.String(), stderr.String())
			}
			for _, out := range []struct {
				label, got, want string
			}{
				{"stdout", stdout.String(), tc.stdout},
				{"stderr", stderr.String(), tc.stderr},
			} {
				if out.want == "" {
					if out.got != "" {
						t.Errorf("%s = %q, want empty", out.label, out.got)
					}
				} else if !strings.Contains(out.got, out.want) {
					t.Errorf("%s = %q, want to contain %q", out.label, out.got, out.want)
				}
			}
		})
	}
}
