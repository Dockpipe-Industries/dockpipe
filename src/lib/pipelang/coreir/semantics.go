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
	if left.Kind != right.Kind || left.Primitive != right.Primitive || left.Name != right.Name || (left.Numeric == nil) != (right.Numeric == nil) || (left.Result == nil) != (right.Result == nil) || (left.Optional == nil) != (right.Optional == nil) || (left.List == nil) != (right.List == nil) || (left.Record == nil) != (right.Record == nil) || (left.Identity == nil) != (right.Identity == nil) || len(left.Arguments) != len(right.Arguments) {
		return false
	}
	if left.Numeric != nil && *left.Numeric != *right.Numeric {
		return false
	}
	if left.Result != nil && (!TypeEqual(left.Result.Success, right.Result.Success) || !TypeEqual(left.Result.Failure, right.Result.Failure)) {
		return false
	}
	if left.Optional != nil && !TypeEqual(left.Optional.Value, right.Optional.Value) {
		return false
	}
	if left.List != nil && !TypeEqual(left.List.Element, right.List.Element) {
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
	if exprContainsRecordEquality(function.Body) {
		if err := validateDirectRecordEquality(function); err != nil {
			return err
		}
	}
	snapshotResult := functionContainsSnapshotResult(function)
	if snapshotResult {
		if err := validateDirectSnapshotResultFunction(function); err != nil {
			return err
		}
	}
	listAt := function.Body.Kind == ExprListAt
	if listAt {
		if err := validateDirectListAtFunction(function); err != nil {
			return err
		}
	}
	if functionContainsOptional(function) && !snapshotResult && !listAt {
		if err := validateDirectOptionalFunction(function); err != nil {
			return err
		}
	}
	if functionContainsList(function) && !snapshotResult && !listAt {
		if err := validateDirectListFunction(function); err != nil {
			return err
		}
	}
	if !TypeEqual(function.ReturnType, function.Body.Type) {
		return fmt.Errorf("function return type does not match its body type")
	}
	return nil
}

