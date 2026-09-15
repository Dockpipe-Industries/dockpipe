package pipelang

import (
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/hir"
)

func lowerGeneralBlockToHIR(a *Analysis, f SemanticIdentity, b *BlockExpr, bindings map[string]hir.Binding, types map[string]ResolvedTypeRef, result ResolvedTypeRef) (hir.Expr, error) {
	body, err := lowerBlockStatementsToHIR(a, f, b, bindings, types, result)
	return hir.Expr{Kind: hir.ExprBlock, Type: toHIRType(a, result), Span: toHIRSpan(b.Span), Block: body}, err
}
func lowerBlockStatementsToHIR(a *Analysis, f SemanticIdentity, b *BlockExpr, outer map[string]hir.Binding, outerTypes map[string]ResolvedTypeRef, result ResolvedTypeRef) (*hir.Block, error) {
	bindings := make(map[string]hir.Binding, len(outer))
	for n, v := range outer {
		bindings[n] = v
	}
	types := make(map[string]ResolvedTypeRef, len(outerTypes))
	for n, v := range outerTypes {
		types[n] = v
	}
	body := &hir.Block{Statements: make([]hir.Statement, 0, len(b.Statements))}
	for _, s := range b.Statements {
		t := hir.Statement{Kind: s.Kind, Span: toHIRSpan(s.Span)}
		if s.Value != nil {
			v, err := lowerExprToHIR(a, f, s.Value, bindings, types)
			if err != nil {
				return nil, err
			}
			t.Value = &v
		}
		if s.Kind == "local" {
			v, err := a.checked.resolveType(s.Type)
			if err != nil {
				return nil, err
			}
			binding := hir.Binding{Kind: hir.BindingLocal, Function: toHIRSemanticIdentity(f), Position: len(bindings), Name: s.Name}
			t.Local = &hir.Parameter{Binding: binding, Type: toHIRType(a, v), TypeSpan: toHIRSpan(s.Type.Span), Span: toHIRSpan(s.NameSpan)}
			bindings[s.Name] = binding
			types[s.Name] = v
		}
		var err error
		if s.Then != nil {
			t.Then, err = lowerBlockStatementsToHIR(a, f, s.Then, bindings, types, result)
			if err != nil {
				return nil, err
			}
		}
		if s.Else != nil {
			t.Else, err = lowerBlockStatementsToHIR(a, f, s.Else, bindings, types, result)
			if err != nil {
				return nil, err
			}
		}
		body.Statements = append(body.Statements, t)
	}
	return body, nil
}

func generalHIRBlockToCore(e hir.Expr, parameters []hir.Parameter) (coreir.Expr, error) {
	if e.Block == nil {
		return coreir.Expr{}, coreLoweringError(e.Span, "typed HIR block is missing")
	}
	b, err := hirBlockStatementsToCore(e.Block, parameters)
	return coreir.Expr{Kind: coreir.ExprBlock, Type: hirTypeToCore(e.Type), Block: b}, err
}
func hirBlockStatementsToCore(b *hir.Block, parameters []hir.Parameter) (*coreir.Block, error) {
	scope := append([]hir.Parameter{}, parameters...)
	result := &coreir.Block{Statements: make([]coreir.Statement, 0, len(b.Statements))}
	for _, s := range b.Statements {
		t := coreir.Statement{Kind: s.Kind}
		if s.Value != nil {
			v, err := hirExprToCore(*s.Value, scope)
			if err != nil {
				return nil, err
			}
			t.Value = &v
		}
		if s.Local != nil {
			l := s.Local
			if l.Binding.Kind != hir.BindingLocal || l.Binding.Position != len(scope) || l.Binding.Name == "" {
				return nil, coreLoweringError(s.Span, "typed HIR block local is not canonically bound")
			}
			t.Local = &coreir.Parameter{Position: l.Binding.Position, Name: l.Binding.Name, Type: hirTypeToCore(l.Type)}
			scope = append(scope, *l)
		}
		var err error
		if s.Then != nil {
			t.Then, err = hirBlockStatementsToCore(s.Then, scope)
			if err != nil {
				return nil, err
			}
		}
		if s.Else != nil {
			t.Else, err = hirBlockStatementsToCore(s.Else, scope)
			if err != nil {
				return nil, err
			}
		}
		result.Statements = append(result.Statements, t)
	}
	return result, nil
}
