package pipelang

import (
	"fmt"
	"strconv"
)

type parser struct {
	sources          *SourceSet
	file             *SourceFile
	languageContract LanguageContract
	toks             []token
	idx              int
}

func Parse(src []byte) (*Program, error) {
	return ParseFile("<input>", src)
}

func ParseFile(path string, src []byte) (*Program, error) {
	sources, diagnostics := NewSourceSet([]SourceInput{{Path: path, Data: src}})
	if diagnostics.HasErrors() {
		return nil, diagnosticError(sources, diagnostics)
	}
	files := sources.Files()
	if len(files) != 1 {
		return nil, oneDiagnostic(sources, CodeInvalidProgram, CategorySource, Span{File: FileID(normalizeSourcePath(path))}, "source file is unavailable")
	}
	return parseSourceFile(sources, files[0])
}

func parseSourceFile(sources *SourceSet, file *SourceFile) (*Program, error) {
	return parseSourceFileWithLanguageContract(sources, file, LegacyLanguageContract)
}

func parseSourceFileWithLanguageContract(sources *SourceSet, file *SourceFile, languageContract LanguageContract) (*Program, error) {
	toks, err := lex(sources, file)
	if err != nil {
		return nil, err
	}
	p := &parser{sources: sources, file: file, languageContract: languageContract, toks: toks}
	return p.parseProgram()
}

func (p *parser) parseProgram() (*Program, error) {
	prog := &Program{Span: Span{File: p.file.ID, Start: 0, End: len(p.file.Text)}, sources: p.sources}
	for p.peek().kind != tokEOF {
		declStart := p.peek().span
		anns, err := p.parseAnnotations()
		if err != nil {
			return nil, err
		}
		vis, err := p.parseOptionalVisibility()
		if err != nil {
			return nil, err
		}
		switch p.peek().kind {
		case tokInterface:
			i, err := p.parseInterface(vis, anns, declStart)
			if err != nil {
				return nil, err
			}
			prog.Interfaces = append(prog.Interfaces, i)
		case tokClass, tokStruct:
			c, err := p.parseClass(vis, anns, declStart)
			if err != nil {
				return nil, err
			}
			prog.Classes = append(prog.Classes, c)
		case tokIdent:
			if p.peek().lit != "Record" || !hasPrimitiveRecordSourceContract(p.languageContract) {
				return nil, p.errf("expected Interface, Class, or Struct")
			}
			r, err := p.parseRecord(vis, anns, declStart)
			if err != nil {
				return nil, err
			}
			prog.Records = append(prog.Records, r)
		default:
			return nil, p.errf("expected Interface, Class, or Struct")
		}
	}
	return prog, nil
}

func (p *parser) parseRecord(vis Visibility, anns []Annotation, start Span) (*RecordDecl, error) {
	keyword, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if keyword.lit != "Record" {
		return nil, oneDiagnostic(p.sources, CodeUnexpectedToken, CategorySyntax, keyword.span, fmt.Sprintf("expected Record, got %q", keyword.lit))
	}
	nameTok, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	decl := &RecordDecl{Name: nameTok.lit, Visibility: normalizeVisibility(vis), Annotations: anns}
	if p.peek().kind == tokColon {
		p.next()
		implTok, err := p.expect(tokIdent)
		if err != nil {
			return nil, err
		}
		implements := UnresolvedTypeRef{Kind: TypeRefNamed, Name: implTok.lit, Span: implTok.span}
		decl.Implements = &implements
	}
	if _, err := p.expect(tokLBrace); err != nil {
		return nil, err
	}
	for p.peek().kind != tokRBrace {
		memberStart := p.peek().span
		memberAnns, err := p.parseAnnotations()
		if err != nil {
			return nil, err
		}
		memberVis, err := p.parseOptionalVisibility()
		if err != nil {
			return nil, err
		}
		t, n, params, isMethod, err := p.parseTypedMemberHeader()
		if err != nil {
			return nil, err
		}
		if isMethod {
			if _, err := p.expect(tokArrow); err != nil {
				return nil, err
			}
			expr, err := p.parseExpr(1)
			if err != nil {
				return nil, err
			}
			end, err := p.expect(tokSemi)
			if err != nil {
				return nil, err
			}
			decl.Methods = append(decl.Methods, MethodDecl{Visibility: normalizeVisibility(memberVis), Annotations: memberAnns, ReturnType: t, Name: n, Params: params, Body: expr, Span: mergeSpans(memberStart, end.span)})
			continue
		}
		field := FieldDecl{Visibility: normalizeVisibility(memberVis), Annotations: memberAnns, Type: t, Name: n}
		if p.peek().kind == tokAssign {
			p.next()
			expr, err := p.parseExpr(1)
			if err != nil {
				return nil, err
			}
			field.Default = expr
		}
		end, err := p.expect(tokSemi)
		if err != nil {
			return nil, err
		}
		field.Span = mergeSpans(memberStart, end.span)
		decl.Fields = append(decl.Fields, field)
	}
	end, err := p.expect(tokRBrace)
	if err != nil {
		return nil, err
	}
	decl.Span = mergeSpans(start, end.span)
	return decl, nil
}