func validateType(value Type) error {
	if value.Kind != TypeResult && value.Result != nil {
		return fmt.Errorf("non-result type carries a result representation")
	}
	if value.Kind == TypeResult {
		if value.Result == nil {
			return fmt.Errorf("result type has no success/failure shape")
		}
		if err := validateType(value.Result.Success); err != nil {
			return fmt.Errorf("result success type: %w", err)
		}
		if err := validateType(value.Result.Failure); err != nil {
			return fmt.Errorf("result failure type: %w", err)
		}
		if !isArithmeticResultType(value) && !isSnapshotResultType(value) {
			return fmt.Errorf("result type is outside the checked-arithmetic and snapshot envelopes")
		}
		if value.Primitive != "" || value.Numeric != nil || value.Optional != nil || value.List != nil || value.Record != nil || value.Identity != nil || value.Name != "" || len(value.Arguments) != 0 {
			return fmt.Errorf("result type carries a non-result representation")
		}
		return nil
	}
	if value.Kind != TypeList && value.List != nil {
		return fmt.Errorf("non-list type carries a list representation")
	}
	if value.Kind == TypeList {
		if value.List == nil || value.List.Element.Kind != TypeRecord || value.Identity == nil || value.Identity.PackageID != BuiltinPackageID || value.Identity.Path != ListSemanticPath || value.Identity.Callable != nil || value.Name != "List" {
			return fmt.Errorf("list type requires one identified primitive-record element type")
		}
		if err := validateType(value.List.Element); err != nil {
			return fmt.Errorf("list element type: %w", err)
		}
		if value.Primitive != "" || value.Numeric != nil || value.Result != nil || value.Optional != nil || value.Record != nil || len(value.Arguments) != 0 {
			return fmt.Errorf("list type carries a non-list representation")
		}
		return nil
	}
	if value.Kind != TypeOptional && value.Optional != nil {
		return fmt.Errorf("non-optional type carries an optional representation")
	}
	if value.Kind == TypeOptional {
		if value.Optional == nil || !isOptionalValueType(value.Optional.Value) {
			return fmt.Errorf("optional type requires one primitive or primitive-record value type")
		}
		if value.Primitive != "" || value.Numeric != nil || value.Result != nil || value.List != nil || value.Record != nil || value.Identity != nil || value.Name != "" || len(value.Arguments) != 0 {
			return fmt.Errorf("optional type carries a non-optional representation")
		}
		return nil
	}
	if value.Kind != TypeRecord {
		return nil
	}
	if value.Record == nil || value.Identity == nil || value.Identity.PackageID == "" || value.Identity.Path == "" || value.Identity.Callable != nil || value.Name == "" || len(value.Record.Fields) == 0 {
		return fmt.Errorf("record type has an invalid identity or field schema")
	}
	if value.Primitive != "" || value.Numeric != nil || value.Result != nil || value.Optional != nil || value.List != nil || len(value.Arguments) != 0 {
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

func isArithmeticResultType(value Type) bool {
	return value.Kind == TypeResult && value.Result != nil && value.Result.Failure.Kind == TypeArithmeticError && (TypeEqual(value.Result.Success, SignedInteger(64)) || TypeEqual(value.Result.Success, BinaryFloat(64)))
}

func isSnapshotResultType(value Type) bool {
	return value.Kind == TypeResult && value.Result != nil && value.Result.Success.Kind == TypeList && value.Result.Success.List != nil && value.Result.Success.List.Element.Kind == TypeRecord && value.Result.Failure.Kind == TypePrimitive && value.Result.Failure.Primitive == PrimitiveString
}

func isPrimitiveOptionalValueType(value Type) bool {
	if value.Kind == TypePrimitive {
		return (value.Primitive == PrimitiveString || value.Primitive == PrimitiveBool) && value.Numeric == nil && value.Result == nil && value.Optional == nil && value.Record == nil && value.Identity == nil && value.Name == "" && len(value.Arguments) == 0
	}
	if value.Kind != TypeNumeric || value.Numeric == nil || value.Primitive != "" || value.Result != nil || value.Optional != nil || value.Record != nil || value.Identity != nil || value.Name != "" || len(value.Arguments) != 0 {
		return false
	}
	return (value.Numeric.Representation == NumericInteger && value.Numeric.Bits == 64 && value.Numeric.Signed) ||
		(value.Numeric.Representation == NumericBinaryFloat && value.Numeric.Bits == 64 && !value.Numeric.Signed)
}

func isOptionalValueType(value Type) bool {
	if isPrimitiveOptionalValueType(value) {
		return true
	}
	return value.Kind == TypeRecord && validateType(value) == nil
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
			if expression.Binary.Left.Type.Kind == TypeRecord && operator != OperatorEqual && operator != OperatorNotEqual {
				return fmt.Errorf("record values support structural equality only, not operator %q", operator)
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
	case ExprOptionalSome:
		if expression.Some == nil || expression.Some.Value == nil || expression.Type.Kind != TypeOptional || expression.Type.Optional == nil {
			return fmt.Errorf("optional some expression is incomplete")
		}
		if err := validateType(expression.Type); err != nil {
			return fmt.Errorf("optional some result type: %w", err)
		}
		if err := validateExpr(*expression.Some.Value, parameters); err != nil {
			return fmt.Errorf("optional some value: %w", err)
		}
		if !TypeEqual(expression.Some.Value.Type, expression.Type.Optional.Value) {
			return fmt.Errorf("optional some value type does not match its result type")
		}
	case ExprOptionalNone:
		if expression.None == nil || expression.Type.Kind != TypeOptional || expression.Type.Optional == nil {
			return fmt.Errorf("optional none expression is incomplete")
		}
		if err := validateType(expression.Type); err != nil {
			return fmt.Errorf("optional none result type: %w", err)
		}
	case ExprOptionalHasValue:
		boolean := Type{Kind: TypePrimitive, Primitive: PrimitiveBool}
		if expression.HasValue == nil || expression.HasValue.Value == nil || !TypeEqual(expression.Type, boolean) {
			return fmt.Errorf("optional has_value expression is incomplete or does not return bool")
		}
		if err := validateExpr(*expression.HasValue.Value, parameters); err != nil {
			return fmt.Errorf("optional has_value operand: %w", err)
		}
		if expression.HasValue.Value.Type.Kind != TypeOptional {
			return fmt.Errorf("optional has_value operand is not Optional")
		}
	case ExprOptionalValueOr:
		if expression.ValueOr == nil || expression.ValueOr.Value == nil || expression.ValueOr.Fallback == nil {
			return fmt.Errorf("optional value_or expression is incomplete")
		}
		if err := validateExpr(*expression.ValueOr.Value, parameters); err != nil {
			return fmt.Errorf("optional value_or operand: %w", err)
		}
		if err := validateExpr(*expression.ValueOr.Fallback, parameters); err != nil {
			return fmt.Errorf("optional value_or fallback: %w", err)
		}
		if expression.ValueOr.Value.Type.Kind != TypeOptional || expression.ValueOr.Value.Type.Optional == nil {
			return fmt.Errorf("optional value_or first operand is not Optional")
		}
		if !TypeEqual(expression.ValueOr.Value.Type.Optional.Value, expression.ValueOr.Fallback.Type) || !TypeEqual(expression.Type, expression.ValueOr.Fallback.Type) {
			return fmt.Errorf("optional value_or payload, fallback, and result types do not match")
		}
	case ExprListEmpty:
		if expression.ListEmpty == nil || expression.Type.Kind != TypeList || expression.Type.List == nil {
			return fmt.Errorf("list empty expression is incomplete")
		}
		if err := validateType(expression.Type); err != nil {
			return fmt.Errorf("list empty result type: %w", err)
		}
	case ExprListSingleton:
		if expression.ListOne == nil || expression.ListOne.Value == nil || expression.Type.Kind != TypeList || expression.Type.List == nil {
			return fmt.Errorf("list singleton expression is incomplete")
		}
		if err := validateType(expression.Type); err != nil {
			return fmt.Errorf("list singleton result type: %w", err)
		}
		if err := validateExpr(*expression.ListOne.Value, parameters); err != nil {
			return fmt.Errorf("list singleton value: %w", err)
		}
		if !TypeEqual(expression.ListOne.Value.Type, expression.Type.List.Element) {
			return fmt.Errorf("list singleton value type does not match its element type")
		}
	case ExprListCount:
		if expression.ListCount == nil || expression.ListCount.Value == nil || !TypeEqual(expression.Type, SignedInteger(64)) {
			return fmt.Errorf("list count expression is incomplete or does not return signed 64-bit int")
		}
		if err := validateExpr(*expression.ListCount.Value, parameters); err != nil {
			return fmt.Errorf("list count operand: %w", err)
		}
		if expression.ListCount.Value.Type.Kind != TypeList {
			return fmt.Errorf("list count operand is not a List")
		}
	case ExprListAppend:
		if expression.ListAppend == nil || expression.ListAppend.Values == nil || expression.ListAppend.Value == nil || expression.Type.Kind != TypeList || expression.Type.List == nil {
			return fmt.Errorf("list append expression is incomplete")
		}
		if err := validateType(expression.Type); err != nil {
			return fmt.Errorf("list append result type: %w", err)
		}
		if err := validateExpr(*expression.ListAppend.Values, parameters); err != nil {
			return fmt.Errorf("list append values: %w", err)
		}
		if err := validateExpr(*expression.ListAppend.Value, parameters); err != nil {
			return fmt.Errorf("list append value: %w", err)
		}
		if !TypeEqual(expression.ListAppend.Values.Type, expression.Type) {
			return fmt.Errorf("list append values type does not match its result type")
		}
		if !TypeEqual(expression.ListAppend.Value.Type, expression.Type.List.Element) {
			return fmt.Errorf("list append value type does not match its element type")
		}
	case ExprListAt:
		if expression.ListAt == nil || expression.ListAt.Values == nil || expression.ListAt.Index == nil || expression.Type.Kind != TypeOptional || expression.Type.Optional == nil {
			return fmt.Errorf("list at expression is incomplete")
		}
		if err := validateType(expression.Type); err != nil {
			return fmt.Errorf("list at result type: %w", err)
		}
		if err := validateExpr(*expression.ListAt.Values, parameters); err != nil {
			return fmt.Errorf("list at values: %w", err)
		}
		if err := validateExpr(*expression.ListAt.Index, parameters); err != nil {
			return fmt.Errorf("list at index: %w", err)
		}
		if expression.ListAt.Values.Type.Kind != TypeList || expression.ListAt.Values.Type.List == nil || !TypeEqual(expression.ListAt.Values.Type.List.Element, expression.Type.Optional.Value) {
			return fmt.Errorf("list at values element type does not match its Optional result")
		}
		if !TypeEqual(expression.ListAt.Index.Type, SignedInteger(64)) {
			return fmt.Errorf("list at index is not signed 64-bit int")
		}
	case ExprResultOK:
		if expression.ResultOK == nil || expression.ResultOK.Value == nil || !isSnapshotResultType(expression.Type) {
			return fmt.Errorf("result ok expression is incomplete or has an invalid result type")
		}
		if err := validateExpr(*expression.ResultOK.Value, parameters); err != nil {
			return fmt.Errorf("result ok value: %w", err)
		}
		if !TypeEqual(expression.ResultOK.Value.Type, expression.Type.Result.Success) {
			return fmt.Errorf("result ok value type does not match its success type")
		}
	case ExprResultErr:
		if expression.ResultErr == nil || expression.ResultErr.Error == nil || !isSnapshotResultType(expression.Type) {
			return fmt.Errorf("result err expression is incomplete or has an invalid result type")
		}
		if err := validateExpr(*expression.ResultErr.Error, parameters); err != nil {
			return fmt.Errorf("result err value: %w", err)
		}
		if !TypeEqual(expression.ResultErr.Error.Type, expression.Type.Result.Failure) {
			return fmt.Errorf("result err value type does not match its failure type")
		}
	case ExprResultIsOK:
		boolean := Type{Kind: TypePrimitive, Primitive: PrimitiveBool}
		if expression.ResultIsOK == nil || expression.ResultIsOK.Value == nil || !TypeEqual(expression.Type, boolean) {
			return fmt.Errorf("result is_ok expression is incomplete or does not return bool")
		}
		if err := validateExpr(*expression.ResultIsOK.Value, parameters); err != nil {
			return fmt.Errorf("result is_ok value: %w", err)
		}
		if !isSnapshotResultType(expression.ResultIsOK.Value.Type) {
			return fmt.Errorf("result is_ok operand is not a snapshot Result")
		}
	case ExprResultSuccessOr:
		if expression.SuccessOr == nil || expression.SuccessOr.Value == nil || expression.SuccessOr.Fallback == nil {
			return fmt.Errorf("result success_or expression is incomplete")
		}
		if err := validateExpr(*expression.SuccessOr.Value, parameters); err != nil {
			return fmt.Errorf("result success_or value: %w", err)
		}
		if err := validateExpr(*expression.SuccessOr.Fallback, parameters); err != nil {
			return fmt.Errorf("result success_or fallback: %w", err)
		}
		resultType := expression.SuccessOr.Value.Type
		if !isSnapshotResultType(resultType) || !TypeEqual(expression.SuccessOr.Fallback.Type, resultType.Result.Success) || !TypeEqual(expression.Type, resultType.Result.Success) {
			return fmt.Errorf("result success_or operand, fallback, and return types do not match")
		}
	case ExprResultFailureOr:
		if expression.FailureOr == nil || expression.FailureOr.Value == nil || expression.FailureOr.Fallback == nil {
			return fmt.Errorf("result failure_or expression is incomplete")
		}
		if err := validateExpr(*expression.FailureOr.Value, parameters); err != nil {
			return fmt.Errorf("result failure_or value: %w", err)
		}
		if err := validateExpr(*expression.FailureOr.Fallback, parameters); err != nil {
			return fmt.Errorf("result failure_or fallback: %w", err)
		}
		resultType := expression.FailureOr.Value.Type
		if !isSnapshotResultType(resultType) || !TypeEqual(expression.FailureOr.Fallback.Type, resultType.Result.Failure) || !TypeEqual(expression.Type, resultType.Result.Failure) {
			return fmt.Errorf("result failure_or operand, fallback, and return types do not match")
		}
	default:
		return fmt.Errorf("unsupported expression kind %q", expression.Kind)
	}
	return nil
}

func functionContainsSnapshotResult(function Function) bool {
	if isSnapshotResultType(function.ReturnType) || exprContainsSnapshotResult(function.Body) {
		return true
	}
	for _, parameter := range function.Parameters {
		if isSnapshotResultType(parameter.Type) {
			return true
		}
	}
	return false
}

func exprContainsSnapshotResult(expression Expr) bool {
	if isSnapshotResultType(expression.Type) {
		return true
	}
	switch expression.Kind {
	case ExprResultOK, ExprResultErr, ExprResultIsOK, ExprResultSuccessOr, ExprResultFailureOr:
		return true
	case ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && exprContainsSnapshotResult(*expression.Unary.Operand)
	case ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (exprContainsSnapshotResult(*expression.Binary.Left) || exprContainsSnapshotResult(*expression.Binary.Right))
	case ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && exprContainsSnapshotResult(*expression.Field.Receiver)
	case ExprListAt:
		return expression.ListAt != nil && expression.ListAt.Values != nil && expression.ListAt.Index != nil && (exprContainsSnapshotResult(*expression.ListAt.Values) || exprContainsSnapshotResult(*expression.ListAt.Index))
	}
	return false
}

func validateDirectSnapshotResultFunction(function Function) error {
	directParameter := func(expression *Expr, position int) bool {
		return expression != nil && expression.Kind == ExprReference && expression.Parameter != nil && *expression.Parameter == position && position < len(function.Parameters) && TypeEqual(expression.Type, function.Parameters[position].Type)
	}
	switch function.Body.Kind {
	case ExprResultOK:
		if !isSnapshotResultType(function.ReturnType) || function.Body.ResultOK == nil || len(function.Parameters) != 1 || !TypeEqual(function.Parameters[0].Type, function.ReturnType.Result.Success) || !directParameter(function.Body.ResultOK.Value, 0) {
			return fmt.Errorf("snapshot Result ok requires one direct matching success parameter")
		}
	case ExprResultErr:
		if !isSnapshotResultType(function.ReturnType) || function.Body.ResultErr == nil || len(function.Parameters) != 1 || !TypeEqual(function.Parameters[0].Type, function.ReturnType.Result.Failure) || !directParameter(function.Body.ResultErr.Error, 0) {
			return fmt.Errorf("snapshot Result err requires one direct matching failure parameter")
		}
	case ExprReference:
		if !isSnapshotResultType(function.ReturnType) || len(function.Parameters) != 1 || !TypeEqual(function.Parameters[0].Type, function.ReturnType) || !directParameter(&function.Body, 0) {
			return fmt.Errorf("snapshot Result identity requires one identical direct parameter and return")
		}
	case ExprResultIsOK:
		boolean := Type{Kind: TypePrimitive, Primitive: PrimitiveBool}
		if function.Body.ResultIsOK == nil || len(function.Parameters) != 1 || !isSnapshotResultType(function.Parameters[0].Type) || !TypeEqual(function.ReturnType, boolean) || !directParameter(function.Body.ResultIsOK.Value, 0) {
			return fmt.Errorf("snapshot Result is_ok requires one direct Result parameter and bool return")
		}
	case ExprResultSuccessOr:
		if function.Body.SuccessOr == nil || len(function.Parameters) != 2 || !isSnapshotResultType(function.Parameters[0].Type) || !TypeEqual(function.Parameters[0].Type.Result.Success, function.Parameters[1].Type) || !TypeEqual(function.ReturnType, function.Parameters[1].Type) || !directParameter(function.Body.SuccessOr.Value, 0) || !directParameter(function.Body.SuccessOr.Fallback, 1) {
			return fmt.Errorf("snapshot Result success_or requires direct Result and matching success fallback parameters")
		}
	case ExprResultFailureOr:
		if function.Body.FailureOr == nil || len(function.Parameters) != 2 || !isSnapshotResultType(function.Parameters[0].Type) || !TypeEqual(function.Parameters[0].Type.Result.Failure, function.Parameters[1].Type) || !TypeEqual(function.ReturnType, function.Parameters[1].Type) || !directParameter(function.Body.FailureOr.Value, 0) || !directParameter(function.Body.FailureOr.Fallback, 1) {
			return fmt.Errorf("snapshot Result failure_or requires direct Result and matching failure fallback parameters")
		}
	default:
		return fmt.Errorf("snapshot Result types are admitted only in direct ok, err, identity, is_ok, success_or, or failure_or functions")
	}
	return nil
}

func functionContainsList(function Function) bool {
	if function.ReturnType.Kind == TypeList || exprContainsList(function.Body) {
		return true
	}
	for _, parameter := range function.Parameters {
		if parameter.Type.Kind == TypeList {
			return true
		}
	}
	return false
}

func exprContainsList(expression Expr) bool {
	switch expression.Kind {
	case ExprListEmpty, ExprListSingleton, ExprListCount, ExprListAppend, ExprListAt:
		return true
	case ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && exprContainsList(*expression.Unary.Operand)
	case ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (exprContainsList(*expression.Binary.Left) || exprContainsList(*expression.Binary.Right))
	case ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && exprContainsList(*expression.Field.Receiver)
	case ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				if field.Value != nil && exprContainsList(*field.Value) {
					return true
				}
			}
		}
	}
	return false
}

