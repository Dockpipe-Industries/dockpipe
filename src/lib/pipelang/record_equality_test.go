package pipelang

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
)

func TestV120PrimitiveRecordEqualityHIRCoreAndGo(t *testing.T) {
	source, err := os.ReadFile("testdata/record-equality.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-equality.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV120
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}

	record := semanticDeclarationNamed(t, analysis, SemanticRecord, "Row")
	method := semanticMethodNamed(t, analysis, "Same")
	callable := method.Identity.Callable
	if callable == nil || len(callable.Parameters) != 2 || callable.Returns.Primitive != TypeBool || callable.Parameters[0].Kind != TypeRefNamed || callable.Parameters[0].PackageID != record.Identity.PackageID || callable.Parameters[0].Path != record.Identity.Path || callable.Parameters[0].String() != callable.Parameters[1].String() {
		t.Fatalf("record equality callable identity = %#v", callable)
	}

	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	projected := projectedMemberNamed(t, projectedTypeNamed(t, projection, "Root"), "Same")
	if projection.Schema != PipeLangSemanticProjectionVersion || projection.LanguageContract != PipeLangLanguageContractV120 || len(projected.Parameters) != 2 || projected.Type.Primitive != TypeBool || projected.Parameters[0].Type.Identity == nil || projected.Parameters[0].Type.Identity.Path != record.Identity.Path || projected.Parameters[1].Type.Identity == nil || projected.Parameters[1].Type.Identity.Path != record.Identity.Path {
		t.Fatalf("record equality semantic projection = %#v", projected)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, method.Identity)
	if err != nil {
		t.Fatal(err)
	}
	function := typed.Functions[0]
	if typed.LanguageContract != coreir.LanguageContractV120 || function.Body.Kind != hir.ExprBinary || function.Body.Binary == nil || function.Body.Binary.Operator != hir.OperatorEqual || function.Body.Binary.Left == nil || function.Body.Binary.Left.Reference == nil || function.Body.Binary.Left.Reference.Position != 0 || function.Body.Binary.Right == nil || function.Body.Binary.Right.Reference == nil || function.Body.Binary.Right.Reference.Position != 1 || function.Body.Type.Primitive != hir.PrimitiveBool || function.Parameters[0].Type.Identity == nil || function.Parameters[0].Type.Identity.Path != string(record.Identity.Path) {
		t.Fatalf("record equality HIR = %#v", typed)
	}

	malformedHIR := typed
	malformedHIR.Functions = append([]hir.Function(nil), typed.Functions...)
	malformedHIR.Functions[0].Body = typed.Functions[0].Body
	malformedBinary := *typed.Functions[0].Body.Binary
	malformedRight := *malformedBinary.Right
	malformedReference := *malformedRight.Reference
	malformedReference.Position = 0
	malformedRight.Reference = &malformedReference
	malformedBinary.Right = &malformedRight
	malformedHIR.Functions[0].Body.Binary = &malformedBinary
	_, malformedHIRErr := LowerHIRToCore(malformedHIR)
	diagnostics, ok := AsDiagnostics(malformedHIRErr)
	if !ok || len(diagnostics) != 1 || diagnostics[0].Code != CodeCoreLowering {
		t.Fatalf("malformed record equality HIR error = %#v (%v)", diagnostics, malformedHIRErr)
	}

	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	coreFunction := core.Functions[0]
	if core.LanguageContract != coreir.LanguageContractV120 || coreFunction.Body.Kind != coreir.ExprBinary || coreFunction.Body.Binary == nil || coreFunction.Body.Binary.Operator != coreir.OperatorEqual || coreFunction.Body.Binary.Left == nil || coreFunction.Body.Binary.Left.Parameter == nil || *coreFunction.Body.Binary.Left.Parameter != 0 || coreFunction.Body.Binary.Right == nil || coreFunction.Body.Binary.Right.Parameter == nil || *coreFunction.Body.Binary.Right.Parameter != 1 {
		t.Fatalf("record equality Core = %#v", core)
	}
	for _, mutate := range []func(*coreir.Function){
		func(function *coreir.Function) { function.Body.Binary.Operator = coreir.OperatorLessThan },
		func(function *coreir.Function) { *function.Body.Binary.Left.Parameter = 1 },
		func(function *coreir.Function) { *function.Body.Binary.Right.Parameter = 0 },
		func(function *coreir.Function) {
			function.Parameters = append(function.Parameters, coreir.Parameter{Position: 2, Name: "extra", Type: function.Parameters[0].Type})
		},
		func(function *coreir.Function) { function.ReturnType = function.Parameters[0].Type },
	} {
		malformed := cloneCoreFunctionForRecordEquality(coreFunction)
		mutate(&malformed)
		if err := coreir.ValidateFunction(malformed); err == nil {
			t.Fatal("Core accepted malformed direct record equality")
		}
	}

	equal := recordEqualityValue(coreFunction.Parameters[0].Type, "row-1", 42, 1.5, true)
	for _, test := range []struct {
		name  string
		left  coreeval.Value
		right coreeval.Value
		want  bool
	}{
		{name: "equal", left: equal, right: equal, want: true},
		{name: "string differs", left: equal, right: recordEqualityValue(coreFunction.Parameters[0].Type, "row-2", 42, 1.5, true)},
		{name: "integer differs", left: equal, right: recordEqualityValue(coreFunction.Parameters[0].Type, "row-1", 43, 1.5, true)},
		{name: "float differs", left: equal, right: recordEqualityValue(coreFunction.Parameters[0].Type, "row-1", 42, 2.5, true)},
		{name: "bool differs", left: equal, right: recordEqualityValue(coreFunction.Parameters[0].Type, "row-1", 42, 1.5, false)},
		{name: "signed zero", left: recordEqualityValue(coreFunction.Parameters[0].Type, "row-1", 42, 0, true), right: recordEqualityValue(coreFunction.Parameters[0].Type, "row-1", 42, math.Copysign(0, -1), true), want: true},
		{name: "NaN", left: recordEqualityValue(coreFunction.Parameters[0].Type, "row-1", 42, math.NaN(), true), right: recordEqualityValue(coreFunction.Parameters[0].Type, "row-1", 42, math.NaN(), true)},
	} {
		t.Run(test.name, func(t *testing.T) {
			outcome, err := coreeval.Evaluate(coreFunction, []coreeval.Value{test.left, test.right})
			if err != nil || !outcome.OK || outcome.Error != "" || outcome.Value.Type.Kind != coreir.TypePrimitive || outcome.Value.Type.Primitive != coreir.PrimitiveBool || outcome.Value.Bool != test.want {
				t.Fatalf("Core equality outcome = %#v (%v)", outcome, err)
			}
		})
	}
	invalid := recordEqualityValue(coreFunction.Parameters[0].Type, string([]byte{0xff}), 42, 1.5, true)
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{invalid, equal}); err == nil {
		t.Fatal("Core evaluator accepted invalid UTF-8 record equality input")
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
		t.Fatal("record equality generated Go is nondeterministic")
	}
	assertCompilerGolden(t, "record-equality.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "record-equality.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "record-equality.go", generated)
	compileAndRunRecordEqualityGo(t, generated, gobackend.FunctionName(coreFunction))
}

