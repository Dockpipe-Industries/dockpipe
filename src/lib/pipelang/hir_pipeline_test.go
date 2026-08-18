package pipelang

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	goparser "go/parser"
	gotoken "go/token"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
)

var _ func(coreir.Program) ([]byte, error) = gobackend.Generate

const tinyPureFunctionSource = `public Class Root { public bool Ready(int count) => count > 0; }`
const checkedAddSource = `public Class Root { public Result<int, ArithmeticError> Add(int left, int right) => left + right; }`
const checkedSubtractSource = `public Class Root { public Result<int, ArithmeticError> Subtract(int left, int right) => left - right; }`
const checkedMultiplySource = `public Class Root { public Result<int, ArithmeticError> Multiply(int left, int right) => left * right; }`
const checkedNegateSource = `public Class Root { public Result<int, ArithmeticError> Negate(int value) => -value; }`
const checkedDivideSource = `public Class Root { public Result<float, ArithmeticError> Divide(float left, float right) => left / right; }`

func TestTinyPureFunctionLowersThroughTypedHIRCoreAndGo(t *testing.T) {
	module := testModule("app.root", "root.pipe", tinyPureFunctionSource)
	analysis := AnalyzeSemanticModuleSet(semanticTestModuleSet("app.root", []ModuleInput{module}, nil))
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	method := semanticMethod(t, analysis)
	typed, err := LowerSemanticMethodToHIR(analysis, method.Identity)
	if err != nil {
		t.Fatal(err)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	coreJSON := canonicalJSON(t, core)
	for _, forbidden := range [][]byte{[]byte("root.pipe"), []byte("symbol_id"), []byte("source_span"), []byte("go_type")} {
		if bytes.Contains(coreJSON, forbidden) {
			t.Fatalf("Core IR leaked source, analysis-local, or Go concept %q:\n%s", forbidden, coreJSON)
		}
	}
	assertCompilerGolden(t, "tiny-pure-function.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "tiny-pure-function.core.json", coreJSON)
	assertCompilerGolden(t, "tiny-pure-function.go", generated)

	secondGenerated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, secondGenerated) {
		t.Fatal("Go backend output changed for identical Core IR")
	}

	evaluated, err := InvokeFiles(map[string][]byte{"root.pipe": []byte(tinyPureFunctionSource)}, "Root", "Ready", []string{"2"})
	if err != nil {
		t.Fatal(err)
	}
	if evaluated.Value.Type != TypeBool {
		t.Fatalf("reference evaluator result = %#v", evaluated.Value)
	}
	compileAndRunGeneratedGo(t, generated, gobackend.FunctionName(core.Functions[0]), evaluated.Value.Bool)
}

func TestHIRLoweringFailureRetainsStructuredDiagnostic(t *testing.T) {
	module := testModule("app.root", "root.pipe", `public Class Root { public int Count; public bool Ready(int threshold) => Count > threshold; }`)
	analysis := AnalyzeSemanticModuleSet(semanticTestModuleSet("app.root", []ModuleInput{module}, nil))
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	method := semanticMethod(t, analysis)
	_, err := LowerSemanticMethodToHIR(analysis, method.Identity)
	diagnostics, ok := AsDiagnostics(err)
	if !ok || len(diagnostics) != 1 {
		t.Fatalf("HIR error = %v, diagnostics = %#v", err, diagnostics)
	}
	diagnostic := diagnostics[0]
	if diagnostic.Code != CodeHIRLowering || diagnostic.Category != CategorySemantic || diagnostic.Primary.File != "root.pipe" || !reflectSemanticIdentities(diagnostic.SemanticIdentities, []SemanticIdentity{method.Identity}) {
		t.Fatalf("HIR diagnostic = %#v", diagnostic)
	}
	first, jsonErr := DiagnosticsJSON(analysis.Sources, diagnostics)
	if jsonErr != nil {
		t.Fatal(jsonErr)
	}
	_, err = LowerSemanticMethodToHIR(analysis, method.Identity)
	secondDiagnostics, _ := AsDiagnostics(err)
	second, jsonErr := DiagnosticsJSON(analysis.Sources, secondDiagnostics)
	if jsonErr != nil {
		t.Fatal(jsonErr)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("HIR diagnostic bytes are nondeterministic:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestHIRLoweringPreservesExistingAnalysisFailure(t *testing.T) {
	module := testModule("app.root", "root.pipe", `public Class Root { public bool Ready(int threshold) => threshold + true; }`)
	analysis := AnalyzeSemanticModuleSet(semanticTestModuleSet("app.root", []ModuleInput{module}, nil))
	if len(analysis.Diagnostics) != 1 || analysis.Diagnostics[0].Code != CodeExpressionType {
		t.Fatalf("analysis diagnostics = %#v", analysis.Diagnostics)
	}
	_, err := LowerSemanticMethodToHIR(analysis, SemanticIdentity{})
	diagnostics, ok := AsDiagnostics(err)
	if !ok || len(diagnostics) != 1 || diagnostics[0].Code != CodeExpressionType || diagnostics[0].Primary != analysis.Diagnostics[0].Primary {
		t.Fatalf("lowering replaced existing diagnostic: %#v (%v)", diagnostics, err)
	}
}

func TestGoBackendRejectsUnsupportedCoreCapability(t *testing.T) {
	integer := coreInteger64()
	floating := coreBinary64()
	for _, operator := range []coreir.Operator{coreir.OperatorAdd, coreir.OperatorSubtract, coreir.OperatorMultiply, coreir.OperatorDivide} {
		t.Run(string(operator), func(t *testing.T) {
			operandType := integer
			resultType := integer
			if operator == coreir.OperatorDivide {
				operandType = floating
				resultType = floating
			}
			left := coreir.Expr{Kind: coreir.ExprLiteral, Type: operandType, Literal: &coreir.Literal{Int: 4, Float: 4}}
			right := coreir.Expr{Kind: coreir.ExprLiteral, Type: operandType, Literal: &coreir.Literal{Int: 2, Float: 2}}
			program := coreir.Program{LanguageContract: coreir.LanguageContractV010, CompilerContract: coreir.CompilerContractV1, Functions: []coreir.Function{{
				Identity: coreir.SemanticIdentity{PackageID: "test.package", Path: "app.root.calculate"}, Name: "Calculate", ReturnType: resultType,
				Body: coreir.Expr{Kind: coreir.ExprBinary, Type: resultType, Binary: &coreir.Binary{Operator: operator, Left: &left, Right: &right}},
			}}}
			_, err := gobackend.Generate(program)
			var backendErr *gobackend.Error
			if !errors.As(err, &backendErr) || backendErr.Code != "PLGO0001" || !strings.Contains(backendErr.Message, "invalid Result type") {
				t.Fatalf("backend capability error = %#v (%v)", backendErr, err)
			}
		})
	}
}

func TestV010NumericPolicyRejectsImplicitConversionButLegacyRemainsFrozen(t *testing.T) {
	source := `public Class Root { public bool Compare(int left, float right) => left < right; }`
	legacy := AnalyzeFiles(map[string][]byte{"root.pipe": []byte(source)})
	if err := legacy.Error(); err != nil {
		t.Fatalf("legacy numeric policy drifted: %v", err)
	}
	analysis := AnalyzeSemanticModuleSet(semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil))
	diagnostic := assertDiagnosticCode(t, analysis, CodeNumericSemantics)
	if diagnostic.Category != CategorySemantic || !strings.Contains(diagnostic.Message, "does not implicitly convert int and float") {
		t.Fatalf("numeric conversion diagnostic = %#v", diagnostic)
	}
}

func TestV010NumericPolicyRejectsUncheckedArithmetic(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "add", source: `public Class Root { public int Calculate(int left, int right) => left + right; }`},
		{name: "subtract", source: `public Class Root { public int Calculate(int left, int right) => left - right; }`},
		{name: "multiply", source: `public Class Root { public int Calculate(int left, int right) => left * right; }`},
		{name: "divide", source: `public Class Root { public float Calculate(int left, int right) => left / right; }`},
		{name: "negate", source: `public Class Root { public int Calculate(int value) => -value; }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analysis := AnalyzeSemanticModuleSet(semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.source)}, nil))
			diagnostic := assertDiagnosticCode(t, analysis, CodeNumericSemantics)
			if !strings.Contains(diagnostic.Message, "checked Result return") {
				t.Fatalf("numeric arithmetic diagnostic = %#v", diagnostic)
			}
		})
	}
}

func TestGoBackendRejectsMismatchedNumericCoreOperands(t *testing.T) {
	integer := coreInteger64()
	floating := coreBinary64()
	left := coreir.Expr{Kind: coreir.ExprLiteral, Type: integer, Literal: &coreir.Literal{Int: 1}}
	right := coreir.Expr{Kind: coreir.ExprLiteral, Type: floating, Literal: &coreir.Literal{Float: 2}}
	program := coreir.Program{LanguageContract: coreir.LanguageContractV010, CompilerContract: coreir.CompilerContractV1, Functions: []coreir.Function{{
		Identity: coreir.SemanticIdentity{PackageID: "test.package", Path: "app.root.compare"}, Name: "Compare", ReturnType: coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveBool},
		Body: coreir.Expr{Kind: coreir.ExprBinary, Type: coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveBool}, Binary: &coreir.Binary{Operator: coreir.OperatorLessThan, Left: &left, Right: &right}},
	}}}
	_, err := gobackend.Generate(program)
	var backendErr *gobackend.Error
	if !errors.As(err, &backendErr) || backendErr.Code != "PLGO0001" || !strings.Contains(backendErr.Message, "mismatched operand or result types") {
		t.Fatalf("backend mismatch error = %#v (%v)", backendErr, err)
	}
}

func TestCheckedArithmeticHIRCoreEvaluatorAndGoConform(t *testing.T) {
	typed := hir.Program{LanguageContract: coreir.LanguageContractV010, CompilerContract: coreir.CompilerContractV1, Functions: []hir.Function{
		hirCheckedBinaryFunction("Add", hir.OperatorAdd, hirInteger64()),
		hirCheckedBinaryFunction("Subtract", hir.OperatorSubtract, hirInteger64()),
		hirCheckedBinaryFunction("Multiply", hir.OperatorMultiply, hirInteger64()),
		hirCheckedUnaryFunction("Negate", hir.OperatorNegate, hirInteger64()),
		hirCheckedBinaryFunction("Divide", hir.OperatorDivide, hirBinary64()),
	}}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	second, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, second) {
		t.Fatal("checked arithmetic Go output is nondeterministic")
	}
	functions := map[string]coreir.Function{}
	goNames := map[string]string{}
	for _, function := range core.Functions {
		functions[function.Name] = function
		goNames[function.Name] = gobackend.FunctionName(function)
	}
	tests := []struct {
		name      string
		arguments []coreeval.Value
		wantInt   int64
		wantFloat float64
		wantError coreir.ArithmeticError
		wantNaN   bool
	}{
		{name: "Add", arguments: coreIntArguments(2, 3), wantInt: 5},
		{name: "Add", arguments: coreIntArguments(math.MaxInt64, 1), wantError: coreir.ArithmeticOverflow},
		{name: "Subtract", arguments: coreIntArguments(5, 3), wantInt: 2},
		{name: "Subtract", arguments: coreIntArguments(math.MinInt64, 1), wantError: coreir.ArithmeticOverflow},
		{name: "Multiply", arguments: coreIntArguments(-3, 4), wantInt: -12},
		{name: "Multiply", arguments: coreIntArguments(math.MaxInt64, 2), wantError: coreir.ArithmeticOverflow},
		{name: "Negate", arguments: coreIntArguments(5), wantInt: -5},
		{name: "Negate", arguments: coreIntArguments(math.MinInt64), wantError: coreir.ArithmeticOverflow},
		{name: "Divide", arguments: coreFloatArguments(6, 2), wantFloat: 3},
		{name: "Divide", arguments: coreFloatArguments(1, 0), wantError: coreir.ArithmeticDivisionByZero},
		{name: "Divide", arguments: coreFloatArguments(1, math.Copysign(0, -1)), wantError: coreir.ArithmeticDivisionByZero},
		{name: "Divide", arguments: coreFloatArguments(math.NaN(), 2), wantNaN: true},
	}
	for _, test := range tests {
		outcome, err := coreeval.Evaluate(functions[test.name], test.arguments)
		if err != nil {
			t.Fatal(err)
		}
		if outcome.Error != test.wantError || outcome.OK != (test.wantError == "") {
			t.Fatalf("%s outcome = %#v", test.name, outcome)
		}
		if outcome.OK && ((test.wantNaN && !math.IsNaN(outcome.Value.Float)) || (!test.wantNaN && test.name == "Divide" && outcome.Value.Float != test.wantFloat) || (test.name != "Divide" && outcome.Value.Int != test.wantInt)) {
			t.Fatalf("%s outcome value = %#v", test.name, outcome.Value)
		}
	}
	compileAndRunCheckedArithmeticGo(t, generated, goNames)
}

func TestCheckedAddHIRCoreAndGoGoldens(t *testing.T) {
	module := testModule("app.root", "root.pipe", checkedAddSource)
	input := semanticTestModuleSet("app.root", []ModuleInput{module}, nil)
	input.LanguageContract = PipeLangLanguageContractV020
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	method := semanticMethodNamed(t, analysis, "Add")
	if method.Identity.Callable == nil {
		t.Fatal("checked Add has no callable identity")
	}
	returns := method.Identity.Callable.Returns
	if returns.Kind != TypeRefApplied || returns.Name != "Result" || returns.PackageID != PipeLangBuiltinPackageID || returns.Path != PipeLangResultSemanticPath || len(returns.Arguments) != 2 || returns.Arguments[0].Primitive != TypeInt || returns.Arguments[1].Kind != TypeRefNamed || returns.Arguments[1].PackageID != PipeLangBuiltinPackageID || returns.Arguments[1].Path != PipeLangArithmeticErrorSemanticPath {
		t.Fatalf("checked Add callable return = %#v", returns)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV020 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("checked Add projection header = %#v", projection)
	}
	projected := projectedMemberNamed(t, projection.Modules[0].Types[0], "Add").Type
	if projected.Kind != TypeRefApplied || projected.Name != "Result" || projected.Identity == nil || projected.Identity.PackageID != PipeLangBuiltinPackageID || projected.Identity.Path != PipeLangResultSemanticPath || len(projected.Arguments) != 2 || projected.Arguments[1].Identity == nil || projected.Arguments[1].Identity.PackageID != PipeLangBuiltinPackageID || projected.Arguments[1].Identity.Path != PipeLangArithmeticErrorSemanticPath {
		t.Fatalf("checked Add projected return = %#v", projected)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, method.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if typed.LanguageContract != coreir.LanguageContractV020 || len(typed.Functions) != 1 || typed.Functions[0].ReturnType.Kind != hir.TypeResult || typed.Functions[0].Body.Type.Kind != hir.TypeResult {
		t.Fatalf("checked Add HIR = %#v", typed)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name      string
		arguments []coreeval.Value
		ok        bool
		value     int64
		failure   coreir.ArithmeticError
	}{
		{name: "success", arguments: coreIntArguments(2, 3), ok: true, value: 5},
		{name: "overflow", arguments: coreIntArguments(math.MaxInt64, 1), failure: coreir.ArithmeticOverflow},
	} {
		outcome, evaluateErr := coreeval.Evaluate(core.Functions[0], test.arguments)
		if evaluateErr != nil {
			t.Fatalf("%s Core evaluation: %v", test.name, evaluateErr)
		}
		if outcome.OK != test.ok || outcome.Value.Int != test.value || outcome.Error != test.failure {
			t.Fatalf("%s Core outcome = %#v", test.name, outcome)
		}
	}
	compileAndRunCheckedAddGo(t, generated, gobackend.FunctionName(core.Functions[0]))
	assertCompilerGolden(t, "checked-add.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "checked-add.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "checked-add.go", generated)
}

func TestCheckedSubtractHIRCoreAndGoGoldens(t *testing.T) {
	module := testModule("app.root", "root.pipe", checkedSubtractSource)
	input := semanticTestModuleSet("app.root", []ModuleInput{module}, nil)
	input.LanguageContract = PipeLangLanguageContractV030
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	method := semanticMethodNamed(t, analysis, "Subtract")
	if method.Identity.Callable == nil {
		t.Fatal("checked Subtract has no callable identity")
	}
	callable := method.Identity.Callable
	returns := callable.Returns
	if len(callable.Parameters) != 2 || callable.Parameters[0].Primitive != TypeInt || callable.Parameters[1].Primitive != TypeInt || returns.Kind != TypeRefApplied || returns.Name != "Result" || returns.PackageID != PipeLangBuiltinPackageID || returns.Path != PipeLangResultSemanticPath || len(returns.Arguments) != 2 || returns.Arguments[0].Primitive != TypeInt || returns.Arguments[1].Kind != TypeRefNamed || returns.Arguments[1].PackageID != PipeLangBuiltinPackageID || returns.Arguments[1].Path != PipeLangArithmeticErrorSemanticPath {
		t.Fatalf("checked Subtract callable identity = %#v", callable)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV030 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("checked Subtract projection header = %#v", projection)
	}
	projected := projectedMemberNamed(t, projection.Modules[0].Types[0], "Subtract").Type
	if projected.Kind != TypeRefApplied || projected.Name != "Result" || projected.Identity == nil || projected.Identity.PackageID != PipeLangBuiltinPackageID || projected.Identity.Path != PipeLangResultSemanticPath || len(projected.Arguments) != 2 || projected.Arguments[0].Primitive != TypeInt || projected.Arguments[1].Identity == nil || projected.Arguments[1].Identity.PackageID != PipeLangBuiltinPackageID || projected.Arguments[1].Identity.Path != PipeLangArithmeticErrorSemanticPath {
		t.Fatalf("checked Subtract projected return = %#v", projected)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, method.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if typed.LanguageContract != coreir.LanguageContractV030 || len(typed.Functions) != 1 || typed.Functions[0].ReturnType.Kind != hir.TypeResult || typed.Functions[0].Body.Type.Kind != hir.TypeResult || typed.Functions[0].Body.Binary == nil || typed.Functions[0].Body.Binary.Operator != hir.OperatorSubtract {
		t.Fatalf("checked Subtract HIR = %#v", typed)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if core.Functions[0].Body.Binary == nil || core.Functions[0].Body.Binary.Operator != coreir.OperatorSubtract {
		t.Fatalf("checked Subtract Core = %#v", core)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	secondGenerated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, secondGenerated) {
		t.Fatal("checked Subtract generated Go is nondeterministic")
	}
	for _, test := range []struct {
		name      string
		arguments []coreeval.Value
		ok        bool
		value     int64
		failure   coreir.ArithmeticError
	}{
		{name: "success", arguments: coreIntArguments(5, 3), ok: true, value: 2},
		{name: "negative success", arguments: coreIntArguments(-5, -3), ok: true, value: -2},
		{name: "minimum underflow", arguments: coreIntArguments(math.MinInt64, 1), failure: coreir.ArithmeticOverflow},
		{name: "maximum overflow", arguments: coreIntArguments(math.MaxInt64, -1), failure: coreir.ArithmeticOverflow},
	} {
		outcome, evaluateErr := coreeval.Evaluate(core.Functions[0], test.arguments)
		if evaluateErr != nil {
			t.Fatalf("%s Core evaluation: %v", test.name, evaluateErr)
		}
		if outcome.OK != test.ok || outcome.Value.Int != test.value || outcome.Error != test.failure {
			t.Fatalf("%s Core outcome = %#v", test.name, outcome)
		}
	}
	compileAndRunCheckedSubtractGo(t, generated, gobackend.FunctionName(core.Functions[0]))
	assertCompilerGolden(t, "checked-subtract.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "checked-subtract.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "checked-subtract.go", generated)
}

func TestCheckedMultiplyHIRCoreAndGoGoldens(t *testing.T) {
	module := testModule("app.root", "root.pipe", checkedMultiplySource)
	input := semanticTestModuleSet("app.root", []ModuleInput{module}, nil)
	input.LanguageContract = PipeLangLanguageContractV040
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	method := semanticMethodNamed(t, analysis, "Multiply")
	if method.Identity.Callable == nil {
		t.Fatal("checked Multiply has no callable identity")
	}
	callable := method.Identity.Callable
	returns := callable.Returns
	if len(callable.Parameters) != 2 || callable.Parameters[0].Primitive != TypeInt || callable.Parameters[1].Primitive != TypeInt || returns.Kind != TypeRefApplied || returns.Name != "Result" || returns.PackageID != PipeLangBuiltinPackageID || returns.Path != PipeLangResultSemanticPath || len(returns.Arguments) != 2 || returns.Arguments[0].Primitive != TypeInt || returns.Arguments[1].Kind != TypeRefNamed || returns.Arguments[1].PackageID != PipeLangBuiltinPackageID || returns.Arguments[1].Path != PipeLangArithmeticErrorSemanticPath {
		t.Fatalf("checked Multiply callable identity = %#v", callable)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV040 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("checked Multiply projection header = %#v", projection)
	}
	projected := projectedMemberNamed(t, projection.Modules[0].Types[0], "Multiply").Type
	if projected.Kind != TypeRefApplied || projected.Name != "Result" || projected.Identity == nil || projected.Identity.PackageID != PipeLangBuiltinPackageID || projected.Identity.Path != PipeLangResultSemanticPath || len(projected.Arguments) != 2 || projected.Arguments[0].Primitive != TypeInt || projected.Arguments[1].Identity == nil || projected.Arguments[1].Identity.PackageID != PipeLangBuiltinPackageID || projected.Arguments[1].Identity.Path != PipeLangArithmeticErrorSemanticPath {
		t.Fatalf("checked Multiply projected return = %#v", projected)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, method.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if typed.LanguageContract != coreir.LanguageContractV040 || len(typed.Functions) != 1 || typed.Functions[0].ReturnType.Kind != hir.TypeResult || typed.Functions[0].Body.Type.Kind != hir.TypeResult || typed.Functions[0].Body.Binary == nil || typed.Functions[0].Body.Binary.Operator != hir.OperatorMultiply {
		t.Fatalf("checked Multiply HIR = %#v", typed)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if core.Functions[0].Body.Binary == nil || core.Functions[0].Body.Binary.Operator != coreir.OperatorMultiply {
		t.Fatalf("checked Multiply Core = %#v", core)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	secondGenerated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, secondGenerated) {
		t.Fatal("checked Multiply generated Go is nondeterministic")
	}
	for _, test := range []struct {
		name      string
		arguments []coreeval.Value
		ok        bool
		value     int64
		failure   coreir.ArithmeticError
	}{
		{name: "positive success", arguments: coreIntArguments(6, 7), ok: true, value: 42},
		{name: "negative success", arguments: coreIntArguments(-3, 4), ok: true, value: -12},
		{name: "zero success", arguments: coreIntArguments(math.MinInt64, 0), ok: true},
		{name: "positive overflow", arguments: coreIntArguments(math.MaxInt64, 2), failure: coreir.ArithmeticOverflow},
		{name: "minimum by negative one overflow", arguments: coreIntArguments(math.MinInt64, -1), failure: coreir.ArithmeticOverflow},
	} {
		outcome, evaluateErr := coreeval.Evaluate(core.Functions[0], test.arguments)
		if evaluateErr != nil {
			t.Fatalf("%s Core evaluation: %v", test.name, evaluateErr)
		}
		if outcome.OK != test.ok || outcome.Value.Int != test.value || outcome.Error != test.failure {
			t.Fatalf("%s Core outcome = %#v", test.name, outcome)
		}
	}
	compileAndRunCheckedMultiplyGo(t, generated, gobackend.FunctionName(core.Functions[0]))
	assertCompilerGolden(t, "checked-multiply.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "checked-multiply.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "checked-multiply.go", generated)
}

func TestCheckedNegateHIRCoreAndGoGoldens(t *testing.T) {
	module := testModule("app.root", "root.pipe", checkedNegateSource)
	input := semanticTestModuleSet("app.root", []ModuleInput{module}, nil)
	input.LanguageContract = PipeLangLanguageContractV050
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	method := semanticMethodNamed(t, analysis, "Negate")
	if method.Identity.Callable == nil {
		t.Fatal("checked Negate has no callable identity")
	}
	callable := method.Identity.Callable
	returns := callable.Returns
	if len(callable.Parameters) != 1 || callable.Parameters[0].Primitive != TypeInt || returns.Kind != TypeRefApplied || returns.Name != "Result" || returns.PackageID != PipeLangBuiltinPackageID || returns.Path != PipeLangResultSemanticPath || len(returns.Arguments) != 2 || returns.Arguments[0].Primitive != TypeInt || returns.Arguments[1].Kind != TypeRefNamed || returns.Arguments[1].PackageID != PipeLangBuiltinPackageID || returns.Arguments[1].Path != PipeLangArithmeticErrorSemanticPath {
		t.Fatalf("checked Negate callable identity = %#v", callable)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV050 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("checked Negate projection header = %#v", projection)
	}
	projected := projectedMemberNamed(t, projection.Modules[0].Types[0], "Negate").Type
	if projected.Kind != TypeRefApplied || projected.Name != "Result" || projected.Identity == nil || projected.Identity.PackageID != PipeLangBuiltinPackageID || projected.Identity.Path != PipeLangResultSemanticPath || len(projected.Arguments) != 2 || projected.Arguments[0].Primitive != TypeInt || projected.Arguments[1].Identity == nil || projected.Arguments[1].Identity.PackageID != PipeLangBuiltinPackageID || projected.Arguments[1].Identity.Path != PipeLangArithmeticErrorSemanticPath {
		t.Fatalf("checked Negate projected return = %#v", projected)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, method.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if typed.LanguageContract != coreir.LanguageContractV050 || len(typed.Functions) != 1 || typed.Functions[0].ReturnType.Kind != hir.TypeResult || typed.Functions[0].Body.Type.Kind != hir.TypeResult || typed.Functions[0].Body.Unary == nil || typed.Functions[0].Body.Unary.Operator != hir.OperatorNegate {
		t.Fatalf("checked Negate HIR = %#v", typed)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if core.Functions[0].Body.Unary == nil || core.Functions[0].Body.Unary.Operator != coreir.OperatorNegate {
		t.Fatalf("checked Negate Core = %#v", core)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	secondGenerated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, secondGenerated) {
		t.Fatal("checked Negate generated Go is nondeterministic")
	}
	for _, test := range []struct {
		name      string
		arguments []coreeval.Value
		ok        bool
		value     int64
		failure   coreir.ArithmeticError
	}{
		{name: "positive success", arguments: coreIntArguments(5), ok: true, value: -5},
		{name: "negative success", arguments: coreIntArguments(-5), ok: true, value: 5},
		{name: "zero success", arguments: coreIntArguments(0), ok: true},
		{name: "minimum overflow", arguments: coreIntArguments(math.MinInt64), failure: coreir.ArithmeticOverflow},
	} {
		outcome, evaluateErr := coreeval.Evaluate(core.Functions[0], test.arguments)
		if evaluateErr != nil {
			t.Fatalf("%s Core evaluation: %v", test.name, evaluateErr)
		}
		if outcome.OK != test.ok || outcome.Value.Int != test.value || outcome.Error != test.failure {
			t.Fatalf("%s Core outcome = %#v", test.name, outcome)
		}
	}
	compileAndRunCheckedNegateGo(t, generated, gobackend.FunctionName(core.Functions[0]))
	assertCompilerGolden(t, "checked-negate.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "checked-negate.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "checked-negate.go", generated)
}

func TestCheckedDivideHIRCoreAndGoGoldens(t *testing.T) {
	module := testModule("app.root", "root.pipe", checkedDivideSource)
	input := semanticTestModuleSet("app.root", []ModuleInput{module}, nil)
	input.LanguageContract = PipeLangLanguageContractV060
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	method := semanticMethodNamed(t, analysis, "Divide")
	if method.Identity.Callable == nil {
		t.Fatal("checked Divide has no callable identity")
	}
	callable := method.Identity.Callable
	returns := callable.Returns
	if len(callable.Parameters) != 2 || callable.Parameters[0].Primitive != TypeFloat || callable.Parameters[1].Primitive != TypeFloat || returns.Kind != TypeRefApplied || returns.Name != "Result" || returns.PackageID != PipeLangBuiltinPackageID || returns.Path != PipeLangResultSemanticPath || len(returns.Arguments) != 2 || returns.Arguments[0].Primitive != TypeFloat || returns.Arguments[1].Kind != TypeRefNamed || returns.Arguments[1].PackageID != PipeLangBuiltinPackageID || returns.Arguments[1].Path != PipeLangArithmeticErrorSemanticPath {
		t.Fatalf("checked Divide callable identity = %#v", callable)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV060 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("checked Divide projection header = %#v", projection)
	}
	projected := projectedMemberNamed(t, projection.Modules[0].Types[0], "Divide").Type
	if projected.Kind != TypeRefApplied || projected.Name != "Result" || projected.Identity == nil || projected.Identity.PackageID != PipeLangBuiltinPackageID || projected.Identity.Path != PipeLangResultSemanticPath || len(projected.Arguments) != 2 || projected.Arguments[0].Primitive != TypeFloat || projected.Arguments[1].Identity == nil || projected.Arguments[1].Identity.PackageID != PipeLangBuiltinPackageID || projected.Arguments[1].Identity.Path != PipeLangArithmeticErrorSemanticPath {
		t.Fatalf("checked Divide projected return = %#v", projected)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, method.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if typed.LanguageContract != coreir.LanguageContractV060 || len(typed.Functions) != 1 || typed.Functions[0].ReturnType.Kind != hir.TypeResult || typed.Functions[0].ReturnType.Result == nil || typed.Functions[0].ReturnType.Result.Success.Numeric == nil || typed.Functions[0].ReturnType.Result.Success.Numeric.Representation != hir.NumericBinaryFloat || typed.Functions[0].ReturnType.Result.Success.Numeric.Bits != 64 || typed.Functions[0].Body.Type.Kind != hir.TypeResult || typed.Functions[0].Body.Binary == nil || typed.Functions[0].Body.Binary.Operator != hir.OperatorDivide {
		t.Fatalf("checked Divide HIR = %#v", typed)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if core.Functions[0].Body.Binary == nil || core.Functions[0].Body.Binary.Operator != coreir.OperatorDivide {
		t.Fatalf("checked Divide Core = %#v", core)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	secondGenerated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, secondGenerated) {
		t.Fatal("checked Divide generated Go is nondeterministic")
	}
	negativeZero := math.Copysign(0, -1)
	for _, test := range []struct {
		name      string
		arguments []coreeval.Value
		ok        bool
		value     float64
		failure   coreir.ArithmeticError
		wantNaN   bool
	}{
		{name: "ordinary success", arguments: coreFloatArguments(7.5, 2.5), ok: true, value: 3},
		{name: "positive zero divisor", arguments: coreFloatArguments(1, 0), failure: coreir.ArithmeticDivisionByZero},
		{name: "negative zero divisor", arguments: coreFloatArguments(1, negativeZero), failure: coreir.ArithmeticDivisionByZero},
		{name: "NaN dividend", arguments: coreFloatArguments(math.NaN(), 2), ok: true, wantNaN: true},
		{name: "NaN divisor", arguments: coreFloatArguments(2, math.NaN()), ok: true, wantNaN: true},
	} {
		outcome, evaluateErr := coreeval.Evaluate(core.Functions[0], test.arguments)
		if evaluateErr != nil {
			t.Fatalf("%s Core evaluation: %v", test.name, evaluateErr)
		}
		valueMatches := outcome.Value.Float == test.value
		if test.wantNaN {
			valueMatches = math.IsNaN(outcome.Value.Float)
		}
		if outcome.OK != test.ok || outcome.Error != test.failure || (test.ok && !valueMatches) {
			t.Fatalf("%s Core outcome = %#v", test.name, outcome)
		}
	}
	if outcome, evaluateErr := coreeval.Evaluate(core.Functions[0], coreFloatArguments(math.Inf(1), 2)); evaluateErr != nil || !outcome.OK || outcome.Error != "" || !math.IsInf(outcome.Value.Float, 1) {
		t.Fatalf("infinity Core outcome = %#v (%v)", outcome, evaluateErr)
	}
	if outcome, evaluateErr := coreeval.Evaluate(core.Functions[0], coreFloatArguments(negativeZero, 2)); evaluateErr != nil || !outcome.OK || outcome.Error != "" || outcome.Value.Float != 0 || !math.Signbit(outcome.Value.Float) {
		t.Fatalf("signed-zero Core outcome = %#v (%v)", outcome, evaluateErr)
	}
	compileAndRunCheckedDivideGo(t, generated, gobackend.FunctionName(core.Functions[0]))
	assertCompilerGolden(t, "checked-divide.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "checked-divide.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "checked-divide.go", generated)
}

func TestV020CheckedAddRequiresExplicitExactDirectResult(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "ordinary integer return", source: `public Class Root { public int Add(int left, int right) => left + right; }`},
		{name: "nested addition", source: `public Class Root { public Result<int, ArithmeticError> Add(int left, int right) => 1 + (left + right); }`},
		{name: "different arithmetic", source: `public Class Root { public Result<int, ArithmeticError> Add(int left, int right) => left - right; }`},
		{name: "different success type", source: `public Class Root { public Result<float, ArithmeticError> Add(int left, int right) => left + right; }`},
		{name: "different failure type", source: `public Class Root { public Result<int, Root> Add(int left, int right) => left + right; }`},
		{name: "result field", source: `public Class Root { public Result<int, ArithmeticError> Value; }`},
		{name: "result parameter", source: `public Class Root { public bool Check(Result<int, ArithmeticError> value) => true; }`},
		{name: "interface result", source: `public Interface Math { public Result<int, ArithmeticError> Add(int left, int right); } public Class Root { public bool Ready() => true; }`},
		{name: "bare arithmetic error", source: `public Class Root { public ArithmeticError Error() => 1; }`},
		{name: "reserved declaration", source: `public Class ArithmeticError { public int Value; }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV020
			analysis := AnalyzeSemanticModuleSet(input)
			if !analysis.Diagnostics.HasErrors() {
				t.Fatal("invalid v0.2.0 Result shape was accepted")
			}
			if test.name == "ordinary integer return" || test.name == "nested addition" || test.name == "different arithmetic" {
				assertDiagnosticCode(t, analysis, CodeNumericSemantics)
			}
		})
	}
}

