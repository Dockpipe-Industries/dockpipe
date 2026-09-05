package pipelang

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

func comparisonOracle[T int64 | float64](op string, left, right T) bool {
	switch op {
	case "==":
		return left == right
	case "!=":
		return left != right
	case "<":
		return left < right
	case "<=":
		return left <= right
	case ">":
		return left > right
	case ">=":
		return left >= right
	default:
		panic("unexpected test operator")
	}
}

func TestNumericComparisonParity(t *testing.T) {
	integers := []int64{math.MinInt64, math.MinInt64 + 1, -(1 << 53) - 1, -1, 0, 1, 1 << 53, 1<<53 + 1, math.MaxInt64 - 1, math.MaxInt64}
	floats := []float64{math.Inf(-1), -math.MaxFloat64, -1, -math.SmallestNonzeroFloat64, math.Copysign(0, -1), 0,
		math.SmallestNonzeroFloat64, math.Nextafter(1, 0), 1, math.Nextafter(1, 2), math.MaxFloat64, math.Inf(1), math.NaN(), math.Float64frombits(0xfff8000000000001)}
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV820, PipeLangLanguageContractV830} {
		for _, spelling := range []string{"int", "float"} {
			t.Run(string(contract)+"/"+spelling, func(t *testing.T) {
				var program coreir.Program
				var checks strings.Builder
				checks.WriteString("package pipelanggenerated\nimport (\"math\"; \"testing\")\nfunc TestComparisons(t *testing.T) { _ = math.Float64frombits\n")
				for index, op := range []string{"==", "!=", "<", "<=", ">", ">="} {
					name := fmt.Sprintf("Compare%d", index)
					source := fmt.Sprintf("public Class Root { public bool %s(%s left, %s right) => left %s right; }", name, spelling, spelling, op)
					unit := reviewCore(t, contract, source, name)
					if !reflect.DeepEqual(unit, reviewCore(t, contract, source, name)) {
						t.Fatal("repeated source/HIR/Core lowering differs")
					}
					function := coreFunctionNamed(t, unit, name)
					typ := coreir.SignedInteger(64)
					count := len(integers)
					if spelling == "float" {
						typ, count = coreir.BinaryFloat(64), len(floats)
					}
					if !coreir.TypeEqual(function.Body.Binary.Left.Type, typ) || !coreir.TypeEqual(function.Body.Binary.Right.Type, typ) ||
						function.Identity.Callable.Parameters[0].Primitive != coreir.PrimitiveType(spelling) {
						t.Fatal("normalized executable types or source-level identity changed")
					}
					for i := 0; i < count; i++ {
						for j := 0; j < count; j++ {
							left, right := coreeval.Value{Type: typ}, coreeval.Value{Type: typ}
							var leftGo, rightGo string
							var want bool
							if spelling == "int" {
								left.Int, right.Int = integers[i], integers[j]
								leftGo, rightGo = fmt.Sprintf("int64(%d)", left.Int), fmt.Sprintf("int64(%d)", right.Int)
								want = comparisonOracle(op, left.Int, right.Int)
							} else {
								left.Float, right.Float = floats[i], floats[j]
								leftGo, rightGo = fmt.Sprintf("math.Float64frombits(%d)", math.Float64bits(left.Float)), fmt.Sprintf("math.Float64frombits(%d)", math.Float64bits(right.Float))
								want = comparisonOracle(op, left.Float, right.Float)
							}
							args := []coreeval.Value{left, right}
							outcome, err := coreeval.EvaluateProgram(unit, function.Identity, args)
							if err != nil || !outcome.OK || outcome.Value.Bool != want || !coreir.TypeEqual(outcome.Value.Type, function.ReturnType) {
								t.Fatalf("%s pair %d,%d: %#v, %v; want %t", op, i, j, outcome, err, want)
							}
							direct, err := coreeval.Evaluate(function, args)
							if err != nil || !reflect.DeepEqual(direct, outcome) {
								t.Fatalf("standalone evaluation: %#v, %v", direct, err)
							}
							fmt.Fprintf(&checks, "if got := PipeLang%s(%s, %s); got != %t { t.Fatal(%q, got) }\n", name, leftGo, rightGo, want, fmt.Sprintf("%s pair %d,%d", op, i, j))
						}
					}
					if program.CompilerContract == "" {
						program = unit
					} else {
						program.Functions = append(program.Functions, unit.Functions...)
					}
				}
				checks.WriteString("}\n")
				generated, err := gobackend.Generate(program)
				if err != nil {
					t.Fatal(err)
				}
				again, err := gobackend.Generate(program)
				if err != nil || !bytes.Equal(generated, again) {
					t.Fatal("generated Go is nondeterministic")
				}
				compileAndRunGeneratedGoFiles(t, generated, []byte(checks.String()))
			})
		}
	}
}

