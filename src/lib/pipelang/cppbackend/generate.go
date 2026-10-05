// Package cppbackend emits a bounded C++17 pilot from validated Core only.
// It intentionally exposes no parser, Qt, host process or CLI dependency.
package cppbackend

import (
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"dockpipe/src/lib/pipelang/coreir"
)

//go:embed runtime.hpp
var support string

const Capability = "pipelang.cpp.pilot.v1"

type Binding struct {
	Identity coreir.SemanticIdentity
	Name     string
}
type Generated struct {
	Source    []byte
	Functions []Binding
}

func key(id coreir.SemanticIdentity) string { return id.PackageID + "\x00" + id.Path }
func name(prefix, value string) string {
	h := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%s%x", prefix, h[:12])
}
func typeKey(t coreir.Type) string { b, _ := json.Marshal(t); return string(b) }
func TypeName(t coreir.Type) (string, error) {
	switch t.Kind {
	case coreir.TypePrimitive:
		switch t.Primitive {
		case coreir.PrimitiveBool:
			return "bool", nil
		case coreir.PrimitiveString:
			return "std::string", nil
		}
	case coreir.TypeNumeric:
		if t.Numeric != nil && *t.Numeric == (coreir.NumericType{Representation: coreir.NumericInteger, Bits: 64, Signed: true}) {
			return "std::int64_t", nil
		}
	case coreir.TypeArithmeticError:
		return "Error", nil
	case coreir.TypeRecord:
		if t.Record != nil {
			for _, f := range t.Record.Fields {
				if f.Type.Kind != coreir.TypePrimitive && f.Type.Kind != coreir.TypeNumeric {
					return "", fmt.Errorf("pilot requires flat primitive record fields")
				}
				if _, e := TypeName(f.Type); e != nil {
					return "", e
				}
			}
			return name("Record_", typeKey(t)), nil
		}
	case coreir.TypeList:
		if t.List != nil && t.List.Element.Kind == coreir.TypeRecord {
			v, e := TypeName(t.List.Element)
			return "std::vector<" + v + ">", e
		}
	case coreir.TypeOptional:
		if t.Optional != nil && t.Optional.Value.Kind == coreir.TypeRecord {
			v, e := TypeName(t.Optional.Value)
			return "Optional<" + v + ">", e
		}
	case coreir.TypeResult:
		if t.Result != nil && t.Result.Failure.Kind == coreir.TypeArithmeticError && t.Result.Success.Kind == coreir.TypeNumeric {
			v, e := TypeName(t.Result.Success)
			return "Result<" + v + ">", e
		}
	}
	return "", fmt.Errorf("unsupported C++ pilot type %s", t.Kind)
}