func validateDirectListFunction(function Function) error {
	if function.Body.Kind == ExprListCount {
		if function.Body.ListCount == nil || function.Body.ListCount.Value == nil || len(function.Parameters) != 1 || function.Parameters[0].Type.Kind != TypeList || !TypeEqual(function.ReturnType, SignedInteger(64)) {
			return fmt.Errorf("count requires one record-list parameter and a signed 64-bit int return")
		}
		value := function.Body.ListCount.Value
		if value.Kind != ExprReference || value.Parameter == nil || *value.Parameter != 0 || !TypeEqual(value.Type, function.Parameters[0].Type) {
			return fmt.Errorf("count operand must be its sole direct record-list parameter")
		}
		return nil
	}
	if function.ReturnType.Kind != TypeList || function.ReturnType.List == nil {
		return fmt.Errorf("record-list values are admitted only as direct list-returning functions")
	}
	switch function.Body.Kind {
	case ExprListEmpty:
		if function.Body.ListEmpty == nil || len(function.Parameters) != 0 {
			return fmt.Errorf("empty_list requires no parameters and a record-list return")
		}
	case ExprListSingleton:
		if function.Body.ListOne == nil || function.Body.ListOne.Value == nil || len(function.Parameters) != 1 {
			return fmt.Errorf("list singleton requires one record parameter and a matching record-list return")
		}
		value := function.Body.ListOne.Value
		if value.Kind != ExprReference || value.Parameter == nil || *value.Parameter != 0 || !TypeEqual(function.Parameters[0].Type, function.ReturnType.List.Element) || !TypeEqual(value.Type, function.Parameters[0].Type) {
			return fmt.Errorf("list singleton value must be its sole corresponding direct record parameter")
		}
	case ExprReference:
		if len(function.Parameters) != 1 || function.Body.Parameter == nil || *function.Body.Parameter != 0 || !TypeEqual(function.Parameters[0].Type, function.ReturnType) {
			return fmt.Errorf("record-list identity transport requires one identical direct parameter and return")
		}
	case ExprListAppend:
		if function.Body.ListAppend == nil || function.Body.ListAppend.Values == nil || function.Body.ListAppend.Value == nil || len(function.Parameters) != 2 {
			return fmt.Errorf("append requires one record-list parameter, one matching record parameter, and a matching record-list return")
		}
		values := function.Body.ListAppend.Values
		value := function.Body.ListAppend.Value
		if values.Kind != ExprReference || values.Parameter == nil || *values.Parameter != 0 || !TypeEqual(values.Type, function.Parameters[0].Type) || !TypeEqual(function.Parameters[0].Type, function.ReturnType) {
			return fmt.Errorf("append values must be its first direct record-list parameter")
		}
		if value.Kind != ExprReference || value.Parameter == nil || *value.Parameter != 1 || !TypeEqual(value.Type, function.Parameters[1].Type) || !TypeEqual(function.Parameters[1].Type, function.ReturnType.List.Element) {
			return fmt.Errorf("append value must be its second direct matching record parameter")
		}
	default:
		return fmt.Errorf("record-list values are admitted only in direct empty_list, singleton list, identity-transport, or append functions")
	}
	return nil
}