func TestV030CheckedSubtractRequiresExplicitExactDirectResult(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "ordinary integer return", source: `public Class Root { public int Subtract(int left, int right) => left - right; }`},
		{name: "nested subtraction", source: `public Class Root { public Result<int, ArithmeticError> Subtract(int left, int right) => 1 - (left - right); }`},
		{name: "multiplication remains closed", source: `public Class Root { public Result<int, ArithmeticError> Multiply(int left, int right) => left * right; }`},
		{name: "negation remains closed", source: `public Class Root { public Result<int, ArithmeticError> Negate(int value) => -value; }`},
		{name: "division remains closed", source: `public Class Root { public Result<int, ArithmeticError> Divide(int left, int right) => left / right; }`},
		{name: "different success type", source: `public Class Root { public Result<float, ArithmeticError> Subtract(int left, int right) => left - right; }`},
		{name: "different failure type", source: `public Class Root { public Result<int, Root> Subtract(int left, int right) => left - right; }`},
		{name: "result field", source: `public Class Root { public Result<int, ArithmeticError> Value; }`},
		{name: "result parameter", source: `public Class Root { public bool Check(Result<int, ArithmeticError> value) => true; }`},
		{name: "interface result", source: `public Interface Math { public Result<int, ArithmeticError> Subtract(int left, int right); } public Class Root { public bool Ready() => true; }`},
		{name: "bare arithmetic error", source: `public Class Root { public ArithmeticError Error() => 1; }`},
		{name: "reserved declaration", source: `public Class Result { public int Value; }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV030
			analysis := AnalyzeSemanticModuleSet(input)
			if !analysis.Diagnostics.HasErrors() {
				t.Fatal("invalid v0.3.0 Result shape was accepted")
			}
			if test.name == "ordinary integer return" || test.name == "nested subtraction" || strings.Contains(test.name, "remains closed") {
				assertDiagnosticCode(t, analysis, CodeNumericSemantics)
			}
		})
	}
}

func TestV040CheckedMultiplyRequiresExplicitExactDirectResult(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "ordinary integer return", source: `public Class Root { public int Multiply(int left, int right) => left * right; }`},
		{name: "nested multiplication", source: `public Class Root { public Result<int, ArithmeticError> Multiply(int left, int right) => 2 * (left * right); }`},
		{name: "negation remains closed", source: `public Class Root { public Result<int, ArithmeticError> Negate(int value) => -value; }`},
		{name: "division remains closed", source: `public Class Root { public Result<float, ArithmeticError> Divide(float left, float right) => left / right; }`},
		{name: "different success type", source: `public Class Root { public Result<float, ArithmeticError> Multiply(int left, int right) => left * right; }`},
		{name: "different failure type", source: `public Class Root { public Result<int, Root> Multiply(int left, int right) => left * right; }`},
		{name: "result field", source: `public Class Root { public Result<int, ArithmeticError> Value; }`},
		{name: "result parameter", source: `public Class Root { public bool Check(Result<int, ArithmeticError> value) => true; }`},
		{name: "interface result", source: `public Interface Math { public Result<int, ArithmeticError> Multiply(int left, int right); } public Class Root { public bool Ready() => true; }`},
		{name: "bare arithmetic error", source: `public Class Root { public ArithmeticError Error() => 1; }`},
		{name: "reserved declaration", source: `public Class Result { public int Value; }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV040
			analysis := AnalyzeSemanticModuleSet(input)
			if !analysis.Diagnostics.HasErrors() {
				t.Fatal("invalid v0.4.0 Result shape was accepted")
			}
			if test.name == "ordinary integer return" || test.name == "nested multiplication" || test.name == "negation remains closed" {
				assertDiagnosticCode(t, analysis, CodeNumericSemantics)
			}
		})
	}
}