// Validate first bounds the pointer graph, then applies full Core admission and
// a closed capability set. No unsupported expression can silently fall through.
func Validate(p coreir.Program) error {
	contracts := map[string]bool{coreir.LanguageContractV1130: true, coreir.LanguageContractV120: true, coreir.LanguageContractV170: true, coreir.LanguageContractV200: true, coreir.LanguageContractV220: true}
	if !contracts[p.LanguageContract] || p.CompilerContract != coreir.CompilerContractV1 {
		return fmt.Errorf("unsupported pilot Core contract")
	}
	if len(p.Functions) == 0 {
		return fmt.Errorf("empty pilot program")
	}
	ids := map[string]bool{}
	for _, f := range p.Functions {
		k := key(f.Identity)
		if f.Identity.PackageID == "" || f.Identity.Path == "" || ids[k] {
			return fmt.Errorf("missing or duplicate function identity")
		}
		ids[k] = true
		for i, p := range f.Parameters {
			if p.Position != i {
				return fmt.Errorf("noncanonical parameter position")
			}
		}
	}
	nodes := 0
	stack := map[uintptr]bool{}
	var walk func(reflect.Value, int) error
	walk = func(v reflect.Value, depth int) error {
		nodes++
		if nodes > 100000 || depth > 512 {
			return fmt.Errorf("pilot Core graph limit")
		}
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return nil
			}
			ptr := v.Pointer()
			if stack[ptr] {
				return fmt.Errorf("cyclic Core")
			}
			stack[ptr] = true
			defer delete(stack, ptr)
			return walk(v.Elem(), depth+1)
		}
		switch v.Kind() {
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if e := walk(v.Field(i), depth+1); e != nil {
					return e
				}
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				if e := walk(v.Index(i), depth+1); e != nil {
					return e
				}
			}
		}
		return nil
	}
	if e := walk(reflect.ValueOf(p), 0); e != nil {
		return e
	}
	var capability func(reflect.Value) error
	capability = func(v reflect.Value) error {
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return nil
			}
			return capability(v.Elem())
		}
		if v.Type() == reflect.TypeOf(coreir.Type{}) {
			if _, e := TypeName(v.Interface().(coreir.Type)); e != nil {
				return e
			}
		}
		if v.Type() == reflect.TypeOf(coreir.Expr{}) {
			x := v.Interface().(coreir.Expr)
			switch x.Kind {
			case coreir.ExprLiteral, coreir.ExprReference, coreir.ExprUnary, coreir.ExprBinary, coreir.ExprConditional, coreir.ExprImmutableLocal, coreir.ExprCall, coreir.ExprFieldProjection, coreir.ExprRecordConstruct, coreir.ExprOptionalSome, coreir.ExprOptionalNone, coreir.ExprOptionalHasValue, coreir.ExprOptionalValueOr, coreir.ExprPropagate, coreir.ExprMatch, coreir.ExprListEmpty, coreir.ExprListSingleton, coreir.ExprListCount, coreir.ExprListAppend, coreir.ExprListAt, coreir.ExprListFilterByText, coreir.ExprResultOK, coreir.ExprResultErr, coreir.ExprResultIsOK, coreir.ExprResultSuccessOr, coreir.ExprResultFailureOr:
			default:
				return fmt.Errorf("unsupported C++ pilot operation %s", x.Kind)
			}
		}
		switch v.Kind() {
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if e := capability(v.Field(i)); e != nil {
					return e
				}
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				if e := capability(v.Index(i)); e != nil {
					return e
				}
			}
		}
		return nil
	}
	if e := capability(reflect.ValueOf(p)); e != nil {
		return e
	}
	return coreir.ValidateProgram(p)
}

type emitter struct {
	strings.Builder
	n         int
	ret       string
	functions map[string]string
}