func validateDirectListAtFunction(function Function) error {
	if function.Body.ListAt == nil || function.Body.ListAt.Values == nil || function.Body.ListAt.Index == nil || len(function.Parameters) != 2 || function.Parameters[0].Type.Kind != TypeList || function.Parameters[0].Type.List == nil || !TypeEqual(function.Parameters[1].Type, SignedInteger(64)) || function.ReturnType.Kind != TypeOptional || function.ReturnType.Optional == nil || !TypeEqual(function.ReturnType.Optional.Value, function.Parameters[0].Type.List.Element) {
		return fmt.Errorf("at requires direct List<R> and signed 64-bit int parameters with Optional<R> return")
	}
	values := function.Body.ListAt.Values
	index := function.Body.ListAt.Index
	if values.Kind != ExprReference || values.Parameter == nil || *values.Parameter != 0 || !TypeEqual(values.Type, function.Parameters[0].Type) {
		return fmt.Errorf("at values must be its first direct record-list parameter")
	}
	if index.Kind != ExprReference || index.Parameter == nil || *index.Parameter != 1 || !TypeEqual(index.Type, function.Parameters[1].Type) {
		return fmt.Errorf("at index must be its second direct signed 64-bit parameter")
	}
	return nil
}

func functionContainsOptional(function Function) bool {
	if function.ReturnType.Kind == TypeOptional || exprContainsOptional(function.Body) {
		return true
	}
	for _, parameter := range function.Parameters {
		if parameter.Type.Kind == TypeOptional {
			return true
		}
	}
	return false
}

