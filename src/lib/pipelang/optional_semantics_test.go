package pipelang

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
)

func TestV130PrimitiveOptionalHIRCoreAndGo(t *testing.T) {
	source, err := os.ReadFile("testdata/optional-value.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "optional-value.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV130
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}

	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	projectedRoot := projectedTypeNamed(t, projection, "Root")
	for _, name := range []string{"Present", "Absent", "Forward"} {
		member := projectedMemberNamed(t, projectedRoot, name)
		if member.Type.Kind != TypeRefApplied || member.Type.Identity == nil || member.Type.Identity.PackageID != PipeLangBuiltinPackageID || member.Type.Identity.Path != PipeLangOptionalSemanticPath || len(member.Type.Arguments) != 1 || member.Type.Arguments[0].Primitive != TypeString {
			t.Fatalf("%s Optional projection = %#v", name, member.Type)
		}
	}
	hasValueProjection := projectedMemberNamed(t, projectedRoot, "HasValue")
	if hasValueProjection.Type.Primitive != TypeBool || len(hasValueProjection.Parameters) != 1 || hasValueProjection.Parameters[0].Type.Identity == nil || hasValueProjection.Parameters[0].Type.Identity.Path != PipeLangOptionalSemanticPath {
		t.Fatalf("HasValue projection = %#v", hasValueProjection)
	}
	if _, err := LowerSemanticMethodToHIR(analysis, SemanticIdentity{PackageID: "test.package", Path: "app.root.root.missing"}); err == nil {
		t.Fatal("HIR lowering accepted a missing Optional method identity")
	} else if diagnostics, ok := AsDiagnostics(err); !ok || len(diagnostics) != 1 || diagnostics[0].Code != CodeHIRLowering {
		t.Fatalf("malformed Optional HIR request = %#v (%v)", diagnostics, err)
	}

	var typed hir.Program
	for index, name := range []string{"Present", "Absent", "Forward", "HasValue"} {
		lowered, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, name).Identity)
		if err != nil {
			t.Fatal(err)
		}
		if index == 0 {
			typed = lowered
		} else {
			typed.Functions = append(typed.Functions, lowered.Functions...)
		}
	}
	if typed.LanguageContract != coreir.LanguageContractV130 || len(typed.Functions) != 4 || typed.Functions[0].Body.Kind != hir.ExprOptionalSome || typed.Functions[1].Body.Kind != hir.ExprOptionalNone || typed.Functions[2].Body.Kind != hir.ExprReference || typed.Functions[3].Body.Kind != hir.ExprOptionalHasValue {
		t.Fatalf("Optional HIR = %#v", typed)
	}

	malformedHIR := typed
	malformedHIR.Functions = append([]hir.Function(nil), typed.Functions...)
	malformedHIR.Functions[0].Body = typed.Functions[0].Body
	malformedSome := *typed.Functions[0].Body.Some
	malformedValue := *malformedSome.Value
	malformedReference := *malformedValue.Reference
	malformedReference.Position = 1
	malformedValue.Reference = &malformedReference
	malformedSome.Value = &malformedValue
	malformedHIR.Functions[0].Body.Some = &malformedSome
	if _, err := LowerHIRToCore(malformedHIR); err == nil {
		t.Fatal("Core lowering accepted malformed Optional HIR")
	} else if diagnostics, ok := AsDiagnostics(err); !ok || len(diagnostics) != 1 || diagnostics[0].Code != CodeCoreLowering {
		t.Fatalf("malformed Optional HIR error = %#v (%v)", diagnostics, err)
	}

	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if core.LanguageContract != coreir.LanguageContractV130 || len(core.Functions) != 4 || core.Functions[0].Body.Kind != coreir.ExprOptionalSome || core.Functions[1].Body.Kind != coreir.ExprOptionalNone || core.Functions[2].Body.Kind != coreir.ExprReference || core.Functions[3].Body.Kind != coreir.ExprOptionalHasValue {
		t.Fatalf("Optional Core = %#v", core)
	}
	priorHIR := typed
	priorHIR.LanguageContract = coreir.LanguageContractV120
	if _, err := LowerHIRToCore(priorHIR); err == nil {
		t.Fatal("v0.12.0 HIR implicitly accepted primitive Optional")
	}
	priorCore := core
	priorCore.LanguageContract = coreir.LanguageContractV120
	if _, err := gobackend.Generate(priorCore); err == nil {
		t.Fatal("v0.12.0 Core implicitly accepted primitive Optional")
	}

	stringType := core.Functions[0].Parameters[0].Type
	presentOutcome, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{{Type: stringType, String: "value"}})
	if err != nil || !presentOutcome.OK || presentOutcome.Value.Optional == nil || !presentOutcome.Value.Optional.Present || presentOutcome.Value.Optional.Value == nil || presentOutcome.Value.Optional.Value.String != "value" {
		t.Fatalf("present outcome = %#v (%v)", presentOutcome, err)
	}
	absentOutcome, err := coreeval.Evaluate(core.Functions[1], nil)
	if err != nil || !absentOutcome.OK || absentOutcome.Value.Optional == nil || absentOutcome.Value.Optional.Present || absentOutcome.Value.Optional.Value != nil {
		t.Fatalf("absent outcome = %#v (%v)", absentOutcome, err)
	}
	forwardOutcome, err := coreeval.Evaluate(core.Functions[2], []coreeval.Value{presentOutcome.Value})
	if err != nil || forwardOutcome.Value.Optional == nil || !forwardOutcome.Value.Optional.Present || forwardOutcome.Value.Optional.Value == nil || forwardOutcome.Value.Optional.Value.String != "value" {
		t.Fatalf("forward outcome = %#v (%v)", forwardOutcome, err)
	}
	for _, test := range []struct {
		name  string
		value coreeval.Value
		want  bool
	}{{name: "present", value: presentOutcome.Value, want: true}, {name: "absent", value: absentOutcome.Value}} {
		t.Run("has value "+test.name, func(t *testing.T) {
			outcome, err := coreeval.Evaluate(core.Functions[3], []coreeval.Value{test.value})
			if err != nil || !outcome.OK || outcome.Value.Type.Primitive != coreir.PrimitiveBool || outcome.Value.Bool != test.want {
				t.Fatalf("has_value outcome = %#v (%v)", outcome, err)
			}
		})
	}
	if _, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{{Type: stringType, String: string([]byte{0xff})}}); err == nil {
		t.Fatal("Core evaluator accepted invalid UTF-8 Optional payload")
	}
	invalidOptional := presentOutcome.Value
	invalidOptional.Optional = nil
	if _, err := coreeval.Evaluate(core.Functions[2], []coreeval.Value{invalidOptional}); err == nil {
		t.Fatal("Core evaluator accepted a non-canonical Optional value")
	}

	malformedCore := core.Functions[0]
	malformedCore.Body = core.Functions[0].Body
	malformedCoreSome := *malformedCore.Body.Some
	malformedCoreValue := *malformedCoreSome.Value
	wrongPosition := 1
	malformedCoreValue.Parameter = &wrongPosition
	malformedCoreSome.Value = &malformedCoreValue
	malformedCore.Body.Some = &malformedCoreSome
	if err := coreir.ValidateFunction(malformedCore); err == nil {
		t.Fatal("Core accepted malformed direct Optional construction")
	}
	malformedCore = core.Functions[1]
	malformedOptional := *malformedCore.ReturnType.Optional
	malformedPayloadType := malformedOptional.Value
	malformedPayloadType.Optional = &coreir.OptionalType{Value: stringType}
	malformedOptional.Value = malformedPayloadType
	malformedCore.ReturnType.Optional = &malformedOptional
	malformedCore.Body.Type = malformedCore.ReturnType
	if err := coreir.ValidateFunction(malformedCore); err == nil {
		t.Fatal("Core accepted a nested/foreign Optional payload representation")
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
		t.Fatal("Optional generated Go is nondeterministic")
	}
	assertCompilerGolden(t, "optional-value.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "optional-value.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "optional-value.go", generated)
	compileAndRunOptionalGo(t, generated)
}

