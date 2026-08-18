// Package coreeval is PipeLang's target-independent Core IR conformance
// evaluator. It is offline and inert and imports no parser, HIR, or backend.
package coreeval

import (
	"fmt"

	"dockpipe/src/lib/pipelang/coreir"
)

type Value struct {
	Type     coreir.Type
	String   string
	Int      int64
	Float    float64
	Bool     bool
	Result   *Outcome
	Record   []Value
	Optional *OptionalValue
}

type OptionalValue struct {
	Present bool
	Value   *Value
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
		if expression.Type.Kind == coreir.TypeRecord {
			return Outcome{OK: true, Value: cloneRecordValue(arguments[*expression.Parameter])}, nil
		}
		if expression.Type.Kind == coreir.TypeOptional {
			return Outcome{OK: true, Value: cloneOptionalValue(arguments[*expression.Parameter])}, nil
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
		if expression.Binary.Left.Type.Kind == coreir.TypePrimitive && expression.Binary.Left.Type.Primitive == coreir.PrimitiveString {
			return evalTextBinary(expression, left.Value.String, right.Value.String)
		}
		if expression.Binary.Left.Type.Kind == coreir.TypeRecord {
			equal, err := equalValues(left.Value, right.Value)
			if err != nil {
				return Outcome{}, err
			}
			if expression.Binary.Operator == coreir.OperatorNotEqual {
				equal = !equal
			}
			return Outcome{OK: true, Value: Value{Type: expression.Type, Bool: equal}}, nil
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
	case coreir.ExprFieldProjection:
		receiver, err := evalExpr(*expression.Field.Receiver, arguments)
		if err != nil || !receiver.OK {
			return receiver, err
		}
		position := expression.Field.Position
		if position < 0 || position >= len(receiver.Value.Record) {
			return Outcome{}, fmt.Errorf("record field projection position is outside the value")
		}
		return Outcome{OK: true, Value: receiver.Value.Record[position]}, nil
	case coreir.ExprRecordConstruct:
		fields := make([]Value, 0, len(expression.Record.Fields))
		for position, initialized := range expression.Record.Fields {
			value, err := evalExpr(*initialized.Value, arguments)
			if err != nil {
				return Outcome{}, fmt.Errorf("record construction field %d: %w", position, err)
			}
			if !value.OK {
				return value, nil
			}
			fields = append(fields, value.Value)
		}
		value := Value{Type: expression.Type, Record: fields}
		if err := validateValue(value); err != nil {
			return Outcome{}, fmt.Errorf("record construction: %w", err)
		}
		return Outcome{OK: true, Value: cloneRecordValue(value)}, nil
	case coreir.ExprOptionalSome:
		value, err := evalExpr(*expression.Some.Value, arguments)
		if err != nil || !value.OK {
			return value, err
		}
		payload := value.Value
		optional := Value{Type: expression.Type, Optional: &OptionalValue{Present: true, Value: &payload}}
		if err := validateValue(optional); err != nil {
			return Outcome{}, fmt.Errorf("optional some: %w", err)
		}
		return Outcome{OK: true, Value: cloneOptionalValue(optional)}, nil
	case coreir.ExprOptionalNone:
		optional := Value{Type: expression.Type, Optional: &OptionalValue{}}
		if err := validateValue(optional); err != nil {
			return Outcome{}, fmt.Errorf("optional none: %w", err)
		}
		return Outcome{OK: true, Value: optional}, nil
	case coreir.ExprOptionalHasValue:
		value, err := evalExpr(*expression.HasValue.Value, arguments)
		if err != nil || !value.OK {
			return value, err
		}
		if err := validateValue(value.Value); err != nil {
			return Outcome{}, fmt.Errorf("optional has_value: %w", err)
		}
		return Outcome{OK: true, Value: Value{Type: expression.Type, Bool: value.Value.Optional.Present}}, nil
	default:
		return Outcome{}, fmt.Errorf("unsupported expression kind %q", expression.Kind)
	}
}

func evalTextBinary(expression coreir.Expr, left, right string) (Outcome, error) {
	operator := expression.Binary.Operator
	if operator == coreir.OperatorAdd {
		if err := coreir.ValidateText(left); err != nil {
			return Outcome{}, err
		}
		if err := coreir.ValidateText(right); err != nil {
			return Outcome{}, err
		}
		return Outcome{OK: true, Value: Value{Type: expression.Type, String: left + right}}, nil
	}
	comparison, err := coreir.CompareOrdinalText(left, right)
	if err != nil {
		return Outcome{}, err
	}
	var value bool
	switch operator {
	case coreir.OperatorEqual:
		value = comparison == 0
	case coreir.OperatorNotEqual:
		value = comparison != 0
	case coreir.OperatorLessThan:
		value = comparison < 0
	case coreir.OperatorLessOrEqual:
		value = comparison <= 0
	case coreir.OperatorGreaterThan:
		value = comparison > 0
	case coreir.OperatorGreaterOrEqual:
		value = comparison >= 0
	default:
		return Outcome{}, fmt.Errorf("unsupported text operator %q", operator)
	}
	return Outcome{OK: true, Value: Value{Type: expression.Type, Bool: value}}, nil
}

func validateValue(value Value) error {
	if value.Type.Kind == coreir.TypeOptional {
		if value.Type.Optional == nil || value.Optional == nil || value.Result != nil || len(value.Record) != 0 {
			return fmt.Errorf("Optional value does not match its type")
		}
		if !value.Optional.Present {
			if value.Optional.Value != nil {
				return fmt.Errorf("absent Optional carries a value")
			}
			return nil
		}
		if value.Optional.Value == nil || !coreir.TypeEqual(value.Optional.Value.Type, value.Type.Optional.Value) {
			return fmt.Errorf("present Optional value type does not match its payload type")
		}
		return validateValue(*value.Optional.Value)
	}
	if value.Type.Kind == coreir.TypeRecord {
		if value.Type.Record == nil || value.Result != nil || len(value.Record) != len(value.Type.Record.Fields) {
			return fmt.Errorf("record value does not match its field schema")
		}
		for index, field := range value.Type.Record.Fields {
			if !coreir.TypeEqual(value.Record[index].Type, field.Type) {
				return fmt.Errorf("record field %d type does not match its declaration", index)
			}
			if err := validateValue(value.Record[index]); err != nil {
				return fmt.Errorf("record field %d: %w", index, err)
			}
		}
		return nil
	}
	if value.Type.Kind != coreir.TypeResult {
		if value.Result != nil {
			return fmt.Errorf("non-Result value carries a Result outcome")
		}
		if len(value.Record) != 0 {
			return fmt.Errorf("non-record value carries record fields")
		}
		if value.Optional != nil {
			return fmt.Errorf("non-Optional value carries an Optional payload")
		}
		if value.Type.Kind == coreir.TypePrimitive && value.Type.Primitive == coreir.PrimitiveString {
			return coreir.ValidateText(value.String)
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

func cloneOptionalValue(value Value) Value {
	cloned := value
	if value.Optional == nil {
		return cloned
	}
	cloned.Optional = &OptionalValue{Present: value.Optional.Present}
	if value.Optional.Value != nil {
		payload := *value.Optional.Value
		cloned.Optional.Value = &payload
	}
	return cloned
}

func equalValues(left, right Value) (bool, error) {
	if !coreir.TypeEqual(left.Type, right.Type) {
		return false, fmt.Errorf("structural equality operands have different types")
	}
	if err := validateValue(left); err != nil {
		return false, fmt.Errorf("left structural equality operand: %w", err)
	}
	if err := validateValue(right); err != nil {
		return false, fmt.Errorf("right structural equality operand: %w", err)
	}
	switch left.Type.Kind {
	case coreir.TypeRecord:
		for index := range left.Record {
			equal, err := equalValues(left.Record[index], right.Record[index])
			if err != nil {
				return false, fmt.Errorf("record field %d: %w", index, err)
			}
			if !equal {
				return false, nil
			}
		}
		return true, nil
	case coreir.TypePrimitive:
		switch left.Type.Primitive {
		case coreir.PrimitiveString:
			return left.String == right.String, nil
		case coreir.PrimitiveBool:
			return left.Bool == right.Bool, nil
		}
	case coreir.TypeNumeric:
		if left.Type.Numeric == nil {
			return false, fmt.Errorf("numeric equality operand has no representation")
		}
		switch left.Type.Numeric.Representation {
		case coreir.NumericInteger:
			return left.Int == right.Int, nil
		case coreir.NumericBinaryFloat:
			return left.Float == right.Float, nil
		}
	}
	return false, fmt.Errorf("type %q is outside structural record equality", left.Type.Kind)
}

func cloneRecordValue(value Value) Value {
	cloned := value
	cloned.Record = make([]Value, len(value.Record))
	for index, field := range value.Record {
		if field.Type.Kind == coreir.TypeRecord {
			cloned.Record[index] = cloneRecordValue(field)
		} else {
			cloned.Record[index] = field
		}
	}
	return cloned
}

func arithmeticOutcome(resultType coreir.Type, value Value, arithmeticError coreir.ArithmeticError) Outcome {
	value.Type = resultType.Result.Success
	if arithmeticError != "" {
		return Outcome{OK: false, Value: value, Error: arithmeticError}
	}
	return Outcome{OK: true, Value: value}
}