func TestV050CheckedNegateRequiresExplicitExactDirectResult(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "ordinary integer return", source: `public Class Root { public int Negate(int value) => -value; }`},
		{name: "nested negation", source: `public Class Root { public Result<int, ArithmeticError> Negate(int value) => -(-value); }`},
		{name: "division remains closed", source: `public Class Root { public Result<float, ArithmeticError> Divide(float left, float right) => left / right; }`},
		{name: "different success type", source: `public Class Root { public Result<float, ArithmeticError> Negate(float value) => -value; }`},
		{name: "different failure type", source: `public Class Root { public Result<int, Root> Negate(int value) => -value; }`},
		{name: "result field", source: `public Class Root { public Result<int, ArithmeticError> Value; }`},
		{name: "result parameter", source: `public Class Root { public bool Check(Result<int, ArithmeticError> value) => true; }`},
		{name: "interface result", source: `public Interface Math { public Result<int, ArithmeticError> Negate(int value); } public Class Root { public bool Ready() => true; }`},
		{name: "bare arithmetic error", source: `public Class Root { public ArithmeticError Error() => 1; }`},
		{name: "reserved declaration", source: `public Class Result { public int Value; }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV050
			analysis := AnalyzeSemanticModuleSet(input)
			if !analysis.Diagnostics.HasErrors() {
				t.Fatal("invalid v0.5.0 Result shape was accepted")
			}
			if test.name == "ordinary integer return" || test.name == "nested negation" {
				assertDiagnosticCode(t, analysis, CodeNumericSemantics)
			}
		})
	}
}

func TestV060CheckedDivideRequiresExplicitExactDirectResult(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "ordinary float return", source: `public Class Root { public float Divide(float left, float right) => left / right; }`},
		{name: "nested division", source: `public Class Root { public Result<float, ArithmeticError> Divide(float left, float right) => left / (right / 2.0); }`},
		{name: "different arithmetic", source: `public Class Root { public Result<float, ArithmeticError> Divide(float left, float right) => left + right; }`},
		{name: "integer operands", source: `public Class Root { public Result<float, ArithmeticError> Divide(int left, int right) => left / right; }`},
		{name: "different success type", source: `public Class Root { public Result<int, ArithmeticError> Divide(float left, float right) => left / right; }`},
		{name: "different failure type", source: `public Class Root { public Result<float, Root> Divide(float left, float right) => left / right; }`},
		{name: "result field", source: `public Class Root { public Result<float, ArithmeticError> Value; }`},
		{name: "result parameter", source: `public Class Root { public bool Check(Result<float, ArithmeticError> value) => true; }`},
		{name: "interface result", source: `public Interface Math { public Result<float, ArithmeticError> Divide(float left, float right); } public Class Root { public bool Ready() => true; }`},
		{name: "bare arithmetic error", source: `public Class Root { public ArithmeticError Error() => 1; }`},
		{name: "reserved declaration", source: `public Class Result { public int Value; }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV060
			analysis := AnalyzeSemanticModuleSet(input)
			if !analysis.Diagnostics.HasErrors() {
				t.Fatal("invalid v0.6.0 Result shape was accepted")
			}
			if test.name == "ordinary float return" || test.name == "nested division" || test.name == "different arithmetic" || test.name == "integer operands" || test.name == "different success type" {
				assertDiagnosticCode(t, analysis, CodeNumericSemantics)
			}
		})
	}
}

