package coreir

import (
	"fmt"
	"reflect"
)

// Block is structured control flow. Only a function body carries ExprBlock;
// nested statement blocks share that function's return target.
type Block struct {
	Statements []Statement `json:"statements"`
}
type Statement struct {
	Mutable bool       `json:"mutable,omitempty"`
	Target  *int       `json:"target,omitempty"`
	Kind    string     `json:"kind"`
	Value   *Expr      `json:"value,omitempty"`
	Local   *Parameter `json:"local,omitempty"`
	Then    *Block     `json:"then,omitempty"`
	Else    *Block     `json:"else,omitempty"`
}

// BlockChildren exposes values in lexical source order to dependency/admission
// walks. Structural validation runs before these recursive traversals.
func BlockChildren(b *Block) []*Expr {
	var out []*Expr
	if b == nil {
		return out
	}
	for _, s := range b.Statements {
		if s.Value != nil {
			out = append(out, s.Value)
		}
		out = append(out, BlockChildren(s.Then)...)
		out = append(out, BlockChildren(s.Else)...)
	}
	return out
}

func validateBlockFunction(f Function) error {
	extra := f.Body
	extra.Kind = ""
	extra.Type = Type{}
	extra.Block = nil
	if !reflect.DeepEqual(extra, Expr{}) {
		return fmt.Errorf("block body has extraneous expression payload")
	}
	if f.Body.Block == nil {
		return fmt.Errorf("block body is missing")
	}
	if !TypeEqual(f.Body.Type, f.ReturnType) {
		return fmt.Errorf("block result differs from function return type")
	}
	if err := validateType(f.ReturnType); err != nil {
		return err
	}
	names := map[string]bool{}
	for i, p := range f.Parameters {
		if p.Position != i || p.Name == "" || names[p.Name] {
			return fmt.Errorf("invalid block parameter binding")
		}
		names[p.Name] = true
		if err := validateType(p.Type); err != nil {
			return err
		}
	}
	state := make([]blockAssignmentState, len(f.Parameters))
	for i := range state {
		state[i] = blockAssignmentState{definite: true, possible: true}
	}
	returned, err := validateBlock(f.Body.Block, f.Parameters, state, f.ReturnType, map[*Block]bool{})
	if err != nil {
		return err
	}
	if !returned {
		return fmt.Errorf("block function can fall through without returning")
	}
	return nil
}

// Block assignment analysis is deliberately independent of source analysis.
type blockAssignmentState struct{ mutable, definite, possible bool }