func TestV130PrimitiveOptionalAdmitsAllPrimitivePayloads(t *testing.T) {
	for _, primitive := range []string{"string", "int", "float", "bool"} {
		t.Run(primitive, func(t *testing.T) {
			source := fmt.Sprintf(`public Class Root {
public Optional<%[1]s> Present(%[1]s value) => some(value);
public Optional<%[1]s> Absent() => none<%[1]s>();
public Optional<%[1]s> Forward(Optional<%[1]s> value) => value;
public bool HasValue(Optional<%[1]s> value) => has_value(value);
}`, primitive)
			analysis := analyzeOptionalSource(t, source, PipeLangLanguageContractV130)
			var typed hir.Program
			for index, name := range []string{"Present", "Absent", "Forward", "HasValue"} {
				lowered, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, name).Identity)
				if err != nil {
					t.Fatal(err)
				}
				if index == 0 {
					typed = lowered
				} else {
					typed.Functions = append(typed.Functions, lowered.Functions...)
				}
			}
			core, err := LowerHIRToCore(typed)
			if err != nil {
				t.Fatal(err)
			}
			argument := coreeval.Value{Type: core.Functions[0].Parameters[0].Type}
			goValue := "true"
			switch primitive {
			case "string":
				argument.String = "value"
				goValue = `"value"`
			case "int":
				argument.Int = 42
				goValue = "int64(42)"
			case "float":
				argument.Float = 1.5
				goValue = "float64(1.5)"
			case "bool":
				argument.Bool = true
			}
			present, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{argument})
			if err != nil || present.Value.Optional == nil || !present.Value.Optional.Present {
				t.Fatalf("%s present outcome = %#v (%v)", primitive, present, err)
			}
			inspected, err := coreeval.Evaluate(core.Functions[3], []coreeval.Value{present.Value})
			if err != nil || !inspected.Value.Bool {
				t.Fatalf("%s has_value outcome = %#v (%v)", primitive, inspected, err)
			}
			generated, err := gobackend.Generate(core)
			if err != nil {
				t.Fatal(err)
			}
			generatedTest := fmt.Sprintf("package %s\n\nimport \"testing\"\n\nfunc TestPrimitiveOptional(t *testing.T) { if !PipeLangHasValue(PipeLangForward(PipeLangPresent(%s))) || PipeLangHasValue(PipeLangAbsent()) { t.Fatal(\"Optional mismatch\") } }\n", gobackend.PackageName, goValue)
			compileAndRunGeneratedGoFiles(t, generated, []byte(generatedTest))
		})
	}
}