func exprContainsOptional(expression Expr) bool {
	switch expression.Kind {
	case ExprOptionalSome, ExprOptionalNone, ExprOptionalHasValue, ExprOptionalValueOr:
		return true
	case ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && exprContainsOptional(*expression.Unary.Operand)
	case ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (exprContainsOptional(*expression.Binary.Left) || exprContainsOptional(*expression.Binary.Right))
	case ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && exprContainsOptional(*expression.Field.Receiver)
	case ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				if field.Value != nil && exprContainsOptional(*field.Value) {
					return true
				}
			}
		}
	case ExprListSingleton:
		return expression.ListOne != nil && expression.ListOne.Value != nil && exprContainsOptional(*expression.ListOne.Value)
	case ExprListCount:
		return expression.ListCount != nil && expression.ListCount.Value != nil && exprContainsOptional(*expression.ListCount.Value)
	case ExprListAppend:
		return expression.ListAppend != nil && expression.ListAppend.Values != nil && expression.ListAppend.Value != nil && (exprContainsOptional(*expression.ListAppend.Values) || exprContainsOptional(*expression.ListAppend.Value))
	case ExprListAt:
		return true
	}
	return false
}

func validateDirectOptionalFunction(function Function) error {
	boolean := Type{Kind: TypePrimitive, Primitive: PrimitiveBool}
	switch function.Body.Kind {
	case ExprOptionalSome:
		if function.Body.Some == nil || function.Body.Some.Value == nil || function.ReturnType.Kind != TypeOptional || function.ReturnType.Optional == nil || len(function.Parameters) != 1 {
			return fmt.Errorf("optional some requires one matching value parameter and an Optional return")
		}
		value := function.Body.Some.Value
		if value.Kind != ExprReference || value.Parameter == nil || *value.Parameter != 0 || !TypeEqual(function.Parameters[0].Type, function.ReturnType.Optional.Value) {
			return fmt.Errorf("optional some value must be its sole corresponding direct parameter")
		}
	case ExprOptionalNone:
		if function.Body.None == nil || function.ReturnType.Kind != TypeOptional || len(function.Parameters) != 0 {
			return fmt.Errorf("optional none requires no parameters and an Optional return")
		}
	case ExprReference:
		if function.ReturnType.Kind != TypeOptional || len(function.Parameters) != 1 || function.Body.Parameter == nil || *function.Body.Parameter != 0 || !TypeEqual(function.Parameters[0].Type, function.ReturnType) {
			return fmt.Errorf("optional identity transport requires one identical direct parameter and return")
		}
	case ExprOptionalHasValue:
		if function.Body.HasValue == nil || function.Body.HasValue.Value == nil || len(function.Parameters) != 1 || function.Parameters[0].Type.Kind != TypeOptional || !TypeEqual(function.ReturnType, boolean) {
			return fmt.Errorf("optional has_value requires one Optional parameter and a bool return")
		}
		value := function.Body.HasValue.Value
		if value.Kind != ExprReference || value.Parameter == nil || *value.Parameter != 0 || !TypeEqual(value.Type, function.Parameters[0].Type) {
			return fmt.Errorf("optional has_value operand must be its sole direct parameter")
		}
	case ExprOptionalValueOr:
		if function.Body.ValueOr == nil || function.Body.ValueOr.Value == nil || function.Body.ValueOr.Fallback == nil || len(function.Parameters) != 2 || function.Parameters[0].Type.Kind != TypeOptional || function.Parameters[0].Type.Optional == nil {
			return fmt.Errorf("optional value_or requires one Optional parameter, one matching fallback parameter, and a matching return")
		}
		value := function.Body.ValueOr.Value
		fallback := function.Body.ValueOr.Fallback
		if value.Kind != ExprReference || value.Parameter == nil || *value.Parameter != 0 || !TypeEqual(value.Type, function.Parameters[0].Type) {
			return fmt.Errorf("optional value_or operand must be its first direct parameter")
		}
		if fallback.Kind != ExprReference || fallback.Parameter == nil || *fallback.Parameter != 1 || !TypeEqual(fallback.Type, function.Parameters[1].Type) {
			return fmt.Errorf("optional value_or fallback must be its second direct parameter")
		}
		if !TypeEqual(function.Parameters[0].Type.Optional.Value, function.Parameters[1].Type) || !TypeEqual(function.ReturnType, function.Parameters[1].Type) {
			return fmt.Errorf("optional value_or payload, fallback, and return types must match")
		}
	default:
		return fmt.Errorf("Optional types are admitted only in direct some, none, identity transport, has_value, or value_or functions")
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
	case ExprListSingleton:
		return expression.ListOne != nil && expression.ListOne.Value != nil && exprContainsRecordConstruction(*expression.ListOne.Value)
	case ExprListCount:
		return expression.ListCount != nil && expression.ListCount.Value != nil && exprContainsRecordConstruction(*expression.ListCount.Value)
	case ExprListAppend:
		return expression.ListAppend != nil && expression.ListAppend.Values != nil && expression.ListAppend.Value != nil && (exprContainsRecordConstruction(*expression.ListAppend.Values) || exprContainsRecordConstruction(*expression.ListAppend.Value))
	case ExprListAt:
		return expression.ListAt != nil && expression.ListAt.Values != nil && expression.ListAt.Index != nil && (exprContainsRecordConstruction(*expression.ListAt.Values) || exprContainsRecordConstruction(*expression.ListAt.Index))
	default:
		return false
	}
}

