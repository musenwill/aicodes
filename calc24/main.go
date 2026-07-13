package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
	tag       = "none"
)

// Rational represents an exact rational number.
type Rational struct {
	Num, Den int64
}

// Is24 reports whether the rational equals 24. Division by zero produces
// a zero-denominator sentinel that is never equal to 24.
func (r Rational) Is24() bool { return r.Den != 0 && r.Num == 24*r.Den }

// reduce returns the rational in lowest terms.
func (r Rational) reduce() Rational {
	if r.Num == 0 {
		return Rational{0, 1}
	}
	g := gcd(abs64(r.Num), abs64(r.Den))
	return Rational{r.Num / g, r.Den / g}
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// Expr represents an expression tree node.
type Expr interface {
	Eval() Rational
	String() string
	Normalize() Expr
	toString(parentPrec int) string
}

// Literal represents a leaf number. All inputs are integers, so Den==1.
type Literal int64

func (l Literal) Eval() Rational { return Rational{int64(l), 1} }

func (l Literal) String() string { return l.toString(0) }

func (l Literal) Normalize() Expr { return l }

func (l Literal) toString(parentPrec int) string { return strconv.FormatInt(int64(l), 10) }

// BinaryOp represents a binary operation.
type BinaryOp struct {
	Op          string
	Left, Right Expr
}

func (b BinaryOp) Eval() Rational {
	left := b.Left.Eval()
	right := b.Right.Eval()
	switch b.Op {
	case "+":
		return Rational{left.Num*right.Den + right.Num*left.Den, left.Den * right.Den}.reduce()
	case "-":
		return Rational{left.Num*right.Den - right.Num*left.Den, left.Den * right.Den}.reduce()
	case "*":
		return Rational{left.Num * right.Num, left.Den * right.Den}.reduce()
	case "/":
		if right.Num == 0 {
			return Rational{0, 0} // undefined; filtered out by Is24 check
		}
		return Rational{left.Num * right.Den, left.Den * right.Num}.reduce()
	}
	panic("invalid op: " + b.Op)
}

func (b BinaryOp) String() string { return b.toString(0) }

func (b BinaryOp) toString(parentPrec int) string {
	myPrec := 1
	if b.Op == "*" || b.Op == "/" {
		myPrec = 2
	}

	// For non-associative operators (-, /), the right operand needs a higher
	// effective parent precedence to force parentheses around same-precedence
	// sub-expressions. Otherwise e.g. a/(b/c) would render as a/b/c, which
	// standard left-to-right evaluation interprets as (a/b)/c.
	rightParentPrec := myPrec
	if b.Op == "-" || b.Op == "/" {
		rightParentPrec = myPrec + 1
	}

	s := fmt.Sprintf("%s %s %s", b.Left.toString(myPrec), b.Op, b.Right.toString(rightParentPrec))
	if parentPrec > myPrec {
		return "(" + s + ")"
	}
	return s
}

func (b BinaryOp) Normalize() Expr {
	left := b.Left.Normalize()
	right := b.Right.Normalize()

	// Push negation inward to eliminate nested inverse operators.
	if b.Op == "-" {
		if rightBin, ok := right.(BinaryOp); ok {
			if rightBin.Op == "+" {
				// a - (b + c) → (a - b) - c
				return BinaryOp{
					Op:    "-",
					Left:  BinaryOp{Op: "-", Left: left, Right: rightBin.Left},
					Right: rightBin.Right,
				}.Normalize()
			}
			if rightBin.Op == "-" {
				// a - (b - c) → (a - b) + c
				return BinaryOp{
					Op:    "+",
					Left:  BinaryOp{Op: "-", Left: left, Right: rightBin.Left},
					Right: rightBin.Right,
				}.Normalize()
			}
		}
	}

	// Push reciprocal inward.
	if b.Op == "/" {
		if rightBin, ok := right.(BinaryOp); ok {
			if rightBin.Op == "*" {
				// a / (b * c) → (a / b) / c
				return BinaryOp{
					Op:    "/",
					Left:  BinaryOp{Op: "/", Left: left, Right: rightBin.Left},
					Right: rightBin.Right,
				}.Normalize()
			}
			if rightBin.Op == "/" {
				// a / (b / c) → (a / b) * c
				return BinaryOp{
					Op:    "*",
					Left:  BinaryOp{Op: "/", Left: left, Right: rightBin.Left},
					Right: rightBin.Right,
				}.Normalize()
			}
		}
	}

	// Flatten mixed +/- chains.
	if b.Op == "+" || b.Op == "-" {
		pos, neg := flattenWithInverse(BinaryOp{Op: b.Op, Left: left, Right: right}, "+", "-")
		sort.Slice(pos, func(i, j int) bool { return pos[i].String() < pos[j].String() })
		sort.Slice(neg, func(i, j int) bool { return neg[i].String() < neg[j].String() })
		return buildChainWithInverse(pos, neg, "+", "-")
	}

	// Flatten mixed */ chains.
	if b.Op == "*" || b.Op == "/" {
		nums, dens := flattenWithInverse(BinaryOp{Op: b.Op, Left: left, Right: right}, "*", "/")
		sort.Slice(nums, func(i, j int) bool { return nums[i].String() < nums[j].String() })
		sort.Slice(dens, func(i, j int) bool { return dens[i].String() < dens[j].String() })
		return buildChainWithInverse(nums, dens, "*", "/")
	}

	panic("invalid op: " + b.Op)
}

// flattenWithInverse collects operands from a chain of directOp and inverseOp.
// For an inverseOp node, the right child's operands swap categories (e.g. for "-",
// right positives become negatives and vice versa; for "/", right numerators become
// denominators and vice versa).
func flattenWithInverse(expr Expr, directOp, inverseOp string) (direct []Expr, inverse []Expr) {
	if b, ok := expr.(BinaryOp); ok {
		if b.Op == directOp {
			ld, li := flattenWithInverse(b.Left, directOp, inverseOp)
			rd, ri := flattenWithInverse(b.Right, directOp, inverseOp)
			return append(ld, rd...), append(li, ri...)
		}
		if b.Op == inverseOp {
			ld, li := flattenWithInverse(b.Left, directOp, inverseOp)
			rd, ri := flattenWithInverse(b.Right, directOp, inverseOp)
			return append(ld, ri...), append(li, rd...)
		}
	}
	return []Expr{expr}, nil
}

// buildChainWithInverse builds a left-associative tree: all direct operands via
// directOp, then each inverse operand via inverseOp.
func buildChainWithInverse(direct, inverse []Expr, directOp, inverseOp string) Expr {
	if len(direct) == 0 && len(inverse) == 0 {
		return Literal(0)
	}
	var result Expr
	if len(direct) > 0 {
		result = direct[0]
		for i := 1; i < len(direct); i++ {
			result = BinaryOp{Op: directOp, Left: result, Right: direct[i]}
		}
	} else {
		// identity element: 0 for +/-, 1 for */
		if directOp == "+" {
			result = Literal(0)
		} else {
			result = Literal(1)
		}
	}
	for _, inv := range inverse {
		result = BinaryOp{Op: inverseOp, Left: result, Right: inv}
	}
	return result
}

// Templates
func template1(a, b, c, d Expr, op1, op2, op3 string) Expr {
	return BinaryOp{Op: op3, Left: BinaryOp{Op: op2, Left: BinaryOp{Op: op1, Left: a, Right: b}, Right: c}, Right: d}
}

func template2(a, b, c, d Expr, op1, op2, op3 string) Expr {
	return BinaryOp{Op: op2, Left: BinaryOp{Op: op1, Left: a, Right: b}, Right: BinaryOp{Op: op3, Left: c, Right: d}}
}

func template3(a, b, c, d Expr, op1, op2, op3 string) Expr {
	return BinaryOp{Op: op3, Left: BinaryOp{Op: op1, Left: a, Right: BinaryOp{Op: op2, Left: b, Right: c}}, Right: d}
}

func template4(a, b, c, d Expr, op1, op2, op3 string) Expr {
	return BinaryOp{Op: op1, Left: a, Right: BinaryOp{Op: op3, Left: BinaryOp{Op: op2, Left: b, Right: c}, Right: d}}
}

func template5(a, b, c, d Expr, op1, op2, op3 string) Expr {
	return BinaryOp{Op: op1, Left: a, Right: BinaryOp{Op: op2, Left: b, Right: BinaryOp{Op: op3, Left: c, Right: d}}}
}

var templates = []func(Expr, Expr, Expr, Expr, string, string, string) Expr{
	template1, template2, template3, template4, template5,
}

var ops = []string{"+", "-", "*", "/"}

// generatePermutations generates all unique permutations of nums
func generatePermutations(nums []int) [][]int {
	var result [][]int
	used := make([]bool, len(nums))
	var backtrack func([]int)
	backtrack = func(current []int) {
		if len(current) == len(nums) {
			perm := make([]int, len(nums))
			copy(perm, current)
			result = append(result, perm)
			return
		}
		seen := make(map[int]bool)
		for i := 0; i < len(nums); i++ {
			if !used[i] && !seen[nums[i]] {
				seen[nums[i]] = true
				used[i] = true
				backtrack(append(current, nums[i]))
				used[i] = false
			}
		}
	}
	backtrack([]int{})
	return result
}

// generateOpCombos generates all combinations of 3 operators
func generateOpCombos() [][]string {
	var result [][]string
	var backtrack func([]string)
	backtrack = func(current []string) {
		if len(current) == 3 {
			combo := make([]string, 3)
			copy(combo, current)
			result = append(result, combo)
			return
		}
		for _, op := range ops {
			backtrack(append(current, op))
		}
	}
	backtrack([]string{})
	return result
}

// Solve finds all unique 24-point expressions for the given 4 numbers.
// Each number must be in the range [1, 13]. Results are sorted.
func Solve(nums []int) []string {
	perms := generatePermutations(nums)
	opCombos := generateOpCombos()
	expressions := make(map[string]bool)
	for _, perm := range perms {
		a := Literal(perm[0])
		b := Literal(perm[1])
		c := Literal(perm[2])
		d := Literal(perm[3])
		for _, combo := range opCombos {
			for _, tmpl := range templates {
				expr := tmpl(a, b, c, d, combo[0], combo[1], combo[2])
				if result := expr.Eval(); result.Is24() {
					normalized := expr.Normalize()
					expressions[normalized.String()] = true
				}
			}
		}
	}
	var results []string
	for expr := range expressions {
		results = append(results, "24 = "+expr)
	}
	sort.Strings(results)
	return results
}

func main() {
	var showVersion bool
	var showHelp bool
	flag.BoolVar(&showVersion, "V", false, "show version")
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.BoolVar(&showHelp, "h", false, "show help")
	flag.BoolVar(&showHelp, "help", false, "show help")
	flag.Parse()

	if showHelp {
		fmt.Println("Usage: calc24 <num1> <num2> <num3> <num4>")
		fmt.Println("Calculate 24-point expressions using four numbers.")
		fmt.Println("Each number must be an integer between 1 and 13.")
		flag.PrintDefaults()
		return
	}

	if showVersion {
		fmt.Printf("calc24 version %s\n", version)
		fmt.Printf("Commit: %s\n", commit)
		fmt.Printf("Built: %s\n", buildTime)
		if tag != "none" {
			fmt.Printf("Tag: %s\n", tag)
		}
		return
	}

	// 1. Parse and validate command-line input
	args := flag.Args()
	if len(args) != 4 {
		fmt.Println("Error: exactly 4 numbers required")
		os.Exit(1)
	}

	nums := make([]int, 4)
	for i, arg := range args {
		n, err := strconv.Atoi(arg)
		if err != nil || n < 1 || n > 13 {
			fmt.Printf("Error: %s is not a valid number (1-13)\n", arg)
			os.Exit(1)
		}
		nums[i] = n
	}

	// 2. Compute 24-point expressions
	results := Solve(nums)

	// 3. Print results
	for _, r := range results {
		fmt.Println(r)
	}
}