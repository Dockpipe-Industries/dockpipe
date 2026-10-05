package coreeval

import (
	"reflect"
	"testing"

	"dockpipe/src/lib/pipelang/coreir"
)

// Exercise a local expression used as another local's initializer: its slot
// must disappear before the enclosing binding checks its canonical position.
func TestArgumentFramesRestoreNestedScopesAndErrors(t *testing.T) {
	typ := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}
	literal := func(value string) *coreir.Expr {
		return &coreir.Expr{Kind: coreir.ExprLiteral, Type: typ, Literal: &coreir.Literal{String: value}}
	}
	reference := func(position int) *coreir.Expr {
		return &coreir.Expr{Kind: coreir.ExprReference, Type: typ, Parameter: &position}
	}
	local := func(init, body *coreir.Expr) *coreir.Expr {
		return &coreir.Expr{Kind: coreir.ExprImmutableLocal, Type: typ, ImmutableLocal: &coreir.ImmutableLocal{Position: 1, Type: typ, Initializer: init, Return: body}}
	}
	for _, fail := range []bool{false, true} {
		body := reference(1)
		if fail {
			body = &coreir.Expr{Kind: "invalid-test-expression", Type: typ}
		}
		expression := local(local(literal("nested"), reference(1)), body)
		backing := []Value{{Type: typ, String: "parameter"}, {String: "sentinel"}, {String: "sentinel"}}
		before := append([]Value(nil), backing...)
		frame := &argumentFrame{values: backing[:1]}
		got, err := evalExprWithProgram(*expression, frame, nil)
		if (err != nil) != fail {
			t.Fatalf("unexpected result %#v %v", got, err)
		}
		if !fail && (!got.OK || got.Value.String != "nested") {
			t.Fatalf("nested scope lost: %#v", got)
		}
		if len(frame.values) != 1 || !reflect.DeepEqual(backing, before) {
			t.Fatal("lexical frame escaped or changed caller storage")
		}
		// A second expression in the same frame must reuse the vacated position.
		got, err = evalExprWithProgram(*local(literal("next"), reference(1)), frame, nil)
		if err != nil || got.Value.String != "next" {
			t.Fatalf("frame not restored: %#v %v", got, err)
		}
	}
}
