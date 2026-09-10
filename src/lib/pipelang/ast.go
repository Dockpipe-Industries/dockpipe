package pipelang

import (
	"fmt"
)

type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

func normalizeVisibility(v Visibility) Visibility {
	if v == VisibilityPrivate {
		return VisibilityPrivate
	}
	return VisibilityPublic
}

func (v Visibility) IsValid() bool {
	n := normalizeVisibility(v)
	return n == VisibilityPublic || n == VisibilityPrivate
}

func isTypeIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if !(r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
				return false
			}
			continue
		}
		if !(r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

type Program struct {
	Interfaces []*InterfaceDecl
	Classes    []*ClassDecl
	Records    []*RecordDecl
	Span       Span
	sources    *SourceSet
	modules    *ModuleGraph
}

type Annotation struct {
	Name  string
	Value Value
	Span  Span
}

type InterfaceDecl struct {
	Name        string
	Visibility  Visibility
	Annotations []Annotation
	Fields      []FieldSig
	Methods     []MethodSig
	Span        Span
}

type ClassDecl struct {
	Name        string
	Visibility  Visibility
	Annotations []Annotation
	Implements  *UnresolvedTypeRef
	Fields      []FieldDecl
	Methods     []MethodDecl
	Span        Span
}

// RecordDecl is the distinct immutable value declaration introduced by the
// explicit v0.9.0 contract. The parser retains excluded member shapes so the
// checker can reject them with declaration-aware diagnostics; accepted
// records contain public primitive fields only.
type RecordDecl struct {
	Name        string
	Visibility  Visibility
	Annotations []Annotation
	Implements  *UnresolvedTypeRef
	Fields      []FieldDecl
	Methods     []MethodDecl
	Span        Span
}

type FieldSig struct {
	Visibility  Visibility
	Annotations []Annotation
	Type        UnresolvedTypeRef
	Name        string
	Span        Span
}

type MethodSig struct {
	Visibility  Visibility
	Annotations []Annotation
	ReturnType  UnresolvedTypeRef
	Name        string
	Params      []Param
	Span        Span
}

type FieldDecl struct {
	Visibility  Visibility
	Annotations []Annotation
	Type        UnresolvedTypeRef
	Name        string
	Default     Expr
	Span        Span
}

type MethodDecl struct {
	Visibility  Visibility
	Annotations []Annotation
	ReturnType  UnresolvedTypeRef
	Name        string
	Params      []Param
	Body        Expr
	Span        Span
}

type Param struct {
	Type UnresolvedTypeRef
	Name string
	Span Span
}

type Expr interface {
	isExpr()
	SourceSpan() Span
}

type (
	LiteralExpr struct {
		Value Value
		Span  Span
	}
	IdentExpr struct {
		Name string
		Span Span
	}
	UnaryExpr struct {
		Op   string
		Expr Expr
		Span Span
	}
	BinaryExpr struct {
		Op          string
		Left, Right Expr
		Span        Span
	}
	ConditionalExpr struct {
		Condition         Expr
		WhenTrue          Expr
		WhenFalse         Expr
		TerminalStatement bool
		Span              Span
	}
	ImmutableLocalExpr struct {
		Type        UnresolvedTypeRef
		Name        string
		NameSpan    Span
		Initializer Expr
		Return      Expr
		Span        Span
	}
	CallExpr struct {
		Name       string
		NameSpan   Span
		Arguments  []Expr
		TargetSpan Span
		Span       Span
	}
	TextContainsCaseFoldedExpr struct {
		Value Expr
		Query Expr
		Span  Span
	}
	TextTrimExpr struct {
		Value Expr
		Span  Span
	}
	FieldExpr struct {
		Receiver Expr
		Name     string
		NameSpan Span
		Span     Span
	}
	RecordConstructField struct {
		Name     string
		NameSpan Span
		Value    Expr
		Span     Span
	}
	RecordConstructExpr struct {
		Type   UnresolvedTypeRef
		Fields []RecordConstructField
		Span   Span
	}
	OptionalSomeExpr struct {
		Value Expr
		Span  Span
	}
	OptionalNoneExpr struct {
		ValueType UnresolvedTypeRef
		Span      Span
	}
	OptionalHasValueExpr struct {
		Value Expr
		Span  Span
	}
	OptionalValueOrExpr struct {
		Value    Expr
		Fallback Expr
		Span     Span
	}
	PropagateExpr struct {
		Value Expr
		Span  Span
	}
	MatchArm struct {
		Tag         string
		Binding     string
		PatternSpan Span
		Body        Expr
		Span        Span
	}
	MatchExpr struct {
		Value Expr
		Arms  []MatchArm
		Span  Span
	}
	ListEmptyExpr struct {
		ElementType UnresolvedTypeRef
		Span        Span
	}
	ListSingletonExpr struct {
		Value Expr
		Span  Span
	}
	ListCountExpr struct {
		Value Expr
		Span  Span
	}
	ListAppendExpr struct {
		Values Expr
		Value  Expr
		Span   Span
	}
	ListAtExpr struct {
		Values  Expr
		Index   Expr
		Postfix bool
		Span    Span
	}
	ListFindByTextExpr struct {
		Values     Expr
		RecordType UnresolvedTypeRef
		Field      string
		FieldSpan  Span
		Key        Expr
		Span       Span
	}
	ListFilterByTextExpr struct {
		Values     Expr
		RecordType UnresolvedTypeRef
		Field      string
		FieldSpan  Span
		Key        Expr
		Span       Span
	}
	ListFilterPredicateExpr struct {
		Values        Expr
		Predicate     string
		PredicateSpan Span
		Arguments     []Expr
		Span          Span
	}
	ListFilterContainsCaseFoldedExpr struct {
		Values     Expr
		RecordType UnresolvedTypeRef
		Field      string
		FieldSpan  Span
		Query      Expr
		Span       Span
	}
	ListTextFieldSelector struct {
		RecordType UnresolvedTypeRef
		Field      string
		FieldSpan  Span
	}
	ListFilterJoinedContainsCaseFoldedExpr struct {
		Values    Expr
		Selectors []ListTextFieldSelector
		Query     Expr
		Span      Span
	}
	ListSortByOrdinalExpr struct {
		Values     Expr
		RecordType UnresolvedTypeRef
		Field      string
		FieldSpan  Span
		Span       Span
	}
	ListSortByOrdinalsExpr struct {
		Values    Expr
		Selectors []ListTextFieldSelector
		Span      Span
	}
	ListDirectionalTextFieldSelector struct {
		ListTextFieldSelector
		Direction     string
		DirectionSpan Span
	}
	ListSortByOrdinalDirectionsExpr struct {
		Values    Expr
		Selectors []ListDirectionalTextFieldSelector
		Span      Span
	}
	ResultOKExpr struct {
		SuccessType UnresolvedTypeRef
		FailureType UnresolvedTypeRef
		Value       Expr
		Span        Span
	}
	ResultErrExpr struct {
		SuccessType UnresolvedTypeRef
		FailureType UnresolvedTypeRef
		Error       Expr
		Span        Span
	}
	ResultIsOKExpr struct {
		Value Expr
		Span  Span
	}
	ResultSuccessOrExpr struct {
		Value    Expr
		Fallback Expr
		Span     Span
	}
	ResultFailureOrExpr struct {
		Value    Expr
		Fallback Expr
		Span     Span
	}
)

func (*LiteralExpr) isExpr()                            {}
func (*IdentExpr) isExpr()                              {}
func (*UnaryExpr) isExpr()                              {}
func (*BinaryExpr) isExpr()                             {}
func (*ConditionalExpr) isExpr()                        {}
func (*ImmutableLocalExpr) isExpr()                     {}
func (*CallExpr) isExpr()                               {}
func (*TextContainsCaseFoldedExpr) isExpr()             {}
func (*TextTrimExpr) isExpr()                           {}
func (*FieldExpr) isExpr()                              {}
func (*RecordConstructExpr) isExpr()                    {}
func (*OptionalSomeExpr) isExpr()                       {}
func (*OptionalNoneExpr) isExpr()                       {}
func (*OptionalHasValueExpr) isExpr()                   {}
func (*OptionalValueOrExpr) isExpr()                    {}
func (*PropagateExpr) isExpr()                          {}
func (*MatchExpr) isExpr()                              {}
func (*ListEmptyExpr) isExpr()                          {}
func (*ListSingletonExpr) isExpr()                      {}
func (*ListCountExpr) isExpr()                          {}
func (*ListAppendExpr) isExpr()                         {}
func (*ListAtExpr) isExpr()                             {}
func (*ListFindByTextExpr) isExpr()                     {}
func (*ListFilterByTextExpr) isExpr()                   {}
func (*ListFilterPredicateExpr) isExpr()                {}
func (*ListFilterContainsCaseFoldedExpr) isExpr()       {}
func (*ListFilterJoinedContainsCaseFoldedExpr) isExpr() {}
func (*ListSortByOrdinalExpr) isExpr()                  {}
func (*ListSortByOrdinalsExpr) isExpr()                 {}
func (*ListSortByOrdinalDirectionsExpr) isExpr()        {}
func (*ResultOKExpr) isExpr()                           {}
func (*ResultErrExpr) isExpr()                          {}
func (*ResultIsOKExpr) isExpr()                         {}
func (*ResultSuccessOrExpr) isExpr()                    {}
func (*ResultFailureOrExpr) isExpr()                    {}

func (e *LiteralExpr) SourceSpan() Span                            { return e.Span }
func (e *IdentExpr) SourceSpan() Span                              { return e.Span }
func (e *UnaryExpr) SourceSpan() Span                              { return e.Span }
func (e *BinaryExpr) SourceSpan() Span                             { return e.Span }
func (e *ConditionalExpr) SourceSpan() Span                        { return e.Span }
func (e *ImmutableLocalExpr) SourceSpan() Span                     { return e.Span }
func (e *CallExpr) SourceSpan() Span                               { return e.Span }
func (e *TextContainsCaseFoldedExpr) SourceSpan() Span             { return e.Span }
func (e *TextTrimExpr) SourceSpan() Span                           { return e.Span }
func (e *FieldExpr) SourceSpan() Span                              { return e.Span }
func (e *RecordConstructExpr) SourceSpan() Span                    { return e.Span }
func (e *OptionalSomeExpr) SourceSpan() Span                       { return e.Span }
func (e *OptionalNoneExpr) SourceSpan() Span                       { return e.Span }
func (e *OptionalHasValueExpr) SourceSpan() Span                   { return e.Span }
func (e *OptionalValueOrExpr) SourceSpan() Span                    { return e.Span }
func (e *PropagateExpr) SourceSpan() Span                          { return e.Span }
func (e *MatchExpr) SourceSpan() Span                              { return e.Span }
func (e *ListEmptyExpr) SourceSpan() Span                          { return e.Span }
func (e *ListSingletonExpr) SourceSpan() Span                      { return e.Span }
func (e *ListCountExpr) SourceSpan() Span                          { return e.Span }
func (e *ListAppendExpr) SourceSpan() Span                         { return e.Span }
func (e *ListAtExpr) SourceSpan() Span                             { return e.Span }
func (e *ListFindByTextExpr) SourceSpan() Span                     { return e.Span }
func (e *ListFilterByTextExpr) SourceSpan() Span                   { return e.Span }
func (e *ListFilterPredicateExpr) SourceSpan() Span                { return e.Span }
func (e *ListFilterContainsCaseFoldedExpr) SourceSpan() Span       { return e.Span }
func (e *ListFilterJoinedContainsCaseFoldedExpr) SourceSpan() Span { return e.Span }
func (e *ListSortByOrdinalExpr) SourceSpan() Span                  { return e.Span }
func (e *ListSortByOrdinalsExpr) SourceSpan() Span                 { return e.Span }
func (e *ListSortByOrdinalDirectionsExpr) SourceSpan() Span        { return e.Span }
func (e *ResultOKExpr) SourceSpan() Span                           { return e.Span }
func (e *ResultErrExpr) SourceSpan() Span                          { return e.Span }
func (e *ResultIsOKExpr) SourceSpan() Span                         { return e.Span }
func (e *ResultSuccessOrExpr) SourceSpan() Span                    { return e.Span }
func (e *ResultFailureOrExpr) SourceSpan() Span                    { return e.Span }

func setExprSpan(expr Expr, span Span) {
	switch node := expr.(type) {
	case *LiteralExpr:
		node.Span = span
	case *IdentExpr:
		node.Span = span
	case *UnaryExpr:
		node.Span = span
	case *BinaryExpr:
		node.Span = span
	case *ConditionalExpr:
		node.Span = span
	case *ImmutableLocalExpr:
		node.Span = span
	case *CallExpr:
		node.Span = span
	case *TextContainsCaseFoldedExpr:
		node.Span = span
	case *TextTrimExpr:
		node.Span = span
	case *FieldExpr:
		node.Span = span
	case *RecordConstructExpr:
		node.Span = span
	case *OptionalSomeExpr:
		node.Span = span
	case *OptionalNoneExpr:
		node.Span = span
	case *OptionalHasValueExpr:
		node.Span = span
	case *OptionalValueOrExpr:
		node.Span = span
	case *PropagateExpr:
		node.Span = span
	case *MatchExpr:
		node.Span = span
	case *ListEmptyExpr:
		node.Span = span
	case *ListSingletonExpr:
		node.Span = span
	case *ListCountExpr:
		node.Span = span
	case *ListAppendExpr:
		node.Span = span
	case *ListAtExpr:
		node.Span = span
	case *ListFindByTextExpr:
		node.Span = span
	case *ListFilterByTextExpr:
		node.Span = span
	case *ListFilterPredicateExpr:
		node.Span = span
	case *ListFilterContainsCaseFoldedExpr:
		node.Span = span
	case *ListFilterJoinedContainsCaseFoldedExpr:
		node.Span = span
	case *ListSortByOrdinalExpr:
		node.Span = span
	case *ListSortByOrdinalsExpr:
		node.Span = span
	case *ListSortByOrdinalDirectionsExpr:
		node.Span = span
	case *ResultOKExpr:
		node.Span = span
	case *ResultErrExpr:
		node.Span = span
	case *ResultIsOKExpr:
		node.Span = span
	case *ResultSuccessOrExpr:
		node.Span = span
	case *ResultFailureOrExpr:
		node.Span = span
	}
}

func expressionChildren(expr Expr) []Expr {
	switch node := expr.(type) {
	case *UnaryExpr:
		return []Expr{node.Expr}
	case *BinaryExpr:
		return []Expr{node.Left, node.Right}
	case *ConditionalExpr:
		return []Expr{node.Condition, node.WhenTrue, node.WhenFalse}
	case *ImmutableLocalExpr:
		return []Expr{node.Initializer, node.Return}
	case *CallExpr:
		return append([]Expr(nil), node.Arguments...)
	case *TextContainsCaseFoldedExpr:
		return []Expr{node.Value, node.Query}
	case *TextTrimExpr:
		return []Expr{node.Value}
	case *FieldExpr:
		return []Expr{node.Receiver}
	case *RecordConstructExpr:
		children := make([]Expr, 0, len(node.Fields))
		for _, field := range node.Fields {
			children = append(children, field.Value)
		}
		return children
	case *OptionalSomeExpr:
		return []Expr{node.Value}
	case *OptionalHasValueExpr:
		return []Expr{node.Value}
	case *OptionalValueOrExpr:
		return []Expr{node.Value, node.Fallback}
	case *PropagateExpr:
		return []Expr{node.Value}
	case *MatchExpr:
		children := []Expr{node.Value}
		for _, arm := range node.Arms {
			children = append(children, arm.Body)
		}
		return children
	case *ListSingletonExpr:
		return []Expr{node.Value}
	case *ListCountExpr:
		return []Expr{node.Value}
	case *ListAppendExpr:
		return []Expr{node.Values, node.Value}
	case *ListAtExpr:
		return []Expr{node.Values, node.Index}
	case *ListFindByTextExpr:
		return []Expr{node.Values, node.Key}
	case *ListFilterByTextExpr:
		return []Expr{node.Values, node.Key}
	case *ListFilterPredicateExpr:
		return append([]Expr{node.Values}, node.Arguments...)
	case *ListFilterContainsCaseFoldedExpr:
		return []Expr{node.Values, node.Query}
	case *ListFilterJoinedContainsCaseFoldedExpr:
		return []Expr{node.Values, node.Query}
	case *ListSortByOrdinalExpr:
		return []Expr{node.Values}
	case *ListSortByOrdinalsExpr:
		return []Expr{node.Values}
	case *ListSortByOrdinalDirectionsExpr:
		return []Expr{node.Values}
	case *ResultOKExpr:
		return []Expr{node.Value}
	case *ResultErrExpr:
		return []Expr{node.Error}
	case *ResultIsOKExpr:
		return []Expr{node.Value}
	case *ResultSuccessOrExpr:
		return []Expr{node.Value, node.Fallback}
	case *ResultFailureOrExpr:
		return []Expr{node.Value, node.Fallback}
	default:
		return nil
	}
}

func containsCallExpression(expr Expr) bool {
	if _, ok := expr.(*CallExpr); ok {
		return true
	}
	for _, child := range expressionChildren(expr) {
		if containsCallExpression(child) {
			return true
		}
	}
	return false
}

func containsConditionalExpression(expr Expr) bool {
	if _, ok := expr.(*ConditionalExpr); ok {
		return true
	}
	for _, child := range expressionChildren(expr) {
		if containsConditionalExpression(child) {
			return true
		}
	}
	return false
}

func countConditionalExpressions(expr Expr) int {
	count := 0
	if _, ok := expr.(*ConditionalExpr); ok {
		count++
	}
	for _, child := range expressionChildren(expr) {
		count += countConditionalExpressions(child)
	}
	return count
}

func countPropagationExpressions(expr Expr) int {
	count := 0
	if _, ok := expr.(*PropagateExpr); ok {
		count++
	}
	for _, child := range expressionChildren(expr) {
		count += countPropagationExpressions(child)
	}
	return count
}

func validConditionalOperand(expr Expr) bool {
	switch expr.(type) {
	case *ConditionalExpr, *PropagateExpr, *MatchExpr:
		return false
	}
	for _, child := range expressionChildren(expr) {
		if !validConditionalOperand(child) {
			return false
		}
	}
	return true
}

func validBoundedConditionalExpression(expr Expr) bool {
	if countConditionalExpressions(expr) != 1 {
		return false
	}
	valid := false
	var walk func(Expr)
	walk = func(current Expr) {
		if conditional, ok := current.(*ConditionalExpr); ok {
			valid = validConditionalOperand(conditional.Condition) && validConditionalOperand(conditional.WhenTrue) && validConditionalOperand(conditional.WhenFalse)
			return
		}
		for _, child := range expressionChildren(current) {
			walk(child)
		}
	}
	walk(expr)
	return valid
}

func containsTerminalIfStatement(expr Expr) bool {
	if conditional, ok := expr.(*ConditionalExpr); ok && conditional.TerminalStatement {
		return true
	}
	for _, child := range expressionChildren(expr) {
		if containsTerminalIfStatement(child) {
			return true
		}
	}
	return false
}

func countTerminalIfStatements(expr Expr) int {
	count := 0
	if conditional, ok := expr.(*ConditionalExpr); ok && conditional.TerminalStatement {
		count++
	}
	for _, child := range expressionChildren(expr) {
		count += countTerminalIfStatements(child)
	}
	return count
}

func validConditionalExpressions(expr Expr) bool {
	if conditional, ok := expr.(*ConditionalExpr); ok {
		if !validConditionalOperand(conditional.Condition) || !validConditionalOperand(conditional.WhenTrue) || !validConditionalOperand(conditional.WhenFalse) {
			return false
		}
	}
	for _, child := range expressionChildren(expr) {
		if !validConditionalExpressions(child) {
			return false
		}
	}
	return true
}

func countImmutableLocalExpressions(expr Expr) int {
	count := 0
	if _, ok := expr.(*ImmutableLocalExpr); ok {
		count++
	}
	for _, child := range expressionChildren(expr) {
		count += countImmutableLocalExpressions(child)
	}
	return count
}

func terminalBranchLocalShape(expr Expr, limit int) (int, bool) {
	count := 0
	current := expr
	for {
		local, ok := current.(*ImmutableLocalExpr)
		if !ok {
			return count, limit < 0 || count <= limit
		}
		count++
		if (limit >= 0 && count > limit) || local.Initializer == nil || local.Return == nil {
			return count, false
		}
		current = local.Return
	}
}

func terminalBranchTail(expr Expr) (int, Expr, bool) {
	count := 0
	current := expr
	for {
		local, ok := current.(*ImmutableLocalExpr)
		if !ok {
			return count, current, current != nil
		}
		count++
		if local.Initializer == nil || local.Return == nil || !validConditionalOperand(local.Initializer) {
			return count, nil, false
		}
		current = local.Return
	}
}

func validV740NestedTerminalIf(expr Expr) bool {
	outer, ok := expr.(*ConditionalExpr)
	if !ok || !outer.TerminalStatement || outer.Condition == nil || outer.WhenTrue == nil || outer.WhenFalse == nil || !validConditionalOperand(outer.Condition) {
		return false
	}
	validNested := func(branch Expr) bool {
		_, tail, valid := terminalBranchTail(branch)
		nested, ok := tail.(*ConditionalExpr)
		if !valid || !ok || !nested.TerminalStatement || nested.Condition == nil || nested.WhenTrue == nil || nested.WhenFalse == nil {
			return false
		}
		if _, local := nested.WhenTrue.(*ImmutableLocalExpr); local {
			return false
		}
		if _, local := nested.WhenFalse.(*ImmutableLocalExpr); local {
			return false
		}
		return validConditionalOperand(nested.Condition) && validConditionalOperand(nested.WhenTrue) && validConditionalOperand(nested.WhenFalse)
	}
	validOrdinary := func(branch Expr) bool {
		_, tail, valid := terminalBranchTail(branch)
		if !valid {
			return false
		}
		if _, nested := tail.(*ConditionalExpr); nested {
			return false
		}
		return validConditionalOperand(tail)
	}
	trueNested, falseNested := validNested(outer.WhenTrue), validNested(outer.WhenFalse)
	return countConditionalExpressions(expr) == 2 && countTerminalIfStatements(expr) == 2 &&
		((trueNested && validOrdinary(outer.WhenFalse)) || (falseNested && validOrdinary(outer.WhenTrue)))
}

func validV750NestedTerminalIf(expr Expr) bool {
	outer, ok := expr.(*ConditionalExpr)
	if !ok || !outer.TerminalStatement || outer.Condition == nil || outer.WhenTrue == nil || outer.WhenFalse == nil || !validConditionalOperand(outer.Condition) {
		return false
	}
	validInnerLeaf := func(branch Expr) bool {
		_, tail, valid := terminalBranchTail(branch)
		if !valid {
			return false
		}
		if _, nested := tail.(*ConditionalExpr); nested {
			return false
		}
		return validConditionalOperand(tail)
	}
	validNested := func(branch Expr) bool {
		_, tail, valid := terminalBranchTail(branch)
		nested, ok := tail.(*ConditionalExpr)
		if !valid || !ok || !nested.TerminalStatement || nested.Condition == nil || nested.WhenTrue == nil || nested.WhenFalse == nil {
			return false
		}
		return validConditionalOperand(nested.Condition) && validInnerLeaf(nested.WhenTrue) && validInnerLeaf(nested.WhenFalse)
	}
	validOrdinary := func(branch Expr) bool {
		_, tail, valid := terminalBranchTail(branch)
		if !valid {
			return false
		}
		if _, nested := tail.(*ConditionalExpr); nested {
			return false
		}
		return validConditionalOperand(tail)
	}
	trueNested, falseNested := validNested(outer.WhenTrue), validNested(outer.WhenFalse)
	return countConditionalExpressions(expr) == 2 && countTerminalIfStatements(expr) == 2 &&
		((trueNested && validOrdinary(outer.WhenFalse)) || (falseNested && validOrdinary(outer.WhenTrue)))
}

func validV760RootLocalNestedTerminalIf(expr Expr) bool {
	locals, tail, valid := terminalBranchTail(expr)
	return valid && locals > 0 && validV750NestedTerminalIf(tail)
}

func validV770SymmetricRootLocalNestedTerminalIf(expr Expr) bool {
	return validSymmetricNestedTerminalIf(expr, true)
}

func validSymmetricNestedTerminalIf(expr Expr, requireRoot bool) bool {
	locals, tail, valid := terminalBranchTail(expr)
	if !valid || (requireRoot && locals == 0) {
		return false
	}
	outer, ok := tail.(*ConditionalExpr)
	if !ok || !outer.TerminalStatement || outer.Condition == nil || outer.WhenTrue == nil || outer.WhenFalse == nil || !validConditionalOperand(outer.Condition) {
		return false
	}
	validInnerLeaf := func(branch Expr) bool {
		_, leaf, valid := terminalBranchTail(branch)
		if !valid {
			return false
		}
		if _, nested := leaf.(*ConditionalExpr); nested {
			return false
		}
		return validConditionalOperand(leaf)
	}
	validNested := func(branch Expr) bool {
		_, nestedTail, valid := terminalBranchTail(branch)
		nested, ok := nestedTail.(*ConditionalExpr)
		if !valid || !ok || !nested.TerminalStatement || nested.Condition == nil || nested.WhenTrue == nil || nested.WhenFalse == nil {
			return false
		}
		return validConditionalOperand(nested.Condition) && validInnerLeaf(nested.WhenTrue) && validInnerLeaf(nested.WhenFalse)
	}
	return countConditionalExpressions(expr) == 3 && countTerminalIfStatements(expr) == 3 && validNested(outer.WhenTrue) && validNested(outer.WhenFalse)
}

func validV790BoundedDepthThreeTerminalIf(expr Expr) bool {
	return validExpandedDepthThreeTerminalIf(expr, 1)
}

func validV800TwoExpandedTerminalIf(expr Expr) bool {
	return validExpandedDepthThreeTerminalIf(expr, 2)
}

func validExpandedDepthThreeTerminalIf(expr Expr, expandedLeaves int) bool {
	_, tail, valid := terminalBranchTail(expr)
	outer, ok := tail.(*ConditionalExpr)
	if !valid || !ok || !outer.TerminalStatement || outer.Condition == nil || outer.WhenTrue == nil || outer.WhenFalse == nil || !validConditionalOperand(outer.Condition) {
		return false
	}
	validLeaf := func(branch Expr) bool {
		_, leaf, valid := terminalBranchTail(branch)
		if !valid {
			return false
		}
		if _, nested := leaf.(*ConditionalExpr); nested {
			return false
		}
		return validConditionalOperand(leaf)
	}
	classifyDepthTwoLeaf := func(branch Expr) (int, bool) {
		_, leaf, valid := terminalBranchTail(branch)
		if !valid {
			return 0, false
		}
		third, nested := leaf.(*ConditionalExpr)
		if !nested {
			return 0, validConditionalOperand(leaf)
		}
		if !third.TerminalStatement || third.Condition == nil || third.WhenTrue == nil || third.WhenFalse == nil || !validConditionalOperand(third.Condition) {
			return 0, false
		}
		return 1, validLeaf(third.WhenTrue) && validLeaf(third.WhenFalse)
	}
	classifyInner := func(branch Expr) (int, bool) {
		_, nestedTail, valid := terminalBranchTail(branch)
		inner, ok := nestedTail.(*ConditionalExpr)
		if !valid || !ok || !inner.TerminalStatement || inner.Condition == nil || inner.WhenTrue == nil || inner.WhenFalse == nil || !validConditionalOperand(inner.Condition) {
			return 0, false
		}
		trueThird, validTrue := classifyDepthTwoLeaf(inner.WhenTrue)
		falseThird, validFalse := classifyDepthTwoLeaf(inner.WhenFalse)
		return trueThird + falseThird, validTrue && validFalse
	}
	trueThird, validTrue := classifyInner(outer.WhenTrue)
	falseThird, validFalse := classifyInner(outer.WhenFalse)
	return validTrue && validFalse && trueThird+falseThird == expandedLeaves && countConditionalExpressions(expr) == 3+expandedLeaves && countTerminalIfStatements(expr) == 3+expandedLeaves
}

func validTerminalIfStatement(contract LanguageContract, expr Expr) bool {
	if contract == PipeLangLanguageContractV1090 && validTerminalCombinedSelectorArms(expr) {
		return true
	}
	if (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090) && validTerminalInnerSelectorArms(expr) {
		return true
	}
	if (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090)) && validTerminalSelectorValueArms(expr) {
		return true
	}
	if (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090))) && validTerminalBooleanSelectorTests(expr) {
		return true
	}
	if (contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090)))) && validDepthThreeTerminalConditionalTests(expr) {
		return true
	}
	if (contract == PipeLangLanguageContractV1040 || (contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090))))) && validNestedTerminalConditionalTests(expr) {
		return true
	}
	if (contract == PipeLangLanguageContractV1030 || (contract == PipeLangLanguageContractV1040 || (contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090)))))) && validTerminalConditionalTests(expr) {
		return true
	}
	if (contract == PipeLangLanguageContractV1020 || (contract == PipeLangLanguageContractV1030 || (contract == PipeLangLanguageContractV1040 || (contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090))))))) && validTerminalBooleanSelectorInitializers(expr) {
		return true
	}
	if (contract == PipeLangLanguageContractV990 || (contract == PipeLangLanguageContractV1000 || (contract == PipeLangLanguageContractV1010 || (contract == PipeLangLanguageContractV1020 || (contract == PipeLangLanguageContractV1030 || (contract == PipeLangLanguageContractV1040 || (contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090)))))))))) && validTerminalLeafBooleanSelectors(expr) {
		return true
	}
	if (contract == PipeLangLanguageContractV970 || (contract == PipeLangLanguageContractV980 || (contract == PipeLangLanguageContractV990 || (contract == PipeLangLanguageContractV1000 || (contract == PipeLangLanguageContractV1010 || (contract == PipeLangLanguageContractV1020 || (contract == PipeLangLanguageContractV1030 || (contract == PipeLangLanguageContractV1040 || (contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090)))))))))))) && validDepthThreeTerminalInitializers(expr) {
		return true
	}
	if (contract == PipeLangLanguageContractV940 || (contract == PipeLangLanguageContractV950 || (contract == PipeLangLanguageContractV960 || (contract == PipeLangLanguageContractV970 || (contract == PipeLangLanguageContractV980 || (contract == PipeLangLanguageContractV990 || (contract == PipeLangLanguageContractV1000 || (contract == PipeLangLanguageContractV1010 || (contract == PipeLangLanguageContractV1020 || (contract == PipeLangLanguageContractV1030 || (contract == PipeLangLanguageContractV1040 || (contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090))))))))))))))) && validDepthThreeTerminalLeafReturns(expr) {
		return true
	}
	if (contract == PipeLangLanguageContractV910 || (contract == PipeLangLanguageContractV920 || (contract == PipeLangLanguageContractV930 || (contract == PipeLangLanguageContractV940 || (contract == PipeLangLanguageContractV950 || (contract == PipeLangLanguageContractV960 || (contract == PipeLangLanguageContractV970 || (contract == PipeLangLanguageContractV980 || (contract == PipeLangLanguageContractV990 || (contract == PipeLangLanguageContractV1000 || (contract == PipeLangLanguageContractV1010 || (contract == PipeLangLanguageContractV1020 || (contract == PipeLangLanguageContractV1030 || (contract == PipeLangLanguageContractV1040 || (contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090)))))))))))))))))) && validNestedTerminalInitializers(expr) {
		return true
	}
	if (contract == PipeLangLanguageContractV890 || (contract == PipeLangLanguageContractV900 || (contract == PipeLangLanguageContractV910 || (contract == PipeLangLanguageContractV920 || (contract == PipeLangLanguageContractV930 || (contract == PipeLangLanguageContractV940 || (contract == PipeLangLanguageContractV950 || (contract == PipeLangLanguageContractV960 || (contract == PipeLangLanguageContractV970 || (contract == PipeLangLanguageContractV980 || (contract == PipeLangLanguageContractV990 || (contract == PipeLangLanguageContractV1000 || (contract == PipeLangLanguageContractV1010 || (contract == PipeLangLanguageContractV1020 || (contract == PipeLangLanguageContractV1030 || (contract == PipeLangLanguageContractV1040 || (contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090)))))))))))))))))))) && validNestedTerminalLeafReturns(expr) {
		return true
	}
	if ((contract == PipeLangLanguageContractV890 || (contract == PipeLangLanguageContractV900 || (contract == PipeLangLanguageContractV910 || (contract == PipeLangLanguageContractV920 || (contract == PipeLangLanguageContractV930 || (contract == PipeLangLanguageContractV940 || (contract == PipeLangLanguageContractV950 || (contract == PipeLangLanguageContractV960 || (contract == PipeLangLanguageContractV970 || (contract == PipeLangLanguageContractV980 || (contract == PipeLangLanguageContractV990 || (contract == PipeLangLanguageContractV1000 || (contract == PipeLangLanguageContractV1010 || (contract == PipeLangLanguageContractV1020 || (contract == PipeLangLanguageContractV1030 || (contract == PipeLangLanguageContractV1040 || (contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090)))))))))))))))))))) || contract == PipeLangLanguageContractV880) || contract == PipeLangLanguageContractV870 {
		return validTerminalLeafConditionalReturns(expr) || validTerminalIfStatement(PipeLangLanguageContractV860, expr)
	}
	if contract == PipeLangLanguageContractV860 || contract == PipeLangLanguageContractV850 || contract == PipeLangLanguageContractV840 {
		return validConditionalLocalTree(expr, 0) || validTerminalIfStatement(PipeLangLanguageContractV830, expr)
	}
	if contract == PipeLangLanguageContractV830 {
		return validConditionalLocalTree(expr, 2) || validTerminalIfStatement(PipeLangLanguageContractV820, expr)
	}
	if contract == PipeLangLanguageContractV820 {
		return validV820ConditionalLocalTree(expr) || validTerminalIfStatement(PipeLangLanguageContractV810, expr)
	}
	if contract == PipeLangLanguageContractV810 {
		return validV810TerminalTree(expr, 3) || validTerminalIfStatement(PipeLangLanguageContractV800, expr)
	}
	if contract == PipeLangLanguageContractV800 {
		return validTerminalIfStatement(PipeLangLanguageContractV790, expr) || validV800TwoExpandedTerminalIf(expr)
	}
	if contract == PipeLangLanguageContractV790 {
		return validTerminalIfStatement(PipeLangLanguageContractV780, expr) || validV790BoundedDepthThreeTerminalIf(expr)
	}
	if contract == PipeLangLanguageContractV780 {
		return validTerminalIfStatement(PipeLangLanguageContractV770, expr) || validSymmetricNestedTerminalIf(expr, false)
	}
	if contract == PipeLangLanguageContractV770 {
		return validTerminalIfStatement(PipeLangLanguageContractV760, expr) || validV770SymmetricRootLocalNestedTerminalIf(expr)
	}
	if contract == PipeLangLanguageContractV760 {
		return validTerminalIfStatement(PipeLangLanguageContractV750, expr) || validV760RootLocalNestedTerminalIf(expr)
	}
	if contract == PipeLangLanguageContractV750 {
		return validTerminalIfStatement(PipeLangLanguageContractV730, expr) || validV750NestedTerminalIf(expr)
	}
	if contract == PipeLangLanguageContractV740 {
		return validTerminalIfStatement(PipeLangLanguageContractV730, expr) || validV740NestedTerminalIf(expr)
	}
	locals := 0
	current := expr
	for {
		local, ok := current.(*ImmutableLocalExpr)
		if !ok {
			break
		}
		locals++
		current = local.Return
	}
	conditional, ok := current.(*ConditionalExpr)
	if !ok || !conditional.TerminalStatement {
		return false
	}
	branchLocalLimit := 0
	if hasBranchLocalSourceContract(contract) {
		branchLocalLimit = 1
	}
	if hasTwoBranchLocalsSourceContract(contract) {
		branchLocalLimit = 2
	}
	if hasGeneralBranchLocalSequenceSourceContract(contract) {
		branchLocalLimit = -1
	}
	trueLocals, validTrue := terminalBranchLocalShape(conditional.WhenTrue, branchLocalLimit)
	falseLocals, validFalse := terminalBranchLocalShape(conditional.WhenFalse, branchLocalLimit)
	branchLocals := trueLocals + falseLocals
	conditionalCount := countConditionalExpressions(expr)
	minimumTopLevelLocals := 1
	if contract == PipeLangLanguageContractV730 {
		minimumTopLevelLocals = 0
	}
	return locals >= minimumTopLevelLocals && countImmutableLocalExpressions(expr) == locals+branchLocals &&
		countTerminalIfStatements(expr) == 1 && conditionalCount >= 1 && conditionalCount <= 2 &&
		validTrue && validFalse && validConditionalExpressions(expr)
}