func TestNumericComparisonRejectsMalformedCore(t *testing.T) {
	for name, typ := range map[string]coreir.Type{
		"missing":       {Kind: coreir.TypeNumeric},
		"width":         coreir.SignedInteger(32),
		"unsigned":      {Kind: coreir.TypeNumeric, Numeric: &coreir.NumericType{Representation: coreir.NumericInteger, Bits: 64}},
		"signed-float":  {Kind: coreir.TypeNumeric, Numeric: &coreir.NumericType{Representation: coreir.NumericBinaryFloat, Bits: 64, Signed: true}},
		"source-int":    {Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveInt},
		"source-float":  {Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveFloat},
		"contradictory": {Kind: coreir.TypeNumeric, Primitive: coreir.PrimitiveBool, Numeric: coreir.SignedInteger(64).Numeric},
	} {
		t.Run(name, func(t *testing.T) {
			program := reviewCore(t, PipeLangLanguageContractV820, `public Class Root { public bool Run(int left, int right) => left < right; }`, "Run")
			function := &program.Functions[0]
			function.Parameters[0].Type, function.Parameters[1].Type = typ, typ
			function.Body.Binary.Left.Type, function.Body.Binary.Right.Type = typ, typ
			if err := coreir.ValidateFunction(*function); err == nil {
				t.Fatal("malformed comparison admitted")
			}
			if _, err := coreeval.Evaluate(*function, []coreeval.Value{{Type: typ}, {Type: typ}}); err == nil {
				t.Fatal("standalone evaluator accepted malformed comparison")
			}
			assertAdmissionRejected(t, program, "")
		})
	}
	t.Run("mixed-representations", func(t *testing.T) {
		program := reviewCore(t, PipeLangLanguageContractV820, `public Class Root { public bool Run(int left, int right) => left < right; }`, "Run")
		function := &program.Functions[0]
		function.Parameters[1].Type = coreir.BinaryFloat(64)
		function.Body.Binary.Right.Type = coreir.BinaryFloat(64)
		assertAdmissionRejected(t, program, "")
	})
}

func TestNumericComparisonSourceBoundary(t *testing.T) {
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV820, PipeLangLanguageContractV830} {
		for _, expression := range []string{"integer < floating", "floating == integer", "integer >= flag"} {
			t.Run(string(contract)+"/"+expression, func(t *testing.T) {
				source := "public Class Root { public bool Run(int integer, float floating, bool flag) => " + expression + "; }"
				input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "invalid.pipe", source)}, nil)
				input.LanguageContract = contract
				diagnostics, ok := AsDiagnostics(AnalyzeSemanticModuleSet(input).Error())
				if !ok || len(diagnostics) == 0 {
					t.Fatal("mixed comparison accepted without a source diagnostic")
				}
				for _, diagnostic := range diagnostics {
					if !diagnostic.Primary.IsValid() || diagnostic.Primary.File != "invalid.pipe" || diagnostic.Primary.End > len(source) {
						t.Fatal("invalid comparison diagnostic location")
					}
				}
			})
		}
	}
}