func TestV130PrimitiveOptionalRejectsExcludedForms(t *testing.T) {
	tests := []struct {
		name string
		src  string
		code DiagnosticCode
	}{
		{name: "record payload", src: `public Record Row { public string Id; } public Class Root { public Optional<Row> Present(Row value) => some(value); }`, code: CodeInvalidType},
		{name: "nested payload", src: `public Class Root { public Optional<Optional<string>> Forward(Optional<Optional<string>> value) => value; }`, code: CodeInvalidType},
		{name: "record field", src: `public Record Row { public Optional<string> Id; }`, code: CodeInvalidType},
		{name: "class field", src: `public Class Root { public Optional<string> Value; }`, code: CodeInvalidType},
		{name: "optional default", src: `public Class Root { public string Value = some("fixed"); }`, code: CodeExpressionType},
		{name: "interface signature", src: `public Interface IRoot { public Optional<string> Present(string value); }`, code: CodeInvalidType},
		{name: "some literal", src: `public Class Root { public Optional<string> Present(string value) => some("fixed"); }`, code: CodeExpressionType},
		{name: "some computed", src: `public Class Root { public Optional<int> Present(int value) => some(value + 1); }`, code: CodeExpressionType},
		{name: "some extra parameter", src: `public Class Root { public Optional<string> Present(string value, bool extra) => some(value); }`, code: CodeInvalidType},
		{name: "none mismatch", src: `public Class Root { public Optional<string> Absent() => none<int>(); }`, code: CodeExpressionType},
		{name: "none parameter", src: `public Class Root { public Optional<string> Absent(string value) => none<string>(); }`, code: CodeExpressionType},
		{name: "transport mismatch", src: `public Class Root { public Optional<string> Forward(Optional<int> value) => value; }`, code: CodeInvalidType},
		{name: "transport extra parameter", src: `public Class Root { public Optional<string> Forward(Optional<string> value, bool extra) => value; }`, code: CodeInvalidType},
		{name: "nested inspection", src: `public Class Root { public bool HasValue(string value) => has_value(some(value)); }`, code: CodeExpressionType},
		{name: "optional equality", src: `public Class Root { public bool Same(Optional<string> left, Optional<string> right) => left == right; }`, code: CodeInvalidType},
		{name: "optional projection", src: `public Class Root { public string Value(Optional<string> value) => value.Value; }`, code: CodeInvalidType},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analysis := analyzeOptionalSource(t, test.src, PipeLangLanguageContractV130)
			if !analysis.Diagnostics.HasErrors() {
				t.Fatal("excluded v0.13.0 Optional form was accepted")
			}
			assertDiagnosticCode(t, analysis, test.code)
		})
	}
}

