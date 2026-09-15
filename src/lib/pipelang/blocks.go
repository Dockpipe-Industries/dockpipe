package pipelang

import "fmt"

// BlockExpr is a callable body, never a value expression. Nested blocks are
// statements and share the enclosing callable's return target.
type BlockExpr struct {
	Statements []BlockStatement
	Span       Span
}
type BlockStatement struct {
	Kind       string
	Type       UnresolvedTypeRef
	Name       string
	NameSpan   Span
	Value      Expr
	Then, Else *BlockExpr
	Span       Span
}

func (*BlockExpr) isExpr()            {}
func (b *BlockExpr) SourceSpan() Span { return b.Span }

func blockExpressionChildren(b *BlockExpr) []Expr {
	var result []Expr
	for _, s := range b.Statements {
		if s.Value != nil {
			result = append(result, s.Value)
		}
		if s.Then != nil {
			result = append(result, s.Then)
		}
		if s.Else != nil {
			result = append(result, s.Else)
		}
	}
	return result
}

func (p *parser) parseGeneralBlock() (*BlockExpr, error) {
	start, err := p.expect(tokLBrace)
	if err != nil {
		return nil, err
	}
	b := &BlockExpr{}
	for p.peek().kind != tokRBrace {
		first := p.peek()
		s := BlockStatement{Span: first.span}
		switch {
		case first.kind == tokLBrace:
			s.Kind = "block"
			s.Then, err = p.parseGeneralBlock()
		case first.kind == tokIdent && first.lit == "if":
			p.next()
			s.Kind = "if"
			if _, err = p.expect(tokLParen); err != nil {
				return nil, err
			}
			s.Value, err = p.parseExpr(1)
			if err != nil {
				return nil, err
			}
			if _, err = p.expect(tokRParen); err != nil {
				return nil, err
			}
			s.Then, err = p.parseGeneralBlock()
			if err != nil {
				return nil, err
			}
			if p.peek().kind == tokIdent && p.peek().lit == "else" {
				p.next()
				s.Else, err = p.parseGeneralBlock()
			}
		case first.kind == tokIdent && first.lit == "return":
			p.next()
			s.Kind = "return"
			s.Value, err = p.parseExpr(1)
			if err == nil {
				var end token
				end, err = p.expect(tokSemi)
				s.Span = mergeSpans(first.span, end.span)
			}
		default:
			s.Kind = "local"
			s.Type, err = p.parseTypeRef()
			if err != nil {
				return nil, err
			}
			var name token
			name, err = p.expect(tokIdent)
			if err != nil {
				return nil, err
			}
			s.Name, s.NameSpan = name.lit, name.span
			if _, err = p.expect(tokAssign); err != nil {
				return nil, err
			}
			s.Value, err = p.parseExpr(1)
			if err == nil {
				var end token
				end, err = p.expect(tokSemi)
				s.Span = mergeSpans(first.span, end.span)
			}
		}
		if err != nil {
			return nil, err
		}
		if s.Then != nil {
			s.Span = mergeSpans(first.span, s.Then.Span)
		}
		if s.Else != nil {
			s.Span = mergeSpans(first.span, s.Else.Span)
		}
		b.Statements = append(b.Statements, s)
	}
	end, err := p.expect(tokRBrace)
	if err != nil {
		return nil, err
	}
	b.Span = mergeSpans(start.span, end.span)
	return b, nil
}

func (cp *checkedProgram) blockError(span Span, message string) error {
	return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, span, message)
}
func (cp *checkedProgram) validateGeneralBlockMethod(m MethodDecl, b *BlockExpr) error {
	if cp.modules.LanguageContract() != PipeLangLanguageContractV1140 {
		return cp.blockError(b.Span, "general blocks require v0.114.0")
	}
	if normalizeVisibility(m.Visibility) != VisibilityPublic {
		return cp.blockError(m.Span, "general blocks require public pure methods")
	}
	declared, err := cp.resolveType(m.ReturnType)
	if err != nil {
		return err
	}
	env := map[string]ResolvedTypeRef{}
	for _, p := range m.Params {
		t, e := cp.resolveType(p.Type)
		if e != nil {
			return e
		}
		env[p.Name] = t
	}
	returned, err := cp.checkGeneralBlock(b, env, declared)
	if err != nil {
		return err
	}
	if !returned {
		return cp.blockError(b.Span, "method may reach its end without returning")
	}
	return nil
}

func (cp *checkedProgram) checkGeneralBlock(b *BlockExpr, outer map[string]ResolvedTypeRef, result ResolvedTypeRef) (bool, error) {
	env := make(map[string]ResolvedTypeRef, len(outer))
	for n, t := range outer {
		env[n] = t
	}
	returned := false
	for _, s := range b.Statements {
		if returned {
			return false, cp.blockError(s.Span, "unreachable statement after unconditional return")
		}
		switch s.Kind {
		case "local":
			if _, ok := env[s.Name]; ok {
				return false, cp.blockError(s.NameSpan, fmt.Sprintf("immutable local %q shadows an existing binding", s.Name))
			}
			t, err := cp.resolveType(s.Type)
			if err != nil {
				return false, err
			}
			got, err := cp.inferBlockValue(s.Value, env, t)
			if err != nil {
				return false, err
			}
			if !got.Equal(t) {
				return false, cp.blockError(s.Value.SourceSpan(), fmt.Sprintf("local initializer has type %s, expected %s", got, t))
			}
			env[s.Name] = t
		case "return":
			got, err := cp.inferBlockValue(s.Value, env, result)
			if err != nil {
				return false, err
			}
			if !got.Equal(result) {
				return false, cp.blockError(s.Value.SourceSpan(), fmt.Sprintf("return has type %s, expected %s", got, result))
			}
			returned = true
		case "if":
			got, err := cp.inferExprType(s.Value, env)
			if err != nil {
				return false, err
			}
			if !got.Equal(resolvedPrimitive(TypeBool)) {
				return false, cp.blockError(s.Value.SourceSpan(), "if condition must be bool")
			}
			a, err := cp.checkGeneralBlock(s.Then, env, result)
			if err != nil {
				return false, err
			}
			z := false
			if s.Else != nil {
				z, err = cp.checkGeneralBlock(s.Else, env, result)
				if err != nil {
					return false, err
				}
			}
			returned = a && z
		case "block":
			var err error
			returned, err = cp.checkGeneralBlock(s.Then, env, result)
			if err != nil {
				return false, err
			}
		default:
			return false, cp.blockError(s.Span, "invalid block statement")
		}
	}
	return returned, nil
}

func (cp *checkedProgram) inferBlockValue(e Expr, env map[string]ResolvedTypeRef, want ResolvedTypeRef) (ResolvedTypeRef, error) {
	// Propagation has its own accepted placements. Generalizing its implicit
	// carrier returns is P09, not the lexical block capability.
	if containsPropagationExpression(e) {
		return ResolvedTypeRef{}, cp.blockError(e.SourceSpan(), "propagation in general blocks is not admitted")
	}
	return cp.inferExprType(e, env)
}