func (p *parser) parseAnnotations() ([]Annotation, error) {
	var out []Annotation
	for p.peek().kind == tokLBracket {
		start := p.next().span
		nameTok, err := p.expect(tokIdent)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokAssign); err != nil {
			return nil, err
		}
		value, err := p.parseAnnotationValue()
		if err != nil {
			return nil, err
		}
		end, err := p.expect(tokRBracket)
		if err != nil {
			return nil, err
		}
		out = append(out, Annotation{Name: nameTok.lit, Value: value, Span: mergeSpans(start, end.span)})
	}
	return out, nil
}

func (p *parser) parseAnnotationValue() (Value, error) {
	t := p.peek()
	switch t.kind {
	case tokString:
		p.next()
		return Value{Type: TypeString, String: t.lit}, nil
	case tokInt:
		p.next()
		v, err := strconv.ParseInt(t.lit, 10, 64)
		if err != nil {
			return Value{}, oneDiagnostic(p.sources, CodeInvalidNumber, CategoryLexical, t.span, "invalid integer literal")
		}
		return Value{Type: TypeInt, Int: v}, nil
	case tokFloat:
		p.next()
		v, err := strconv.ParseFloat(t.lit, 64)
		if err != nil {
			return Value{}, oneDiagnostic(p.sources, CodeInvalidNumber, CategoryLexical, t.span, "invalid float literal")
		}
		return Value{Type: TypeFloat, Float: v}, nil
	case tokBool:
		p.next()
		return Value{Type: TypeBool, Bool: t.lit == "true"}, nil
	default:
		return Value{}, p.errf("expected literal annotation value")
	}
}

func (p *parser) parseInterface(vis Visibility, anns []Annotation, start Span) (*InterfaceDecl, error) {
	if _, err := p.expect(tokInterface); err != nil {
		return nil, err
	}
	nameTok, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLBrace); err != nil {
		return nil, err
	}
	decl := &InterfaceDecl{Name: nameTok.lit, Visibility: normalizeVisibility(vis), Annotations: anns}
	for p.peek().kind != tokRBrace {
		memberStart := p.peek().span
		memberAnns, err := p.parseAnnotations()
		if err != nil {
			return nil, err
		}
		memberVis, err := p.parseOptionalVisibility()
		if err != nil {
			return nil, err
		}
		t, n, params, isMethod, err := p.parseTypedMemberHeader()
		if err != nil {
			return nil, err
		}
		if isMethod {
			end, err := p.expect(tokSemi)
			if err != nil {
				return nil, err
			}
			decl.Methods = append(decl.Methods, MethodSig{
				Visibility:  normalizeVisibility(memberVis),
				Annotations: memberAnns,
				ReturnType:  t,
				Name:        n,
				Params:      params,
				Span:        mergeSpans(memberStart, end.span),
			})
			continue
		}
		end, err := p.expect(tokSemi)
		if err != nil {
			return nil, err
		}
		decl.Fields = append(decl.Fields, FieldSig{
			Visibility:  normalizeVisibility(memberVis),
			Annotations: memberAnns,
			Type:        t,
			Name:        n,
			Span:        mergeSpans(memberStart, end.span),
		})
	}
	end, err := p.expect(tokRBrace)
	if err != nil {
		return nil, err
	}
	decl.Span = mergeSpans(start, end.span)
	return decl, nil
}

func (p *parser) parseClass(vis Visibility, anns []Annotation, start Span) (*ClassDecl, error) {
	switch p.peek().kind {
	case tokClass, tokStruct:
		p.next()
	default:
		return nil, p.errf("expected Class or Struct")
	}
	nameTok, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	decl := &ClassDecl{Name: nameTok.lit, Visibility: normalizeVisibility(vis), Annotations: anns}
	if p.peek().kind == tokColon {
		p.next()
		implTok, err := p.expect(tokIdent)
		if err != nil {
			return nil, err
		}
		implements := UnresolvedTypeRef{Kind: TypeRefNamed, Name: implTok.lit, Span: implTok.span}
		decl.Implements = &implements
	}
	if _, err := p.expect(tokLBrace); err != nil {
		return nil, err
	}
	for p.peek().kind != tokRBrace {
		memberStart := p.peek().span
		memberAnns, err := p.parseAnnotations()
		if err != nil {
			return nil, err
		}
		memberVis, err := p.parseOptionalVisibility()
		if err != nil {
			return nil, err
		}
		t, n, params, isMethod, err := p.parseTypedMemberHeader()
		if err != nil {
			return nil, err
		}
		if isMethod {
			if _, err := p.expect(tokArrow); err != nil {
				return nil, err
			}
			expr, err := p.parseExpr(1)
			if err != nil {
				return nil, err
			}
			end, err := p.expect(tokSemi)
			if err != nil {
				return nil, err
			}
			decl.Methods = append(decl.Methods, MethodDecl{
				Visibility:  normalizeVisibility(memberVis),
				Annotations: memberAnns,
				ReturnType:  t,
				Name:        n,
				Params:      params,
				Body:        expr,
				Span:        mergeSpans(memberStart, end.span),
			})
			continue
		}
		f := FieldDecl{Visibility: normalizeVisibility(memberVis), Annotations: memberAnns, Type: t, Name: n}
		if p.peek().kind == tokAssign {
			p.next()
			expr, err := p.parseExpr(1)
			if err != nil {
				return nil, err
			}
			f.Default = expr
		}
		end, err := p.expect(tokSemi)
		if err != nil {
			return nil, err
		}
		f.Span = mergeSpans(memberStart, end.span)
		decl.Fields = append(decl.Fields, f)
	}
	end, err := p.expect(tokRBrace)
	if err != nil {
		return nil, err
	}
	decl.Span = mergeSpans(start, end.span)
	return decl, nil
}

