package coreir

import "fmt"

const (
	maxInt64 = int64(9223372036854775807)
	minInt64 = -maxInt64 - 1
)

func SignedInteger(bits int) Type {
	return Type{Kind: TypeNumeric, Numeric: &NumericType{Representation: NumericInteger, Bits: bits, Signed: true}}
}

func BinaryFloat(bits int) Type {
	return Type{Kind: TypeNumeric, Numeric: &NumericType{Representation: NumericBinaryFloat, Bits: bits}}
}

func ArithmeticErrorType() Type {
	return Type{Kind: TypeArithmeticError}
}

func ArithmeticResult(success Type) Type {
	return Type{Kind: TypeResult, Result: &ResultType{Success: success, Failure: ArithmeticErrorType()}}
}

// ArithmeticResultType is the single target-independent signature contract
// for bounded checked arithmetic. The v0.1.0 source lane remains closed; the
// v0.2.0 maps only direct integer addition here; v0.3.0 additionally maps
// direct integer subtraction; v0.4.0 additionally maps direct integer
// multiplication; v0.5.0 additionally maps direct integer negation; v0.6.0
// additionally maps direct binary64 division. Other operations remain
// compiler-internal.
func ArithmeticResultType(operator Operator, left Type, right *Type) (Type, error) {
	integer64 := SignedInteger(64)
	binary64 := BinaryFloat(64)
	switch operator {
	case OperatorNegate:
		if right != nil || !TypeEqual(left, integer64) {
			return Type{}, fmt.Errorf("operator %q requires one signed 64-bit integer operand", operator)
		}
		return ArithmeticResult(integer64), nil
	case OperatorAdd, OperatorSubtract, OperatorMultiply:
		if right == nil || !TypeEqual(left, integer64) || !TypeEqual(*right, integer64) {
			return Type{}, fmt.Errorf("operator %q requires two signed 64-bit integer operands", operator)
		}
		return ArithmeticResult(integer64), nil
	case OperatorDivide:
		if right == nil || !TypeEqual(left, binary64) || !TypeEqual(*right, binary64) {
			return Type{}, fmt.Errorf("operator %q requires two binary64 operands", operator)
		}
		return ArithmeticResult(binary64), nil
	default:
		return Type{}, fmt.Errorf("operator %q is not in the checked arithmetic contract", operator)
	}
}

func TypeEqual(left, right Type) bool {
	if left.Kind != right.Kind || left.Primitive != right.Primitive || left.Name != right.Name || (left.Numeric == nil) != (right.Numeric == nil) || (left.Result == nil) != (right.Result == nil) || (left.Identity == nil) != (right.Identity == nil) || len(left.Arguments) != len(right.Arguments) {
		return false
	}
	if left.Numeric != nil && *left.Numeric != *right.Numeric {
		return false
	}
	if left.Result != nil && (!TypeEqual(left.Result.Success, right.Result.Success) || !TypeEqual(left.Result.Failure, right.Result.Failure)) {
		return false
	}
	if left.Identity != nil && (left.Identity.PackageID != right.Identity.PackageID || left.Identity.Path != right.Identity.Path) {
		return false
	}
	for index := range left.Arguments {
		if !TypeEqual(left.Arguments[index], right.Arguments[index]) {
			return false
		}
	}
	return true
}

// ValidateFunction checks Core types and operator contracts before either a
// semantic evaluator or a target backend consumes the function.
func ValidateFunction(function Function) error {
	for position, parameter := range function.Parameters {
		if parameter.Position != position {
			return fmt.Errorf("parameter %d is not in normalized position order", position)
		}
	}
	if err := validateExpr(function.Body, function.Parameters); err != nil {
		return err
	}
	if !TypeEqual(function.ReturnType, function.Body.Type) {
		return fmt.Errorf("function return type does not match its body type")
	}
	return nil
}

