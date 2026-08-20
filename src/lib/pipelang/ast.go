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
