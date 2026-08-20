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

const (
	PipeLangBuiltinPackageID            PackageID  = "pipelang"
	PipeLangResultSemanticPath          SemanticID = "result"
	PipeLangArithmeticErrorSemanticPath SemanticID = "arithmetic.error"
	PipeLangOptionalSemanticPath        SemanticID = "optional"
	PipeLangListSemanticPath            SemanticID = "list"
)

// UnresolvedTypeRef is the structured source form shared by the versioned
// parsers. The frozen lane emits primitives, named types, and List<T>; the
// explicit v0.2.0 and later arithmetic lanes additionally emit the bounded
// Result<T,E> shape.
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
		if r.Qualifier != "" {
			return false
		}
		switch r.Name {
		case "List":
			return len(r.Arguments) == 1 && r.Arguments[0].IsValid()
		case "Result":
			return len(r.Arguments) == 2 && r.Arguments[0].IsValid() && r.Arguments[1].IsValid()
		case "Optional":
			return len(r.Arguments) == 1 && r.Arguments[0].IsValid()
		default:
			return false
		}
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

// ResolvedTypeRef is the checked form of a type reference. Source-owned named
// references carry one analysis symbol; language-owned types carry their fixed
// package and semantic-path identity instead.
type ResolvedTypeRef struct {
	Kind      TypeRefKind
	Primitive PrimitiveType
	Name      string
	Symbol    SymbolID
	PackageID PackageID
	Path      SemanticID
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
	if r.Kind != other.Kind || r.Primitive != other.Primitive || r.Name != other.Name || r.Symbol != other.Symbol || r.PackageID != other.PackageID || r.Path != other.Path || len(r.Arguments) != len(other.Arguments) {
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

func resolvedArithmeticError() ResolvedTypeRef {
	return ResolvedTypeRef{Kind: TypeRefNamed, Name: "ArithmeticError", PackageID: PipeLangBuiltinPackageID, Path: PipeLangArithmeticErrorSemanticPath}
}

func resolvedArithmeticResult(success ResolvedTypeRef) ResolvedTypeRef {
	return ResolvedTypeRef{
		Kind: TypeRefApplied, Name: "Result", PackageID: PipeLangBuiltinPackageID, Path: PipeLangResultSemanticPath,
		Arguments: []ResolvedTypeRef{success, resolvedArithmeticError()},
	}
}

func resolvedResult(success, failure ResolvedTypeRef) ResolvedTypeRef {
	return ResolvedTypeRef{
		Kind: TypeRefApplied, Name: "Result", PackageID: PipeLangBuiltinPackageID, Path: PipeLangResultSemanticPath,
		Arguments: []ResolvedTypeRef{success, failure},
	}
}

func resolvedOptional(value ResolvedTypeRef) ResolvedTypeRef {
	return ResolvedTypeRef{
		Kind: TypeRefApplied, Name: "Optional", PackageID: PipeLangBuiltinPackageID, Path: PipeLangOptionalSemanticPath,
		Arguments: []ResolvedTypeRef{value},
	}
}

func resolvedRecordList(value ResolvedTypeRef) ResolvedTypeRef {
	return ResolvedTypeRef{
		Kind: TypeRefApplied, Name: "List", PackageID: PipeLangBuiltinPackageID, Path: PipeLangListSemanticPath,
		Arguments: []ResolvedTypeRef{value},
	}
}

func isResolvedRecordList(ref ResolvedTypeRef) bool {
	return ref.Kind == TypeRefApplied && ref.Name == "List" && ref.PackageID == PipeLangBuiltinPackageID && ref.Path == PipeLangListSemanticPath && len(ref.Arguments) == 1 && ref.Arguments[0].Kind == TypeRefNamed && ref.Arguments[0].Symbol != 0
}

func containsResolvedRecordList(ref ResolvedTypeRef) bool {
	if isResolvedRecordList(ref) {
		return true
	}
	for _, argument := range ref.Arguments {
		if containsResolvedRecordList(argument) {
			return true
		}
	}
	return false
}

func isResolvedPrimitiveOptional(ref ResolvedTypeRef) bool {
	return isResolvedOptional(ref) && ref.Arguments[0].Kind == TypeRefPrimitive
}

func isResolvedOptional(ref ResolvedTypeRef) bool {
	return ref.Kind == TypeRefApplied && ref.Name == "Optional" && ref.PackageID == PipeLangBuiltinPackageID && ref.Path == PipeLangOptionalSemanticPath && len(ref.Arguments) == 1
}

func isResolvedRecordOptional(ref ResolvedTypeRef) bool {
	return isResolvedOptional(ref) && ref.Arguments[0].Kind == TypeRefNamed && ref.Arguments[0].Symbol != 0
}

func containsResolvedOptional(ref ResolvedTypeRef) bool {
	if isResolvedOptional(ref) {
		return true
	}
	for _, argument := range ref.Arguments {
		if containsResolvedOptional(argument) {
			return true
		}
	}
	return false
}

func isResolvedArithmeticError(ref ResolvedTypeRef) bool {
	return ref.Kind == TypeRefNamed && ref.Name == "ArithmeticError" && ref.Symbol == 0 && ref.PackageID == PipeLangBuiltinPackageID && ref.Path == PipeLangArithmeticErrorSemanticPath && len(ref.Arguments) == 0
}

func isResolvedIntArithmeticResult(ref ResolvedTypeRef) bool {
	return ref.Kind == TypeRefApplied && ref.Name == "Result" && ref.PackageID == PipeLangBuiltinPackageID && ref.Path == PipeLangResultSemanticPath && len(ref.Arguments) == 2 && ref.Arguments[0].Equal(resolvedPrimitive(TypeInt)) && isResolvedArithmeticError(ref.Arguments[1])
}

func isResolvedFloatArithmeticResult(ref ResolvedTypeRef) bool {
	return ref.Kind == TypeRefApplied && ref.Name == "Result" && ref.PackageID == PipeLangBuiltinPackageID && ref.Path == PipeLangResultSemanticPath && len(ref.Arguments) == 2 && ref.Arguments[0].Equal(resolvedPrimitive(TypeFloat)) && isResolvedArithmeticError(ref.Arguments[1])
}

func isResolvedArithmeticResult(ref ResolvedTypeRef) bool {
	return isResolvedIntArithmeticResult(ref) || isResolvedFloatArithmeticResult(ref)
}

func isResolvedResult(ref ResolvedTypeRef) bool {
	return ref.Kind == TypeRefApplied && ref.Name == "Result" && ref.PackageID == PipeLangBuiltinPackageID && ref.Path == PipeLangResultSemanticPath && len(ref.Arguments) == 2
}

func isResolvedSnapshotResult(ref ResolvedTypeRef) bool {
	return isResolvedResult(ref) && isResolvedRecordList(ref.Arguments[0]) && ref.Arguments[1].Equal(resolvedPrimitive(TypeString))
}

func isResolvedTextResult(ref ResolvedTypeRef) bool {
	stringType := resolvedPrimitive(TypeString)
	return isResolvedResult(ref) && ref.Arguments[0].Equal(stringType) && ref.Arguments[1].Equal(stringType)
}

func isResolvedBoundedValueResult(contract LanguageContract, ref ResolvedTypeRef) bool {
	return isResolvedSnapshotResult(ref) || (hasTextResultSourceContract(contract) && isResolvedTextResult(ref))
}

func containsResolvedResult(ref ResolvedTypeRef) bool {
	if isResolvedResult(ref) {
		return true
	}
	for _, argument := range ref.Arguments {
		if containsResolvedResult(argument) {
			return true
		}
	}
	return false
}

func isResolvedSourceArithmeticResult(contract LanguageContract, ref ResolvedTypeRef) bool {
	return isResolvedIntArithmeticResult(ref) || ((contract == PipeLangLanguageContractV060 || contract == PipeLangLanguageContractV070 || contract == PipeLangLanguageContractV080 || contract == PipeLangLanguageContractV090 || contract == PipeLangLanguageContractV100 || contract == PipeLangLanguageContractV110 || contract == PipeLangLanguageContractV120 || contract == PipeLangLanguageContractV130 || contract == PipeLangLanguageContractV140 || contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170 || contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200 || contract == PipeLangLanguageContractV210 || contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV230 || contract == PipeLangLanguageContractV240 || contract == PipeLangLanguageContractV250 || contract == PipeLangLanguageContractV260 || contract == PipeLangLanguageContractV270 || contract == PipeLangLanguageContractV280 || contract == PipeLangLanguageContractV290 || contract == PipeLangLanguageContractV300) && isResolvedFloatArithmeticResult(ref))
}

func containsResolvedArithmeticContractType(ref ResolvedTypeRef) bool {
	if isResolvedArithmeticError(ref) || isResolvedArithmeticResult(ref) {
		return true
	}
	for _, argument := range ref.Arguments {
		if containsResolvedArithmeticContractType(argument) {
			return true
		}
	}
	return false
}
