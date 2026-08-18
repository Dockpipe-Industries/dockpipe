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
	CompilerContractV1   = "pipelang.compiler.v1"
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
	ExprLiteral         ExprKind = "literal"
	ExprReference       ExprKind = "reference"
	ExprUnary           ExprKind = "unary"
	ExprBinary          ExprKind = "binary"
	ExprFieldProjection ExprKind = "field_projection"
	ExprRecordConstruct ExprKind = "record_construct"
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

type Expr struct {
	Kind      ExprKind         `json:"kind"`
	Type      Type             `json:"type"`
	Literal   *Literal         `json:"literal,omitempty"`
	Parameter *int             `json:"parameter,omitempty"`
	Unary     *Unary           `json:"unary,omitempty"`
	Binary    *Binary          `json:"binary,omitempty"`
	Field     *FieldProjection `json:"field,omitempty"`
	Record    *RecordConstruct `json:"record,omitempty"`
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