func (p *parser) parseOptionalVisibility() (Visibility, error) {
	switch p.peek().kind {
	case tokPublic:
		p.next()
		return VisibilityPublic, nil
	case tokPrivate:
		p.next()
		return VisibilityPrivate, nil
	default:
		return VisibilityPublic, nil
	}
}

func (p *parser) parseTypedMemberHeader() (UnresolvedTypeRef, string, []Param, bool, error) {
	t, err := p.parseTypeRef()
	if err != nil {
		return UnresolvedTypeRef{}, "", nil, false, err
	}
	nameTok, err := p.expect(tokIdent)
	if err != nil {
		return UnresolvedTypeRef{}, "", nil, false, err
	}
	if p.peek().kind != tokLParen {
		return t, nameTok.lit, nil, false, nil
	}
	p.next()
	params := []Param{}
	if p.peek().kind != tokRParen {
		for {
			paramStart := p.peek().span
			pt, err := p.parseTypeRef()
			if err != nil {
				return UnresolvedTypeRef{}, "", nil, false, err
			}
			pn, err := p.expect(tokIdent)
			if err != nil {
				return UnresolvedTypeRef{}, "", nil, false, err
			}
			params = append(params, Param{Type: pt, Name: pn.lit, Span: mergeSpans(paramStart, pn.span)})
			if p.peek().kind == tokComma {
				p.next()
				continue
			}
			break
		}
	}
	if _, err := p.expect(tokRParen); err != nil {
		return UnresolvedTypeRef{}, "", nil, false, err
	}
	return t, nameTok.lit, params, true, nil
}

func (p *parser) parseTypeRef() (UnresolvedTypeRef, error) {
	tok, err := p.expect(tokIdent)
	if err != nil {
		return UnresolvedTypeRef{}, err
	}
	if primitive, ok := primitiveType(tok.lit); ok {
		return unresolvedPrimitive(primitive, tok.span), nil
	}
	if tok.lit == "List" && p.peek().kind == tokLT {
		p.next()
		inner, err := p.parseTypeRef()
		if err != nil {
			return UnresolvedTypeRef{}, err
		}
		end, err := p.expect(tokGT)
		if err != nil {
			return UnresolvedTypeRef{}, err
		}
		return UnresolvedTypeRef{Kind: TypeRefApplied, Name: "List", Arguments: []UnresolvedTypeRef{inner}, Span: mergeSpans(tok.span, end.span)}, nil
	}
	if tok.lit == "Result" && hasArithmeticResultSourceContract(p.languageContract) && p.peek().kind == tokLT {
		p.next()
		success, err := p.parseTypeRef()
		if err != nil {
			return UnresolvedTypeRef{}, err
		}
		if _, err := p.expect(tokComma); err != nil {
			return UnresolvedTypeRef{}, err
		}
		failure, err := p.parseTypeRef()
		if err != nil {
			return UnresolvedTypeRef{}, err
		}
		end, err := p.expect(tokGT)
		if err != nil {
			return UnresolvedTypeRef{}, err
		}
		return UnresolvedTypeRef{Kind: TypeRefApplied, Name: "Result", Arguments: []UnresolvedTypeRef{success, failure}, Span: mergeSpans(tok.span, end.span)}, nil
	}
	if tok.lit == "Optional" && hasPrimitiveOptionalSourceContract(p.languageContract) && p.peek().kind == tokLT {
		p.next()
		value, err := p.parseTypeRef()
		if err != nil {
			return UnresolvedTypeRef{}, err
		}
		end, err := p.expect(tokGT)
		if err != nil {
			return UnresolvedTypeRef{}, err
		}
		return UnresolvedTypeRef{Kind: TypeRefApplied, Name: "Optional", Arguments: []UnresolvedTypeRef{value}, Span: mergeSpans(tok.span, end.span)}, nil
	}
	return UnresolvedTypeRef{Kind: TypeRefNamed, Name: tok.lit, Span: tok.span}, nil
}

func (p *parser) parseExpr(minPrec int) (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		opTok := p.peek()
		prec := binaryPrecedence(opTok.kind)
		if prec < minPrec {
			break
		}
		p.next()
		right, err := p.parseExpr(prec + 1)
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: opTok.lit, Left: left, Right: right, Span: mergeSpans(left.SourceSpan(), right.SourceSpan())}
	}
	if minPrec == 1 && hasConditionalSourceContract(p.languageContract) && p.peek().kind == tokQuestion {
		p.next()
		whenTrue, err := p.parseExpr(1)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokColon); err != nil {
			return nil, err
		}
		whenFalse, err := p.parseExpr(1)
		if err != nil {
			return nil, err
		}
		left = &ConditionalExpr{Condition: left, WhenTrue: whenTrue, WhenFalse: whenFalse, Span: mergeSpans(left.SourceSpan(), whenFalse.SourceSpan())}
	}
	return left, nil
}

func (p *parser) parseUnary() (Expr, error) {
	switch p.peek().kind {
	case tokBang, tokMinus:
		opTok := p.next()
		ex, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: opTok.lit, Expr: ex, Span: mergeSpans(opTok.span, ex.SourceSpan())}, nil
	default:
		return p.parsePostfix()
	}
}