func validateExpr(expression Expr, parameters []Parameter) error {
	switch expression.Kind {
	case ExprLiteral:
		if expression.Literal == nil || !isLiteralType(expression.Type) {
			return fmt.Errorf("literal has an invalid type")
		}
	case ExprReference:
		if expression.Parameter == nil || *expression.Parameter < 0 || *expression.Parameter >= len(parameters) {
			return fmt.Errorf("reference has an invalid parameter position")
		}
		if !TypeEqual(expression.Type, parameters[*expression.Parameter].Type) {
			return fmt.Errorf("reference type does not match parameter %d", *expression.Parameter)
		}
	case ExprUnary:
		if expression.Unary == nil || expression.Unary.Operand == nil {
			return fmt.Errorf("unary expression is incomplete")
		}
		if err := validateExpr(*expression.Unary.Operand, parameters); err != nil {
			return err
		}
		switch expression.Unary.Operator {
		case OperatorNot:
			boolean := Type{Kind: TypePrimitive, Primitive: PrimitiveBool}
			if !TypeEqual(expression.Unary.Operand.Type, boolean) || !TypeEqual(expression.Type, boolean) {
				return fmt.Errorf("operator %q requires and returns bool", expression.Unary.Operator)
			}
		case OperatorNegate:
			expected, err := ArithmeticResultType(expression.Unary.Operator, expression.Unary.Operand.Type, nil)
			if err != nil {
				return err
			}
			if !TypeEqual(expression.Type, expected) {
				return fmt.Errorf("operator %q has an invalid Result type", expression.Unary.Operator)
			}
		default:
			return fmt.Errorf("unsupported unary operator %q", expression.Unary.Operator)
		}
	case ExprBinary:
		if expression.Binary == nil || expression.Binary.Left == nil || expression.Binary.Right == nil {
			return fmt.Errorf("binary expression is incomplete")
		}
		if err := validateExpr(*expression.Binary.Left, parameters); err != nil {
			return err
		}
		if err := validateExpr(*expression.Binary.Right, parameters); err != nil {
			return err
		}
		operator := expression.Binary.Operator
		switch operator {
		case OperatorAdd:
			text := Type{Kind: TypePrimitive, Primitive: PrimitiveString}
			if TypeEqual(expression.Binary.Left.Type, text) && TypeEqual(expression.Binary.Right.Type, text) && TypeEqual(expression.Type, text) {
				break
			}
			expected, err := ArithmeticResultType(operator, expression.Binary.Left.Type, &expression.Binary.Right.Type)
			if err != nil {
				return err
			}
			if !TypeEqual(expression.Type, expected) {
				return fmt.Errorf("operator %q has an invalid Result type", operator)
			}
		case OperatorSubtract, OperatorMultiply, OperatorDivide:
			expected, err := ArithmeticResultType(operator, expression.Binary.Left.Type, &expression.Binary.Right.Type)
			if err != nil {
				return err
			}
			if !TypeEqual(expression.Type, expected) {
				return fmt.Errorf("operator %q has an invalid Result type", operator)
			}
		case OperatorEqual, OperatorNotEqual, OperatorLessThan, OperatorLessOrEqual, OperatorGreaterThan, OperatorGreaterOrEqual:
			boolean := Type{Kind: TypePrimitive, Primitive: PrimitiveBool}
			if !TypeEqual(expression.Binary.Left.Type, expression.Binary.Right.Type) || !TypeEqual(expression.Type, boolean) {
				return fmt.Errorf("operator %q has mismatched operand or result types", operator)
			}
		case OperatorAnd, OperatorOr:
			boolean := Type{Kind: TypePrimitive, Primitive: PrimitiveBool}
			if !TypeEqual(expression.Binary.Left.Type, boolean) || !TypeEqual(expression.Binary.Right.Type, boolean) || !TypeEqual(expression.Type, boolean) {
				return fmt.Errorf("operator %q requires and returns bool", operator)
			}
		default:
			return fmt.Errorf("unsupported binary operator %q", operator)
		}
	default:
		return fmt.Errorf("unsupported expression kind %q", expression.Kind)
	}
	return nil
}

func isLiteralType(value Type) bool {
	if value.Kind == TypeNumeric && value.Numeric != nil {
		return TypeEqual(value, SignedInteger(64)) || TypeEqual(value, BinaryFloat(64))
	}
	return value.Kind == TypePrimitive && (value.Primitive == PrimitiveString || value.Primitive == PrimitiveBool)
}

func CheckedInt64(operator Operator, left, right int64) (int64, ArithmeticError) {
	switch operator {
	case OperatorAdd:
		if (right > 0 && left > maxInt64-right) || (right < 0 && left < minInt64-right) {
			return 0, ArithmeticOverflow
		}
		return left + right, ""
	case OperatorSubtract:
		if (right < 0 && left > maxInt64+right) || (right > 0 && left < minInt64+right) {
			return 0, ArithmeticOverflow
		}
		return left - right, ""
	case OperatorMultiply:
		if left == 0 || right == 0 {
			return 0, ""
		}
		if (left == minInt64 && right == -1) || (right == minInt64 && left == -1) {
			return 0, ArithmeticOverflow
		}
		value := left * right
		if value/right != left {
			return 0, ArithmeticOverflow
		}
		return value, ""
	default:
		panic(fmt.Sprintf("unsupported checked int64 operator %q", operator))
	}
}

func CheckedNegateInt64(value int64) (int64, ArithmeticError) {
	if value == minInt64 {
		return 0, ArithmeticOverflow
	}
	return -value, ""
}

func CheckedDivideBinary64(left, right float64) (float64, ArithmeticError) {
	if right == 0 {
		return 0, ArithmeticDivisionByZero
	}
	return left / right, ""
}
