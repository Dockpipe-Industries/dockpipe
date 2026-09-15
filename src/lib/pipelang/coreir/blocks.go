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
	Kind  string     `json:"kind"`
	Value *Expr      `json:"value,omitempty"`
	Local *Parameter `json:"local,omitempty"`
	Then  *Block     `json:"then,omitempty"`
	Else  *Block     `json:"else,omitempty"`
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
	returned, err := validateBlock(f.Body.Block, f.Parameters, f.ReturnType, map[*Block]bool{})
	if err != nil {
		return err
	}
	if !returned {
		return fmt.Errorf("block function can fall through without returning")
	}
	return nil
}

func validateBlock(b *Block, parameters []Parameter, result Type, active map[*Block]bool) (bool, error) {
	if b == nil || active[b] {
		return false, fmt.Errorf("missing or cyclic block")
	}
	active[b] = true
	defer delete(active, b)
	scope := append([]Parameter{}, parameters...)
	returned := false
	for _, s := range b.Statements {
		if returned {
			return false, fmt.Errorf("unreachable block statement")
		}
		switch s.Kind {
		case "local":
			if s.Local == nil || s.Value == nil || s.Then != nil || s.Else != nil {
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
			if err := validateBlockValue(*s.Value, scope); err != nil {
				return false, err
			}
			if !TypeEqual(l.Type, s.Value.Type) {
				return false, fmt.Errorf("block local initializer type mismatch")
			}
			scope = append(scope, *l)
		case "return":
			if s.Value == nil || s.Local != nil || s.Then != nil || s.Else != nil {
				return false, fmt.Errorf("malformed return statement")
			}
			if err := validateBlockValue(*s.Value, scope); err != nil {
				return false, err
			}
			if !TypeEqual(result, s.Value.Type) {
				return false, fmt.Errorf("block return type mismatch")
			}
			returned = true
		case "if":
			if s.Value == nil || s.Then == nil || s.Local != nil {
				return false, fmt.Errorf("malformed if statement")
			}
			if err := validateBlockValue(*s.Value, scope); err != nil {
				return false, err
			}
			if !TypeEqual(s.Value.Type, Type{Kind: TypePrimitive, Primitive: PrimitiveBool}) {
				return false, fmt.Errorf("if condition must be bool")
			}
			a, err := validateBlock(s.Then, scope, result, active)
			if err != nil {
				return false, err
			}
			z := false
			if s.Else != nil {
				z, err = validateBlock(s.Else, scope, result, active)
				if err != nil {
					return false, err
				}
			}
			returned = a && z
		case "block":
			if s.Then == nil || s.Value != nil || s.Local != nil || s.Else != nil {
				return false, fmt.Errorf("malformed nested block statement")
			}
			var err error
			returned, err = validateBlock(s.Then, scope, result, active)
			if err != nil {
				return false, err
			}
		default:
			return false, fmt.Errorf("unknown statement kind %q", s.Kind)
		}
	}
	return returned, nil
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