func TestV120AdmitsExactlyDirectPrimitiveRecordEqualityOperators(t *testing.T) {
	for _, operator := range []struct {
		spelling string
		want     hir.Operator
		result   bool
	}{
		{spelling: "==", want: hir.OperatorEqual, result: true},
		{spelling: "!=", want: hir.OperatorNotEqual},
	} {
		t.Run(operator.spelling, func(t *testing.T) {
			source := fmt.Sprintf(`public Record Row { public string Id; } public Class Root { public bool Compare(Row left, Row right) => left %s right; }`, operator.spelling)
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV120
			analysis := AnalyzeSemanticModuleSet(input)
			if err := analysis.Error(); err != nil {
				t.Fatal(err)
			}
			typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Compare").Identity)
			if err != nil {
				t.Fatal(err)
			}
			if typed.Functions[0].Body.Binary == nil || typed.Functions[0].Body.Binary.Operator != operator.want {
				t.Fatalf("operator %q HIR = %#v", operator.spelling, typed.Functions[0].Body)
			}
			core, err := LowerHIRToCore(typed)
			if err != nil {
				t.Fatal(err)
			}
			recordType := core.Functions[0].Parameters[0].Type
			record := coreeval.Value{Type: recordType, Record: []coreeval.Value{{Type: recordType.Record.Fields[0].Type, String: "same"}}}
			outcome, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{record, record})
			if err != nil || outcome.Value.Bool != operator.result {
				t.Fatalf("operator %q outcome = %#v (%v)", operator.spelling, outcome, err)
			}
			if _, err := gobackend.Generate(core); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestV120PrimitiveRecordEqualityRejectsExcludedForms(t *testing.T) {
	tests := []struct {
		name string
		src  string
		code DiagnosticCode
	}{
		{name: "ordered comparison", src: `public Record Row { public string Id; } public Class Root { public bool Compare(Row left, Row right) => left < right; }`, code: CodeExpressionType},
		{name: "reversed operands", src: `public Record Row { public string Id; } public Class Root { public bool Compare(Row left, Row right) => right == left; }`, code: CodeExpressionType},
		{name: "repeated operand", src: `public Record Row { public string Id; } public Class Root { public bool Compare(Row left, Row right) => left == left; }`, code: CodeExpressionType},
		{name: "nested equality", src: `public Record Row { public string Id; } public Class Root { public bool Compare(Row left, Row right) => (left == right) == true; }`, code: CodeExpressionType},
		{name: "one parameter", src: `public Record Row { public string Id; } public Class Root { public bool Compare(Row value) => value == value; }`, code: CodeExpressionType},
		{name: "extra parameter", src: `public Record Row { public string Id; } public Class Root { public bool Compare(Row left, Row right, bool extra) => left == right; }`, code: CodeInvalidType},
		{name: "different records", src: `public Record Row { public string Id; } public Record Other { public string Id; } public Class Root { public bool Compare(Row left, Other right) => left == right; }`, code: CodeInvalidType},
		{name: "wrong return", src: `public Record Row { public string Id; } public Class Root { public int Compare(Row left, Row right) => left == right; }`, code: CodeInvalidType},
		{name: "record field comparison", src: `public Record Row { public string Id; } public Class Root { public bool Compare(Row left, Row right) => left.Id == right.Id; }`, code: CodeExpressionType},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.src)}, nil)
			input.LanguageContract = PipeLangLanguageContractV120
			analysis := AnalyzeSemanticModuleSet(input)
			if !analysis.Diagnostics.HasErrors() {
				t.Fatal("excluded v0.12.0 record equality form was accepted")
			}
			assertDiagnosticCode(t, analysis, test.code)
		})
	}
}

