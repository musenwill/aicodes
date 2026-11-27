package main

import (
	"flag"
	"fmt"
	"io"
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

// Is24 reports whether the rational equals 24. Division by zero produces a
// zero-denominator sentinel {0, 0}. The sentinel is absorbing: any operation
// involving it stays undefined, so an expression with an undefined
// subexpression never evaluates to 24.
func (r Rational) Is24() bool { return r.Den != 0 && r.Num == 24*r.Den }

// reduce returns the rational in lowest terms. An undefined rational
// (zero denominator) is returned unchanged so the sentinel propagates.
func (r Rational) reduce() Rational {
	if r.Den == 0 {
		return r
	}
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

// Add returns the exact sum r + o, reduced.
func (r Rational) Add(o Rational) Rational {
	return Rational{r.Num*o.Den + o.Num*r.Den, r.Den * o.Den}.reduce()
}

// Sub returns the exact difference r - o, reduced.
func (r Rational) Sub(o Rational) Rational {
	return Rational{r.Num*o.Den - o.Num*r.Den, r.Den * o.Den}.reduce()
}

// Mul returns the exact product r * o, reduced.
func (r Rational) Mul(o Rational) Rational {
	return Rational{r.Num * o.Num, r.Den * o.Den}.reduce()
}

// Div returns the exact quotient r / o, reduced. Division by zero
// (o.Num == 0) produces the undefined sentinel {0, 0}; Is24 filters it.
func (r Rational) Div(o Rational) Rational {
	if o.Num == 0 {
		return Rational{0, 0}
	}
	return Rational{r.Num * o.Den, r.Den * o.Num}.reduce()
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
		return left.Add(right)
	case "-":
		return left.Sub(right)
	case "*":
		return left.Mul(right)
	case "/":
		return left.Div(right)
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

// Normalize canonicalizes an expression tree for deduplication. It assumes
// the expression is fully defined (no division by zero anywhere): Solve
// only normalizes expressions that evaluated to 24, and since the
// zero-denominator sentinel is absorbing, such expressions cannot contain
// an undefined subexpression. On undefined input the flatten transforms
// may change the value (a / (b / 0) → a * 0 / b), which is out of scope.
func (b BinaryOp) Normalize() Expr {
	left := b.Left.Normalize()
	right := b.Right.Normalize()

	// Flatten mixed +/- chains. Flattening crosses the right operand of a
	// '-' node (operand categories swap), which subsumes the inverse
	// identities a-(b+c) = a-b-c and a-(b-c) = a-b+c, so no separate
	// operator-pushdown pass is needed.
	if b.Op == "+" || b.Op == "-" {
		pos, neg := flattenWithInverse(BinaryOp{Op: b.Op, Left: left, Right: right}, "+", "-")
		sort.Slice(pos, func(i, j int) bool { return pos[i].String() < pos[j].String() })
		sort.Slice(neg, func(i, j int) bool { return neg[i].String() < neg[j].String() })
		return buildChainWithInverse(pos, neg, "+", "-")
	}

	// Flatten mixed */ chains. Crossing the right operand of a '/' node
	// (numerator/denominator swap) subsumes a/(b*c) = a/b/c and
	// a/(b/c) = a/b*c.
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
	var result Expr
	if len(direct) > 0 {
		result = direct[0]
		for i := 1; i < len(direct); i++ {
			result = BinaryOp{Op: directOp, Left: result, Right: direct[i]}
		}
	} else {
		// identity element: 0 for +/-, 1 for */ (defensive; a chain built
		// from real inputs always has at least one direct operand)
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

// opCombos holds all 64 combinations of 3 operators (4³), generated once.
var opCombos = generateOpCombos()

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

// run parses args, solves, and writes the results. It returns the process
// exit code: 0 on success, 1 for invalid input, 2 for malformed flags.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("calc24", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var showVersion bool
	var showHelp bool
	fs.BoolVar(&showVersion, "V", false, "show version")
	fs.BoolVar(&showVersion, "version", false, "show version")
	fs.BoolVar(&showHelp, "h", false, "show help")
	fs.BoolVar(&showHelp, "help", false, "show help")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if showHelp {
		fmt.Fprintln(stdout, "Usage: calc24 <num1> <num2> <num3> <num4>")
		fmt.Fprintln(stdout, "Calculate 24-point expressions using four numbers.")
		fmt.Fprintln(stdout, "Each number must be an integer between 1 and 13.")
		fs.SetOutput(stdout) // defaults belong with the help text
		fs.PrintDefaults()
		return 0
	}

	if showVersion {
		fmt.Fprintf(stdout, "calc24 version %s\n", version)
		fmt.Fprintf(stdout, "Commit: %s\n", commit)
		fmt.Fprintf(stdout, "Built: %s\n", buildTime)
		if tag != "none" {
			fmt.Fprintf(stdout, "Tag: %s\n", tag)
		}
		return 0
	}

	// 1. Parse and validate command-line input
	rest := fs.Args()
	if len(rest) != 4 {
		fmt.Fprintln(stderr, "Error: exactly 4 numbers required")
		return 1
	}

	nums := make([]int, 4)
	for i, arg := range rest {
		n, err := strconv.Atoi(arg)
		if err != nil || n < 1 || n > 13 {
			fmt.Fprintf(stderr, "Error: %s is not a valid number (1-13)\n", arg)
			return 1
		}
		nums[i] = n
	}

	// 2. Compute 24-point expressions and print them
	for _, r := range Solve(nums) {
		fmt.Fprintln(stdout, r)
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