func (p *parser) parsePostfix() (Expr, error) {
	expr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	if !hasRecordFieldProjectionSourceContract(p.languageContract) {
		return expr, nil
	}
	for p.peek().kind == tokDot || ((p.languageContract == PipeLangLanguageContractV330 || p.languageContract == PipeLangLanguageContractV340 || p.languageContract == PipeLangLanguageContractV350 || p.languageContract == PipeLangLanguageContractV360 || p.languageContract == PipeLangLanguageContractV370 || p.languageContract == PipeLangLanguageContractV380) && p.peek().kind == tokLBracket) {
		if p.peek().kind == tokLBracket {
			p.next()
			index, err := p.parseExpr(1)
			if err != nil {
				return nil, err
			}
			end, err := p.expect(tokRBracket)
			if err != nil {
				return nil, err
			}
			expr = &ListAtExpr{Values: expr, Index: index, Postfix: true, Span: mergeSpans(expr.SourceSpan(), end.span)}
			continue
		}
		p.next()
		name, err := p.expect(tokIdent)
		if err != nil {
			return nil, err
		}
		expr = &FieldExpr{Receiver: expr, Name: name.lit, NameSpan: name.span, Span: mergeSpans(expr.SourceSpan(), name.span)}
	}
	return expr, nil
}

func (p *parser) parsePrimary() (Expr, error) {
	t := p.peek()
	switch t.kind {
	case tokString:
		p.next()
		return &LiteralExpr{Value: Value{Type: TypeString, String: t.lit}, Span: t.span}, nil
	case tokInt:
		p.next()
		v, err := strconv.ParseInt(t.lit, 10, 64)
		if err != nil {
			return nil, oneDiagnostic(p.sources, CodeInvalidNumber, CategoryLexical, t.span, "invalid integer literal")
		}
		return &LiteralExpr{Value: Value{Type: TypeInt, Int: v}, Span: t.span}, nil
	case tokFloat:
		p.next()
		v, err := strconv.ParseFloat(t.lit, 64)
		if err != nil {
			return nil, oneDiagnostic(p.sources, CodeInvalidNumber, CategoryLexical, t.span, "invalid float literal")
		}
		return &LiteralExpr{Value: Value{Type: TypeFloat, Float: v}, Span: t.span}, nil
	case tokBool:
		p.next()
		return &LiteralExpr{Value: Value{Type: TypeBool, Bool: t.lit == "true"}, Span: t.span}, nil
	case tokIdent:
		if t.lit == "match" && hasMatchSourceContract(p.languageContract) {
			return p.parseMatch()
		}
		if t.lit == "propagate" && hasPropagationSourceContract(p.languageContract) {
			start := p.next()
			if _, err := p.expect(tokLParen); err != nil {
				return nil, err
			}
			value, err := p.parseExpr(1)
			if err != nil {
				return nil, err
			}
			end, err := p.expect(tokRParen)
			if err != nil {
				return nil, err
			}
			return &PropagateExpr{Value: value, Span: mergeSpans(start.span, end.span)}, nil
		}
		if t.lit == "new" && hasPrimitiveRecordConstructionSourceContract(p.languageContract) {
			return p.parseRecordConstruction()
		}
		if t.lit == "contains_casefolded" && hasCaseFoldedTextContainmentSourceContract(p.languageContract) {
			return p.parseTextContainsCaseFolded()
		}
		if t.lit == "trim" && hasTextTrimSourceContract(p.languageContract) {
			return p.parseTextTrim()
		}
		if hasPrimitiveOptionalSourceContract(p.languageContract) {
			switch t.lit {
			case "some":
				return p.parseOptionalSome()
			case "none":
				return p.parseOptionalNone()
			case "has_value":
				return p.parseOptionalHasValue()
			case "value_or":
				if hasPrimitiveOptionalDefaultSourceContract(p.languageContract) {
					return p.parseOptionalValueOr()
				}
			}
		}
		if hasPrimitiveRecordListSourceContract(p.languageContract) {
			switch t.lit {
			case "filter":
				if hasNamedRecordPredicateSourceContract(p.languageContract) {
					return p.parseListFilterPredicate()
				}
			case "empty_list":
				return p.parseListEmpty()
			case "list":
				return p.parseListSingleton()
			case "count":
				if hasPrimitiveRecordListCountSourceContract(p.languageContract) {
					return p.parseListCount()
				}
			case "append":
				if hasPrimitiveRecordListAppendSourceContract(p.languageContract) {
					return p.parseListAppend()
				}
			case "at":
				if hasPrimitiveRecordListAtSourceContract(p.languageContract) {
					return p.parseListAt()
				}
			case "find_by":
				if hasPrimitiveRecordListFindByTextSourceContract(p.languageContract) {
					return p.parseListFindByText()
				}
			case "filter_by":
				if hasPrimitiveRecordListFilterByTextSourceContract(p.languageContract) {
					return p.parseListFilterByText()
				}
			case "filter_contains_casefolded":
				if hasPrimitiveRecordListFilterContainsCaseFoldedSourceContract(p.languageContract) {
					return p.parseListFilterContainsCaseFolded()
				}
			case "filter_joined_contains_casefolded":
				if hasPrimitiveRecordListFilterJoinedContainsCaseFoldedSourceContract(p.languageContract) {
					return p.parseListFilterJoinedContainsCaseFolded()
				}
			case "sort_by_ordinal":
				if hasPrimitiveRecordListSortByOrdinalSourceContract(p.languageContract) {
					return p.parseListSortByOrdinal()
				}
			}
		}
		if hasSnapshotResultSourceContract(p.languageContract) {
			switch t.lit {
			case "ok":
				return p.parseResultOK()
			case "err":
				return p.parseResultErr()
			case "is_ok":
				return p.parseResultIsOK()
			case "success_or":
				return p.parseResultSuccessOr()
			case "failure_or":
				return p.parseResultFailureOr()
			}
		}
		if hasPureCallSourceContract(p.languageContract) && p.peekAt(1).kind == tokLParen {
			return p.parsePureCall()
		}
		p.next()
		return &IdentExpr{Name: t.lit, Span: t.span}, nil
	case tokLParen:
		start := p.next().span
		ex, err := p.parseExpr(1)
		if err != nil {
			return nil, err
		}
		end, err := p.expect(tokRParen)
		if err != nil {
			return nil, err
		}
		setExprSpan(ex, mergeSpans(start, end.span))
		return ex, nil
	default:
		return nil, p.errf("unexpected token %q in expression", t.lit)
	}
}