func TestV030PreservesDirectCheckedAdd(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", checkedAddSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV030
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	method := semanticMethodNamed(t, analysis, "Add")
	typed, err := LowerSemanticMethodToHIR(analysis, method.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if typed.Functions[0].Body.Binary == nil || typed.Functions[0].Body.Binary.Operator != hir.OperatorAdd {
		t.Fatalf("v0.3.0 changed direct checked Add = %#v", typed.Functions[0].Body)
	}
}

func TestV040PreservesDirectCheckedAddAndSubtract(t *testing.T) {
	for _, test := range []struct {
		name     string
		source   string
		operator hir.Operator
	}{
		{name: "Add", source: checkedAddSource, operator: hir.OperatorAdd},
		{name: "Subtract", source: checkedSubtractSource, operator: hir.OperatorSubtract},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV040
			analysis := AnalyzeSemanticModuleSet(input)
			if err := analysis.Error(); err != nil {
				t.Fatal(err)
			}
			method := semanticMethodNamed(t, analysis, test.name)
			typed, err := LowerSemanticMethodToHIR(analysis, method.Identity)
			if err != nil {
				t.Fatal(err)
			}
			if typed.Functions[0].Body.Binary == nil || typed.Functions[0].Body.Binary.Operator != test.operator {
				t.Fatalf("v0.4.0 changed direct checked %s = %#v", test.name, typed.Functions[0].Body)
			}
		})
	}
}