func TestPrimitiveRecordEqualityRequiresExplicitV120Migration(t *testing.T) {
	source := `public Record Row { public string Id; } public Class Root { public bool Equal(Row left, Row right) => left == right; }`
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040, PipeLangLanguageContractV050, PipeLangLanguageContractV060, PipeLangLanguageContractV070, PipeLangLanguageContractV080, PipeLangLanguageContractV090, PipeLangLanguageContractV100, PipeLangLanguageContractV110} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
		input.LanguageContract = contract
		analysis := AnalyzeSemanticModuleSet(input)
		if !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted primitive record equality", contract)
		}
	}
}

func TestV120PreservesFrozenArithmeticResultTextAndRecordContracts(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
	}{
		{name: "checked add", source: checkedAddSource},
		{name: "checked subtract", source: checkedSubtractSource},
		{name: "checked multiply", source: checkedMultiplySource},
		{name: "checked negate", source: checkedNegateSource},
		{name: "checked divide", source: checkedDivideSource},
		{name: "Result transport", source: resultTransportIntSource},
		{name: "ordinal text", source: `public Class Root { public bool Before(string left, string right) => left < right; }`},
		{name: "record identity transport", source: primitiveRecordTransportSource},
		{name: "record field projection", source: `public Record Row { public string Id; } public Class Root { public string IdOf(Row value) => value.Id; }`},
		{name: "record construction", source: `public Record Row { public string Id; } public Class Root { public Row Create(string id) => new Row { Id = id }; }`},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV120
			analysis := AnalyzeSemanticModuleSet(input)
			if err := analysis.Error(); err != nil {
				t.Fatal(err)
			}
			typed, err := LowerSemanticMethodToHIR(analysis, semanticMethod(t, analysis).Identity)
			if err != nil {
				t.Fatal(err)
			}
			core, err := LowerHIRToCore(typed)
			if err != nil {
				t.Fatal(err)
			}
			if core.LanguageContract != coreir.LanguageContractV120 {
				t.Fatalf("Core contract = %q", core.LanguageContract)
			}
			if _, err := gobackend.Generate(core); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func cloneCoreFunctionForRecordEquality(function coreir.Function) coreir.Function {
	cloned := function
	cloned.Parameters = append([]coreir.Parameter(nil), function.Parameters...)
	cloned.Body = function.Body
	binary := *function.Body.Binary
	left := *binary.Left
	right := *binary.Right
	leftPosition := *left.Parameter
	rightPosition := *right.Parameter
	left.Parameter = &leftPosition
	right.Parameter = &rightPosition
	binary.Left = &left
	binary.Right = &right
	cloned.Body.Binary = &binary
	return cloned
}

func recordEqualityValue(recordType coreir.Type, id string, count int64, ratio float64, ready bool) coreeval.Value {
	return coreeval.Value{Type: recordType, Record: []coreeval.Value{
		{Type: recordType.Record.Fields[0].Type, String: id},
		{Type: recordType.Record.Fields[1].Type, Int: count},
		{Type: recordType.Record.Fields[2].Type, Float: ratio},
		{Type: recordType.Record.Fields[3].Type, Bool: ready},
	}}
}

func compileAndRunRecordEqualityGo(t *testing.T, generated []byte, functionName string) {
	t.Helper()
	testSource := fmt.Sprintf(`package %s

import (
	"math"
	"testing"
)

func TestGeneratedRecordEquality(t *testing.T) {
	equal := PipeLangRecordTestPackageAppRootRow{Id: "row-1", Count: 42, Ratio: 1.5, Ready: true}
	if !%s(equal, equal) {
		t.Fatal("equal records compared unequal")
	}
	different := equal
	different.Count++
	if %s(equal, different) {
		t.Fatal("different records compared equal")
	}
	positiveZero := equal
	positiveZero.Ratio = 0
	negativeZero := positiveZero
	negativeZero.Ratio = math.Copysign(0, -1)
	if !%s(positiveZero, negativeZero) {
		t.Fatal("signed zeros compared unequal")
	}
	nanLeft := equal
	nanLeft.Ratio = math.NaN()
	nanRight := nanLeft
	if %s(nanLeft, nanRight) {
		t.Fatal("NaN records compared equal")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("invalid UTF-8 record field did not fail")
		}
	}()
	invalid := equal
	invalid.Id = string([]byte{0xff})
	_ = %s(invalid, equal)
}
`, gobackend.PackageName, functionName, functionName, functionName, functionName, functionName)
	compileAndRunGeneratedGoFiles(t, generated, []byte(testSource))
}