func validateBlock(b *Block, parameters []Parameter, outer []blockAssignmentState, result Type, active map[*Block]bool) (bool, error) {
	if b == nil || active[b] {
		return false, fmt.Errorf("missing or cyclic block")
	}
	active[b] = true
	defer delete(active, b)
	scope := append([]Parameter{}, parameters...)
	state := append([]blockAssignmentState{}, outer...)
	returned := false
	value := func(e *Expr) error {
		if e == nil {
			return fmt.Errorf("missing block value")
		}
		if err := validateBlockValue(*e, scope); err != nil {
			return err
		}
		var uninitialized bool
		WalkExpression(*e, func(x Expr) bool {
			if x.Kind == ExprReference && x.Parameter != nil && *x.Parameter >= 0 && *x.Parameter < len(state) && !state[*x.Parameter].definite {
				uninitialized = true
			}
			return true
		})
		if uninitialized {
			return fmt.Errorf("block local read before definite assignment")
		}
		return nil
	}
	for _, s := range b.Statements {
		if returned {
			return false, fmt.Errorf("unreachable block statement")
		}
		if s.Kind != "local" && s.Mutable || s.Kind != "assign" && s.Target != nil {
			return false, fmt.Errorf("extraneous assignment payload")
		}
		switch s.Kind {
		case "local":
			if s.Local == nil || s.Then != nil || s.Else != nil {
				return false, fmt.Errorf("malformed local statement")
			}
			l := s.Local
			if l.Name == "" || l.Position != len(scope) {
				return false, fmt.Errorf("noncanonical block local binding")
			}
			for _, p := range scope {
				if p.Name == l.Name {
					return false, fmt.Errorf("block local shadows existing binding")
				}
			}
			if err := validateType(l.Type); err != nil {
				return false, err
			}
			if s.Value != nil {
				if err := value(s.Value); err != nil {
					return false, err
				}
				if !TypeEqual(l.Type, s.Value.Type) {
					return false, fmt.Errorf("block local initializer type mismatch")
				}
			}
			scope = append(scope, *l)
			state = append(state, blockAssignmentState{mutable: s.Mutable, definite: s.Value != nil, possible: s.Value != nil})
		case "assign":
			if s.Target == nil || s.Local != nil || s.Then != nil || s.Else != nil {
				return false, fmt.Errorf("malformed assignment statement")
			}
			p := *s.Target
			if p < 0 || p >= len(scope) {
				return false, fmt.Errorf("assignment target outside lexical scope")
			}
			if !state[p].mutable && state[p].possible {
				return false, fmt.Errorf("immutable binding may already be assigned")
			}
			if err := value(s.Value); err != nil {
				return false, err
			}
			if !TypeEqual(scope[p].Type, s.Value.Type) {
				return false, fmt.Errorf("assignment type mismatch")
			}
			state[p].definite, state[p].possible = true, true
		case "return":
			if s.Local != nil || s.Then != nil || s.Else != nil {
				return false, fmt.Errorf("malformed return statement")
			}
			if err := value(s.Value); err != nil {
				return false, err
			}
			if !TypeEqual(result, s.Value.Type) {
				return false, fmt.Errorf("block return type mismatch")
			}
			returned = true
		case "if":
			if s.Then == nil || s.Local != nil {
				return false, fmt.Errorf("malformed if statement")
			}
			if err := value(s.Value); err != nil {
				return false, err
			}
			if !TypeEqual(s.Value.Type, Type{Kind: TypePrimitive, Primitive: PrimitiveBool}) {
				return false, fmt.Errorf("if condition must be bool")
			}
			left, right := append([]blockAssignmentState{}, state...), append([]blockAssignmentState{}, state...)
			a, err := validateBlock(s.Then, scope, left, result, active)
			if err != nil {
				return false, err
			}
			z := false
			if s.Else != nil {
				z, err = validateBlock(s.Else, scope, right, result, active)
				if err != nil {
					return false, err
				}
			}
			for i := range state {
				switch {
				case a && z:
				case a:
					state[i] = right[i]
				case z:
					state[i] = left[i]
				default:
					state[i].definite, state[i].possible = left[i].definite && right[i].definite, left[i].possible || right[i].possible
				}
			}
			returned = a && z
		case "block":
			if s.Then == nil || s.Value != nil || s.Local != nil || s.Else != nil {
				return false, fmt.Errorf("malformed nested block statement")
			}
			var err error
			returned, err = validateBlock(s.Then, scope, state, result, active)
			if err != nil {
				return false, err
			}
		default:
			return false, fmt.Errorf("unknown statement kind %q", s.Kind)
		}
	}
	copy(outer, state[:len(outer)])
	return returned, nil
}

// Called only after structural validation has rejected cyclic blocks.
func blockUsesAssignment(b *Block) bool {
	if b == nil {
		return false
	}
	for _, s := range b.Statements {
		if s.Mutable || s.Kind == "assign" || s.Kind == "local" && s.Value == nil || blockUsesAssignment(s.Then) || blockUsesAssignment(s.Else) {
			return true
		}
	}
	return false
}

func validateBlockValue(e Expr, scope []Parameter) error {
	if err := validateExpr(e, scope); err != nil {
		return err
	}
	var invalid bool
	WalkExpression(e, func(x Expr) bool {
		if x.Kind == ExprImmutableLocal || x.Kind == ExprPropagate || x.Kind == ExprConditional && x.Conditional != nil && x.Conditional.TerminalStatement {
			invalid = true
			return false
		}
		return true
	})
	if invalid {
		return fmt.Errorf("statement or propagation in block value expression")
	}
	return nil
}

// BlockLocalTypes includes declaration-only types, which have no initializer
// expression to expose them to ordinary expression walks. Validate first.
func BlockLocalTypes(b *Block) []Type {
	var out []Type
	if b == nil {
		return out
	}
	for _, s := range b.Statements {
		if s.Local != nil {
			out = append(out, s.Local.Type)
		}
		out = append(out, BlockLocalTypes(s.Then)...)
		out = append(out, BlockLocalTypes(s.Else)...)
	}
	return out
}
