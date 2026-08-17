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
	ExprLiteral   ExprKind = "literal"
	ExprReference ExprKind = "reference"
	ExprUnary     ExprKind = "unary"
	ExprBinary    ExprKind = "binary"
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

type Expr struct {
	Kind      ExprKind   `json:"kind"`
	Type      Type       `json:"type"`
	Span      SourceSpan `json:"span"`
	Literal   *Literal   `json:"literal,omitempty"`
	Reference *Binding   `json:"reference,omitempty"`
	Unary     *Unary     `json:"unary,omitempty"`
	Binary    *Binary    `json:"binary,omitempty"`
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
