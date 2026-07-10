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
	if b.Op == "+" || b.Op == "*" {
		// commutative, sort by string
		ls := left.String()
		rs := right.String()
		if ls > rs {
			return BinaryOp{Op: b.Op, Left: right, Right: left}
		}
	}
	return BinaryOp{Op: b.Op, Left: left, Right: right}
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
	return BinaryOp{Op: op1, Left: a, Right: BinaryOp{Op: op2, Left: BinaryOp{Op: op3, Left: b, Right: c}, Right: d}}
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