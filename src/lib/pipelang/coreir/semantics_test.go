package coreir

import (
	"math"
	"math/big"
	"testing"
)

func TestCheckedInt64MatchesMathematicalRange(t *testing.T) {
	values := []int64{math.MinInt64, math.MinInt64 + 1, -3037000500, -2, -1, 0, 1, 2, 3037000500, math.MaxInt64 - 1, math.MaxInt64}
	for _, operator := range []Operator{OperatorAdd, OperatorSubtract, OperatorMultiply} {
		for _, left := range values {
			for _, right := range values {
				got, gotError := CheckedInt64(operator, left, right)
				want, overflow := mathematicalInt64(operator, left, right)
				if overflow != (gotError == ArithmeticOverflow) || (!overflow && got != want) {
					t.Fatalf("%s(%d, %d) = (%d, %q), want (%d, overflow=%t)", operator, left, right, got, gotError, want, overflow)
				}
			}
		}
	}
}

func TestCheckedNegateAndBinary64DivisionBoundaries(t *testing.T) {
	if _, arithmeticError := CheckedNegateInt64(math.MinInt64); arithmeticError != ArithmeticOverflow {
		t.Fatalf("negate minimum error = %q", arithmeticError)
	}
	if value, arithmeticError := CheckedNegateInt64(7); arithmeticError != "" || value != -7 {
		t.Fatalf("negate = (%d, %q)", value, arithmeticError)
	}
	for _, zero := range []float64{0, math.Copysign(0, -1)} {
		if _, arithmeticError := CheckedDivideBinary64(1, zero); arithmeticError != ArithmeticDivisionByZero {
			t.Fatalf("divide by %v error = %q", zero, arithmeticError)
		}
	}
	if value, arithmeticError := CheckedDivideBinary64(math.NaN(), 2); arithmeticError != "" || !math.IsNaN(value) {
		t.Fatalf("NaN division = (%v, %q)", value, arithmeticError)
	}
}

func mathematicalInt64(operator Operator, left, right int64) (int64, bool) {
	result := new(big.Int)
	switch operator {
	case OperatorAdd:
		result.Add(big.NewInt(left), big.NewInt(right))
	case OperatorSubtract:
		result.Sub(big.NewInt(left), big.NewInt(right))
	case OperatorMultiply:
		result.Mul(big.NewInt(left), big.NewInt(right))
	}
	if !result.IsInt64() {
		return 0, true
	}
	return result.Int64(), false
}