func (g *emitter) temp(typ, expr string) string {
	g.n++
	v := fmt.Sprintf("v%d", g.n)
	fmt.Fprintf(&g.Builder, "%s %s = %s;\n", typ, v, expr)
	return v
}
func clone(s map[int]string) map[int]string {
	n := map[int]string{}
	for k, v := range s {
		n[k] = v
	}
	return n
}
func cppString(s string) string {
	var b strings.Builder
	b.WriteString("std::string(\"")
	for _, c := range []byte(s) {
		fmt.Fprintf(&b, "\\%03o", c)
	}
	fmt.Fprintf(&b, "\", %d)", len(s))
	return b.String()
}
func integer(v int64) string {
	if v == (-1 << 63) {
		return "std::numeric_limits<std::int64_t>::min()"
	}
	return "std::int64_t(" + strconv.FormatInt(v, 10) + "LL)"
}
func Generate(p coreir.Program) ([]byte, error) { g, e := GenerateWithNames(p); return g.Source, e }
func GenerateWithNames(p coreir.Program) (Generated, error) {
	if e := Validate(p); e != nil {
		return Generated{}, e
	}
	fs := append([]coreir.Function(nil), p.Functions...)
	sort.Slice(fs, func(i, j int) bool { return key(fs[i].Identity) < key(fs[j].Identity) })
	g := &emitter{functions: map[string]string{}}
	_, _ = g.WriteString("// Generated by " + Capability + ". C++17 / GCC overflow intrinsics.\n" + support + "\nnamespace pipelang {\n")
	records := map[string]coreir.Type{}
	var collect func(reflect.Value)
	collect = func(v reflect.Value) {
		if v.Kind() == reflect.Pointer {
			if !v.IsNil() {
				collect(v.Elem())
			}
			return
		}
		if v.Type() == reflect.TypeOf(coreir.Type{}) {
			t := v.Interface().(coreir.Type)
			if t.Kind == coreir.TypeRecord {
				records[typeKey(t)] = t
			}
		}
		switch v.Kind() {
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				collect(v.Field(i))
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				collect(v.Index(i))
			}
		}
	}
	collect(reflect.ValueOf(p))
	keys := []string{}
	for k := range records {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t := records[k]
		n, _ := TypeName(t)
		fmt.Fprintf(&g.Builder, "struct %s {\n", n)
		for i, f := range t.Record.Fields {
			ft, _ := TypeName(f.Type)
			fmt.Fprintf(&g.Builder, "%s f%d{};\n", ft, i)
		}
		_, _ = g.WriteString("};\n")
		fmt.Fprintf(&g.Builder, "inline bool operator==(const %s& a,const %s& b){return true", n, n)
		for i := range t.Record.Fields {
			fmt.Fprintf(&g.Builder, " && a.f%d==b.f%d", i, i)
		}
		_, _ = g.WriteString(";}\n")
		fmt.Fprintf(&g.Builder, "inline bool operator!=(const %s& a,const %s& b){return !(a==b);}\n", n, n)
		fmt.Fprintf(&g.Builder, "inline void valid(const %s& x){", n)
		for i := range t.Record.Fields {
			fmt.Fprintf(&g.Builder, "valid(x.f%d);", i)
		}
		_, _ = g.WriteString("}\n")
	}
	bindings := []Binding{}
	declaration := func(f coreir.Function) string {
		ret, _ := TypeName(f.ReturnType)
		args := []string{}
		for i, p := range f.Parameters {
			t, _ := TypeName(p.Type)
			args = append(args, fmt.Sprintf("%s p%d", t, i))
		}
		return ret + " " + g.functions[key(f.Identity)] + "(" + strings.Join(args, ",") + ")"
	}
	for _, f := range fs {
		n := name("fn_", key(f.Identity))
		g.functions[key(f.Identity)] = n
		bindings = append(bindings, Binding{f.Identity, n})
	}
	for _, f := range fs {
		_, _ = g.WriteString("inline " + declaration(f) + ";\n")
	}
	for _, f := range fs {
		g.ret, _ = TypeName(f.ReturnType)
		_, _ = g.WriteString("inline " + declaration(f) + " {\n")
		scope := map[int]string{}
		for i := range f.Parameters {
			scope[i] = fmt.Sprintf("p%d", i)
			fmt.Fprintf(&g.Builder, "valid(p%d);\n", i)
		}
		v, e := g.expr(f.Body, scope)
		if e != nil {
			return Generated{}, fmt.Errorf("%s: %w", f.Name, e)
		}
		fmt.Fprintf(&g.Builder, "return %s;\n}\n", v)
	}
	_, _ = g.WriteString("}\n")
	return Generated{[]byte(g.String()), bindings}, nil
}
func (g *emitter) expr(x coreir.Expr, s map[int]string) (string, error) {
	typ, e := TypeName(x.Type)
	if e != nil {
		return "", e
	}
	sub := func(x *coreir.Expr) (string, error) { return g.expr(*x, s) }
	emit := func(expr string) (string, error) { return g.temp("auto", expr), nil }
	switch x.Kind {
	case coreir.ExprReference:
		v, ok := s[*x.Parameter]
		if !ok {
			return "", fmt.Errorf("missing reference")
		}
		return v, nil
	case coreir.ExprLiteral:
		switch x.Type.Kind {
		case coreir.TypeArithmeticError:
			if x.Literal.String == "overflow" {
				return emit("Error::overflow")
			}
			if x.Literal.String == "division_by_zero" {
				return emit("Error::division_by_zero")
			}
			return "", fmt.Errorf("bad error literal")
		case coreir.TypeNumeric:
			return emit(integer(x.Literal.Int))
		}
		if x.Type.Primitive == coreir.PrimitiveString {
			return emit(cppString(x.Literal.String))
		}
		return emit(strconv.FormatBool(x.Literal.Bool))
	case coreir.ExprUnary:
		a, e := sub(x.Unary.Operand)
		if e != nil {
			return "", e
		}
		if x.Unary.Operator == coreir.OperatorNot {
			return emit("!" + a)
		}
		return emit("negate(" + a + ")")
	case coreir.ExprBinary:
		b := x.Binary
		a, e := sub(b.Left)
		if e != nil {
			return "", e
		}
		if b.Operator == coreir.OperatorAnd || b.Operator == coreir.OperatorOr {
			v := g.temp("bool", a)
			cond := v
			if b.Operator == coreir.OperatorOr {
				cond = "!" + v
			}
			fmt.Fprintf(&g.Builder, "if(%s){\n", cond)
			c, e := sub(b.Right)
			if e != nil {
				return "", e
			}
			fmt.Fprintf(&g.Builder, "%s=%s;\n}\n", v, c)
			return v, nil
		}
		c, e := sub(b.Right)
		if e != nil {
			return "", e
		}
		switch b.Operator {
		case coreir.OperatorAdd, coreir.OperatorSubtract, coreir.OperatorMultiply:
			return emit(string(b.Operator) + "(" + a + "," + c + ")")
		}
		op := map[coreir.Operator]string{coreir.OperatorEqual: "==", coreir.OperatorNotEqual: "!=", coreir.OperatorLessThan: "<", coreir.OperatorLessOrEqual: "<=", coreir.OperatorGreaterThan: ">", coreir.OperatorGreaterOrEqual: ">="}[b.Operator]
		if op == "" {
			return "", fmt.Errorf("unsupported operator")
		}
		if b.Left.Type.Primitive == coreir.PrimitiveString {
			return emit("compare(" + a + "," + c + ")" + op + "0")
		}
		return emit(a + op + c)
	case coreir.ExprConditional:
		c, e := sub(x.Conditional.Condition)
		if e != nil {
			return "", e
		}
		v := g.temp(typ, "{}")
		fmt.Fprintf(&g.Builder, "if(%s){\n", c)
		a, e := g.expr(*x.Conditional.WhenTrue, clone(s))
		if e != nil {
			return "", e
		}
		fmt.Fprintf(&g.Builder, "%s=%s;\n}else{\n", v, a)
		b, e := g.expr(*x.Conditional.WhenFalse, clone(s))
		if e != nil {
			return "", e
		}
		fmt.Fprintf(&g.Builder, "%s=%s;\n}\n", v, b)
		return v, nil
	case coreir.ExprImmutableLocal:
		a, e := sub(x.ImmutableLocal.Initializer)
		if e != nil {
			return "", e
		}
		next := clone(s)
		next[x.ImmutableLocal.Position] = g.temp("auto", a)
		return g.expr(*x.ImmutableLocal.Return, next)
	case coreir.ExprCall:
		args := []string{}
		for _, a := range x.Call.Arguments {
			v, e := sub(a)
			if e != nil {
				return "", e
			}
			args = append(args, v)
		}
		n, ok := g.functions[key(x.Call.Target)]
		if !ok {
			return "", fmt.Errorf("unknown call")
		}
		return emit(n + "(" + strings.Join(args, ",") + ")")
	case coreir.ExprFieldProjection:
		a, e := sub(x.Field.Receiver)
		if e != nil {
			return "", e
		}
		return emit(fmt.Sprintf("%s.f%d", a, x.Field.Position))
	case coreir.ExprRecordConstruct:
		args := []string{}
		for _, f := range x.Record.Fields {
			v, e := sub(f.Value)
			if e != nil {
				return "", e
			}
			args = append(args, v)
		}
		return emit(typ + "{" + strings.Join(args, ",") + "}")
	case coreir.ExprListEmpty, coreir.ExprOptionalNone:
		return emit(typ + "{}")
	case coreir.ExprListSingleton:
		a, e := sub(x.ListOne.Value)
		if e != nil {
			return "", e
		}
		return emit(typ + "{" + a + "}")
	case coreir.ExprListCount:
		a, e := sub(x.ListCount.Value)
		if e != nil {
			return "", e
		}
		return emit("static_cast<std::int64_t>(" + a + ".size())")
	case coreir.ExprListAppend:
		a, e := sub(x.ListAppend.Values)
		if e != nil {
			return "", e
		}
		b, e := sub(x.ListAppend.Value)
		if e != nil {
			return "", e
		}
		return emit("append(" + a + "," + b + ")")
	case coreir.ExprListAt:
		a, e := sub(x.ListAt.Values)
		if e != nil {
			return "", e
		}
		b, e := sub(x.ListAt.Index)
		if e != nil {
			return "", e
		}
		return emit("at(" + a + "," + b + ")")
	case coreir.ExprListFilterByText:
		a, e := sub(x.ListFilter.Values)
		if e != nil {
			return "", e
		}
		b, e := sub(x.ListFilter.Key)
		if e != nil {
			return "", e
		}
		v := g.temp(typ, "{}")
		fmt.Fprintf(&g.Builder, "for(const auto& row:%s){if(row.f%d==%s)%s.push_back(row);}\n", a, x.ListFilter.Position, b, v)
		return v, nil
	case coreir.ExprOptionalSome:
		a, e := sub(x.Some.Value)
		if e != nil {
			return "", e
		}
		return emit(typ + "{true," + a + "}")
	case coreir.ExprOptionalHasValue:
		a, e := sub(x.HasValue.Value)
		if e != nil {
			return "", e
		}
		return emit(a + ".present")
	case coreir.ExprResultOK:
		a, e := sub(x.ResultOK.Value)
		if e != nil {
			return "", e
		}
		return emit("success(" + a + ")")
	case coreir.ExprResultErr:
		a, e := sub(x.ResultErr.Error)
		if e != nil {
			return "", e
		}
		return emit("failure<std::int64_t>(" + a + ")")
	case coreir.ExprResultIsOK:
		a, e := sub(x.ResultIsOK.Value)
		if e != nil {
			return "", e
		}
		return emit(a + ".ok")
	case coreir.ExprOptionalValueOr, coreir.ExprResultSuccessOr, coreir.ExprResultFailureOr:
		var input, fallback *coreir.Expr
		flag, field := "ok", "value"
		if x.ValueOr != nil {
			input = x.ValueOr.Value
			fallback = x.ValueOr.Fallback
			flag = "present"
		} else if x.SuccessOr != nil {
			input = x.SuccessOr.Value
			fallback = x.SuccessOr.Fallback
		} else {
			input = x.FailureOr.Value
			fallback = x.FailureOr.Fallback
			flag = "!ok"
			field = "error"
		}
		a, e := sub(input)
		if e != nil {
			return "", e
		}
		b, e := sub(fallback)
		if e != nil {
			return "", e
		}
		cond := a + "." + flag
		if flag == "!ok" {
			cond = "!" + a + ".ok"
		}
		return emit(cond + "?" + a + "." + field + ":" + b)
	case coreir.ExprPropagate:
		a, e := sub(x.Propagate.Value)
		if e != nil {
			return "", e
		}
		if x.Propagate.Carrier.Kind == coreir.TypeOptional {
			fmt.Fprintf(&g.Builder, "if(!%s.present)return {};\n", a)
		} else {
			fmt.Fprintf(&g.Builder, "if(!%s.ok)return %s{false,{},%s.error};\n", a, g.ret, a)
		}
		return emit(a + ".value")
	case coreir.ExprMatch:
		a, e := sub(x.Match.Value)
		if e != nil {
			return "", e
		}
		v := g.temp(typ, "{}")
		for i, arm := range x.Match.Arms {
			cond := a + ".ok"
			field := "value"
			switch arm.Tag {
			case "Ok", "ok":
			case "Err", "err":
				cond = "!" + a + ".ok"
				field = "error"
			case "Some", "some":
				cond = a + ".present"
			case "None", "none":
				cond = "!" + a + ".present"
			default:
				return "", fmt.Errorf("unsupported match tag %q", arm.Tag)
			}
			if i > 0 {
				_, _ = g.WriteString("else ")
			}
			fmt.Fprintf(&g.Builder, "if(%s){\n", cond)
			scope := clone(s)
			if arm.Binding != nil {
				scope[*arm.Binding] = a + "." + field
			}
			b, e := g.expr(*arm.Body, scope)
			if e != nil {
				return "", e
			}
			fmt.Fprintf(&g.Builder, "%s=%s;\n}\n", v, b)
		}
		return v, nil
	}
	return "", fmt.Errorf("unsupported expression %s", x.Kind)
}