// The new topology rule only widens terminal trees. Earlier expression/statement
// combinations still pass through their own versioned rules.
func validV810TerminalTree(expr Expr, remaining int) bool {
	_, tail, valid := terminalBranchTail(expr)
	if !valid || tail == nil {
		return false
	}
	conditional, ok := tail.(*ConditionalExpr)
	if !ok {
		return validConditionalOperand(tail)
	}
	return remaining > 0 && conditional.TerminalStatement && conditional.Condition != nil &&
		conditional.WhenTrue != nil && conditional.WhenFalse != nil &&
		validConditionalOperand(conditional.Condition) &&
		validV810TerminalTree(conditional.WhenTrue, remaining-1) &&
		validV810TerminalTree(conditional.WhenFalse, remaining-1)
}

func validPureCallPlacement(expr Expr) bool {
	if !containsCallExpression(expr) {
		return true
	}
	call, ok := expr.(*CallExpr)
	if !ok {
		return false
	}
	for _, argument := range call.Arguments {
		if !validPureCallPlacement(argument) {
			return false
		}
	}
	return true
}

// validGeneralPureCallPlacement admits calls anywhere in the existing pure
// expression tree while retaining the direct-carrier boundaries established
// for propagation and matching.
func validGeneralPureCallPlacement(expr Expr) bool {
	switch node := expr.(type) {
	case *PropagateExpr:
		_, direct := node.Value.(*IdentExpr)
		return direct
	case *MatchExpr:
		if _, direct := node.Value.(*IdentExpr); !direct {
			return false
		}
		for _, arm := range node.Arms {
			if !validGeneralPureCallPlacement(arm.Body) {
				return false
			}
		}
		return true
	default:
		for _, child := range expressionChildren(expr) {
			if !validGeneralPureCallPlacement(child) {
				return false
			}
		}
		return true
	}
}