func TestV050PreservesDirectCheckedAddSubtractAndMultiply(t *testing.T) {
	for _, test := range []struct {
		name     string
		source   string
		operator hir.Operator
	}{
		{name: "Add", source: checkedAddSource, operator: hir.OperatorAdd},
		{name: "Subtract", source: checkedSubtractSource, operator: hir.OperatorSubtract},
		{name: "Multiply", source: checkedMultiplySource, operator: hir.OperatorMultiply},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV050
			analysis := AnalyzeSemanticModuleSet(input)
			if err := analysis.Error(); err != nil {
				t.Fatal(err)
			}
			method := semanticMethodNamed(t, analysis, test.name)
			typed, err := LowerSemanticMethodToHIR(analysis, method.Identity)
			if err != nil {
				t.Fatal(err)
			}
			if typed.Functions[0].Body.Binary == nil || typed.Functions[0].Body.Binary.Operator != test.operator {
				t.Fatalf("v0.5.0 changed direct checked %s = %#v", test.name, typed.Functions[0].Body)
			}
		})
	}
}

func TestV060PreservesPriorDirectCheckedArithmetic(t *testing.T) {
	for _, test := range []struct {
		name     string
		source   string
		operator hir.Operator
		unary    bool
	}{
		{name: "Add", source: checkedAddSource, operator: hir.OperatorAdd},
		{name: "Subtract", source: checkedSubtractSource, operator: hir.OperatorSubtract},
		{name: "Multiply", source: checkedMultiplySource, operator: hir.OperatorMultiply},
		{name: "Negate", source: checkedNegateSource, operator: hir.OperatorNegate, unary: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV060
			analysis := AnalyzeSemanticModuleSet(input)
			if err := analysis.Error(); err != nil {
				t.Fatal(err)
			}
			method := semanticMethodNamed(t, analysis, test.name)
			typed, err := LowerSemanticMethodToHIR(analysis, method.Identity)
			if err != nil {
				t.Fatal(err)
			}
			if test.unary {
				if typed.Functions[0].Body.Unary == nil || typed.Functions[0].Body.Unary.Operator != test.operator {
					t.Fatalf("v0.6.0 changed direct checked %s = %#v", test.name, typed.Functions[0].Body)
				}
				return
			}
			if typed.Functions[0].Body.Binary == nil || typed.Functions[0].Body.Binary.Operator != test.operator {
				t.Fatalf("v0.6.0 changed direct checked %s = %#v", test.name, typed.Functions[0].Body)
			}
		})
	}
}

