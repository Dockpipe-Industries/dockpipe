package coreir

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

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
	if left.Kind != right.Kind || left.Primitive != right.Primitive || left.Name != right.Name || (left.Numeric == nil) != (right.Numeric == nil) || (left.Result == nil) != (right.Result == nil) || (left.Record == nil) != (right.Record == nil) || (left.Identity == nil) != (right.Identity == nil) || len(left.Arguments) != len(right.Arguments) {
		return false
	}
	if left.Numeric != nil && *left.Numeric != *right.Numeric {
		return false
	}
	if left.Result != nil && (!TypeEqual(left.Result.Success, right.Result.Success) || !TypeEqual(left.Result.Failure, right.Result.Failure)) {
		return false
	}
	if left.Record != nil {
		if len(left.Record.Fields) != len(right.Record.Fields) {
			return false
		}
		for index := range left.Record.Fields {
			leftField, rightField := left.Record.Fields[index], right.Record.Fields[index]
			if leftField.Name != rightField.Name || leftField.Identity.PackageID != rightField.Identity.PackageID || leftField.Identity.Path != rightField.Identity.Path || !TypeEqual(leftField.Type, rightField.Type) {
				return false
			}
		}
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

// ValidateText enforces PipeLang's target-independent string invariant at
// Core boundaries. PipeLang text is always a valid UTF-8 encoding of a
// preserved Unicode scalar sequence.
func ValidateText(value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("string value is not valid UTF-8")
	}
	return nil
}

// CompareOrdinalText compares preserved Unicode scalar sequences without
// normalization, case folding, locale, or target collation.
func CompareOrdinalText(left, right string) (int, error) {
	if err := ValidateText(left); err != nil {
		return 0, fmt.Errorf("left %w", err)
	}
	if err := ValidateText(right); err != nil {
		return 0, fmt.Errorf("right %w", err)
	}
	for len(left) > 0 && len(right) > 0 {
		leftScalar, leftWidth := utf8.DecodeRuneInString(left)
		rightScalar, rightWidth := utf8.DecodeRuneInString(right)
		if leftScalar < rightScalar {
			return -1, nil
		}
		if leftScalar > rightScalar {
			return 1, nil
		}
		left = left[leftWidth:]
		right = right[rightWidth:]
	}
	if len(left) < len(right) {
		return -1, nil
	}
	if len(left) > len(right) {
		return 1, nil
	}
	return 0, nil
}

// ValidateFunction checks Core types and operator contracts before either a
// semantic evaluator or a target backend consumes the function.
func ValidateFunction(function Function) error {
	for position, parameter := range function.Parameters {
		if parameter.Position != position {
			return fmt.Errorf("parameter %d is not in normalized position order", position)
		}
		if err := validateType(parameter.Type); err != nil {
			return fmt.Errorf("parameter %d type: %w", position, err)
		}
	}
	if err := validateType(function.ReturnType); err != nil {
		return fmt.Errorf("return type: %w", err)
	}
	if exprContainsRecordConstruction(function.Body) && function.Body.Kind != ExprRecordConstruct {
		return fmt.Errorf("record construction must be the complete function body")
	}
	if err := validateExpr(function.Body, function.Parameters); err != nil {
		return err
	}
	if !TypeEqual(function.ReturnType, function.Body.Type) {
		return fmt.Errorf("function return type does not match its body type")
	}
	return nil
}

func validateType(value Type) error {
	if value.Kind != TypeRecord {
		return nil
	}
	if value.Record == nil || value.Identity == nil || value.Identity.PackageID == "" || value.Identity.Path == "" || value.Identity.Callable != nil || value.Name == "" || len(value.Record.Fields) == 0 {
		return fmt.Errorf("record type has an invalid identity or field schema")
	}
	if value.Primitive != "" || value.Numeric != nil || value.Result != nil || len(value.Arguments) != 0 {
		return fmt.Errorf("record type carries a non-record representation")
	}
	seenNames := map[string]struct{}{}
	seenIdentities := map[string]struct{}{}
	for index, field := range value.Record.Fields {
		if field.Name == "" || field.Identity.PackageID == "" || field.Identity.Path == "" || field.Identity.Callable != nil {
			return fmt.Errorf("record field %d has an invalid identity", index)
		}
		if field.Identity.PackageID != value.Identity.PackageID || !strings.HasPrefix(field.Identity.Path, value.Identity.Path+".") {
			return fmt.Errorf("record field %d identity is outside its record identity", index)
		}
		if _, duplicate := seenNames[field.Name]; duplicate {
			return fmt.Errorf("record field %d repeats name %q", index, field.Name)
		}
		seenNames[field.Name] = struct{}{}
		identity := field.Identity.PackageID + "\x00" + field.Identity.Path
		if _, duplicate := seenIdentities[identity]; duplicate {
			return fmt.Errorf("record field %d repeats semantic identity", index)
		}
		seenIdentities[identity] = struct{}{}
		if !isPrimitiveRecordFieldType(field.Type) {
			return fmt.Errorf("record field %d has non-primitive type %q", index, field.Type.Kind)
		}
	}
	return nil
}