func exprContainsRecordEquality(expression Expr) bool {
	switch expression.Kind {
	case ExprBinary:
		if expression.Binary == nil || expression.Binary.Left == nil || expression.Binary.Right == nil {
			return false
		}
		if expression.Binary.Left.Type.Kind == TypeRecord || expression.Binary.Right.Type.Kind == TypeRecord {
			return true
		}
		return exprContainsRecordEquality(*expression.Binary.Left) || exprContainsRecordEquality(*expression.Binary.Right)
	case ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && exprContainsRecordEquality(*expression.Unary.Operand)
	case ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && exprContainsRecordEquality(*expression.Field.Receiver)
	case ExprListCount:
		return expression.ListCount != nil && expression.ListCount.Value != nil && exprContainsRecordEquality(*expression.ListCount.Value)
	case ExprListAppend:
		return expression.ListAppend != nil && expression.ListAppend.Values != nil && expression.ListAppend.Value != nil && (exprContainsRecordEquality(*expression.ListAppend.Values) || exprContainsRecordEquality(*expression.ListAppend.Value))
	case ExprListAt:
		return expression.ListAt != nil && expression.ListAt.Values != nil && expression.ListAt.Index != nil && (exprContainsRecordEquality(*expression.ListAt.Values) || exprContainsRecordEquality(*expression.ListAt.Index))
	case ExprRecordConstruct:
		if expression.Record == nil {
			return false
		}
		for _, field := range expression.Record.Fields {
			if field.Value != nil && exprContainsRecordEquality(*field.Value) {
				return true
			}
		}
	}
	return false
}

