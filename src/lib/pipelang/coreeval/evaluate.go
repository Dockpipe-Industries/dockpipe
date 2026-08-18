// Package coreeval is PipeLang's target-independent Core IR conformance
// evaluator. It is offline and inert and imports no parser, HIR, or backend.
package coreeval

import (
	"fmt"

	"dockpipe/src/lib/pipelang/coreir"
)

type Value struct {
	Type   coreir.Type
	String string
	Int    int64
	Float  float64
	Bool   bool
	Result *Outcome
}

type Outcome struct {
	OK    bool
	Value Value
	Error coreir.ArithmeticError
}

func Evaluate(function coreir.Function, arguments []Value) (Outcome, error) {
	if err := coreir.ValidateFunction(function); err != nil {
		return Outcome{}, err
	}
	if len(arguments) != len(function.Parameters) {
		return Outcome{}, fmt.Errorf("function expects %d arguments, got %d", len(function.Parameters), len(arguments))
	}
	for index := range arguments {
		if !coreir.TypeEqual(arguments[index].Type, function.Parameters[index].Type) {
			return Outcome{}, fmt.Errorf("argument %d type does not match parameter", index)
		}
		if err := validateValue(arguments[index]); err != nil {
			return Outcome{}, fmt.Errorf("argument %d: %w", index, err)
		}
	}
	return evalExpr(function.Body, arguments)
}

func evalExpr(expression coreir.Expr, arguments []Value) (Outcome, error) {
	switch expression.Kind {
	case coreir.ExprLiteral:
		literal := expression.Literal
		return Outcome{OK: true, Value: Value{Type: expression.Type, String: literal.String, Int: literal.Int, Float: literal.Float, Bool: literal.Bool}}, nil
	case coreir.ExprReference:
		if expression.Type.Kind == coreir.TypeResult {
			result := arguments[*expression.Parameter].Result
			if result == nil {
				return Outcome{}, fmt.Errorf("Result reference has no canonical value")
			}
			return *result, nil
		}
		return Outcome{OK: true, Value: arguments[*expression.Parameter]}, nil
	case coreir.ExprUnary:
		operand, err := evalExpr(*expression.Unary.Operand, arguments)
		if err != nil || !operand.OK {
			return operand, err
		}
		switch expression.Unary.Operator {
		case coreir.OperatorNegate:
			value, arithmeticError := coreir.CheckedNegateInt64(operand.Value.Int)
			return arithmeticOutcome(expression.Type, Value{Int: value}, arithmeticError), nil
		case coreir.OperatorNot:
			return Outcome{OK: true, Value: Value{Type: expression.Type, Bool: !operand.Value.Bool}}, nil
		default:
			return Outcome{}, fmt.Errorf("unsupported unary operator %q", expression.Unary.Operator)
		}
	case coreir.ExprBinary:
		left, err := evalExpr(*expression.Binary.Left, arguments)
		if err != nil || !left.OK {
			return left, err
		}
		right, err := evalExpr(*expression.Binary.Right, arguments)
		if err != nil || !right.OK {
			return right, err
		}
		switch expression.Binary.Operator {
		case coreir.OperatorAdd, coreir.OperatorSubtract, coreir.OperatorMultiply:
			value, arithmeticError := coreir.CheckedInt64(expression.Binary.Operator, left.Value.Int, right.Value.Int)
			return arithmeticOutcome(expression.Type, Value{Int: value}, arithmeticError), nil
		case coreir.OperatorDivide:
			value, arithmeticError := coreir.CheckedDivideBinary64(left.Value.Float, right.Value.Float)
			return arithmeticOutcome(expression.Type, Value{Float: value}, arithmeticError), nil
		default:
			return Outcome{}, fmt.Errorf("operator %q is outside the arithmetic conformance evaluator", expression.Binary.Operator)
		}
	default:
		return Outcome{}, fmt.Errorf("unsupported expression kind %q", expression.Kind)
	}
}

func validateValue(value Value) error {
	if value.Type.Kind != coreir.TypeResult {
		if value.Result != nil {
			return fmt.Errorf("non-Result value carries a Result outcome")
		}
		return nil
	}
	if value.Type.Result == nil || value.Type.Result.Failure.Kind != coreir.TypeArithmeticError || value.Result == nil {
		return fmt.Errorf("Result value has an invalid arithmetic Result shape")
	}
	result := value.Result
	if !coreir.TypeEqual(result.Value.Type, value.Type.Result.Success) {
		return fmt.Errorf("Result payload type does not match its success type")
	}
	if result.OK {
		if result.Error != "" {
			return fmt.Errorf("successful Result carries an error")
		}
		return nil
	}
	if result.Error != coreir.ArithmeticOverflow && result.Error != coreir.ArithmeticDivisionByZero {
		return fmt.Errorf("failed Result carries an unknown arithmetic error")
	}
	return nil
}

func arithmeticOutcome(resultType coreir.Type, value Value, arithmeticError coreir.ArithmeticError) Outcome {
	value.Type = resultType.Result.Success
	if arithmeticError != "" {
		return Outcome{OK: false, Value: value, Error: arithmeticError}
	}
	return Outcome{OK: true, Value: value}
}