func TestPrimitiveOptionalRequiresExplicitV130Migration(t *testing.T) {
	source := `public Class Root { public Optional<string> Present(string value) => some(value); }`
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040, PipeLangLanguageContractV050, PipeLangLanguageContractV060, PipeLangLanguageContractV070, PipeLangLanguageContractV080, PipeLangLanguageContractV090, PipeLangLanguageContractV100, PipeLangLanguageContractV110, PipeLangLanguageContractV120} {
		analysis := analyzeOptionalSource(t, source, contract)
		if !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted primitive Optional", contract)
		}
	}
}

func TestV130PreservesFrozenResultTextAndRecordContracts(t *testing.T) {
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
		{name: "record identity", source: primitiveRecordTransportSource},
		{name: "record projection", source: `public Record Row { public string Id; } public Class Root { public string IdOf(Row value) => value.Id; }`},
		{name: "record construction", source: `public Record Row { public string Id; } public Class Root { public Row Create(string id) => new Row { Id = id }; }`},
		{name: "record equality", source: `public Record Row { public string Id; } public Class Root { public bool Same(Row left, Row right) => left == right; }`},
	} {
		t.Run(test.name, func(t *testing.T) {
			analysis := analyzeOptionalSource(t, test.source, PipeLangLanguageContractV130)
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
			if core.LanguageContract != coreir.LanguageContractV130 {
				t.Fatalf("Core contract = %q", core.LanguageContract)
			}
			if _, err := gobackend.Generate(core); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func analyzeOptionalSource(t *testing.T, source string, contract LanguageContract) *Analysis {
	t.Helper()
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
	input.LanguageContract = contract
	return AnalyzeSemanticModuleSet(input)
}

func compileAndRunOptionalGo(t *testing.T, generated []byte) {
	t.Helper()
	testSource := fmt.Sprintf(`package %s

import "testing"

func TestGeneratedOptional(t *testing.T) {
	present := PipeLangPresent("value")
	absent := PipeLangAbsent()
	if !PipeLangHasValue(present) || PipeLangHasValue(absent) {
		t.Fatal("generated Optional presence inspection disagrees")
	}
	if !PipeLangHasValue(PipeLangForward(present)) {
		t.Fatal("generated Optional identity transport disagrees")
	}
	var invalid PipeLangOptional[string]
	defer func() {
		if recover() == nil {
			t.Fatal("generated Go accepted a zero/nil Optional")
		}
	}()
	PipeLangForward(invalid)
}
`, gobackend.PackageName)
	compileAndRunGeneratedGoFiles(t, generated, []byte(testSource))
}