func validateDirectRecordEquality(function Function) error {
	boolean := Type{Kind: TypePrimitive, Primitive: PrimitiveBool}
	if len(function.Parameters) != 2 || function.Body.Kind != ExprBinary || function.Body.Binary == nil || function.Body.Binary.Left == nil || function.Body.Binary.Right == nil {
		return fmt.Errorf("record equality requires exactly two parameters and one direct binary body")
	}
	binary := function.Body.Binary
	if binary.Operator != OperatorEqual && binary.Operator != OperatorNotEqual {
		return fmt.Errorf("record equality requires operator %q or %q", OperatorEqual, OperatorNotEqual)
	}
	if function.Parameters[0].Type.Kind != TypeRecord || !TypeEqual(function.Parameters[0].Type, function.Parameters[1].Type) || !TypeEqual(function.ReturnType, boolean) || !TypeEqual(function.Body.Type, boolean) {
		return fmt.Errorf("record equality requires two identical record parameters and a bool result")
	}
	if binary.Left.Kind != ExprReference || binary.Left.Parameter == nil || *binary.Left.Parameter != 0 || binary.Right.Kind != ExprReference || binary.Right.Parameter == nil || *binary.Right.Parameter != 1 {
		return fmt.Errorf("record equality operands must reference the two parameters in declared order")
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