// validHelperCarrierMatchPureCallPlacement widens only the complete
// method-body match carrier to one helper call over one or more direct
// references. Exact caller-to-helper parameter correspondence is checked by
// semantic analysis. Arm expressions retain v0.37.0 general pure-call
// composition and its direct nested-control-flow carrier boundaries.
func validHelperCarrierMatchPureCallPlacement(expr Expr) bool {
	match, ok := expr.(*MatchExpr)
	if !ok {
		return validGeneralPureCallPlacement(expr)
	}
	call, ok := match.Value.(*CallExpr)
	if !ok || len(call.Arguments) == 0 {
		return validGeneralPureCallPlacement(expr)
	}
	for _, argument := range call.Arguments {
		if _, direct := argument.(*IdentExpr); !direct {
			return false
		}
	}
	for _, arm := range match.Arms {
		if !validGeneralPureCallPlacement(arm.Body) {
			return false
		}
	}
	return true
}

// validHelperCarrierMatchLocalPureCallPlacement widens only the first
// immutable-local initializer to one v0.44-compatible helper-carrier match.
// Existing top-level helper matches and all other v0.44 call placement remain
// valid without allowing matches in later locals or the terminal return.
func validHelperCarrierMatchLocalPureCallPlacement(expr Expr) bool {
	local, ok := expr.(*ImmutableLocalExpr)
	if !ok {
		return validHelperCarrierMatchPureCallPlacement(expr)
	}
	if match, matched := local.Initializer.(*MatchExpr); matched {
		if !validHelperCarrierMatchPureCallPlacement(match) {
			return false
		}
		return validGeneralPureCallPlacement(local.Return)
	}
	return validHelperCarrierMatchPureCallPlacement(expr)
}

