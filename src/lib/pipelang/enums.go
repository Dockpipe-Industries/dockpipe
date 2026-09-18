package pipelang

import (
	"fmt"
	"sort"
	"strings"

	"dockpipe/src/lib/pipelang/hir"
)

// EnumDecl introduces a closed nominal value type. Stable tags are explicitly
// authored and never inferred from declaration order or backend representation.
type EnumDecl struct {
	Name        string
	Visibility  Visibility
	Annotations []Annotation
	Members     []EnumMemberDecl
	Span        Span
}
type EnumMemberDecl struct {
	Name, Tag string
	Span      Span
}

func (p *parser) parseEnum(vis Visibility, anns []Annotation, start Span) (*EnumDecl, error) {
	p.next()
	name, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	d := &EnumDecl{Name: name.lit, Visibility: normalizeVisibility(vis), Annotations: anns}
	if _, err = p.expect(tokLBrace); err != nil {
		return nil, err
	}
	for p.peek().kind != tokRBrace {
		member, e := p.expect(tokIdent)
		if e != nil {
			return nil, e
		}
		if _, e = p.expect(tokAssign); e != nil {
			return nil, e
		}
		tag, e := p.expect(tokString)
		if e != nil {
			return nil, e
		}
		end, e := p.expect(tokSemi)
		if e != nil {
			return nil, e
		}
		d.Members = append(d.Members, EnumMemberDecl{Name: member.lit, Tag: tag.lit, Span: mergeSpans(member.span, end.span)})
	}
	end, err := p.expect(tokRBrace)
	if err != nil {
		return nil, err
	}
	d.Span = mergeSpans(start, end.span)
	return d, nil
}

func (cp *checkedProgram) enumError(span Span, message string) error {
	return oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, span, message)
}
func (cp *checkedProgram) validateEnum(d *EnumDecl) error {
	if d == nil {
		return cp.enumError(Span{}, "enum declaration is nil")
	}
	if cp.modules == nil || (cp.modules.LanguageContract() != PipeLangLanguageContractV1130 && (cp.modules.LanguageContract() != PipeLangLanguageContractV1140 && cp.modules.LanguageContract() != PipeLangLanguageContractV1150)) {
		return cp.enumError(d.Span, "nominal enums require v0.113.0")
	}
	if d.Visibility != VisibilityPublic || len(d.Annotations) != 0 || len(d.Members) == 0 {
		return cp.enumError(d.Span, "enum requires public visibility, at least one member and no annotations")
	}
	names, tags := map[string]Span{}, map[string]Span{}
	for _, m := range d.Members {
		if !isTypeIdentifier(m.Name) || m.Tag == "" {
			return cp.enumError(m.Span, "enum members require identifiers and nonempty stable tags")
		}
		if old, ok := names[m.Name]; ok {
			return oneDiagnostic(cp.sources, CodeDuplicateMember, CategorySemantic, m.Span, "duplicate enum member "+m.Name, RelatedSpan{Span: old, Message: "first member"})
		}
		if old, ok := tags[m.Tag]; ok {
			return oneDiagnostic(cp.sources, CodeDuplicateMember, CategorySemantic, m.Span, "duplicate enum tag "+m.Tag, RelatedSpan{Span: old, Message: "first tag"})
		}
		names[m.Name] = m.Span
		tags[m.Tag] = m.Span
	}
	return nil
}
func (cp *checkedProgram) enumDecl(t ResolvedTypeRef) *EnumDecl {
	if cp == nil || cp.symbols == nil || t.Kind != TypeRefNamed || t.Symbol == 0 {
		return nil
	}
	e, ok := cp.symbols.lookupIDEntry(t.Symbol)
	if !ok || e.symbol.Kind != SymbolEnum {
		return nil
	}
	return e.enumDecl
}
func (cp *checkedProgram) isResolvedEnumType(t ResolvedTypeRef) bool { return cp.enumDecl(t) != nil }
func (cp *checkedProgram) containsResolvedEnumType(t ResolvedTypeRef) bool {
	if cp.isResolvedEnumType(t) {
		return true
	}
	for _, a := range t.Arguments {
		if cp.containsResolvedEnumType(a) {
			return true
		}
	}
	return false
}
func (cp *checkedProgram) isEnumCompositionValue(t ResolvedTypeRef) bool {
	return t.Kind == TypeRefPrimitive || cp.isResolvedEnumType(t)
}

