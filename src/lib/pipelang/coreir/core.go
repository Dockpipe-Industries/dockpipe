// Package coreir defines PipeLang's normalized backend-neutral executable IR.
// It contains no source-tree, parser, HIR, or target-language concepts.
package coreir

const (
	LanguageContractV010 = "v0.1.0"
	LanguageContractV020 = "v0.2.0"
	LanguageContractV030 = "v0.3.0"
	LanguageContractV040 = "v0.4.0"
	LanguageContractV050 = "v0.5.0"
	LanguageContractV060 = "v0.6.0"
	LanguageContractV070 = "v0.7.0"
	LanguageContractV080 = "v0.8.0"
	LanguageContractV090 = "v0.9.0"
	LanguageContractV100 = "v0.10.0"
	LanguageContractV110 = "v0.11.0"
	LanguageContractV120 = "v0.12.0"
	LanguageContractV130 = "v0.13.0"
	LanguageContractV140 = "v0.14.0"
	LanguageContractV150 = "v0.15.0"
	LanguageContractV160 = "v0.16.0"
	LanguageContractV170 = "v0.17.0"
	LanguageContractV180 = "v0.18.0"
	LanguageContractV190 = "v0.19.0"
	LanguageContractV200 = "v0.20.0"
	LanguageContractV210 = "v0.21.0"
	LanguageContractV220 = "v0.22.0"
	LanguageContractV230 = "v0.23.0"
	LanguageContractV240 = "v0.24.0"
	LanguageContractV250 = "v0.25.0"
	LanguageContractV260 = "v0.26.0"
	LanguageContractV270 = "v0.27.0"
	LanguageContractV280 = "v0.28.0"
	LanguageContractV290 = "v0.29.0"
	CompilerContractV1   = "pipelang.compiler.v1"
	BuiltinPackageID     = "pipelang"
	ListSemanticPath     = "list"
)

type PrimitiveType string

const (
	PrimitiveString PrimitiveType = "string"
	PrimitiveInt    PrimitiveType = "int"
	PrimitiveBool   PrimitiveType = "bool"
	PrimitiveFloat  PrimitiveType = "float"
)

type TypeKind string

const (
	TypePrimitive       TypeKind = "primitive"
	TypeNumeric         TypeKind = "numeric"
	TypeResult          TypeKind = "result"
	TypeArithmeticError TypeKind = "arithmetic_error"
	TypeRecord          TypeKind = "record"
	TypeOptional        TypeKind = "optional"
	TypeList            TypeKind = "list"
	TypeNamed           TypeKind = "named"
	TypeApplied         TypeKind = "applied"
)

type NumericRepresentation string

const (
	NumericInteger     NumericRepresentation = "integer"
	NumericBinaryFloat NumericRepresentation = "binary_float"
)

// NumericType fully describes a backend-neutral fixed numeric value.
// Signed applies only to integer representations.
type NumericType struct {
	Representation NumericRepresentation `json:"representation"`
	Bits           int                   `json:"bits"`
	Signed         bool                  `json:"signed,omitempty"`
}

// ResultType is a backend-neutral closed success/failure value. It is an IR
// contract and does not choose a production source spelling.
type ResultType struct {
	Success Type `json:"success"`
	Failure Type `json:"failure"`
}

type OptionalType struct {
	Value Type `json:"value"`
}

type ListType struct {
	Element Type `json:"element"`
}

type RecordField struct {
	Identity SemanticIdentity `json:"identity"`
	Name     string           `json:"name"`
	Type     Type             `json:"type"`
}

type RecordType struct {
	Fields []RecordField `json:"fields"`
}

type ArithmeticError string

const (
	ArithmeticOverflow       ArithmeticError = "overflow"
	ArithmeticDivisionByZero ArithmeticError = "division_by_zero"
)

type SemanticType struct {
	Kind      TypeKind       `json:"kind"`
	Primitive PrimitiveType  `json:"primitive,omitempty"`
	PackageID string         `json:"package_id,omitempty"`
	Path      string         `json:"path,omitempty"`
	Name      string         `json:"name,omitempty"`
	Arguments []SemanticType `json:"arguments,omitempty"`
}

type CallableIdentity struct {
	Parameters []SemanticType `json:"parameters"`
	Returns    SemanticType   `json:"returns"`
}

type SemanticIdentity struct {
	PackageID string            `json:"package_id"`
	Path      string            `json:"path"`
	Callable  *CallableIdentity `json:"callable,omitempty"`
}