// validHelperCarrierMatchLaterLocalPureCallPlacement preserves the inherited
// complete-body helper match while admitting one helper-carrier match as any
// immutable-local initializer in the existing ordered local sequence. The
// terminal return retains the general v0.37 call-placement rules, so a helper
// match cannot enter there by implication.
func validHelperCarrierMatchLaterLocalPureCallPlacement(expr Expr) bool {
	local, ok := expr.(*ImmutableLocalExpr)
	if !ok {
		return validHelperCarrierMatchPureCallPlacement(expr)
	}
	return validHelperCarrierMatchLocalSequence(local)
}

func validHelperCarrierMatchLocalSequence(local *ImmutableLocalExpr) bool {
	if match, matched := local.Initializer.(*MatchExpr); matched {
		if !validHelperCarrierMatchPureCallPlacement(match) {
			return false
		}
	} else if !validGeneralPureCallPlacement(local.Initializer) {
		return false
	}
	if next, ok := local.Return.(*ImmutableLocalExpr); ok {
		return validHelperCarrierMatchLocalSequence(next)
	}
	return validGeneralPureCallPlacement(local.Return)
}

func countMatchExpressions(expr Expr) int {
	count := 0
	if _, ok := expr.(*MatchExpr); ok {
		count++
	}
	for _, child := range expressionChildren(expr) {
		count += countMatchExpressions(child)
	}
	return count
}