func (cp *checkedProgram) enumMemberValue(e *FieldExpr, env map[string]ResolvedTypeRef) (ResolvedTypeRef, string, bool, error) {
	id, ok := e.Receiver.(*IdentExpr)
	if !ok || cp == nil || cp.modules == nil || (cp.modules.LanguageContract() != PipeLangLanguageContractV1130 && (cp.modules.LanguageContract() != PipeLangLanguageContractV1140 && cp.modules.LanguageContract() != PipeLangLanguageContractV1150)) {
		return ResolvedTypeRef{}, "", false, nil
	}
	if _, shadowed := env[id.Name]; shadowed {
		return ResolvedTypeRef{}, "", false, nil
	}
	t, err := cp.resolveType(UnresolvedTypeRef{Kind: TypeRefNamed, Name: id.Name, Span: id.Span})
	if err != nil || !cp.isResolvedEnumType(t) {
		return ResolvedTypeRef{}, "", false, nil
	}
	for _, m := range cp.enumDecl(t).Members {
		if m.Name == e.Name {
			return t, m.Tag, true, nil
		}
	}
	return t, "", true, cp.enumError(e.NameSpan, "unknown enum member "+id.Name+"."+e.Name)
}
func (cp *checkedProgram) enumPatternTag(t ResolvedTypeRef, arm MatchArm) (string, error) {
	parts := strings.Split(arm.Tag, ".")
	if len(parts) != 2 || arm.Binding != "" {
		return "", cp.enumError(arm.PatternSpan, "enum patterns require a qualified member without a payload binding or wildcard")
	}
	pattern, err := cp.resolveType(UnresolvedTypeRef{Kind: TypeRefNamed, Name: parts[0], Span: arm.PatternSpan})
	if err != nil {
		return "", err
	}
	if !pattern.Equal(t) {
		return "", cp.enumError(arm.PatternSpan, "enum pattern has a different nominal type")
	}
	for _, m := range cp.enumDecl(t).Members {
		if m.Name == parts[1] {
			return m.Tag, nil
		}
	}
	return "", cp.enumError(arm.PatternSpan, "unknown enum pattern "+arm.Tag)
}
func (cp *checkedProgram) inferEnumExpr(expr Expr, env map[string]ResolvedTypeRef) (ResolvedTypeRef, bool, error) {
	if cp == nil || cp.modules == nil || (cp.modules.LanguageContract() != PipeLangLanguageContractV1130 && (cp.modules.LanguageContract() != PipeLangLanguageContractV1140 && cp.modules.LanguageContract() != PipeLangLanguageContractV1150)) {
		return ResolvedTypeRef{}, false, nil
	}
	switch e := expr.(type) {
	case *FieldExpr:
		t, _, ok, err := cp.enumMemberValue(e, env)
		return t, ok, err
	case *BinaryExpr:
		left, err := cp.inferExprType(e.Left, env)
		if err != nil {
			return ResolvedTypeRef{}, true, err
		}
		right, err := cp.inferExprType(e.Right, env)
		if err != nil {
			return ResolvedTypeRef{}, true, err
		}
		if !cp.isResolvedEnumType(left) && !cp.isResolvedEnumType(right) {
			if isOrdinalTextOrderingOperator(e.Op) && left.Equal(resolvedPrimitive(TypeString)) && right.Equal(left) {
				return resolvedPrimitive(TypeBool), true, nil
			}
			t, err := inferBinaryTypeWithPolicy(cp.sources, e.Span, e.Op, left, right, true)
			return t, true, err
		}
		if !left.Equal(right) || (e.Op != "==" && e.Op != "!=") {
			return ResolvedTypeRef{}, true, cp.enumError(e.Span, "enum comparison requires exactly matching nominal types and == or !=")
		}
		return resolvedPrimitive(TypeBool), true, nil
	case *MatchExpr:
		t, err := cp.inferExprType(e.Value, env)
		if err != nil {
			return ResolvedTypeRef{}, true, err
		}
		if !cp.isResolvedEnumType(t) {
			return ResolvedTypeRef{}, false, nil
		}
		seen := map[string]bool{}
		var result ResolvedTypeRef
		for i, arm := range e.Arms {
			tag, err := cp.enumPatternTag(t, arm)
			if err != nil {
				return ResolvedTypeRef{}, true, err
			}
			if seen[tag] {
				return ResolvedTypeRef{}, true, cp.enumError(arm.PatternSpan, "duplicate enum match member")
			}
			seen[tag] = true
			actual, err := cp.inferExprType(arm.Body, env)
			if err != nil {
				return ResolvedTypeRef{}, true, err
			}
			if i == 0 {
				result = actual
			} else if !result.Equal(actual) {
				return ResolvedTypeRef{}, true, cp.enumError(arm.Body.SourceSpan(), "enum match arms require exactly matching result types")
			}
		}
		if len(seen) != len(cp.enumDecl(t).Members) {
			return ResolvedTypeRef{}, true, cp.enumError(e.Span, "enum match is not exhaustive")
		}
		return result, true, nil
	}
	return ResolvedTypeRef{}, false, nil
}
func (cp *checkedProgram) isEnumMethod(m MethodDecl) bool {
	if cp.modules == nil || (cp.modules.LanguageContract() != PipeLangLanguageContractV1130 && (cp.modules.LanguageContract() != PipeLangLanguageContractV1140 && cp.modules.LanguageContract() != PipeLangLanguageContractV1150)) {
		return false
	}
	if t, err := cp.resolveType(m.ReturnType); err == nil && cp.isResolvedEnumType(t) {
		return true
	}
	for _, p := range m.Params {
		if t, err := cp.resolveType(p.Type); err == nil && cp.isResolvedEnumType(t) {
			return true
		}
	}
	var walk func(Expr) bool
	walk = func(e Expr) bool {
		if call, ok := e.(*CallExpr); ok {
			_, target := methodBySpan(cp.program, call.TargetSpan)
			if target != nil {
				if typ, err := cp.resolveType(target.ReturnType); err == nil && cp.isResolvedEnumType(typ) {
					return true
				}
			}
		}
		if local, ok := e.(*ImmutableLocalExpr); ok {
			if typ, err := cp.resolveType(local.Type); err == nil && cp.isResolvedEnumType(typ) {
				return true
			}
		}
		if field, ok := e.(*FieldExpr); ok {
			if _, _, found, _ := cp.enumMemberValue(field, nil); found {
				return true
			}
		}
		for _, child := range expressionChildren(e) {
			if walk(child) {
				return true
			}
		}
		return false
	}
	return walk(m.Body)
}
func (cp *checkedProgram) validateEnumMethod(m MethodDecl) error {
	if m.Visibility != VisibilityPublic {
		return cp.enumError(m.Span, "enum executable methods must be public")
	}
	if err := cp.validateEnumSourceShape(m.Body, 0, 0, true); err != nil {
		return err
	}
	env := map[string]ResolvedTypeRef{}
	for _, p := range m.Params {
		t, err := cp.resolveType(p.Type)
		if err != nil {
			return err
		}
		if !cp.isEnumCompositionValue(t) {
			return cp.enumError(p.Type.Span, "enum composition parameters require primitive or enum values")
		}
		env[p.Name] = t
	}
	// All expressions still pass the normal typed recursive checker. Core owns the
	// independent enum-composition envelope, including malformed-input refusal.
	result, err := cp.inferExprType(m.Body, env)
	if err != nil {
		return err
	}
	declared, err := cp.resolveType(m.ReturnType)
	if err != nil {
		return err
	}
	if !cp.isEnumCompositionValue(declared) {
		return cp.enumError(m.ReturnType.Span, "enum composition results require primitive or enum values")
	}
	if !result.Equal(declared) {
		return cp.enumError(m.Body.SourceSpan(), fmt.Sprintf("enum method returns %s, expected %s", result, declared))
	}
	return nil
}
func enumTypeToHIR(a *Analysis, t ResolvedTypeRef) hir.Type {
	d := a.checked.enumDecl(t)
	identity, _ := a.SemanticIDs.IdentityForSpan(d.Span)
	id := toHIRSemanticIdentity(identity)
	result := hir.Type{Kind: hir.TypeEnum, Name: d.Name, SymbolID: uint32(t.Symbol), Identity: &id, Enum: &hir.EnumType{}}
	for _, m := range d.Members {
		memberID, _ := a.SemanticIDs.IdentityForSpan(m.Span)
		result.Enum.Members = append(result.Enum.Members, hir.EnumMember{Name: m.Name, Tag: m.Tag, Identity: toHIRSemanticIdentity(memberID)})
	}
	sort.Slice(result.Enum.Members, func(i, j int) bool { return result.Enum.Members[i].Tag < result.Enum.Members[j].Tag })
	return result
}

