package hir

// Block is structured control flow. Only a function body carries ExprBlock;
// nested statement blocks share that function's return target.
type Block struct {
	Statements []Statement `json:"statements"`
}
type Statement struct {
	Kind  string     `json:"kind"`
	Value *Expr      `json:"value,omitempty"`
	Local *Parameter `json:"local,omitempty"`
	Then  *Block     `json:"then,omitempty"`
	Else  *Block     `json:"else,omitempty"`
	Span  SourceSpan `json:"span"`
}
