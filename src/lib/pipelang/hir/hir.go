// Package hir defines PipeLang's typed, target-independent high-level IR.
// It deliberately has no dependency on parser or backend packages.
package hir

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

// NumericType is the target-independent representation selected by the
// post-legacy semantic contracts. Signed applies only to integer representations.
type NumericType struct {
	Representation NumericRepresentation `json:"representation"`
	Bits           int                   `json:"bits"`
	Signed         bool                  `json:"signed,omitempty"`
}

// ResultType is the target-independent shape of a successful value or a
// closed failure value. No production source spelling is selected here.
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

type SourceSpan struct {
	File  string `json:"file"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

type Type struct {
	Kind      TypeKind          `json:"kind"`
	Primitive PrimitiveType     `json:"primitive,omitempty"`
	Numeric   *NumericType      `json:"numeric,omitempty"`
	Result    *ResultType       `json:"result,omitempty"`
	Optional  *OptionalType     `json:"optional,omitempty"`
	List      *ListType         `json:"list,omitempty"`
	Record    *RecordType       `json:"record,omitempty"`
	SymbolID  uint32            `json:"symbol_id,omitempty"`
	Identity  *SemanticIdentity `json:"identity,omitempty"`
	Name      string            `json:"name,omitempty"`
	Arguments []Type            `json:"arguments,omitempty"`
}

type Owner struct {
	Module     string           `json:"module"`
	SymbolID   uint32           `json:"symbol_id"`
	Identity   SemanticIdentity `json:"identity"`
	SourceSpan SourceSpan       `json:"source_span"`
}

type BindingKind string

const BindingParameter BindingKind = "parameter"

type Binding struct {
	Kind     BindingKind      `json:"kind"`
	Function SemanticIdentity `json:"function"`
	Position int              `json:"position"`
	Name     string           `json:"name"`
}

type Parameter struct {
	Binding  Binding    `json:"binding"`
	Type     Type       `json:"type"`
	TypeSpan SourceSpan `json:"type_span"`
	Span     SourceSpan `json:"span"`
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
	ExprListSortByOrdinalTexts             ExprKind = "list_sort_by_ordinal_texts"
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

type ListSortByOrdinalTexts struct {
	Values    *Expr                   `json:"values"`
	Selectors []ListTextFieldSelector `json:"selectors"`
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
	Span                               SourceSpan                          `json:"span"`
	Literal                            *Literal                            `json:"literal,omitempty"`
	Reference                          *Binding                            `json:"reference,omitempty"`
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
	ListSortByOrdinalTexts             *ListSortByOrdinalTexts             `json:"list_sort_by_ordinal_texts,omitempty"`
	ResultOK                           *ResultOK                           `json:"result_ok,omitempty"`
	ResultErr                          *ResultErr                          `json:"result_err,omitempty"`
	ResultIsOK                         *ResultIsOK                         `json:"result_is_ok,omitempty"`
	SuccessOr                          *ResultSuccessOr                    `json:"result_success_or,omitempty"`
	FailureOr                          *ResultFailureOr                    `json:"result_failure_or,omitempty"`
}

type Function struct {
	Identity       SemanticIdentity `json:"identity"`
	Owner          Owner            `json:"owner"`
	Name           string           `json:"name"`
	Parameters     []Parameter      `json:"parameters"`
	ReturnType     Type             `json:"return_type"`
	ReturnTypeSpan SourceSpan       `json:"return_type_span"`
	Body           Expr             `json:"body"`
	Span           SourceSpan       `json:"span"`
}

type Program struct {
	LanguageContract string     `json:"language_contract"`
	CompilerContract string     `json:"compiler_contract"`
	Functions        []Function `json:"functions"`
}