func TestCheckedSubtractRequiresExplicitV030Migration(t *testing.T) {
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", checkedSubtractSource)}, nil)
		input.LanguageContract = contract
		analysis := AnalyzeSemanticModuleSet(input)
		if !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted the v0.3.0 checked subtraction", contract)
		}
	}
}

func TestCheckedMultiplyRequiresExplicitV040Migration(t *testing.T) {
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", checkedMultiplySource)}, nil)
		input.LanguageContract = contract
		analysis := AnalyzeSemanticModuleSet(input)
		if !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted the v0.4.0 checked multiplication", contract)
		}
	}
}

func TestCheckedNegateRequiresExplicitV050Migration(t *testing.T) {
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", checkedNegateSource)}, nil)
		input.LanguageContract = contract
		analysis := AnalyzeSemanticModuleSet(input)
		if !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted the v0.5.0 checked negation", contract)
		}
	}
}

func TestCheckedDivideRequiresExplicitV060Migration(t *testing.T) {
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040, PipeLangLanguageContractV050} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", checkedDivideSource)}, nil)
		input.LanguageContract = contract
		analysis := AnalyzeSemanticModuleSet(input)
		if !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted the v0.6.0 checked division", contract)
		}
	}
}

func TestResultSourceSpellingRequiresExplicitV020Migration(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", checkedAddSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV010
	analysis := AnalyzeSemanticModuleSet(input)
	if !analysis.Diagnostics.HasErrors() {
		t.Fatal("v0.1.0 implicitly accepted the v0.2.0 Result spelling")
	}
}

func TestCheckedArithmeticHIRRejectsMalformedResultType(t *testing.T) {
	function := hirCheckedBinaryFunction("Add", hir.OperatorAdd, hirInteger64())
	function.ReturnType = hirInteger64()
	function.Body.Type = hirInteger64()
	_, err := LowerHIRToCore(hir.Program{LanguageContract: coreir.LanguageContractV010, CompilerContract: coreir.CompilerContractV1, Functions: []hir.Function{function}})
	diagnostics, ok := AsDiagnostics(err)
	if !ok || len(diagnostics) != 1 || diagnostics[0].Code != CodeCoreLowering || !strings.Contains(diagnostics[0].Message, "invalid Result type") {
		t.Fatalf("malformed arithmetic HIR error = %#v (%v)", diagnostics, err)
	}
}

func TestBinary64ComparisonAndEqualityMatchReferenceEvaluator(t *testing.T) {
	source := `public Class Root {
  public bool Less(float left, float right) => left < right;
  public bool LessEqual(float left, float right) => left <= right;
  public bool Greater(float left, float right) => left > right;
  public bool GreaterEqual(float left, float right) => left >= right;
  public bool Equal(float left, float right) => left == right;
  public bool NotEqual(float left, float right) => left != right;
}`
	analysis := AnalyzeSemanticModuleSet(semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil))
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	var typedFunctions []hir.Function
	functionNames := map[string]string{}
	for _, name := range []string{"Less", "LessEqual", "Greater", "GreaterEqual", "Equal", "NotEqual"} {
		method := semanticMethodNamed(t, analysis, name)
		typed, err := LowerSemanticMethodToHIR(analysis, method.Identity)
		if err != nil {
			t.Fatal(err)
		}
		typedFunctions = append(typedFunctions, typed.Functions[0])
	}
	core, err := LowerHIRToCore(hir.Program{LanguageContract: coreir.LanguageContractV010, CompilerContract: coreir.CompilerContractV1, Functions: typedFunctions})
	if err != nil {
		t.Fatal(err)
	}
	for _, function := range core.Functions {
		functionNames[function.Name] = gobackend.FunctionName(function)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		method   string
		left     string
		right    string
		expected bool
	}{
		{method: "Less", left: "NaN", right: "1", expected: false},
		{method: "LessEqual", left: "NaN", right: "NaN", expected: false},
		{method: "Greater", left: "1", right: "NaN", expected: false},
		{method: "GreaterEqual", left: "NaN", right: "1", expected: false},
		{method: "Equal", left: "NaN", right: "NaN", expected: false},
		{method: "NotEqual", left: "NaN", right: "NaN", expected: true},
		{method: "Equal", left: "0", right: "-0", expected: true},
		{method: "Less", left: "1", right: "2", expected: true},
	}
	for _, test := range cases {
		evaluated, err := InvokeFiles(map[string][]byte{"root.pipe": []byte(source)}, "Root", test.method, []string{test.left, test.right})
		if err != nil {
			t.Fatal(err)
		}
		if evaluated.Value.Type != TypeBool || evaluated.Value.Bool != test.expected {
			t.Fatalf("reference %s(%s, %s) = %#v", test.method, test.left, test.right, evaluated.Value)
		}
	}
	compileAndRunNumericConformanceGo(t, generated, functionNames)
}

func TestGoBackendCannotImportParserASTOrHIR(t *testing.T) {
	assertCompilerPackageImportsOnlyCoreIR(t, "gobackend")
}

func TestCoreEvaluatorCannotImportParserASTHIROrBackend(t *testing.T) {
	assertCompilerPackageImportsOnlyCoreIR(t, "coreeval")
}

