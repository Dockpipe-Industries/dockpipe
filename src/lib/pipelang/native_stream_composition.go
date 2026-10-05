package pipelang

import (
	"fmt"
	"strings"

	"dockpipe/src/lib/pipelang/streamir"
)

type streamLocal struct {
	slot int
	typ  streamir.Type
}
type streamCompositionLowerer struct {
	source   *Program
	program  streamir.Program
	methods  map[string]int
	native   map[string]int
	reserved map[string]bool
	function int
	nextSlot int
}

func streamCompositionType(t UnresolvedTypeRef) streamir.Type {
	typ := streamir.Type(t.Name)
	kind := TypeRefNamed
	if typ == streamir.Int || typ == streamir.Bool {
		kind = TypeRefPrimitive
	}
	if !streamir.ParameterType(typ) || !nativeStreamType(t, t.Name, kind) {
		return ""
	}
	return typ
}
func (l *streamCompositionLowerer) fail(span Span, message string) error {
	return oneDiagnostic(l.source.sources, CodeExpressionType, CategorySemantic, span, message)
}
func lowerNativeComposition(source *Program, base streamir.Program, reserved map[string]bool) (streamir.Program, error) {
	l := streamCompositionLowerer{source: source, program: base, methods: map[string]int{}, native: map[string]int{}, reserved: reserved}
	names := []string{"StreamStatus", "StreamResult", "ReadStream", "WriteStream"}
	if base.Profile == streamir.IncrementalProfile {
		names = append(names, "StreamSession", "InputBuffer", "OutputBuffer", "StreamStep")
	}
	for _, name := range names {
		l.reserved[name] = true
	}
	for i, b := range base.Bindings {
		l.native[b.Operation.Name] = i
	}
	classes := map[string]bool{}
	var declarations []MethodDecl
	for _, class := range source.Classes {
		if classes[class.Name] || l.reserved[class.Name] || class.Visibility != VisibilityPublic || len(class.Annotations) != 0 || class.Implements != nil || len(class.Fields) != 0 || len(class.Methods) == 0 {
			return streamir.Program{}, l.fail(class.Span, "composition requires distinct public stateless classes")
		}
		classes[class.Name] = true
		for _, method := range class.Methods {
			ret := streamCompositionType(method.ReturnType)
			if !streamir.ValueTypeFor(base.Profile, ret) || len(method.Annotations) != 0 || len(method.Params) > 16 {
				return streamir.Program{}, l.fail(method.Span, "invalid composition method signature")
			}
			key := class.Name + "." + method.Name
			if _, exists := l.methods[key]; exists {
				return streamir.Program{}, l.fail(method.Span, "duplicate composition method")
			}
			f := streamir.Function{Class: class.Name, Name: method.Name, Public: method.Visibility == VisibilityPublic, ReturnType: ret}
			for _, param := range method.Params {
				typ := streamCompositionType(param.Type)
				if typ == "" || !streamir.ParameterTypeFor(base.Profile, typ) {
					return streamir.Program{}, l.fail(param.Span, "unsupported composition parameter type")
				}
				f.Parameters = append(f.Parameters, typ)
			}
			l.methods[key] = len(l.program.Functions)
			l.program.Functions = append(l.program.Functions, f)
			declarations = append(declarations, method)
		}
	}
	if len(l.program.Functions) > 64 {
		return streamir.Program{}, l.fail(source.Span, "too many composition functions")
	}
	for i, method := range declarations {
		l.function = i
		l.nextSlot = len(method.Params)
		env := map[string]streamLocal{}
		for j, param := range method.Params {
			if _, exists := env[param.Name]; exists || l.reserved[param.Name] {
				return streamir.Program{}, l.fail(param.Span, "duplicate parameter or SDK namespace shadowing")
			}
			env[param.Name] = streamLocal{j, l.program.Functions[i].Parameters[j]}
		}
		var body []streamir.Statement
		var err error
		if block, ok := method.Body.(*BlockExpr); ok {
			body, err = l.block(block, env)
		} else {
			var value *streamir.Expression
			value, err = l.expression(method.Body, env, l.program.Functions[i].ReturnType)
			body = []streamir.Statement{{Kind: "return", Value: value}}
		}
		if err != nil {
			return streamir.Program{}, err
		}
		l.program.Functions[i].Body = body
	}
	if err := streamir.Validate(l.program); err != nil {
		return streamir.Program{}, l.fail(source.Span, err.Error())
	}
	return l.program, nil
}
func (l *streamCompositionLowerer) block(block *BlockExpr, outer map[string]streamLocal) ([]streamir.Statement, error) {
	if block == nil {
		return nil, nil
	}
	env := map[string]streamLocal{}
	for k, v := range outer {
		env[k] = v
	}
	var result []streamir.Statement
	for _, source := range block.Statements {
		s := streamir.Statement{Kind: source.Kind}
		var err error
		switch source.Kind {
		case "local":
			s.Type = streamCompositionType(source.Type)
			_, exists := env[source.Name]
			if exists || l.reserved[source.Name] || !streamir.ValueTypeFor(l.program.Profile, s.Type) || l.nextSlot >= 256 {
				return nil, l.fail(source.Span, "invalid local type, shadowing or local extent")
			}
			s.Value, err = l.expression(source.Value, env, s.Type)
			s.Slot = l.nextSlot
			l.nextSlot++
			env[source.Name] = streamLocal{s.Slot, s.Type}
		case "return":
			s.Value, err = l.expression(source.Value, env, l.program.Functions[l.function].ReturnType)
		case "if":
			s.Value, err = l.expression(source.Value, env, streamir.Bool)
			if err == nil {
				s.Then, err = l.block(source.Then, env)
			}
			if err == nil {
				s.Else, err = l.block(source.Else, env)
			}
		case "block":
			s.Then, err = l.block(source.Then, env)
		default:
			return nil, l.fail(source.Span, "unsupported composition statement")
		}
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, nil
}
func (l *streamCompositionLowerer) expression(source Expr, env map[string]streamLocal, expected streamir.Type) (*streamir.Expression, error) {
	if source == nil {
		return nil, fmt.Errorf("missing composition expression")
	}
	x := &streamir.Expression{}
	var err error
	switch e := source.(type) {
	case *IdentExpr:
		v, ok := env[e.Name]
		if !ok {
			return nil, l.fail(e.Span, "unknown or out-of-scope composition variable")
		}
		x.Kind = "variable"
		x.Type = v.typ
		x.Slot = v.slot
	case *LiteralExpr:
		switch e.Value.Type {
		case TypeBool:
			x.Kind = "bool"
			x.Type = streamir.Bool
			x.Bool = e.Value.Bool
		case TypeInt:
			x.Kind = "int"
			x.Type = streamir.Int
			x.Int = e.Value.Int
			if expected == streamir.Uint64 && e.Value.Int >= 0 {
				x.Kind = "uint64"
				x.Type = streamir.Uint64
				x.Uint = uint64(e.Value.Int)
				x.Int = 0
			}
		default:
			return nil, l.fail(e.Span, "unsupported composition literal")
		}
	case *FieldExpr:
		if id, ok := e.Receiver.(*IdentExpr); ok && id.Name == "StreamStatus" {
			status, ok := streamir.ParseStatus(e.Name)
			if !ok {
				return nil, l.fail(e.Span, "unknown stream status")
			}
			x.Kind = "status"
			x.Type = streamir.StatusType
			x.Status = status
		} else {
			value, e2 := l.expression(e.Receiver, env, "")
			err = e2
			x.Kind = "field"
			if e2 != nil {
				return nil, e2
			}
			x.Type = streamir.ResultMemberType(value.Type, e.Name)
			x.Member = e.Name
			x.Arguments = []*streamir.Expression{value}
			if x.Type == "" {
				return nil, l.fail(e.Span, "unknown stream result member")
			}
		}
	case *CallExpr:
		var params []streamir.Type
		if target, ok := l.native[e.Name]; ok {
			x.Kind = "native"
			x.Target = target
			params, x.Type = streamir.NativeSignature(l.program.Profile)
		} else {
			key := e.Name
			if !strings.Contains(key, ".") {
				key = l.program.Functions[l.function].Class + "." + key
			}
			target, ok := l.methods[key]
			if !ok {
				return nil, l.fail(e.Span, "unknown SDK operation or helper")
			}
			f := l.program.Functions[target]
			if !f.Public && f.Class != l.program.Functions[l.function].Class {
				return nil, l.fail(e.Span, "private cross-class helper")
			}
			x.Kind = "helper"
			x.Target = target
			x.Type = f.ReturnType
			params = f.Parameters
		}
		if len(e.Arguments) != len(params) {
			return nil, l.fail(e.Span, "composition call arity mismatch")
		}
		for i, arg := range e.Arguments {
			value, e2 := l.expression(arg, env, params[i])
			if e2 != nil {
				return nil, e2
			}
			x.Arguments = append(x.Arguments, value)
		}
	case *UnaryExpr:
		if e.Op == "-" {
			literal, ok := e.Expr.(*LiteralExpr)
			if !ok || literal.Value.Type != TypeInt {
				return nil, l.fail(e.Span, "only signed literal negation is supported")
			}
			x.Kind = "int"
			x.Type = streamir.Int
			x.Int = -literal.Value.Int
		} else if e.Op == "!" {
			value, e2 := l.expression(e.Expr, env, streamir.Bool)
			err = e2
			x.Kind = "not"
			x.Type = streamir.Bool
			x.Arguments = []*streamir.Expression{value}
		} else {
			return nil, l.fail(e.Span, "unsupported unary expression")
		}
	case *BinaryExpr:
		left, e2 := l.expression(e.Left, env, "")
		if e2 != nil {
			return nil, e2
		}
		right, e2 := l.expression(e.Right, env, "")
		if e2 != nil {
			return nil, e2
		}
		if left.Type == streamir.Uint64 && right.Kind == "int" && right.Int >= 0 {
			right = &streamir.Expression{Kind: "uint64", Type: streamir.Uint64, Uint: uint64(right.Int)}
		}
		if right.Type == streamir.Uint64 && left.Kind == "int" && left.Int >= 0 {
			left = &streamir.Expression{Kind: "uint64", Type: streamir.Uint64, Uint: uint64(left.Int)}
		}
		x.Kind = "binary"
		x.Type = streamir.Bool
		x.Member = e.Op
		x.Arguments = []*streamir.Expression{left, right}
	case *ConditionalExpr:
		condition, e2 := l.expression(e.Condition, env, streamir.Bool)
		if e2 != nil {
			return nil, e2
		}
		a, e2 := l.expression(e.WhenTrue, env, expected)
		if e2 != nil {
			return nil, e2
		}
		b, e2 := l.expression(e.WhenFalse, env, a.Type)
		if e2 != nil {
			return nil, e2
		}
		x.Kind = "conditional"
		x.Type = a.Type
		x.Arguments = []*streamir.Expression{condition, a, b}
	default:
		return nil, l.fail(source.SourceSpan(), "unsupported expression in stream composition")
	}
	if err != nil {
		return nil, err
	}
	if expected != "" && x.Type != expected {
		return nil, l.fail(source.SourceSpan(), "composition expression type mismatch")
	}
	return x, nil
}