func (p *parser) parsePureCall() (Expr, error) {
	name, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	arguments := []Expr{}
	for p.peek().kind != tokRParen {
		argument, err := p.parseExpr(1)
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, argument)
		if p.peek().kind != tokComma {
			break
		}
		p.next()
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &CallExpr{Name: name.lit, NameSpan: name.span, Arguments: arguments, Span: mergeSpans(name.span, end.span)}, nil
}

func (p *parser) parseMatch() (Expr, error) {
	start := p.next()
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	value, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	if _, err = p.expect(tokRParen); err != nil {
		return nil, err
	}
	if _, err = p.expect(tokLBrace); err != nil {
		return nil, err
	}
	arms := []MatchArm{}
	for p.peek().kind != tokRBrace {
		pattern := p.peek()
		if pattern.kind != tokIdent {
			return nil, p.errf("expected match pattern")
		}
		p.next()
		arm := MatchArm{Tag: pattern.lit, PatternSpan: pattern.span}
		if p.peek().kind == tokLParen {
			p.next()
			binding, e := p.expect(tokIdent)
			if e != nil {
				return nil, e
			}
			arm.Binding = binding.lit
			if _, e = p.expect(tokRParen); e != nil {
				return nil, e
			}
		}
		if _, err = p.expect(tokArrow); err != nil {
			return nil, err
		}
		arm.Body, err = p.parseExpr(1)
		if err != nil {
			return nil, err
		}
		arm.Span = mergeSpans(pattern.span, arm.Body.SourceSpan())
		arms = append(arms, arm)
		if p.peek().kind == tokComma {
			p.next()
			continue
		}
		if p.peek().kind != tokRBrace {
			return nil, p.errf("expected comma or closing brace")
		}
	}
	end, err := p.expect(tokRBrace)
	if err != nil {
		return nil, err
	}
	return &MatchExpr{Value: value, Arms: arms, Span: mergeSpans(start.span, end.span)}, nil
}

func (p *parser) parseTextContainsCaseFolded() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	value, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokComma); err != nil {
		return nil, err
	}
	query, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &TextContainsCaseFoldedExpr{Value: value, Query: query, Span: mergeSpans(start.span, end.span)}, nil
}

func (p *parser) parseTextTrim() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	value, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &TextTrimExpr{Value: value, Span: mergeSpans(start.span, end.span)}, nil
}

func (p *parser) parseResultTypeArguments() (token, UnresolvedTypeRef, UnresolvedTypeRef, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return token{}, UnresolvedTypeRef{}, UnresolvedTypeRef{}, err
	}
	if _, err := p.expect(tokLT); err != nil {
		return token{}, UnresolvedTypeRef{}, UnresolvedTypeRef{}, err
	}
	success, err := p.parseTypeRef()
	if err != nil {
		return token{}, UnresolvedTypeRef{}, UnresolvedTypeRef{}, err
	}
	if _, err := p.expect(tokComma); err != nil {
		return token{}, UnresolvedTypeRef{}, UnresolvedTypeRef{}, err
	}
	failure, err := p.parseTypeRef()
	if err != nil {
		return token{}, UnresolvedTypeRef{}, UnresolvedTypeRef{}, err
	}
	if _, err := p.expect(tokGT); err != nil {
		return token{}, UnresolvedTypeRef{}, UnresolvedTypeRef{}, err
	}
	return start, success, failure, nil
}

func (p *parser) parseResultOK() (Expr, error) {
	start, success, failure, err := p.parseResultTypeArguments()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	value, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &ResultOKExpr{SuccessType: success, FailureType: failure, Value: value, Span: mergeSpans(start.span, end.span)}, nil
}

func (p *parser) parseResultErr() (Expr, error) {
	start, success, failure, err := p.parseResultTypeArguments()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	failureValue, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &ResultErrExpr{SuccessType: success, FailureType: failure, Error: failureValue, Span: mergeSpans(start.span, end.span)}, nil
}

func (p *parser) parseResultIsOK() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	value, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &ResultIsOKExpr{Value: value, Span: mergeSpans(start.span, end.span)}, nil
}

