package pipelang

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	goparser "go/parser"
	gotoken "go/token"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

type hostArgumentCase struct {
	name    string
	value   coreeval.Value
	goValue string
	valid   bool
}

func TestCanonicalArgumentValidationUnusedOnlyHelpers(t *testing.T) {
	for _, tc := range []struct{ sourceType, invalid, valid string }{
		{"string", `string([]byte{0xff})`, `"valid"`},
		{"Result<int,ArithmeticError>", `PipeLangArithmeticResult[int64]{Error:"invented"}`, `PipeLangArithmeticResult[int64]{Error:PipeLangArithmeticOverflow}`},
		{"Result<float,ArithmeticError>", `PipeLangArithmeticResult[float64]{Error:"invented"}`, `PipeLangArithmeticResult[float64]{Error:PipeLangArithmeticDivisionByZero}`},
	} {
		t.Run(tc.sourceType, func(t *testing.T) {
			// Only the unused signature needs support: no other function, return
			// type, or operation can cause the helper/import to be emitted.
			program := reviewCore(t, PipeLangLanguageContractV800, fmt.Sprintf(`public Class Root {
 public bool Run(%s value) {
  if (true) { return true; } else { return true; }
 }
}`, tc.sourceType), "Run")
			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package pipelanggenerated
import "testing"
func TestUnused(t *testing.T) {
 if !PipeLangRun(%s) { t.Fatal("valid input changed") }
 defer func() { if recover() == nil { t.Fatal("malformed unused argument accepted") } }()
 PipeLangRun(%s)
}`, tc.valid, tc.invalid)))
		})
	}
}

// Exercise the actual source pipeline and host entrypoints. No match, propagation,
// or text operation in these bodies can accidentally supply argument validation.
func TestCanonicalArgumentValidation(t *testing.T) {
	for _, sourceType := range []string{"string", "Result<int,ArithmeticError>", "Result<float,ArithmeticError>", "Row", "Optional<string>", "Optional<Row>", "List<Row>", "Result<string,string>", "Result<List<Row>,string>"} {
		t.Run(sourceType, func(t *testing.T) {
			source := fmt.Sprintf(`public Record Row { public string Text; }
public Class Root {
 public %[1]s Echo(%[1]s value) => value;
 public string Unused(%[1]s value) {
  if (true) { return "kept"; } else { return "kept"; }
 }
 public string Branch(%[1]s value, bool selected) {
  if (selected) { return "selected"; }
  else { %[1]s used = value; return "other"; }
 }
}`, sourceType)
			var program coreir.Program
			for _, name := range []string{"Echo", "Unused", "Branch"} {
				lowered := reviewCore(t, PipeLangLanguageContractV800, source, name)
				if name == "Echo" {
					program = lowered
				} else {
					program.Functions = append(program.Functions, lowered.Functions...)
				}
			}
			prepared, err := coreeval.PrepareProgram(program)
			if err != nil {
				t.Fatal(err)
			}
			parameterType := coreFunctionNamed(t, program, "Echo").Parameters[0].Type
			cases := canonicalHostArguments(parameterType)
			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			again, err := gobackend.Generate(program)
			if err != nil || !bytes.Equal(generated, again) {
				t.Fatalf("non-deterministic generation: %v", err)
			}
			goType, payloadType := generatedArgumentTypes(t, generated)
			var tests strings.Builder
			tests.WriteString(hostArgumentGoHelpers)
			tests.WriteString("func TestArguments(t *testing.T) {\n")
			for _, tc := range cases {
				for _, entry := range []struct {
					name     string
					selected bool
				}{{"Echo", false}, {"Unused", false}, {"Branch", true}, {"Branch", false}} {
					label := fmt.Sprintf("%s/%s/%t", tc.name, entry.name, entry.selected)
					t.Run(label, func(t *testing.T) {
						function := coreFunctionNamed(t, program, entry.name)
						arguments := []coreeval.Value{tc.value}
						if entry.name == "Branch" {
							arguments = append(arguments, coreeval.Value{Type: function.Parameters[1].Type, Bool: entry.selected})
						}
						for _, evaluate := range []func() (coreeval.Outcome, error){
							func() (coreeval.Outcome, error) { return coreeval.Evaluate(function, arguments) },
							func() (coreeval.Outcome, error) { return prepared.Evaluate(function.Identity, arguments) },
							func() (coreeval.Outcome, error) {
								return coreeval.EvaluateProgram(program, function.Identity, arguments)
							},
						} {
							got, err := evaluate()
							if !tc.valid {
								if err == nil {
									t.Fatalf("evaluator accepted malformed argument: %#v", got)
								}
								continue
							}
							if err != nil {
								t.Fatal(err)
							}
							wantText := "other"
							if entry.name == "Unused" {
								wantText = "kept"
							} else if entry.selected {
								wantText = "selected"
							}
							want := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: function.ReturnType, String: wantText}}
							if entry.name == "Echo" {
								want.Value = tc.value
								if tc.value.Result != nil {
									want = *tc.value.Result
								}
							}
							// NaN is deliberately a valid successful binary64 payload.
							if math.IsNaN(want.Value.Float) {
								if !got.OK || !math.IsNaN(got.Value.Float) {
									t.Fatalf("NaN transport: %#v", got)
								}
							} else if !reflect.DeepEqual(got, want) {
								t.Fatalf("got %#v, want %#v", got, want)
							}
						}
					})
					expression := strings.NewReplacer("$T", goType, "$P", payloadType).Replace(tc.goValue)
					extra := ""
					if entry.name == "Branch" {
						extra = fmt.Sprintf(", %t", entry.selected)
					}
					fmt.Fprintf(&tests, "t.Run(%q, func(t *testing.T) { value := %s; checkHostArgument(t, %t, func() { got := PipeLang%s(value%s); ", label, expression, tc.valid, entry.name, extra)
					if entry.name == "Echo" {
						tests.WriteString("if !sameHostValue(reflect.ValueOf(got), reflect.ValueOf(value)) { t.Fatalf(\"transport changed: %v -> %v\", value, got) }; ")
					} else {
						wantText := "other"
						if entry.name == "Unused" {
							wantText = "kept"
						} else if entry.selected {
							wantText = "selected"
						}
						fmt.Fprintf(&tests, "if got != %q { t.Fatal(got) }; ", wantText)
					}
					tests.WriteString("}) })\n")
				}
			}
			tests.WriteString("}\n")
			compileAndRunGeneratedGoFiles(t, generated, []byte(tests.String()))
		})
	}
}

func canonicalHostArguments(typ coreir.Type) []hostArgumentCase {
	text := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}
	invalidText := string([]byte{0xff})
	var cases []hostArgumentCase
	add := func(name string, value coreeval.Value, goValue string, valid bool) {
		cases = append(cases, hostArgumentCase{name, value, goValue, valid})
	}
	switch typ.Kind {
	case coreir.TypePrimitive:
		for index, content := range []string{"", "é\u0301 世界", invalidText, string([]byte{0xc0, 0x80}), string([]byte{0xed, 0xa0, 0x80}), string([]byte{0xe2, 0x82})} {
			add(fmt.Sprintf("text-%x", content), coreeval.Value{Type: typ, String: content}, strconv.Quote(content), index < 2)
		}
	case coreir.TypeRecord, coreir.TypeList, coreir.TypeOptional:
		for _, valid := range []bool{true, false} {
			content := "é 世界"
			if !valid {
				content = invalidText
			}
			var makeValue func(coreir.Type) coreeval.Value
			makeValue = func(target coreir.Type) coreeval.Value {
				value := coreeval.Value{Type: target}
				switch target.Kind {
				case coreir.TypePrimitive:
					value.String = content
				case coreir.TypeRecord:
					value.Record = []coreeval.Value{makeValue(target.Record.Fields[0].Type)}
				case coreir.TypeList:
					value.List = []coreeval.Value{makeValue(target.List.Element)}
				case coreir.TypeOptional:
					payload := makeValue(target.Optional.Value)
					value.Optional = &coreeval.OptionalValue{Present: true, Value: &payload}
				}
				return value
			}
			goValue := "hostTextValue[$T](" + strconv.Quote(content) + ")"
			if typ.Kind == coreir.TypeOptional {
				goValue = "($T)(pipelangOptionalSome[$P]{value: hostTextValue[$P](" + strconv.Quote(content) + ")})"
			}
			add(fmt.Sprintf("nested-text-%t", valid), makeValue(typ), goValue, valid)
		}
		if typ.Kind == coreir.TypeOptional {
			add("none", coreeval.Value{Type: typ, Optional: &coreeval.OptionalValue{}}, "($T)(pipelangOptionalNone[$P]{})", true)
			add("nil-optional", coreeval.Value{Type: typ}, "($T)(nil)", false)
		}
		if typ.Kind == coreir.TypeList {
			add("empty-list", coreeval.Value{Type: typ, List: []coreeval.Value{}}, "make($T, 0)", true)
			add("nil-list", coreeval.Value{Type: typ}, "($T)(nil)", false)
		}
	case coreir.TypeResult:
		payload := typ.Result.Success
		result := func(ok bool, value coreeval.Value, failure string) coreeval.Value {
			outcome := &coreeval.Outcome{OK: ok, Value: value}
			if typ.Result.Failure.Kind == coreir.TypeArithmeticError {
				outcome.Error = coreir.ArithmeticError(failure)
			} else if !ok {
				outcome.Failure = &coreeval.Value{Type: text, String: failure}
			}
			return coreeval.Value{Type: typ, Result: outcome}
		}
		if typ.Result.Failure.Kind == coreir.TypeArithmeticError {
			for _, success := range []bool{true, false} {
				for _, failure := range []string{"", "overflow", "division_by_zero", "invented"} {
					for _, nonzero := range []bool{false, true} {
						value := coreeval.Value{Type: payload}
						numeric := "0"
						if nonzero {
							numeric = "7"
							if payload.Numeric.Representation == coreir.NumericInteger {
								value.Int = 7
							} else {
								value.Float = 7
							}
						}
						valid := success && failure == "" || !success && !nonzero && (failure == "overflow" || failure == "division_by_zero")
						add(fmt.Sprintf("arithmetic-%t-%s-%t", success, failure, nonzero), result(success, value, failure), fmt.Sprintf("$T{OK:%t, Value:%s, Error:%q}", success, numeric, failure), valid)
					}
				}
			}
			if payload.Numeric.Representation == coreir.NumericBinaryFloat {
				for _, number := range []struct {
					name, expr string
					value      float64
				}{{"nan", "math.NaN()", math.NaN()}, {"infinity", "math.Inf(1)", math.Inf(1)}, {"negative-zero", "math.Copysign(0, -1)", math.Copysign(0, -1)}} {
					value := coreeval.Value{Type: payload, Float: number.value}
					add(number.name+"-success", result(true, value, ""), "$T{OK:true, Value:"+number.expr+"}", true)
					add(number.name+"-failure", result(false, value, "overflow"), "$T{Value:"+number.expr+", Error:PipeLangArithmeticOverflow}", number.value == 0)
				}
			}
		} else {
			for _, valid := range []bool{true, false} {
				content := "é 世界"
				if !valid {
					content = invalidText
				}
				value := coreeval.Value{Type: payload, String: content}
				if payload.Kind == coreir.TypeList {
					value.String = ""
					value.List = []coreeval.Value{{Type: payload.List.Element, Record: []coreeval.Value{{Type: text, String: content}}}}
				}
				add(fmt.Sprintf("result-success-text-%t", valid), result(true, value, ""), "$T{OK:true, Value:hostTextValue[$P]("+strconv.Quote(content)+")}", valid)
				add(fmt.Sprintf("result-failure-text-%t", valid), result(false, coreeval.Value{Type: payload}, content), "$T{Error:"+strconv.Quote(content)+"}", valid)
			}
		}
	}
	return cases
}

// Read generated type spelling so tests do not duplicate record/name allocation.
func generatedArgumentTypes(t *testing.T, generated []byte) (string, string) {
	t.Helper()
	fset := gotoken.NewFileSet()
	file, err := goparser.ParseFile(fset, "generated.go", generated, 0)
	if err != nil {
		t.Fatal(err)
	}
	render := func(node ast.Node) string {
		var out bytes.Buffer
		if err := format.Node(&out, fset, node); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "PipeLangEcho" {
			continue
		}
		typ := function.Type.Params.List[0].Type
		payload := ""
		switch generic := typ.(type) {
		case *ast.IndexExpr:
			payload = render(generic.Index)
		case *ast.IndexListExpr:
			payload = render(generic.Indices[0])
		}
		return render(typ), payload
	}
	t.Fatal("generated Echo declaration missing")
	return "", ""
}

const hostArgumentGoHelpers = `package pipelanggenerated
import ("math"; "reflect"; "testing")
func checkHostArgument(t *testing.T, valid bool, call func()) {
 t.Helper()
 panicked := true
 func() { defer func() { _ = recover() }(); call(); panicked = false }()
 if panicked == valid { t.Fatalf("panic=%t, valid=%t", panicked, valid) }
}
func hostTextValue[T any](text string) T {
 var value T
 var fill func(reflect.Value)
 fill = func(v reflect.Value) {
  switch v.Kind() {
  case reflect.String: v.SetString(text)
  case reflect.Struct: for i:=0; i<v.NumField(); i++ { fill(v.Field(i)) }
  case reflect.Slice: v.Set(reflect.MakeSlice(v.Type(), 1, 1)); fill(v.Index(0))
  default: panic("unexpected test payload")
  }
 }
 fill(reflect.ValueOf(&value).Elem())
 return value
}
func sameHostValue(a, b reflect.Value) bool {
 if a.Type() != b.Type() { return false }
 switch a.Kind() {
 case reflect.Float64: return (a.Float() == b.Float() && math.Signbit(a.Float()) == math.Signbit(b.Float())) || (math.IsNaN(a.Float()) && math.IsNaN(b.Float()))
 case reflect.Interface: if a.IsNil() || b.IsNil() { return a.IsNil() == b.IsNil() }; return sameHostValue(a.Elem(), b.Elem())
 case reflect.Struct: for i:=0; i<a.NumField(); i++ { if !sameHostValue(a.Field(i), b.Field(i)) { return false } }; return true
 case reflect.Slice: if a.IsNil() != b.IsNil() || a.Len() != b.Len() { return false }; for i:=0; i<a.Len(); i++ { if !sameHostValue(a.Index(i), b.Index(i)) { return false } }; return true
 case reflect.String: return a.String() == b.String()
 case reflect.Bool: return a.Bool() == b.Bool()
 case reflect.Int64: return a.Int() == b.Int()
 default: panic("unexpected test comparison")
 }
}
`
