package main

import (
	"flag"
	"fmt"
	"math"
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

// Expr represents an expression
type Expr interface {
	Eval() float64
	String() string
	Normalize() Expr
	toString(parentPrec int) string
}

// Literal represents a number
type Literal float64

func (l Literal) Eval() float64 { return float64(l) }

func (l Literal) String() string { return l.toString(0) }

func (l Literal) Normalize() Expr { return l }

func (l Literal) toString(parentPrec int) string { return fmt.Sprintf("%.0f", float64(l)) }

// BinaryOp represents a binary operation
type BinaryOp struct {
	Op          string
	Left, Right Expr
}

func (b BinaryOp) Eval() float64 {
	left := b.Left.Eval()
	right := b.Right.Eval()
	switch b.Op {
	case "+":
		return left + right
	case "-":
		return left - right
	case "*":
		return left * right
	case "/":
		if right == 0 {
			return math.Inf(1)
		}
		return left / right
	}
	return 0
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

	// Flatten mixed addition/subtraction chains into canonical positive/negative form.
	if b.Op == "+" || b.Op == "-" {
		pos, neg := flattenAddSub(BinaryOp{Op: b.Op, Left: left, Right: right})
		sort.Slice(pos, func(i, j int) bool {
			return pos[i].String() < pos[j].String()
		})
		sort.Slice(neg, func(i, j int) bool {
			return neg[i].String() < neg[j].String()
		})
		return buildAddSubChain(pos, neg)
	}

	// Flatten mixed multiplication/division chains into canonical numerator/denominator form.
	if b.Op == "*" || b.Op == "/" {
		nums, dens := flattenMultDiv(BinaryOp{Op: b.Op, Left: left, Right: right})
		sort.Slice(nums, func(i, j int) bool {
			return nums[i].String() < nums[j].String()
		})
		sort.Slice(dens, func(i, j int) bool {
			return dens[i].String() < dens[j].String()
		})
		return buildMultDivChain(nums, dens)
	}

	return BinaryOp{Op: b.Op, Left: left, Right: right}
}

// flattenAddSub recursively collects positive and negative operands from a
// mixed chain of + and - operators. For a - node, the right child's role flips:
// positives become negatives and vice versa (negation effect).
func flattenAddSub(expr Expr) (pos []Expr, neg []Expr) {
	if b, ok := expr.(BinaryOp); ok {
		if b.Op == "+" {
			lp, ln := flattenAddSub(b.Left)
			rp, rn := flattenAddSub(b.Right)
			return append(lp, rp...), append(ln, rn...)
		}
		if b.Op == "-" {
			lp, ln := flattenAddSub(b.Left)
			rp, rn := flattenAddSub(b.Right)
			// right side: positives become negatives, negatives become positives
			return append(lp, rn...), append(ln, rp...)
		}
	}
	return []Expr{expr}, nil
}

// buildAddSubChain builds a left-associative tree from positive and negative
// operand lists: first all positives via +, then subtract each negative via -.
// For example, buildAddSubChain([a,b], [c,d]) → (((a+b)-c)-d).
func buildAddSubChain(pos []Expr, neg []Expr) Expr {
	if len(pos) == 0 && len(neg) == 0 {
		return Literal(0)
	}

	var result Expr
	if len(pos) > 0 {
		result = pos[0]
		for i := 1; i < len(pos); i++ {
			result = BinaryOp{Op: "+", Left: result, Right: pos[i]}
		}
	} else {
		// only negatives: start with 0 - ...
		result = Literal(0)
	}

	for _, n := range neg {
		result = BinaryOp{Op: "-", Left: result, Right: n}
	}
	return result
}

// flattenMultDiv recursively collects numerator and denominator operands from a
// mixed chain of * and / operators. For a / node, the right child's role flips:
// numerators become denominators and vice versa (reciprocal effect).
func flattenMultDiv(expr Expr) (nums []Expr, dens []Expr) {
	if b, ok := expr.(BinaryOp); ok {
		if b.Op == "*" {
			ln, ld := flattenMultDiv(b.Left)
			rn, rd := flattenMultDiv(b.Right)
			return append(ln, rn...), append(ld, rd...)
		}
		if b.Op == "/" {
			ln, ld := flattenMultDiv(b.Left)
			rn, rd := flattenMultDiv(b.Right)
			// right side: numerators become denominators, denominators become numerators
			return append(ln, rd...), append(ld, rn...)
		}
	}
	return []Expr{expr}, nil
}

// buildMultDivChain builds a left-associative tree from numerator and denominator
// lists: first all numerators via *, then divide by each denominator via /.
// For example, buildMultDivChain([a,b,c], [d]) → (((a*b)*c)/d).
func buildMultDivChain(nums []Expr, dens []Expr) Expr {
	if len(nums) == 0 && len(dens) == 0 {
		return Literal(0)
	}

	var result Expr
	if len(nums) > 0 {
		result = nums[0]
		for i := 1; i < len(nums); i++ {
			result = BinaryOp{Op: "*", Left: result, Right: nums[i]}
		}
	} else {
		// only denominators: start with 1 / ...
		result = Literal(1)
	}

	for _, d := range dens {
		result = BinaryOp{Op: "/", Left: result, Right: d}
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
				result := expr.Eval()
				if math.Abs(result-24) < 0.001 {
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