type Value struct {
	Type   PrimitiveType
	String string
	Int    int64
	Float  float64
	Bool   bool
}

func (v Value) StringValue() string {
	switch v.Type {
	case TypeString:
		return v.String
	case TypeInt:
		return fmt.Sprintf("%d", v.Int)
	case TypeFloat:
		return fmt.Sprintf("%g", v.Float)
	case TypeBool:
		if v.Bool {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func ZeroValue(t PrimitiveType) Value {
	switch t {
	case TypeString:
		return Value{Type: TypeString, String: ""}
	case TypeInt:
		return Value{Type: TypeInt, Int: 0}
	case TypeFloat:
		return Value{Type: TypeFloat, Float: 0}
	case TypeBool:
		return Value{Type: TypeBool, Bool: false}
	default:
		return Value{}
	}
}

// Count value choices across the whole method, including unselected scopes.
// Terminal depth is separate from this nonterminal conditional.
func validV820ConditionalLocalTree(expr Expr) bool {
	return validConditionalLocalTree(expr, 1)
}

func validConditionalLocalTree(expr Expr, maxChoices int) bool {
	// A zero bound admits finite sequences; historical callers retain their bounds.
	choices := 0
	hasChoices := false
	var walk func(Expr, int) bool
	walk = func(current Expr, remaining int) bool {
		for {
			local, ok := current.(*ImmutableLocalExpr)
			if !ok {
				break
			}
			if local.Initializer == nil || local.Return == nil {
				return false
			}
			if choice, ok := local.Initializer.(*ConditionalExpr); ok {
				hasChoices = true
				if maxChoices > 0 {
					choices++
				}
				if (maxChoices > 0 && choices > maxChoices) || choice.TerminalStatement || choice.Condition == nil || choice.WhenTrue == nil || choice.WhenFalse == nil ||
					!validConditionalOperand(choice.Condition) || !validConditionalOperand(choice.WhenTrue) || !validConditionalOperand(choice.WhenFalse) {
					return false
				}
			} else if !validConditionalOperand(local.Initializer) {
				return false
			}
			current = local.Return
		}
		if current == nil {
			return false
		}
		branch, ok := current.(*ConditionalExpr)
		if !ok {
			return validConditionalOperand(current)
		}
		return remaining > 0 && branch.TerminalStatement && branch.Condition != nil && branch.WhenTrue != nil && branch.WhenFalse != nil &&
			validConditionalOperand(branch.Condition) && walk(branch.WhenTrue, remaining-1) && walk(branch.WhenFalse, remaining-1)
	}
	return containsTerminalIfStatement(expr) && walk(expr, 3) && hasChoices
}

// This additional placement leaves inherited single-choice expressions unchanged.
func validStraightLineConditionalLocals(expr Expr) bool {
	hasChoice := false
	for {
		local, ok := expr.(*ImmutableLocalExpr)
		if !ok {
			return hasChoice && expr != nil && validConditionalOperand(expr)
		}
		if local.Initializer == nil || local.Return == nil {
			return false
		}
		if choice, ok := local.Initializer.(*ConditionalExpr); ok {
			if choice.TerminalStatement || choice.Condition == nil || choice.WhenTrue == nil || choice.WhenFalse == nil || !validConditionalOperand(choice.Condition) || !validConditionalOperand(choice.WhenTrue) || !validConditionalOperand(choice.WhenFalse) {
				return false
			}
			hasChoice = true
		} else if !validConditionalOperand(local.Initializer) {
			return false
		}
		expr = local.Return
	}
}

// v0.86 adds only a complete return choice after a nonempty local sequence.
func validConditionalReturnComposition(expr Expr) bool {
	hasLocal := false
	for {
		local, ok := expr.(*ImmutableLocalExpr)
		if !ok {
			break
		}
		hasLocal = true
		if local.Initializer == nil || local.Return == nil {
			return false
		}
		if choice, ok := local.Initializer.(*ConditionalExpr); ok {
			if !validReturnCompositionChoice(choice) {
				return false
			}
		} else if !validConditionalOperand(local.Initializer) {
			return false
		}
		expr = local.Return
	}
	choice, ok := expr.(*ConditionalExpr)
	return hasLocal && ok && validReturnCompositionChoice(choice)
}

func validReturnCompositionChoice(choice *ConditionalExpr) bool {
	return choice != nil && !choice.TerminalStatement && choice.Condition != nil && choice.WhenTrue != nil && choice.WhenFalse != nil &&
		validConditionalOperand(choice.Condition) && validConditionalOperand(choice.WhenTrue) && validConditionalOperand(choice.WhenFalse)
}

// v0.88 adds a depth-two value choice only at the tail of a straight-line body.
// Initializers and conditions retain the inherited nonnested operand contracts.
func validNestedStraightLineReturns(expr Expr) bool {
	for {
		local, ok := expr.(*ImmutableLocalExpr)
		if !ok {
			break
		}
		if local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(local.Initializer) != 0 {
			return false
		}
		if choice, ok := local.Initializer.(*ConditionalExpr); ok {
			if !validReturnCompositionChoice(choice) {
				return false
			}
		} else if !validConditionalOperand(local.Initializer) {
			return false
		}
		expr = local.Return
	}
	if _, ok := expr.(*ConditionalExpr); !ok || countImmutableLocalExpressions(expr) != 0 {
		return false
	}
	var walk func(Expr, int) bool
	walk = func(current Expr, remaining int) bool {
		choice, ok := current.(*ConditionalExpr)
		if !ok {
			return current != nil && validConditionalOperand(current)
		}
		return choice != nil && remaining > 0 && !choice.TerminalStatement &&
			choice.Condition != nil && validConditionalOperand(choice.Condition) &&
			walk(choice.WhenTrue, remaining-1) && walk(choice.WhenFalse, remaining-1)
	}
	return walk(expr, 2)
}

// v0.90 permits complete depth-two initializers only in a straight-line local
// sequence. Reuse the value-choice bound, never the statement-tree admission.
func validNestedStraightLineInitializers(expr Expr) bool {
	hasLocal := false
	for {
		local, ok := expr.(*ImmutableLocalExpr)
		if !ok {
			break
		}
		hasLocal = true
		if local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(local.Initializer) != 0 {
			return false
		}
		if !validNestedStraightLineReturns(local.Initializer) && !validConditionalOperand(local.Initializer) {
			return false
		}
		expr = local.Return
	}
	return hasLocal && expr != nil && countImmutableLocalExpressions(expr) == 0 &&
		(validNestedStraightLineReturns(expr) || validConditionalOperand(expr))
}

// v0.87 counts statement depth independently of value choices at return leaves.
func validTerminalLeafConditionalReturns(expr Expr) bool {
	return validTerminalLeafReturnsDepth(expr, 1, 1)
}

func validNestedTerminalLeafReturns(expr Expr) bool {
	return validTerminalLeafReturnsDepth(expr, 2, 1)
}

// v0.91 counts statement depth separately from complete initializer/return choices.
func validNestedTerminalInitializers(expr Expr) bool {
	return validTerminalLeafReturnsDepth(expr, 2, 2)
}

func validTerminalLeafReturnsDepth(expr Expr, returnDepth, initializerDepth int) bool {
	return validTerminalLeafReturnsWithSelectors(expr, returnDepth, initializerDepth, false)
}

// v0.99 adds the flat selector only at complete returns in inherited terminal trees.
func validTerminalLeafBooleanSelectors(expr Expr) bool {
	return validTerminalLeafReturnsWithSelectors(expr, 3, 3, true)
}

func validTerminalLeafReturnsWithSelectors(expr Expr, returnDepth, initializerDepth int, allowSelectors bool) bool {
	hasReturnChoice := false
	var walk func(Expr, int) bool
	walk = func(current Expr, remaining int) bool {
		for {
			local, ok := current.(*ImmutableLocalExpr)
			if !ok {
				break
			}
			if local.Initializer == nil || local.Return == nil {
				return false
			}
			if returnDepth >= 2 && countImmutableLocalExpressions(local.Initializer) != 0 {
				return false
			}
			if choice, ok := local.Initializer.(*ConditionalExpr); ok {
				if (initializerDepth == 3 && !validDepthThreeStraightLineReturns(local.Initializer)) || (initializerDepth == 2 && !validNestedStraightLineReturns(local.Initializer)) || (initializerDepth == 1 && !validReturnCompositionChoice(choice)) {
					return false
				}
			} else if !validConditionalOperand(local.Initializer) {
				return false
			}
			current = local.Return
		}
		if current == nil {
			return false
		}
		choice, ok := current.(*ConditionalExpr)
		if !ok {
			return validConditionalOperand(current) && (returnDepth == 1 || countImmutableLocalExpressions(current) == 0)
		}
		if !choice.TerminalStatement {
			hasReturnChoice = true
			if allowSelectors && validConditionalBooleanSelector(current) {
				return true
			}
			if returnDepth == 3 {
				return validDepthThreeStraightLineReturns(current)
			}
			if returnDepth == 2 {
				return validNestedStraightLineReturns(current)
			}
			return validReturnCompositionChoice(choice)
		}
		return remaining > 0 && choice.Condition != nil && choice.WhenTrue != nil && choice.WhenFalse != nil &&
			validConditionalOperand(choice.Condition) && (returnDepth == 1 || countImmutableLocalExpressions(choice.Condition) == 0) && walk(choice.WhenTrue, remaining-1) && walk(choice.WhenFalse, remaining-1)
	}
	return containsTerminalIfStatement(expr) && walk(expr, 3) && (hasReturnChoice || initializerDepth >= 2)
}

// Only the complete return of a straight-line method gains a third choice level.
// Local initializers retain the inherited depth-two, local-free operand contract.
func validDepthThreeStraightLineReturns(expr Expr) bool {
	for {
		local, ok := expr.(*ImmutableLocalExpr)
		if !ok {
			break
		}
		if local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(local.Initializer) != 0 ||
			(!validConditionalOperand(local.Initializer) && !validNestedStraightLineReturns(local.Initializer)) {
			return false
		}
		expr = local.Return
	}
	if _, ok := expr.(*ConditionalExpr); !ok || countImmutableLocalExpressions(expr) != 0 {
		return false
	}
	var walk func(Expr, int) bool
	walk = func(current Expr, remaining int) bool {
		choice, ok := current.(*ConditionalExpr)
		if !ok {
			return current != nil && validConditionalOperand(current)
		}
		return choice != nil && remaining > 0 && !choice.TerminalStatement && choice.Condition != nil &&
			validConditionalOperand(choice.Condition) && walk(choice.WhenTrue, remaining-1) && walk(choice.WhenFalse, remaining-1)
	}
	return walk(expr, 3)
}

// v0.94 widens complete terminal-leaf returns only; initializer and statement
// depths remain independently bounded. Hidden local operands remain forbidden.
func validDepthThreeTerminalLeafReturns(expr Expr) bool {
	return validTerminalLeafReturnsDepth(expr, 3, 2)
}

// v0.96 admits depth-three complete initializers only in straight-line local sequences.
func validDepthThreeStraightLineInitializers(expr Expr) bool {
	hasLocal := false
	for {
		local, ok := expr.(*ImmutableLocalExpr)
		if !ok {
			break
		}
		hasLocal = true
		if local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(local.Initializer) != 0 ||
			(!validConditionalOperand(local.Initializer) && !validDepthThreeStraightLineReturns(local.Initializer)) {
			return false
		}
		expr = local.Return
	}
	return hasLocal && expr != nil && countImmutableLocalExpressions(expr) == 0 &&
		(validConditionalOperand(expr) || validDepthThreeStraightLineReturns(expr))
}

// v0.97 widens initializer placement within inherited depth-three terminal trees.
// Conditions and operands remain local-free; return and statement limits do not grow.
func validDepthThreeTerminalInitializers(expr Expr) bool {
	return validTerminalLeafReturnsDepth(expr, 3, 3)
}

// v0.98 admits one flat boolean selector only at the complete straight-line return.
// Initializers keep the inherited depth-three limit; no hidden local operands enter.
func validConditionalBooleanSelector(expr Expr) bool {
	for {
		local, ok := expr.(*ImmutableLocalExpr)
		if !ok {
			break
		}
		if local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(local.Initializer) != 0 ||
			(!validConditionalOperand(local.Initializer) && !validDepthThreeStraightLineReturns(local.Initializer)) {
			return false
		}
		expr = local.Return
	}
	outer, ok := expr.(*ConditionalExpr)
	if !ok || outer == nil || outer.TerminalStatement || countImmutableLocalExpressions(expr) != 0 {
		return false
	}
	selector, ok := outer.Condition.(*ConditionalExpr)
	if !ok || selector == nil || selector.TerminalStatement {
		return false
	}
	for _, operand := range []Expr{selector.Condition, selector.WhenTrue, selector.WhenFalse, outer.WhenTrue, outer.WhenFalse} {
		if operand == nil || !validConditionalOperand(operand) {
			return false
		}
	}
	return true
}

// v0.101 adds complete flat selector initializers in straight-line methods only.
// Require an actual new initializer; inherited shapes retain their existing gates.
func validStraightLineBooleanSelectorInitializers(expr Expr) bool {
	hasSelector := false
	for {
		local, ok := expr.(*ImmutableLocalExpr)
		if !ok {
			break
		}
		if local == nil || local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(local.Initializer) != 0 {
			return false
		}
		selector := validConditionalBooleanSelector(local.Initializer)
		if !selector && !validConditionalOperand(local.Initializer) && !validDepthThreeStraightLineReturns(local.Initializer) {
			return false
		}
		hasSelector = hasSelector || selector
		expr = local.Return
	}
	return hasSelector && expr != nil && countImmutableLocalExpressions(expr) == 0 &&
		(validConditionalOperand(expr) || validDepthThreeStraightLineReturns(expr) || validConditionalBooleanSelector(expr))
}

// v0.102 admits flat selector initializers in existing terminal-tree scopes.
// Count statement depth independently of local sequences and expression choices.
func validTerminalBooleanSelectorInitializers(expr Expr) bool {
	hasSelector, hasStatement := false, false
	var validScope func(Expr, int) bool
	validScope = func(expr Expr, depth int) bool {
		for {
			local, ok := expr.(*ImmutableLocalExpr)
			if !ok {
				break
			}
			if local == nil || local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(local.Initializer) != 0 {
				return false
			}
			selector := validConditionalBooleanSelector(local.Initializer)
			if !selector && !validConditionalOperand(local.Initializer) && !validDepthThreeStraightLineReturns(local.Initializer) {
				return false
			}
			hasSelector = hasSelector || selector
			expr = local.Return
		}
		if branch, ok := expr.(*ConditionalExpr); ok && branch != nil && branch.TerminalStatement {
			hasStatement = true
			return depth < 3 && branch.Condition != nil && countImmutableLocalExpressions(branch.Condition) == 0 && validConditionalOperand(branch.Condition) &&
				validScope(branch.WhenTrue, depth+1) && validScope(branch.WhenFalse, depth+1)
		}
		return expr != nil && countImmutableLocalExpressions(expr) == 0 &&
			(validConditionalOperand(expr) || validDepthThreeStraightLineReturns(expr) || validConditionalBooleanSelector(expr))
	}
	return validScope(expr, 0) && hasSelector && hasStatement
}

// v0.103 permits flat conditional tests at existing terminal statement positions.
// Local sequences and inherited value expressions do not increase statement depth.
func validTerminalConditionalTests(expr Expr) bool {
	hasConditionalTest := false
	var validScope func(Expr, int) bool
	validScope = func(expr Expr, depth int) bool {
		for {
			local, ok := expr.(*ImmutableLocalExpr)
			if !ok {
				break
			}
			if local == nil || local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(local.Initializer) != 0 {
				return false
			}
			selector := validConditionalBooleanSelector(local.Initializer)
			if !selector && !validConditionalOperand(local.Initializer) && !validDepthThreeStraightLineReturns(local.Initializer) {
				return false
			}
			expr = local.Return
		}
		if branch, ok := expr.(*ConditionalExpr); ok && branch != nil && branch.TerminalStatement {
			conditional := validFlatConditionalTest(branch.Condition)
			hasConditionalTest = hasConditionalTest || conditional
			return depth < 3 && branch.Condition != nil && countImmutableLocalExpressions(branch.Condition) == 0 && (validConditionalOperand(branch.Condition) || conditional) &&
				validScope(branch.WhenTrue, depth+1) && validScope(branch.WhenFalse, depth+1)
		}
		return expr != nil && countImmutableLocalExpressions(expr) == 0 &&
			(validConditionalOperand(expr) || validDepthThreeStraightLineReturns(expr) || validConditionalBooleanSelector(expr))
	}
	return validScope(expr, 0) && hasConditionalTest
}

func validFlatConditionalTest(expr Expr) bool {
	c, ok := expr.(*ConditionalExpr)
	if !ok || c == nil || c.TerminalStatement || countImmutableLocalExpressions(expr) != 0 {
		return false
	}
	for _, operand := range []Expr{c.Condition, c.WhenTrue, c.WhenFalse} {
		if operand == nil || !validConditionalOperand(operand) {
			return false
		}
	}
	return true
}

// v0.104 permits depth-two value arms in terminal tests; selector nesting stays excluded.
func validNestedTerminalConditionalTests(expr Expr) bool {
	hasConditionalTest := false
	var validScope func(Expr, int) bool
	validScope = func(expr Expr, depth int) bool {
		for {
			local, ok := expr.(*ImmutableLocalExpr)
			if !ok {
				break
			}
			if local == nil || local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(local.Initializer) != 0 {
				return false
			}
			selector := validConditionalBooleanSelector(local.Initializer)
			if !selector && !validConditionalOperand(local.Initializer) && !validDepthThreeStraightLineReturns(local.Initializer) {
				return false
			}
			expr = local.Return
		}
		if branch, ok := expr.(*ConditionalExpr); ok && branch != nil && branch.TerminalStatement {
			conditional := validNestedConditionalTest(branch.Condition)
			hasConditionalTest = hasConditionalTest || conditional
			return depth < 3 && branch.Condition != nil && countImmutableLocalExpressions(branch.Condition) == 0 && (validConditionalOperand(branch.Condition) || conditional) &&
				validScope(branch.WhenTrue, depth+1) && validScope(branch.WhenFalse, depth+1)
		}
		return expr != nil && countImmutableLocalExpressions(expr) == 0 &&
			(validConditionalOperand(expr) || validDepthThreeStraightLineReturns(expr) || validConditionalBooleanSelector(expr))
	}
	return validScope(expr, 0) && hasConditionalTest
}

func validNestedConditionalTest(expr Expr) bool {
	c, ok := expr.(*ConditionalExpr)
	if !ok || c == nil || c.TerminalStatement || countImmutableLocalExpressions(expr) != 0 {
		return false
	}
	if c.Condition == nil || !validConditionalOperand(c.Condition) {
		return false
	}
	for _, arm := range []Expr{c.WhenTrue, c.WhenFalse} {
		if arm == nil || (!validConditionalOperand(arm) && !validFlatConditionalTest(arm)) {
			return false
		}
	}
	return true
}

// v0.105 permits depth-three value arms in terminal tests; selector nesting stays excluded.
func validDepthThreeTerminalConditionalTests(expr Expr) bool {
	hasConditionalTest := false
	var validScope func(Expr, int) bool
	validScope = func(expr Expr, depth int) bool {
		for {
			local, ok := expr.(*ImmutableLocalExpr)
			if !ok {
				break
			}
			if local == nil || local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(local.Initializer) != 0 {
				return false
			}
			selector := validConditionalBooleanSelector(local.Initializer)
			if !selector && !validConditionalOperand(local.Initializer) && !validDepthThreeStraightLineReturns(local.Initializer) {
				return false
			}
			expr = local.Return
		}
		if branch, ok := expr.(*ConditionalExpr); ok && branch != nil && branch.TerminalStatement {
			conditional := validDepthThreeConditionalTest(branch.Condition)
			hasConditionalTest = hasConditionalTest || conditional
			return depth < 3 && branch.Condition != nil && countImmutableLocalExpressions(branch.Condition) == 0 && (validConditionalOperand(branch.Condition) || conditional) &&
				validScope(branch.WhenTrue, depth+1) && validScope(branch.WhenFalse, depth+1)
		}
		return expr != nil && countImmutableLocalExpressions(expr) == 0 &&
			(validConditionalOperand(expr) || validDepthThreeStraightLineReturns(expr) || validConditionalBooleanSelector(expr))
	}
	return validScope(expr, 0) && hasConditionalTest
}

func validDepthThreeConditionalTest(expr Expr) bool {
	c, ok := expr.(*ConditionalExpr)
	if !ok || c == nil || c.TerminalStatement || countImmutableLocalExpressions(expr) != 0 {
		return false
	}
	if c.Condition == nil || !validConditionalOperand(c.Condition) {
		return false
	}
	for _, arm := range []Expr{c.WhenTrue, c.WhenFalse} {
		if arm == nil || (!validConditionalOperand(arm) && !validNestedConditionalTest(arm)) {
			return false
		}
	}
	return true
}

// v0.106 adds complete flat boolean selectors only at existing terminal test positions.
func validTerminalBooleanSelectorTests(expr Expr) bool {
	hasConditionalTest := false
	var validScope func(Expr, int) bool
	validScope = func(expr Expr, depth int) bool {
		for {
			local, ok := expr.(*ImmutableLocalExpr)
			if !ok {
				break
			}
			if local == nil || local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(local.Initializer) != 0 {
				return false
			}
			selector := validConditionalBooleanSelector(local.Initializer)
			if !selector && !validConditionalOperand(local.Initializer) && !validDepthThreeStraightLineReturns(local.Initializer) {
				return false
			}
			expr = local.Return
		}
		if branch, ok := expr.(*ConditionalExpr); ok && branch != nil && branch.TerminalStatement {
			selector := validConditionalBooleanSelector(branch.Condition)
			conditional := selector || validDepthThreeConditionalTest(branch.Condition)
			hasConditionalTest = hasConditionalTest || selector
			return depth < 3 && branch.Condition != nil && countImmutableLocalExpressions(branch.Condition) == 0 && (validConditionalOperand(branch.Condition) || conditional) &&
				validScope(branch.WhenTrue, depth+1) && validScope(branch.WhenFalse, depth+1)
		}
		return expr != nil && countImmutableLocalExpressions(expr) == 0 &&
			(validConditionalOperand(expr) || validDepthThreeStraightLineReturns(expr) || validConditionalBooleanSelector(expr))
	}
	return validScope(expr, 0) && hasConditionalTest
}

// v0.107 permits flat result arms only in complete terminal selector tests.
func validTerminalSelectorValueArms(expr Expr) bool {
	hasConditionalTest := false
	var validScope func(Expr, int) bool
	validScope = func(expr Expr, depth int) bool {
		for {
			local, ok := expr.(*ImmutableLocalExpr)
			if !ok {
				break
			}
			if local == nil || local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(local.Initializer) != 0 {
				return false
			}
			selector := validConditionalBooleanSelector(local.Initializer)
			if !selector && !validConditionalOperand(local.Initializer) && !validDepthThreeStraightLineReturns(local.Initializer) {
				return false
			}
			expr = local.Return
		}
		if branch, ok := expr.(*ConditionalExpr); ok && branch != nil && branch.TerminalStatement {
			selector := validSelectorValueArmTest(branch.Condition)
			conditional := selector || validConditionalBooleanSelector(branch.Condition) || validDepthThreeConditionalTest(branch.Condition)
			hasConditionalTest = hasConditionalTest || selector
			return depth < 3 && branch.Condition != nil && countImmutableLocalExpressions(branch.Condition) == 0 && (validConditionalOperand(branch.Condition) || conditional) &&
				validScope(branch.WhenTrue, depth+1) && validScope(branch.WhenFalse, depth+1)
		}
		return expr != nil && countImmutableLocalExpressions(expr) == 0 &&
			(validConditionalOperand(expr) || validDepthThreeStraightLineReturns(expr) || validConditionalBooleanSelector(expr))
	}
	return validScope(expr, 0) && hasConditionalTest
}

func validSelectorValueArmTest(expr Expr) bool {
	outer, ok := expr.(*ConditionalExpr)
	if !ok || outer == nil || outer.TerminalStatement || countImmutableLocalExpressions(expr) != 0 || !validFlatConditionalTest(outer.Condition) {
		return false
	}
	hasArm := false
	for _, arm := range []Expr{outer.WhenTrue, outer.WhenFalse} {
		if arm == nil {
			return false
		}
		flat := validFlatConditionalTest(arm)
		if !flat && !validConditionalOperand(arm) {
			return false
		}
		hasArm = hasArm || flat
	}
	return hasArm
}

// v0.108 permits flat inner-selector arms only in terminal tests with nonconditional outer arms.
func validTerminalInnerSelectorArms(expr Expr) bool {
	hasConditionalTest := false
	var validScope func(Expr, int) bool
	validScope = func(expr Expr, depth int) bool {
		for {
			local, ok := expr.(*ImmutableLocalExpr)
			if !ok {
				break
			}
			if local == nil || local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(local.Initializer) != 0 {
				return false
			}
			selector := validConditionalBooleanSelector(local.Initializer)
			if !selector && !validConditionalOperand(local.Initializer) && !validDepthThreeStraightLineReturns(local.Initializer) {
				return false
			}
			expr = local.Return
		}
		if branch, ok := expr.(*ConditionalExpr); ok && branch != nil && branch.TerminalStatement {
			selector := validInnerSelectorArmTest(branch.Condition)
			conditional := selector || validSelectorValueArmTest(branch.Condition) || validConditionalBooleanSelector(branch.Condition) || validDepthThreeConditionalTest(branch.Condition)
			hasConditionalTest = hasConditionalTest || selector
			return depth < 3 && branch.Condition != nil && countImmutableLocalExpressions(branch.Condition) == 0 && (validConditionalOperand(branch.Condition) || conditional) &&
				validScope(branch.WhenTrue, depth+1) && validScope(branch.WhenFalse, depth+1)
		}
		return expr != nil && countImmutableLocalExpressions(expr) == 0 &&
			(validConditionalOperand(expr) || validDepthThreeStraightLineReturns(expr) || validConditionalBooleanSelector(expr))
	}
	return validScope(expr, 0) && hasConditionalTest
}

func validInnerSelectorArmTest(expr Expr) bool {
	outer, ok := expr.(*ConditionalExpr)
	if !ok || outer == nil || outer.TerminalStatement || countImmutableLocalExpressions(expr) != 0 || !validConditionalOperand(outer.WhenTrue) || !validConditionalOperand(outer.WhenFalse) {
		return false
	}
	selector, ok := outer.Condition.(*ConditionalExpr)
	if !ok || selector == nil || selector.TerminalStatement || !validConditionalOperand(selector.Condition) {
		return false
	}
	hasArm := false
	for _, arm := range []Expr{selector.WhenTrue, selector.WhenFalse} {
		if arm == nil {
			return false
		}
		flat := validFlatConditionalTest(arm)
		if !flat && !validConditionalOperand(arm) {
			return false
		}
		hasArm = hasArm || flat
	}
	return hasArm
}

// v0.109 combines flat inner-selector and outer result arms only in terminal tests.
func validTerminalCombinedSelectorArms(expr Expr) bool {
	hasConditionalTest := false
	var validScope func(Expr, int) bool
	validScope = func(expr Expr, depth int) bool {
		for {
			local, ok := expr.(*ImmutableLocalExpr)
			if !ok {
				break
			}
			if local == nil || local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(local.Initializer) != 0 {
				return false
			}
			selector := validConditionalBooleanSelector(local.Initializer)
			if !selector && !validConditionalOperand(local.Initializer) && !validDepthThreeStraightLineReturns(local.Initializer) {
				return false
			}
			expr = local.Return
		}
		if branch, ok := expr.(*ConditionalExpr); ok && branch != nil && branch.TerminalStatement {
			selector := validCombinedSelectorArmTest(branch.Condition)
			conditional := selector || validInnerSelectorArmTest(branch.Condition) || validSelectorValueArmTest(branch.Condition) || validConditionalBooleanSelector(branch.Condition) || validDepthThreeConditionalTest(branch.Condition)
			hasConditionalTest = hasConditionalTest || selector
			return depth < 3 && branch.Condition != nil && countImmutableLocalExpressions(branch.Condition) == 0 && (validConditionalOperand(branch.Condition) || conditional) &&
				validScope(branch.WhenTrue, depth+1) && validScope(branch.WhenFalse, depth+1)
		}
		return expr != nil && countImmutableLocalExpressions(expr) == 0 &&
			(validConditionalOperand(expr) || validDepthThreeStraightLineReturns(expr) || validConditionalBooleanSelector(expr))
	}
	return validScope(expr, 0) && hasConditionalTest
}

func validCombinedSelectorArmTest(expr Expr) bool {
	outer, ok := expr.(*ConditionalExpr)
	if !ok || outer == nil || outer.TerminalStatement || countImmutableLocalExpressions(expr) != 0 {
		return false
	}
	selector, ok := outer.Condition.(*ConditionalExpr)
	if !ok || selector == nil || selector.TerminalStatement || !validConditionalOperand(selector.Condition) {
		return false
	}
	for _, arms := range [][]Expr{{selector.WhenTrue, selector.WhenFalse}, {outer.WhenTrue, outer.WhenFalse}} {
		hasArm := false
		for _, arm := range arms {
			if arm == nil {
				return false
			}
			flat := validFlatConditionalTest(arm)
			if !flat && !validConditionalOperand(arm) {
				return false
			}
			hasArm = hasArm || flat
		}
		if !hasArm {
			return false
		}
	}
	return true
}