func (p *parser) parseResultSuccessOr() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	value, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokComma); err != nil {
		return nil, err
	}
	fallback, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &ResultSuccessOrExpr{Value: value, Fallback: fallback, Span: mergeSpans(start.span, end.span)}, nil
}

func (p *parser) parseResultFailureOr() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	value, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokComma); err != nil {
		return nil, err
	}
	fallback, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &ResultFailureOrExpr{Value: value, Fallback: fallback, Span: mergeSpans(start.span, end.span)}, nil
}

func (p *parser) parseListEmpty() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLT); err != nil {
		return nil, err
	}
	elementType, err := p.parseTypeRef()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokGT); err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &ListEmptyExpr{ElementType: elementType, Span: mergeSpans(start.span, end.span)}, nil
}

func (p *parser) parseListSingleton() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	value, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &ListSingletonExpr{Value: value, Span: mergeSpans(start.span, end.span)}, nil
}

func (p *parser) parseListCount() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	value, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &ListCountExpr{Value: value, Span: mergeSpans(start.span, end.span)}, nil
}

func (p *parser) parseListAppend() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	values, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokComma); err != nil {
		return nil, err
	}
	value, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &ListAppendExpr{Values: values, Value: value, Span: mergeSpans(start.span, end.span)}, nil
}

func (p *parser) parseListAt() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	values, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokComma); err != nil {
		return nil, err
	}
	index, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &ListAtExpr{Values: values, Index: index, Span: mergeSpans(start.span, end.span)}, nil
}

func (p *parser) parseListFindByText() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	values, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokComma); err != nil {
		return nil, err
	}
	recordName, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokDot); err != nil {
		return nil, err
	}
	field, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokComma); err != nil {
		return nil, err
	}
	key, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &ListFindByTextExpr{
		Values:     values,
		RecordType: UnresolvedTypeRef{Kind: TypeRefNamed, Name: recordName.lit, Span: recordName.span},
		Field:      field.lit,
		FieldSpan:  field.span,
		Key:        key,
		Span:       mergeSpans(start.span, end.span),
	}, nil
}

func (p *parser) parseListFilterByText() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	values, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokComma); err != nil {
		return nil, err
	}
	recordName, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokDot); err != nil {
		return nil, err
	}
	field, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokComma); err != nil {
		return nil, err
	}
	key, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &ListFilterByTextExpr{
		Values:     values,
		RecordType: UnresolvedTypeRef{Kind: TypeRefNamed, Name: recordName.lit, Span: recordName.span},
		Field:      field.lit,
		FieldSpan:  field.span,
		Key:        key,
		Span:       mergeSpans(start.span, end.span),
	}, nil
}

func (p *parser) parseListFilterPredicate() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	values, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokComma); err != nil {
		return nil, err
	}
	predicate, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	arguments := []Expr{}
	for p.peek().kind == tokComma {
		p.next()
		argument, err := p.parseExpr(1)
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, argument)
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &ListFilterPredicateExpr{Values: values, Predicate: predicate.lit, PredicateSpan: predicate.span, Arguments: arguments, Span: mergeSpans(start.span, end.span)}, nil
}

func (p *parser) parseListFilterContainsCaseFolded() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	values, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokComma); err != nil {
		return nil, err
	}
	recordName, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokDot); err != nil {
		return nil, err
	}
	field, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokComma); err != nil {
		return nil, err
	}
	query, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &ListFilterContainsCaseFoldedExpr{
		Values:     values,
		RecordType: UnresolvedTypeRef{Kind: TypeRefNamed, Name: recordName.lit, Span: recordName.span},
		Field:      field.lit,
		FieldSpan:  field.span,
		Query:      query,
		Span:       mergeSpans(start.span, end.span),
	}, nil
}

func (p *parser) parseListFilterJoinedContainsCaseFolded() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	values, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	if p.languageContract == PipeLangLanguageContractV290 || p.languageContract == PipeLangLanguageContractV300 || p.languageContract == PipeLangLanguageContractV310 {
		selectors := make([]ListTextFieldSelector, 0, 2)
		for {
			if _, err := p.expect(tokComma); err != nil {
				return nil, err
			}
			name, err := p.expect(tokIdent)
			if err != nil {
				return nil, err
			}
			if p.peek().kind != tokDot {
				end, err := p.expect(tokRParen)
				if err != nil {
					return nil, err
				}
				return &ListFilterJoinedContainsCaseFoldedExpr{
					Values: values, Selectors: selectors, Query: &IdentExpr{Name: name.lit, Span: name.span},
					Span: mergeSpans(start.span, end.span),
				}, nil
			}
			p.next()
			field, err := p.expect(tokIdent)
			if err != nil {
				return nil, err
			}
			selectors = append(selectors, ListTextFieldSelector{
				RecordType: UnresolvedTypeRef{Kind: TypeRefNamed, Name: name.lit, Span: name.span},
				Field:      field.lit,
				FieldSpan:  field.span,
			})
		}
	}
	selectors := make([]ListTextFieldSelector, 0, 5)
	for range 5 {
		if _, err := p.expect(tokComma); err != nil {
			return nil, err
		}
		recordName, err := p.expect(tokIdent)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokDot); err != nil {
			return nil, err
		}
		field, err := p.expect(tokIdent)
		if err != nil {
			return nil, err
		}
		selectors = append(selectors, ListTextFieldSelector{
			RecordType: UnresolvedTypeRef{Kind: TypeRefNamed, Name: recordName.lit, Span: recordName.span},
			Field:      field.lit,
			FieldSpan:  field.span,
		})
	}
	if _, err := p.expect(tokComma); err != nil {
		return nil, err
	}
	query, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &ListFilterJoinedContainsCaseFoldedExpr{
		Values: values, Selectors: selectors, Query: query,
		Span: mergeSpans(start.span, end.span),
	}, nil
}

