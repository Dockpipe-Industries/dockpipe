package pipelang

import (
	"fmt"
	"strings"
)

// PrimitiveType identifies one frozen PipeLang v0.0.0.1 primitive value type.
type PrimitiveType string

const (
	TypeString PrimitiveType = "string"
	TypeInt    PrimitiveType = "int"
	TypeBool   PrimitiveType = "bool"
	TypeFloat  PrimitiveType = "float"
)

type TypeRefKind string

const (
	TypeRefPrimitive TypeRefKind = "primitive"
	TypeRefNamed     TypeRefKind = "named"
	TypeRefApplied   TypeRefKind = "applied"
)

// UnresolvedTypeRef is the structured source form of an existing v0.0.0.1 type
// spelling. It accepts only primitives, named types, and the existing List<T>
// application; it does not expand the language grammar.
type UnresolvedTypeRef struct {
	Kind      TypeRefKind
	Name      string
	Qualifier ModuleID
	Arguments []UnresolvedTypeRef
	Span      Span
}

func (r UnresolvedTypeRef) String() string {
	switch r.Kind {
	case TypeRefApplied:
		args := make([]string, 0, len(r.Arguments))
		for _, argument := range r.Arguments {
			args = append(args, argument.String())
		}
		return fmt.Sprintf("%s<%s>", r.Name, strings.Join(args, ","))
	default:
		return r.Name
	}
}

func (r UnresolvedTypeRef) IsValid() bool {
	switch r.Kind {
	case TypeRefPrimitive:
		_, ok := primitiveType(r.Name)
		return ok && r.Qualifier == "" && len(r.Arguments) == 0
	case TypeRefNamed:
		return isTypeIdentifier(r.Name) && validModuleID(r.Qualifier, true) && len(r.Arguments) == 0
	case TypeRefApplied:
		return r.Name == "List" && r.Qualifier == "" && len(r.Arguments) == 1 && r.Arguments[0].IsValid()
	default:
		return false
	}
}

func (r UnresolvedTypeRef) IsPrimitive() bool {
	return r.Kind == TypeRefPrimitive
}

func (r UnresolvedTypeRef) ListElementType() (UnresolvedTypeRef, bool) {
	if r.Kind != TypeRefApplied || r.Name != "List" || len(r.Arguments) != 1 {
		return UnresolvedTypeRef{}, false
	}
	return r.Arguments[0], true
}

func primitiveType(name string) (PrimitiveType, bool) {
	switch PrimitiveType(name) {
	case TypeString, TypeInt, TypeBool, TypeFloat:
		return PrimitiveType(name), true
	default:
		return "", false
	}
}

func unresolvedPrimitive(t PrimitiveType, span Span) UnresolvedTypeRef {
	return UnresolvedTypeRef{Kind: TypeRefPrimitive, Name: string(t), Span: span}
}

// ResolvedTypeRef is the checked form of a type reference. Named references
// carry the identity of exactly one declaration in the analysis symbol table.
type ResolvedTypeRef struct {
	Kind      TypeRefKind
	Primitive PrimitiveType
	Name      string
	Symbol    SymbolID
	Arguments []ResolvedTypeRef
}

func (r ResolvedTypeRef) String() string {
	switch r.Kind {
	case TypeRefPrimitive:
		return string(r.Primitive)
	case TypeRefApplied:
		args := make([]string, 0, len(r.Arguments))
		for _, argument := range r.Arguments {
			args = append(args, argument.String())
		}
		return fmt.Sprintf("%s<%s>", r.Name, strings.Join(args, ","))
	default:
		return r.Name
	}
}

func (r ResolvedTypeRef) Equal(other ResolvedTypeRef) bool {
	if r.Kind != other.Kind || r.Primitive != other.Primitive || r.Name != other.Name || r.Symbol != other.Symbol || len(r.Arguments) != len(other.Arguments) {
		return false
	}
	for i := range r.Arguments {
		if !r.Arguments[i].Equal(other.Arguments[i]) {
			return false
		}
	}
	return true
}

func (r ResolvedTypeRef) IsPrimitive() bool {
	return r.Kind == TypeRefPrimitive
}

func resolvedPrimitive(t PrimitiveType) ResolvedTypeRef {
	return ResolvedTypeRef{Kind: TypeRefPrimitive, Primitive: t}
}