func assertCompilerPackageImportsOnlyCoreIR(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			files = append(files, filepath.Join(directory, entry.Name()))
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatalf("%s has no source files", directory)
	}
	for _, path := range files {
		parsed, err := goparser.ParseFile(gotoken.NewFileSet(), path, nil, goparser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range parsed.Imports {
			name, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if name == "go/ast" || name == "go/parser" || (strings.HasPrefix(name, "dockpipe/src/lib/pipelang") && name != "dockpipe/src/lib/pipelang/coreir") {
				t.Fatalf("%s imports forbidden parser/AST/HIR/backend dependency %q; compiler consumers must accept Core IR only", path, name)
			}
		}
	}
}

func canonicalJSON(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(payload, '\n')
}

func assertCompilerGolden(t *testing.T, name string, actual []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, want) {
		t.Fatalf("%s drifted; actual bytes:\n%s", path, actual)
	}
}

func compileAndRunGeneratedGo(t *testing.T, generated []byte, functionName string, expected bool) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "pipelang-generated-go-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	files := map[string][]byte{
		"go.mod":            []byte("module pipelang-generated-check\n\ngo 1.25\n"),
		"generated.go":      generated,
		"generated_test.go": []byte(fmt.Sprintf("package %s\n\nimport \"testing\"\n\nfunc TestGenerated(t *testing.T) { if got := %s(2); got != %t { t.Fatalf(\"got %%v\", got) } }\n", gobackend.PackageName, functionName, expected)),
	}
	for name, payload := range files {
		if err := os.WriteFile(filepath.Join(dir, name), payload, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	goBinary := filepath.Join(runtime.GOROOT(), "bin", "go")
	command := exec.Command(goBinary, "test", ".")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off", "GOCACHE=/tmp/dockpipe-task021-generated-gocache", "GOTMPDIR=/tmp")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("compile/run generated Go: %v\n%s", err, output)
	}
}

func compileAndRunNumericConformanceGo(t *testing.T, generated []byte, names map[string]string) {
	t.Helper()
	testSource := fmt.Sprintf(`package %s

import (
	"math"
	"testing"
)

func TestGeneratedNumericConformance(t *testing.T) {
	nan := math.NaN()
	negativeZero := math.Copysign(0, -1)
	tests := []struct {
		name string
		got bool
		want bool
	}{
		{"nan-less", %s(nan, 1), false},
		{"nan-less-equal", %s(nan, nan), false},
		{"nan-greater", %s(1, nan), false},
		{"nan-greater-equal", %s(nan, 1), false},
		{"nan-equal", %s(nan, nan), false},
		{"nan-not-equal", %s(nan, nan), true},
		{"signed-zero-equal", %s(0, negativeZero), true},
		{"ordinary-less", %s(1, 2), true},
	}
	for _, test := range tests {
		if test.got != test.want {
			t.Fatalf("%%s: got %%v, want %%v", test.name, test.got, test.want)
		}
	}
}
`, gobackend.PackageName, names["Less"], names["LessEqual"], names["Greater"], names["GreaterEqual"], names["Equal"], names["NotEqual"], names["Equal"], names["Less"])
	compileAndRunGeneratedGoFiles(t, generated, []byte(testSource))
}

func compileAndRunGeneratedGoFiles(t *testing.T, generated, generatedTest []byte) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "pipelang-generated-go-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	files := map[string][]byte{
		"go.mod":            []byte("module pipelang-generated-check\n\ngo 1.25\n"),
		"generated.go":      generated,
		"generated_test.go": generatedTest,
	}
	for name, payload := range files {
		if err := os.WriteFile(filepath.Join(dir, name), payload, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	goBinary := filepath.Join(runtime.GOROOT(), "bin", "go")
	command := exec.Command(goBinary, "test", ".")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off", "GOCACHE=/tmp/dockpipe-task021-generated-gocache", "GOTMPDIR=/tmp")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("compile/run generated Go: %v\n%s", err, output)
	}
}

func semanticMethodNamed(t *testing.T, analysis *Analysis, name string) SemanticDeclaration {
	t.Helper()
	for _, declaration := range analysis.SemanticIDs.Declarations() {
		if declaration.Kind == SemanticMethod && declaration.Name == name {
			return declaration
		}
	}
	t.Fatalf("missing semantic method %q", name)
	return SemanticDeclaration{}
}

func coreInteger64() coreir.Type {
	return coreir.Type{Kind: coreir.TypeNumeric, Numeric: &coreir.NumericType{Representation: coreir.NumericInteger, Bits: 64, Signed: true}}
}

func coreBinary64() coreir.Type {
	return coreir.Type{Kind: coreir.TypeNumeric, Numeric: &coreir.NumericType{Representation: coreir.NumericBinaryFloat, Bits: 64}}
}

func hirInteger64() hir.Type {
	return hir.Type{Kind: hir.TypeNumeric, Numeric: &hir.NumericType{Representation: hir.NumericInteger, Bits: 64, Signed: true}}
}

func hirBinary64() hir.Type {
	return hir.Type{Kind: hir.TypeNumeric, Numeric: &hir.NumericType{Representation: hir.NumericBinaryFloat, Bits: 64}}
}

func hirArithmeticResult(success hir.Type) hir.Type {
	return hir.Type{Kind: hir.TypeResult, Result: &hir.ResultType{Success: success, Failure: hir.Type{Kind: hir.TypeArithmeticError}}}
}

func hirCheckedBinaryFunction(name string, operator hir.Operator, operandType hir.Type) hir.Function {
	identity := hir.SemanticIdentity{PackageID: "test.package", Path: "app.root.root." + strings.ToLower(name)}
	span := hir.SourceSpan{File: "checked-arithmetic.fixture", Start: 1, End: 2}
	parameters := make([]hir.Parameter, 2)
	expressions := make([]hir.Expr, 2)
	for position, parameterName := range []string{"left", "right"} {
		binding := hir.Binding{Kind: hir.BindingParameter, Function: identity, Position: position, Name: parameterName}
		parameters[position] = hir.Parameter{Binding: binding, Type: operandType, TypeSpan: span, Span: span}
		bindingCopy := binding
		expressions[position] = hir.Expr{Kind: hir.ExprReference, Type: operandType, Span: span, Reference: &bindingCopy}
	}
	resultType := hirArithmeticResult(operandType)
	return hir.Function{
		Identity: identity,
		Owner:    hir.Owner{Module: "app.root", SymbolID: 1, Identity: hir.SemanticIdentity{PackageID: "test.package", Path: "app.root.root"}, SourceSpan: span},
		Name:     name, Parameters: parameters, ReturnType: resultType, ReturnTypeSpan: span,
		Body: hir.Expr{Kind: hir.ExprBinary, Type: resultType, Span: span, Binary: &hir.Binary{Operator: operator, Left: &expressions[0], Right: &expressions[1]}}, Span: span,
	}
}

func hirCheckedUnaryFunction(name string, operator hir.Operator, operandType hir.Type) hir.Function {
	identity := hir.SemanticIdentity{PackageID: "test.package", Path: "app.root.root." + strings.ToLower(name)}
	span := hir.SourceSpan{File: "checked-arithmetic.fixture", Start: 1, End: 2}
	binding := hir.Binding{Kind: hir.BindingParameter, Function: identity, Position: 0, Name: "value"}
	operand := hir.Expr{Kind: hir.ExprReference, Type: operandType, Span: span, Reference: &binding}
	resultType := hirArithmeticResult(operandType)
	return hir.Function{
		Identity: identity,
		Owner:    hir.Owner{Module: "app.root", SymbolID: 1, Identity: hir.SemanticIdentity{PackageID: "test.package", Path: "app.root.root"}, SourceSpan: span},
		Name:     name, Parameters: []hir.Parameter{{Binding: binding, Type: operandType, TypeSpan: span, Span: span}}, ReturnType: resultType, ReturnTypeSpan: span,
		Body: hir.Expr{Kind: hir.ExprUnary, Type: resultType, Span: span, Unary: &hir.Unary{Operator: operator, Operand: &operand}}, Span: span,
	}
}

func coreIntArguments(values ...int64) []coreeval.Value {
	arguments := make([]coreeval.Value, len(values))
	for index, value := range values {
		arguments[index] = coreeval.Value{Type: coreInteger64(), Int: value}
	}
	return arguments
}

func coreFloatArguments(values ...float64) []coreeval.Value {
	arguments := make([]coreeval.Value, len(values))
	for index, value := range values {
		arguments[index] = coreeval.Value{Type: coreBinary64(), Float: value}
	}
	return arguments
}