func isPrimitiveRecordFieldType(value Type) bool {
	if value.Kind == TypePrimitive {
		return value.Primitive == PrimitiveString || value.Primitive == PrimitiveBool
	}
	if value.Kind != TypeNumeric || value.Numeric == nil {
		return false
	}
	return (value.Numeric.Representation == NumericInteger && value.Numeric.Bits == 64 && value.Numeric.Signed) ||
		(value.Numeric.Representation == NumericBinaryFloat && value.Numeric.Bits == 64 && !value.Numeric.Signed)
}

func validateExpr(expression Expr, parameters []Parameter) error {
	switch expression.Kind {
	case ExprLiteral:
		if expression.Literal == nil || !isLiteralType(expression.Type) {
			return fmt.Errorf("literal has an invalid type")
		}
		if expression.Type.Kind == TypePrimitive && expression.Type.Primitive == PrimitiveString {
			if err := ValidateText(expression.Literal.String); err != nil {
				return fmt.Errorf("literal %w", err)
			}
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
	case ExprFieldProjection:
		if expression.Field == nil || expression.Field.Receiver == nil {
			return fmt.Errorf("field projection is incomplete")
		}
		if err := validateExpr(*expression.Field.Receiver, parameters); err != nil {
			return err
		}
		receiver := expression.Field.Receiver.Type
		if receiver.Kind != TypeRecord || receiver.Record == nil {
			return fmt.Errorf("field projection receiver is not a record")
		}
		if err := validateType(receiver); err != nil {
			return fmt.Errorf("field projection receiver type: %w", err)
		}
		position := expression.Field.Position
		if position < 0 || position >= len(receiver.Record.Fields) {
			return fmt.Errorf("field projection has an invalid declared position")
		}
		field := receiver.Record.Fields[position]
		if expression.Field.Name != field.Name || expression.Field.Identity.PackageID != field.Identity.PackageID || expression.Field.Identity.Path != field.Identity.Path || expression.Field.Identity.Callable != nil {
			return fmt.Errorf("field projection does not match the record field identity at its declared position")
		}
		if !TypeEqual(expression.Type, field.Type) {
			return fmt.Errorf("field projection result type does not match the declared field type")
		}
	case ExprRecordConstruct:
		if expression.Record == nil {
			return fmt.Errorf("record construction is incomplete")
		}
		if expression.Type.Kind != TypeRecord || expression.Type.Record == nil || expression.Type.Identity == nil {
			return fmt.Errorf("record construction result is not an identified record")
		}
		if err := validateType(expression.Type); err != nil {
			return fmt.Errorf("record construction result type: %w", err)
		}
		if expression.Record.Identity.PackageID != expression.Type.Identity.PackageID || expression.Record.Identity.Path != expression.Type.Identity.Path || expression.Record.Identity.Callable != nil {
			return fmt.Errorf("record construction identity does not match its result type")
		}
		if len(expression.Record.Fields) != len(expression.Type.Record.Fields) {
			return fmt.Errorf("record construction field count does not match its schema")
		}
		if len(parameters) != len(expression.Record.Fields) {
			return fmt.Errorf("record construction requires one corresponding parameter per field")
		}
		for position, initialized := range expression.Record.Fields {
			if initialized.Value == nil {
				return fmt.Errorf("record construction field %d has no value", position)
			}
			if initialized.Position != position {
				return fmt.Errorf("record construction field %d is not in declaration order", position)
			}
			declared := expression.Type.Record.Fields[position]
			if initialized.Name != declared.Name || initialized.Identity.PackageID != declared.Identity.PackageID || initialized.Identity.Path != declared.Identity.Path || initialized.Identity.Callable != nil {
				return fmt.Errorf("record construction field %d does not match its declared identity", position)
			}
			if err := validateExpr(*initialized.Value, parameters); err != nil {
				return fmt.Errorf("record construction field %d: %w", position, err)
			}
			if initialized.Value.Kind != ExprReference || initialized.Value.Parameter == nil || *initialized.Value.Parameter != position {
				return fmt.Errorf("record construction field %d value is not its corresponding direct parameter", position)
			}
			if !TypeEqual(initialized.Value.Type, declared.Type) {
				return fmt.Errorf("record construction field %d value type does not match its declaration", position)
			}
		}
	default:
		return fmt.Errorf("unsupported expression kind %q", expression.Kind)
	}
	return nil
}

func exprContainsRecordConstruction(expression Expr) bool {
	switch expression.Kind {
	case ExprRecordConstruct:
		return true
	case ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && exprContainsRecordConstruction(*expression.Unary.Operand)
	case ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (exprContainsRecordConstruction(*expression.Binary.Left) || exprContainsRecordConstruction(*expression.Binary.Right))
	case ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && exprContainsRecordConstruction(*expression.Field.Receiver)
	default:
		return false
	}
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