type Type struct {
	Kind      TypeKind          `json:"kind"`
	Primitive PrimitiveType     `json:"primitive,omitempty"`
	Numeric   *NumericType      `json:"numeric,omitempty"`
	Result    *ResultType       `json:"result,omitempty"`
	Optional  *OptionalType     `json:"optional,omitempty"`
	List      *ListType         `json:"list,omitempty"`
	Record    *RecordType       `json:"record,omitempty"`
	Identity  *SemanticIdentity `json:"identity,omitempty"`
	Name      string            `json:"name,omitempty"`
	Arguments []Type            `json:"arguments,omitempty"`
}

type Parameter struct {
	Position int    `json:"position"`
	Name     string `json:"name"`
	Type     Type   `json:"type"`
}

type ExprKind string

const (
	ExprLiteral                            ExprKind = "literal"
	ExprReference                          ExprKind = "reference"
	ExprUnary                              ExprKind = "unary"
	ExprBinary                             ExprKind = "binary"
	ExprTextContainsCaseFolded             ExprKind = "text_contains_case_folded"
	ExprTextTrim                           ExprKind = "text_trim"
	ExprFieldProjection                    ExprKind = "field_projection"
	ExprRecordConstruct                    ExprKind = "record_construct"
	ExprOptionalSome                       ExprKind = "optional_some"
	ExprOptionalNone                       ExprKind = "optional_none"
	ExprOptionalHasValue                   ExprKind = "optional_has_value"
	ExprOptionalValueOr                    ExprKind = "optional_value_or"
	ExprListEmpty                          ExprKind = "list_empty"
	ExprListSingleton                      ExprKind = "list_singleton"
	ExprListCount                          ExprKind = "list_count"
	ExprListAppend                         ExprKind = "list_append"
	ExprListAt                             ExprKind = "list_at"
	ExprListFindByText                     ExprKind = "list_find_by_text"
	ExprListFilterByText                   ExprKind = "list_filter_by_text"
	ExprListFilterContainsCaseFolded       ExprKind = "list_filter_contains_case_folded_text"
	ExprListFilterJoinedContainsCaseFolded ExprKind = "list_filter_joined_contains_case_folded_text"
	ExprListSortByOrdinalText              ExprKind = "list_sort_by_ordinal_text"
	ExprResultOK                           ExprKind = "result_ok"
	ExprResultErr                          ExprKind = "result_err"
	ExprResultIsOK                         ExprKind = "result_is_ok"
	ExprResultSuccessOr                    ExprKind = "result_success_or"
	ExprResultFailureOr                    ExprKind = "result_failure_or"
)

type Operator string

const (
	OperatorNot            Operator = "not"
	OperatorNegate         Operator = "negate"
	OperatorAdd            Operator = "add"
	OperatorSubtract       Operator = "subtract"
	OperatorMultiply       Operator = "multiply"
	OperatorDivide         Operator = "divide"
	OperatorEqual          Operator = "equal"
	OperatorNotEqual       Operator = "not_equal"
	OperatorLessThan       Operator = "less_than"
	OperatorLessOrEqual    Operator = "less_or_equal"
	OperatorGreaterThan    Operator = "greater_than"
	OperatorGreaterOrEqual Operator = "greater_or_equal"
	OperatorAnd            Operator = "and"
	OperatorOr             Operator = "or"
)

type Literal struct {
	String string  `json:"string"`
	Int    int64   `json:"int"`
	Float  float64 `json:"float"`
	Bool   bool    `json:"bool"`
}

type Unary struct {
	Operator Operator `json:"operator"`
	Operand  *Expr    `json:"operand"`
}

type Binary struct {
	Operator Operator `json:"operator"`
	Left     *Expr    `json:"left"`
	Right    *Expr    `json:"right"`
}

type TextContainsCaseFolded struct {
	Value *Expr `json:"value"`
	Query *Expr `json:"query"`
}

type TextTrim struct {
	Value *Expr `json:"value"`
}

type FieldProjection struct {
	Receiver *Expr            `json:"receiver"`
	Identity SemanticIdentity `json:"identity"`
	Name     string           `json:"name"`
	Position int              `json:"position"`
}

type RecordConstructField struct {
	Identity SemanticIdentity `json:"identity"`
	Name     string           `json:"name"`
	Position int              `json:"position"`
	Value    *Expr            `json:"value"`
}

type RecordConstruct struct {
	Identity SemanticIdentity       `json:"identity"`
	Fields   []RecordConstructField `json:"fields"`
}

type OptionalSome struct {
	Value *Expr `json:"value"`
}

type OptionalNone struct{}

type OptionalHasValue struct {
	Value *Expr `json:"value"`
}

type OptionalValueOr struct {
	Value    *Expr `json:"value"`
	Fallback *Expr `json:"fallback"`
}

type ListEmpty struct{}

type ListSingleton struct {
	Value *Expr `json:"value"`
}

type ListCount struct {
	Value *Expr `json:"value"`
}

type ListAppend struct {
	Values *Expr `json:"values"`
	Value  *Expr `json:"value"`
}