func compileAndRunCheckedArithmeticGo(t *testing.T, generated []byte, names map[string]string) {
	t.Helper()
	testSource := fmt.Sprintf(`package %s

import (
	"math"
	"testing"
)

func TestGeneratedCheckedArithmetic(t *testing.T) {
	assertInt := func(name string, got PipeLangArithmeticResult[int64], ok bool, value int64, arithmeticError PipeLangArithmeticError) {
		t.Helper()
		if got.OK != ok || got.Value != value || got.Error != arithmeticError {
			t.Fatalf("%%s: got %%#v", name, got)
		}
	}
	assertFloat := func(name string, got PipeLangArithmeticResult[float64], ok bool, value float64, arithmeticError PipeLangArithmeticError) {
		t.Helper()
		if got.OK != ok || got.Error != arithmeticError || (ok && got.Value != value) {
			t.Fatalf("%%s: got %%#v", name, got)
		}
	}
	assertInt("add", %s(2, 3), true, 5, "")
	assertInt("add-overflow", %s(math.MaxInt64, 1), false, 0, PipeLangArithmeticOverflow)
	assertInt("subtract", %s(5, 3), true, 2, "")
	assertInt("subtract-overflow", %s(math.MinInt64, 1), false, 0, PipeLangArithmeticOverflow)
	assertInt("multiply", %s(-3, 4), true, -12, "")
	assertInt("multiply-overflow", %s(math.MaxInt64, 2), false, 0, PipeLangArithmeticOverflow)
	assertInt("negate", %s(5), true, -5, "")
	assertInt("negate-overflow", %s(math.MinInt64), false, 0, PipeLangArithmeticOverflow)
	assertFloat("divide", %s(6, 2), true, 3, "")
	assertFloat("divide-zero", %s(1, 0), false, 0, PipeLangArithmeticDivisionByZero)
	assertFloat("divide-negative-zero", %s(1, math.Copysign(0, -1)), false, 0, PipeLangArithmeticDivisionByZero)
	if got := %s(math.NaN(), 2); !got.OK || got.Error != "" || !math.IsNaN(got.Value) {
		t.Fatalf("divide-nan: got %%#v", got)
	}
}
`, gobackend.PackageName,
		names["Add"], names["Add"], names["Subtract"], names["Subtract"], names["Multiply"], names["Multiply"], names["Negate"], names["Negate"], names["Divide"], names["Divide"], names["Divide"], names["Divide"])
	compileAndRunGeneratedGoFiles(t, generated, []byte(testSource))
}

func compileAndRunCheckedAddGo(t *testing.T, generated []byte, name string) {
	t.Helper()
	testSource := fmt.Sprintf(`package %s

import (
	"math"
	"testing"
)

func TestGeneratedCheckedAdd(t *testing.T) {
	if got := %s(2, 3); !got.OK || got.Value != 5 || got.Error != "" {
		t.Fatalf("success: %%#v", got)
	}
	if got := %s(math.MaxInt64, 1); got.OK || got.Value != 0 || got.Error != PipeLangArithmeticOverflow {
		t.Fatalf("overflow: %%#v", got)
	}
}
`, gobackend.PackageName, name, name)
	compileAndRunGeneratedGoFiles(t, generated, []byte(testSource))
}

func compileAndRunCheckedSubtractGo(t *testing.T, generated []byte, name string) {
	t.Helper()
	testSource := fmt.Sprintf(`package %s

import (
	"math"
	"testing"
)

func TestGeneratedCheckedSubtract(t *testing.T) {
	if got := %s(5, 3); !got.OK || got.Value != 2 || got.Error != "" {
		t.Fatalf("success: %%#v", got)
	}
	if got := %s(math.MinInt64, 1); got.OK || got.Value != 0 || got.Error != PipeLangArithmeticOverflow {
		t.Fatalf("minimum underflow: %%#v", got)
	}
	if got := %s(math.MaxInt64, -1); got.OK || got.Value != 0 || got.Error != PipeLangArithmeticOverflow {
		t.Fatalf("maximum overflow: %%#v", got)
	}
}
`, gobackend.PackageName, name, name, name)
	compileAndRunGeneratedGoFiles(t, generated, []byte(testSource))
}

func compileAndRunCheckedMultiplyGo(t *testing.T, generated []byte, name string) {
	t.Helper()
	testSource := fmt.Sprintf(`package %s

import (
	"math"
	"testing"
)

func TestGeneratedCheckedMultiply(t *testing.T) {
	if got := %s(6, 7); !got.OK || got.Value != 42 || got.Error != "" {
		t.Fatalf("positive success: %%#v", got)
	}
	if got := %s(-3, 4); !got.OK || got.Value != -12 || got.Error != "" {
		t.Fatalf("negative success: %%#v", got)
	}
	if got := %s(math.MinInt64, 0); !got.OK || got.Value != 0 || got.Error != "" {
		t.Fatalf("zero success: %%#v", got)
	}
	if got := %s(math.MaxInt64, 2); got.OK || got.Value != 0 || got.Error != PipeLangArithmeticOverflow {
		t.Fatalf("positive overflow: %%#v", got)
	}
	if got := %s(math.MinInt64, -1); got.OK || got.Value != 0 || got.Error != PipeLangArithmeticOverflow {
		t.Fatalf("minimum by negative one overflow: %%#v", got)
	}
}
`, gobackend.PackageName, name, name, name, name, name)
	compileAndRunGeneratedGoFiles(t, generated, []byte(testSource))
}

func compileAndRunCheckedNegateGo(t *testing.T, generated []byte, name string) {
	t.Helper()
	testSource := fmt.Sprintf(`package %s

import (
	"math"
	"testing"
)

func TestGeneratedCheckedNegate(t *testing.T) {
	if got := %s(5); !got.OK || got.Value != -5 || got.Error != "" {
		t.Fatalf("positive success: %%#v", got)
	}
	if got := %s(-5); !got.OK || got.Value != 5 || got.Error != "" {
		t.Fatalf("negative success: %%#v", got)
	}
	if got := %s(0); !got.OK || got.Value != 0 || got.Error != "" {
		t.Fatalf("zero success: %%#v", got)
	}
	if got := %s(math.MinInt64); got.OK || got.Value != 0 || got.Error != PipeLangArithmeticOverflow {
		t.Fatalf("minimum overflow: %%#v", got)
	}
}
`, gobackend.PackageName, name, name, name, name)
	compileAndRunGeneratedGoFiles(t, generated, []byte(testSource))
}

func compileAndRunCheckedDivideGo(t *testing.T, generated []byte, name string) {
	t.Helper()
	testSource := fmt.Sprintf(`package %s

import (
	"math"
	"testing"
)

func TestGeneratedCheckedDivide(t *testing.T) {
	if got := %s(7.5, 2.5); !got.OK || got.Value != 3 || got.Error != "" {
		t.Fatalf("ordinary success: %%#v", got)
	}
	if got := %s(1, 0); got.OK || got.Value != 0 || got.Error != PipeLangArithmeticDivisionByZero {
		t.Fatalf("positive zero divisor: %%#v", got)
	}
	if got := %s(1, math.Copysign(0, -1)); got.OK || got.Value != 0 || got.Error != PipeLangArithmeticDivisionByZero {
		t.Fatalf("negative zero divisor: %%#v", got)
	}
	if got := %s(math.NaN(), 2); !got.OK || got.Error != "" || !math.IsNaN(got.Value) {
		t.Fatalf("NaN dividend: %%#v", got)
	}
	if got := %s(2, math.NaN()); !got.OK || got.Error != "" || !math.IsNaN(got.Value) {
		t.Fatalf("NaN divisor: %%#v", got)
	}
	if got := %s(math.Inf(1), 2); !got.OK || got.Error != "" || !math.IsInf(got.Value, 1) {
		t.Fatalf("infinity: %%#v", got)
	}
	if got := %s(math.Copysign(0, -1), 2); !got.OK || got.Error != "" || got.Value != 0 || !math.Signbit(got.Value) {
		t.Fatalf("signed zero: %%#v", got)
	}
}
`, gobackend.PackageName, name, name, name, name, name, name, name)
	compileAndRunGeneratedGoFiles(t, generated, []byte(testSource))
}

func reflectSemanticIdentities(left, right []SemanticIdentity) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if semanticIdentityKey(left[index]) != semanticIdentityKey(right[index]) {
			return false
		}
	}
	return true
}