func (p *parser) parseListSortByOrdinal() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	values, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	if p.languageContract == PipeLangLanguageContractV320 || p.languageContract == PipeLangLanguageContractV330 || p.languageContract == PipeLangLanguageContractV340 || p.languageContract == PipeLangLanguageContractV350 || p.languageContract == PipeLangLanguageContractV360 || p.languageContract == PipeLangLanguageContractV370 || p.languageContract == PipeLangLanguageContractV380 {
		if _, err := p.expect(tokComma); err != nil {
			return nil, err
		}
		recordName, err := p.expect(tokIdent)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokDot); err != nil {
			return nil, err
		}
		field, err := p.expect(tokIdent)
		if err != nil {
			return nil, err
		}
		first := ListTextFieldSelector{RecordType: UnresolvedTypeRef{Kind: TypeRefNamed, Name: recordName.lit, Span: recordName.span}, Field: field.lit, FieldSpan: field.span}
		if p.peek().kind == tokRParen {
			end := p.next()
			return &ListSortByOrdinalExpr{Values: values, RecordType: first.RecordType, Field: first.Field, FieldSpan: first.FieldSpan, Span: mergeSpans(start.span, end.span)}, nil
		}
		if _, err := p.expect(tokComma); err != nil {
			return nil, err
		}
		next, err := p.expect(tokIdent)
		if err != nil {
			return nil, err
		}
		if next.lit == "ascending" || next.lit == "descending" {
			directional := []ListDirectionalTextFieldSelector{{ListTextFieldSelector: first, Direction: next.lit, DirectionSpan: next.span}}
			for p.peek().kind != tokRParen {
				if _, err := p.expect(tokComma); err != nil {
					return nil, err
				}
				rn, err := p.expect(tokIdent)
				if err != nil {
					return nil, err
				}
				if _, err := p.expect(tokDot); err != nil {
					return nil, err
				}
				f, err := p.expect(tokIdent)
				if err != nil {
					return nil, err
				}
				if _, err := p.expect(tokComma); err != nil {
					return nil, err
				}
				d, err := p.expect(tokIdent)
				if err != nil {
					return nil, err
				}
				if d.lit != "ascending" && d.lit != "descending" {
					return nil, oneDiagnostic(p.sources, CodeUnexpectedToken, CategorySyntax, d.span, fmt.Sprintf("expected ascending or descending, got %q", d.lit))
				}
				directional = append(directional, ListDirectionalTextFieldSelector{ListTextFieldSelector: ListTextFieldSelector{RecordType: UnresolvedTypeRef{Kind: TypeRefNamed, Name: rn.lit, Span: rn.span}, Field: f.lit, FieldSpan: f.span}, Direction: d.lit, DirectionSpan: d.span})
			}
			end := p.next()
			return &ListSortByOrdinalDirectionsExpr{Values: values, Selectors: directional, Span: mergeSpans(start.span, end.span)}, nil
		}
		legacy := []ListTextFieldSelector{first}
		for {
			if _, err := p.expect(tokDot); err != nil {
				return nil, err
			}
			f, err := p.expect(tokIdent)
			if err != nil {
				return nil, err
			}
			legacy = append(legacy, ListTextFieldSelector{RecordType: UnresolvedTypeRef{Kind: TypeRefNamed, Name: next.lit, Span: next.span}, Field: f.lit, FieldSpan: f.span})
			if p.peek().kind == tokRParen {
				end := p.next()
				return &ListSortByOrdinalsExpr{Values: values, Selectors: legacy, Span: mergeSpans(start.span, end.span)}, nil
			}
			if _, err := p.expect(tokComma); err != nil {
				return nil, err
			}
			next, err = p.expect(tokIdent)
			if err != nil {
				return nil, err
			}
		}
	}
	if p.languageContract == PipeLangLanguageContractV300 || p.languageContract == PipeLangLanguageContractV310 {
		selectors := make([]ListTextFieldSelector, 0, 2)
		for {
			if _, err := p.expect(tokComma); err != nil {
				return nil, err
			}
			recordName, err := p.expect(tokIdent)
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(tokDot); err != nil {
				return nil, err
			}
			field, err := p.expect(tokIdent)
			if err != nil {
				return nil, err
			}
			selectors = append(selectors, ListTextFieldSelector{
				RecordType: UnresolvedTypeRef{Kind: TypeRefNamed, Name: recordName.lit, Span: recordName.span},
				Field:      field.lit,
				FieldSpan:  field.span,
			})
			if p.peek().kind == tokRParen {
				end := p.next()
				if len(selectors) == 1 {
					selector := selectors[0]
					return &ListSortByOrdinalExpr{Values: values, RecordType: selector.RecordType, Field: selector.Field, FieldSpan: selector.FieldSpan, Span: mergeSpans(start.span, end.span)}, nil
				}
				return &ListSortByOrdinalsExpr{Values: values, Selectors: selectors, Span: mergeSpans(start.span, end.span)}, nil
			}
		}
	}
	if _, err := p.expect(tokComma); err != nil {
		return nil, err
	}
	recordName, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokDot); err != nil {
		return nil, err
	}
	field, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &ListSortByOrdinalExpr{
		Values:     values,
		RecordType: UnresolvedTypeRef{Kind: TypeRefNamed, Name: recordName.lit, Span: recordName.span},
		Field:      field.lit,
		FieldSpan:  field.span,
		Span:       mergeSpans(start.span, end.span),
	}, nil
}