func (cp *checkedProgram) validateEnumSourceShape(e Expr, depth, matches int, allowLocal bool) error {
	switch n := e.(type) {
	case *LiteralExpr, *IdentExpr, *FieldExpr:
		return nil
	case *ImmutableLocalExpr:
		if !allowLocal {
			return cp.enumError(n.Span, "enum local has invalid placement")
		}
		if err := cp.validateEnumSourceShape(n.Initializer, depth, matches, false); err != nil {
			return err
		}
		return cp.validateEnumSourceShape(n.Return, depth, matches, true)
	case *ConditionalExpr:
		depth++
		if depth > 3 {
			return cp.enumError(n.Span, "enum composition conditional depth exceeds three")
		}
		if n.TerminalStatement {
			if err := cp.validateEnumSourceShape(n.Condition, depth, matches, false); err != nil {
				return err
			}
			if err := cp.validateEnumSourceShape(n.WhenTrue, depth, matches, true); err != nil {
				return err
			}
			return cp.validateEnumSourceShape(n.WhenFalse, depth, matches, true)
		}
	case *MatchExpr:
		matches++
		if matches > 3 {
			return cp.enumError(n.Span, "enum match depth exceeds three")
		}
	case *CallExpr, *BinaryExpr, *UnaryExpr, *TextTrimExpr, *TextContainsCaseFoldedExpr:
	default:
		return cp.enumError(e.SourceSpan(), "unsupported enum composition expression")
	}
	for _, c := range expressionChildren(e) {
		if err := cp.validateEnumSourceShape(c, depth, matches, false); err != nil {
			return err
		}
	}
	return nil
}