type ListAt struct {
	Values *Expr `json:"values"`
	Index  *Expr `json:"index"`
}

type ListFindByText struct {
	Values   *Expr            `json:"values"`
	Field    SemanticIdentity `json:"field"`
	Name     string           `json:"name"`
	Position int              `json:"position"`
	Key      *Expr            `json:"key"`
}

type ListFilterByText struct {
	Values   *Expr            `json:"values"`
	Field    SemanticIdentity `json:"field"`
	Name     string           `json:"name"`
	Position int              `json:"position"`
	Key      *Expr            `json:"key"`
}

type ListFilterContainsCaseFolded struct {
	Values   *Expr            `json:"values"`
	Field    SemanticIdentity `json:"field"`
	Name     string           `json:"name"`
	Position int              `json:"position"`
	Query    *Expr            `json:"query"`
}

type ListTextFieldSelector struct {
	Field    SemanticIdentity `json:"field"`
	Name     string           `json:"name"`
	Position int              `json:"position"`
}

type ListFilterJoinedContainsCaseFolded struct {
	Values    *Expr                   `json:"values"`
	Selectors []ListTextFieldSelector `json:"selectors"`
	Query     *Expr                   `json:"query"`
}

type ListSortByOrdinalText struct {
	Values   *Expr            `json:"values"`
	Field    SemanticIdentity `json:"field"`
	Name     string           `json:"name"`
	Position int              `json:"position"`
}

type ResultOK struct {
	Value *Expr `json:"value"`
}

type ResultErr struct {
	Error *Expr `json:"error"`
}

type ResultIsOK struct {
	Value *Expr `json:"value"`
}

type ResultSuccessOr struct {
	Value    *Expr `json:"value"`
	Fallback *Expr `json:"fallback"`
}

type ResultFailureOr struct {
	Value    *Expr `json:"value"`
	Fallback *Expr `json:"fallback"`
}

type Expr struct {
	Kind                               ExprKind                            `json:"kind"`
	Type                               Type                                `json:"type"`
	Literal                            *Literal                            `json:"literal,omitempty"`
	Parameter                          *int                                `json:"parameter,omitempty"`
	Unary                              *Unary                              `json:"unary,omitempty"`
	Binary                             *Binary                             `json:"binary,omitempty"`
	TextContains                       *TextContainsCaseFolded             `json:"text_contains_case_folded,omitempty"`
	TextTrim                           *TextTrim                           `json:"text_trim,omitempty"`
	Field                              *FieldProjection                    `json:"field,omitempty"`
	Record                             *RecordConstruct                    `json:"record,omitempty"`
	Some                               *OptionalSome                       `json:"some,omitempty"`
	None                               *OptionalNone                       `json:"none,omitempty"`
	HasValue                           *OptionalHasValue                   `json:"has_value,omitempty"`
	ValueOr                            *OptionalValueOr                    `json:"value_or,omitempty"`
	ListEmpty                          *ListEmpty                          `json:"list_empty,omitempty"`
	ListOne                            *ListSingleton                      `json:"list_singleton,omitempty"`
	ListCount                          *ListCount                          `json:"list_count,omitempty"`
	ListAppend                         *ListAppend                         `json:"list_append,omitempty"`
	ListAt                             *ListAt                             `json:"list_at,omitempty"`
	ListFind                           *ListFindByText                     `json:"list_find_by_text,omitempty"`
	ListFilter                         *ListFilterByText                   `json:"list_filter_by_text,omitempty"`
	ListFilterContainsCaseFolded       *ListFilterContainsCaseFolded       `json:"list_filter_contains_case_folded_text,omitempty"`
	ListFilterJoinedContainsCaseFolded *ListFilterJoinedContainsCaseFolded `json:"list_filter_joined_contains_case_folded_text,omitempty"`
	ListSortByOrdinalText              *ListSortByOrdinalText              `json:"list_sort_by_ordinal_text,omitempty"`
	ResultOK                           *ResultOK                           `json:"result_ok,omitempty"`
	ResultErr                          *ResultErr                          `json:"result_err,omitempty"`
	ResultIsOK                         *ResultIsOK                         `json:"result_is_ok,omitempty"`
	SuccessOr                          *ResultSuccessOr                    `json:"result_success_or,omitempty"`
	FailureOr                          *ResultFailureOr                    `json:"result_failure_or,omitempty"`
}

type Function struct {
	Identity   SemanticIdentity `json:"identity"`
	Name       string           `json:"name"`
	Parameters []Parameter      `json:"parameters"`
	ReturnType Type             `json:"return_type"`
	Body       Expr             `json:"body"`
}

type Program struct {
	LanguageContract string     `json:"language_contract"`
	CompilerContract string     `json:"compiler_contract"`
	Functions        []Function `json:"functions"`
}
