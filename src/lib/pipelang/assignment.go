package pipelang

import "fmt"

// Definite initialization permits reads; possible initialization forbids a
// second write to an immutable binding. These facts join differently.
type localAssignmentState struct {
	typ                         ResolvedTypeRef
	mutable, definite, possible bool
}

func copyAssignmentState(in map[string]localAssignmentState) map[string]localAssignmentState {
	out := make(map[string]localAssignmentState, len(in))
	for name, value := range in {
		out[name] = value
	}
	return out
}

func (cp *checkedProgram) inferAssignedValue(e Expr, state map[string]localAssignmentState, want ResolvedTypeRef) (ResolvedTypeRef, error) {
	var check func(Expr) error
	check = func(e Expr) error {
		if id, ok := e.(*IdentExpr); ok {
			if local, exists := state[id.Name]; exists && !local.definite {
				return cp.blockError(id.Span, fmt.Sprintf("local %q read before definite assignment", id.Name))
			}
		}
		for _, child := range expressionChildren(e) {
			if err := check(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := check(e); err != nil {
		return ResolvedTypeRef{}, err
	}
	types := make(map[string]ResolvedTypeRef, len(state))
	for name, local := range state {
		types[name] = local.typ
	}
	return cp.inferBlockValue(e, types, want)
}

// Only facts for incoming bindings leave a lexical block. Each branch receives
// an independent state, including when the other branch returns.
func (cp *checkedProgram) checkAssignmentBlock(b *BlockExpr, outer map[string]localAssignmentState, result ResolvedTypeRef) (bool, error) {
	state := copyAssignmentState(outer)
	returned := false
	for _, s := range b.Statements {
		if returned {
			return false, cp.blockError(s.Span, "unreachable statement after unconditional return")
		}
		switch s.Kind {
		case "local":
			if _, exists := state[s.Name]; exists {
				return false, cp.blockError(s.NameSpan, "local shadows an existing binding")
			}
			typ, err := cp.resolveType(s.Type)
			if err != nil {
				return false, err
			}
			if !cp.assignmentValueType(typ) {
				return false, cp.blockError(s.Type.Span, "local type is outside the executable value contract")
			}
			local := localAssignmentState{typ: typ, mutable: s.Mutable}
			// Make self references visible but uninitialized while checking the RHS.
			state[s.Name] = local
			if s.Value != nil {
				got, err := cp.inferAssignedValue(s.Value, state, typ)
				if err != nil {
					return false, err
				}
				if !got.Equal(typ) {
					return false, cp.blockError(s.Span, "local initializer type mismatch")
				}
				local.definite, local.possible = true, true
			}
			state[s.Name] = local
		case "assign":
			local, exists := state[s.Name]
			if !exists {
				return false, cp.blockError(s.NameSpan, "assignment target is not an in-scope local")
			}
			if !local.mutable && local.possible {
				return false, cp.blockError(s.NameSpan, "immutable binding may already be assigned")
			}
			got, err := cp.inferAssignedValue(s.Value, state, local.typ)
			if err != nil {
				return false, err
			}
			if !got.Equal(local.typ) {
				return false, cp.blockError(s.Span, "assignment type mismatch")
			}
			local.definite, local.possible = true, true
			state[s.Name] = local
		case "return":
			got, err := cp.inferAssignedValue(s.Value, state, result)
			if err != nil {
				return false, err
			}
			if !got.Equal(result) {
				return false, cp.blockError(s.Span, "return type mismatch")
			}
			returned = true
		case "if":
			got, err := cp.inferAssignedValue(s.Value, state, resolvedPrimitive(TypeBool))
			if err != nil {
				return false, err
			}
			if !got.Equal(resolvedPrimitive(TypeBool)) {
				return false, cp.blockError(s.Span, "if condition must be bool")
			}
			left, right := copyAssignmentState(state), copyAssignmentState(state)
			a, err := cp.checkAssignmentBlock(s.Then, left, result)
			if err != nil {
				return false, err
			}
			z := false
			if s.Else != nil {
				z, err = cp.checkAssignmentBlock(s.Else, right, result)
				if err != nil {
					return false, err
				}
			}
			for name, local := range state {
				x, y := left[name], right[name]
				switch {
				case a && z: // Neither branch reaches the continuation.
				case a:
					local = y
				case z:
					local = x
				default:
					local.definite, local.possible = x.definite && y.definite, x.possible || y.possible
				}
				state[name] = local
			}
			returned = a && z
		case "block":
			var err error
			returned, err = cp.checkAssignmentBlock(s.Then, state, result)
			if err != nil {
				return false, err
			}
		default:
			return false, cp.blockError(s.Span, "invalid block statement")
		}
	}
	for name := range outer {
		outer[name] = state[name]
	}
	return returned, nil
}

func (cp *checkedProgram) assignmentValueType(t ResolvedTypeRef) bool {
	if t.IsPrimitive() || cp.isResolvedRecordType(t) || cp.isResolvedEnumType(t) {
		return true
	}
	if isResolvedRecordList(t) {
		return cp.isResolvedRecordType(t.Arguments[0])
	}
	if cp.isResolvedOptionalValue(cp.modules.LanguageContract(), t) {
		return true
	}
	return isResolvedSourceArithmeticResult(cp.modules.LanguageContract(), t) || isResolvedBoundedValueResult(cp.modules.LanguageContract(), t)
}
