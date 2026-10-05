package coreeval

import (
	"fmt"
	"reflect"

	"dockpipe/src/lib/pipelang/coreir"
)

// PreparedProgram owns a validated snapshot for repeated conformance evaluation.
// It may be shared by concurrent callers. PrepareProgram must not race with
// mutation of its input; after preparation callers may freely change that input.
// No Core nodes or returned values alias the retained snapshot.
type PreparedProgram struct {
	functions map[string]coreir.Function
}

// PrepareProgram copies the complete Core graph, validates the owned copy, and
// builds lookups once. It never trusts a cache keyed by caller-owned identity.
func PrepareProgram(program coreir.Program) (*PreparedProgram, error) {
	owned, err := snapshot(program)
	if err != nil {
		return nil, err
	}
	if err := coreir.ValidateProgram(owned); err != nil {
		return nil, err
	}
	functions := make(map[string]coreir.Function, len(owned.Functions))
	for _, function := range owned.Functions {
		functions[function.Identity.PackageID+"\x00"+function.Identity.Path] = function
	}
	return &PreparedProgram{functions: functions}, nil
}

// Evaluate validates argument types and complete carrier values on every call.
// Returned type metadata is copied too, so caller mutation cannot corrupt later
// evaluations. The zero value (or a nil receiver) is not a prepared program.
func (program *PreparedProgram) Evaluate(identity coreir.SemanticIdentity, arguments []Value) (Outcome, error) {
	if program == nil || program.functions == nil {
		return Outcome{}, fmt.Errorf("program is not prepared")
	}
	outcome, err := evaluateWithFunctions(program.functions, identity, arguments)
	owned, copyErr := snapshot(outcome)
	if copyErr != nil {
		return Outcome{}, copyErr
	}
	return owned, err
}

// Core and evaluator values are data-only graphs. Copy every field, including
// inactive fields and type/semantic metadata, rather than maintaining a second
// expression-kind allowlist that could silently omit a newly added field.
// Unsupported future field kinds fail closed. Active cycles fail preparation;
// shared acyclic subtrees are copied independently.
func snapshot[T any](value T) (T, error) {
	var zero T
	copied, err := copySnapshot(reflect.ValueOf(value), make(map[any]bool))
	if err != nil {
		return zero, err
	}
	return copied.Interface().(T), nil
}

func copySnapshot(value reflect.Value, active map[any]bool) (reflect.Value, error) {
	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type()), nil
		}
		key := value.Interface()
		if active[key] {
			return reflect.Value{}, fmt.Errorf("cyclic Core/value graph cannot be prepared")
		}
		active[key] = true
		child, err := copySnapshot(value.Elem(), active)
		delete(active, key)
		if err != nil {
			return reflect.Value{}, err
		}
		result := reflect.New(value.Type().Elem())
		result.Elem().Set(child)
		return result, nil
	case reflect.Struct:
		result := reflect.New(value.Type()).Elem()
		for i := 0; i < value.NumField(); i++ {
			if !result.Field(i).CanSet() {
				return reflect.Value{}, fmt.Errorf("unexported snapshot field %s", value.Type().Field(i).Name)
			}
			field, err := copySnapshot(value.Field(i), active)
			if err != nil {
				return reflect.Value{}, err
			}
			result.Field(i).Set(field)
		}
		return result, nil
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type()), nil
		}
		key := struct {
			Type    reflect.Type
			Pointer uintptr
			Length  int
		}{value.Type(), value.Pointer(), value.Len()}
		if active[key] {
			return reflect.Value{}, fmt.Errorf("cyclic Core/value graph cannot be prepared")
		}
		active[key] = true
		defer delete(active, key)
		result := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			item, err := copySnapshot(value.Index(i), active)
			if err != nil {
				return reflect.Value{}, err
			}
			result.Index(i).Set(item)
		}
		return result, nil
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int64, reflect.Float64:
		return value, nil
	default:
		return reflect.Value{}, fmt.Errorf("unsupported snapshot field kind %s", value.Kind())
	}
}