func (p *parser) parseOptionalSome() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	value, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &OptionalSomeExpr{Value: value, Span: mergeSpans(start.span, end.span)}, nil
}

func (p *parser) parseOptionalNone() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLT); err != nil {
		return nil, err
	}
	valueType, err := p.parseTypeRef()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokGT); err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &OptionalNoneExpr{ValueType: valueType, Span: mergeSpans(start.span, end.span)}, nil
}

func (p *parser) parseOptionalHasValue() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	value, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &OptionalHasValueExpr{Value: value, Span: mergeSpans(start.span, end.span)}, nil
}

func (p *parser) parseOptionalValueOr() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLParen); err != nil {
		return nil, err
	}
	value, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokComma); err != nil {
		return nil, err
	}
	fallback, err := p.parseExpr(1)
	if err != nil {
		return nil, err
	}
	end, err := p.expect(tokRParen)
	if err != nil {
		return nil, err
	}
	return &OptionalValueOrExpr{Value: value, Fallback: fallback, Span: mergeSpans(start.span, end.span)}, nil
}

func (p *parser) parseRecordConstruction() (Expr, error) {
	start, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if start.lit != "new" {
		return nil, oneDiagnostic(p.sources, CodeUnexpectedToken, CategorySyntax, start.span, fmt.Sprintf("expected new, got %q", start.lit))
	}
	typeName, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	typeRef := UnresolvedTypeRef{Kind: TypeRefNamed, Name: typeName.lit, Span: typeName.span}
	if _, err := p.expect(tokLBrace); err != nil {
		return nil, err
	}
	fields := []RecordConstructField{}
	for p.peek().kind != tokRBrace {
		name, err := p.expect(tokIdent)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokAssign); err != nil {
			return nil, err
		}
		value, err := p.parseExpr(1)
		if err != nil {
			return nil, err
		}
		fields = append(fields, RecordConstructField{Name: name.lit, NameSpan: name.span, Value: value, Span: mergeSpans(name.span, value.SourceSpan())})
		if p.peek().kind != tokComma {
			break
		}
		p.next()
		if p.peek().kind == tokRBrace {
			return nil, oneDiagnostic(p.sources, CodeExpectedToken, CategorySyntax, p.peek().span, "expected record field after comma")
		}
	}
	end, err := p.expect(tokRBrace)
	if err != nil {
		return nil, err
	}
	return &RecordConstructExpr{Type: typeRef, Fields: fields, Span: mergeSpans(start.span, end.span)}, nil
}

func binaryPrecedence(k tokenKind) int {
	switch k {
	case tokOrOr:
		return 1
	case tokAndAnd:
		return 2
	case tokEQ, tokNE:
		return 3
	case tokLT, tokLE, tokGT, tokGE:
		return 4
	case tokPlus, tokMinus:
		return 5
	case tokStar, tokSlash:
		return 6
	default:
		return 0
	}
}

func (p *parser) peek() token {
	if p.idx >= len(p.toks) {
		return token{kind: tokEOF, span: Span{File: p.file.ID, Start: len(p.file.Text), End: len(p.file.Text)}}
	}
	return p.toks[p.idx]
}

func (p *parser) peekAt(offset int) token {
	position := p.idx + offset
	if position < 0 || position >= len(p.toks) {
		return token{kind: tokEOF, span: Span{File: p.file.ID, Start: len(p.file.Text), End: len(p.file.Text)}}
	}
	return p.toks[position]
}

func (p *parser) next() token {
	t := p.peek()
	if p.idx < len(p.toks) {
		p.idx++
	}
	return t
}

func (p *parser) expect(k tokenKind) (token, error) {
	t := p.peek()
	if t.kind != k {
		return token{}, oneDiagnostic(p.sources, CodeExpectedToken, CategorySyntax, t.span, fmt.Sprintf("expected %s, got %q", tokenName(k), t.lit))
	}
	p.next()
	return t, nil
}

func (p *parser) errf(format string, args ...any) error {
	t := p.peek()
	msg := fmt.Sprintf(format, args...)
	return oneDiagnostic(p.sources, CodeUnexpectedToken, CategorySyntax, t.span, msg)
}

func tokenName(k tokenKind) string {
	switch k {
	case tokIdent:
		return "identifier"
	case tokLBrace:
		return "{"
	case tokRBrace:
		return "}"
	case tokLParen:
		return "("
	case tokRParen:
		return ")"
	case tokColon:
		return ":"
	case tokSemi:
		return ";"
	case tokComma:
		return ","
	case tokDot:
		return "."
	case tokAssign:
		return "="
	case tokArrow:
		return "=>"
	default:
		return fmt.Sprintf("token(%d)", k)
	}
}
