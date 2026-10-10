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

func TestV140PrimitiveOptionalDefaultingHIRCoreAndGo(t *testing.T) {
	source, err := os.ReadFile("testdata/optional-value-or.pipe")
	if err != nil {
		t.Fatal(err)
	}
	analysis := analyzeOptionalSource(t, string(source), PipeLangLanguageContractV140)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}

	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	projected := projectedMemberNamed(t, projectedTypeNamed(t, projection, "Root"), "ValueOr")
	if projection.LanguageContract != PipeLangLanguageContractV140 || projected.Type.Primitive != TypeString || len(projected.Parameters) != 2 || projected.Parameters[0].Position != 0 || projected.Parameters[0].Type.Identity == nil || projected.Parameters[0].Type.Identity.PackageID != PipeLangBuiltinPackageID || projected.Parameters[0].Type.Identity.Path != PipeLangOptionalSemanticPath || len(projected.Parameters[0].Type.Arguments) != 1 || projected.Parameters[0].Type.Arguments[0].Primitive != TypeString || projected.Parameters[1].Position != 1 || projected.Parameters[1].Type.Primitive != TypeString {
		t.Fatalf("value_or semantic projection = %#v", projected)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethod(t, analysis).Identity)
	if err != nil {
		t.Fatal(err)
	}
	if typed.LanguageContract != coreir.LanguageContractV140 || len(typed.Functions) != 1 || typed.Functions[0].Body.Kind != hir.ExprOptionalValueOr || typed.Functions[0].Body.ValueOr == nil || typed.Functions[0].Body.ValueOr.Value == nil || typed.Functions[0].Body.ValueOr.Fallback == nil || typed.Functions[0].Body.ValueOr.Value.Reference == nil || typed.Functions[0].Body.ValueOr.Value.Reference.Position != 0 || typed.Functions[0].Body.ValueOr.Fallback.Reference == nil || typed.Functions[0].Body.ValueOr.Fallback.Reference.Position != 1 {
		t.Fatalf("value_or HIR = %#v", typed)
	}

	malformedHIR := typed
	malformedHIR.Functions = append([]hir.Function(nil), typed.Functions...)
	malformedHIR.Functions[0].Body = typed.Functions[0].Body
	malformedValueOr := *typed.Functions[0].Body.ValueOr
	malformedFallback := *malformedValueOr.Fallback
	malformedReference := *malformedFallback.Reference
	malformedReference.Position = 0
	malformedFallback.Reference = &malformedReference
	malformedValueOr.Fallback = &malformedFallback
	malformedHIR.Functions[0].Body.ValueOr = &malformedValueOr
	if _, err := LowerHIRToCore(malformedHIR); err == nil {
		t.Fatal("Core lowering accepted malformed value_or HIR")
	} else if diagnostics, ok := AsDiagnostics(err); !ok || len(diagnostics) != 1 || diagnostics[0].Code != CodeCoreLowering {
		t.Fatalf("malformed value_or HIR error = %#v (%v)", diagnostics, err)
	}

	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if core.LanguageContract != coreir.LanguageContractV140 || len(core.Functions) != 1 || core.Functions[0].Body.Kind != coreir.ExprOptionalValueOr || core.Functions[0].Body.ValueOr == nil {
		t.Fatalf("value_or Core = %#v", core)
	}
	priorHIR := typed
	priorHIR.LanguageContract = coreir.LanguageContractV130
	if _, err := LowerHIRToCore(priorHIR); err == nil {
		t.Fatal("v0.13.0 HIR implicitly accepted Optional defaulting")
	}
	priorCore := core
	priorCore.LanguageContract = coreir.LanguageContractV130
	if _, err := gobackend.Generate(priorCore); err == nil {
		t.Fatal("v0.13.0 Core implicitly accepted Optional defaulting")
	}

	optionalType := core.Functions[0].Parameters[0].Type
	stringType := core.Functions[0].Parameters[1].Type
	payload := coreeval.Value{Type: stringType, String: "chosen"}
	present := coreeval.Value{Type: optionalType, Optional: &coreeval.OptionalValue{Present: true, Value: &payload}}
	absent := coreeval.Value{Type: optionalType, Optional: &coreeval.OptionalValue{}}
	for _, test := range []struct {
		name     string
		optional coreeval.Value
		fallback string
		want     string
	}{
		{name: "present", optional: present, fallback: "fallback", want: "chosen"},
		{name: "absent", optional: absent, fallback: "fallback", want: "fallback"},
	} {
		t.Run(test.name, func(t *testing.T) {
			outcome, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{test.optional, {Type: stringType, String: test.fallback}})
			if err != nil || !outcome.OK || outcome.Value.Type.Primitive != coreir.PrimitiveString || outcome.Value.String != test.want {
				t.Fatalf("value_or outcome = %#v (%v)", outcome, err)
			}
		})
	}
	if _, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{present, {Type: stringType, String: string([]byte{0xff})}}); err == nil {
		t.Fatal("Core evaluator skipped invalid UTF-8 fallback validation for present Optional")
	}
	invalidOptional := present
	invalidOptional.Optional = nil
	if _, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{invalidOptional, {Type: stringType, String: "fallback"}}); err == nil {
		t.Fatal("Core evaluator accepted a non-canonical Optional value")
	}

	malformedCore := core.Functions[0]
	malformedCore.Body = core.Functions[0].Body
	malformedCoreValueOr := *malformedCore.Body.ValueOr
	malformedCoreFallback := *malformedCoreValueOr.Fallback
	wrongPosition := 0
	malformedCoreFallback.Parameter = &wrongPosition
	malformedCoreValueOr.Fallback = &malformedCoreFallback
	malformedCore.Body.ValueOr = &malformedCoreValueOr
	if err := coreir.ValidateFunction(malformedCore); err == nil {
		t.Fatal("Core accepted a non-direct value_or fallback")
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
		t.Fatal("Optional defaulting generated Go is nondeterministic")
	}
	assertCompilerGolden(t, "optional-value-or.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "optional-value-or.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "optional-value-or.go", generated)
	compileAndRunOptionalDefaultingGo(t, generated)
}

func TestV140ParserPreservesOptionalDefaultingOperandsAndSpans(t *testing.T) {
	const source = `public Class Root { public string ValueOr(Optional<string> value, string fallback) => value_or(value, fallback); }`
	sources, diagnostics := NewSourceSet([]SourceInput{{Path: "root.pipe", Data: []byte(source)}})
	if diagnostics.HasErrors() {
		t.Fatal(diagnosticError(sources, diagnostics))
	}
	program, err := parseSourceFileWithLanguageContract(sources, sources.Files()[0], PipeLangLanguageContractV140)
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Classes) != 1 || len(program.Classes[0].Methods) != 1 {
		t.Fatalf("value_or program = %#v", program)
	}
	valueOr, ok := program.Classes[0].Methods[0].Body.(*OptionalValueOrExpr)
	if !ok {
		t.Fatalf("value_or AST = %#v", program.Classes[0].Methods[0].Body)
	}
	value, valueOK := valueOr.Value.(*IdentExpr)
	fallback, fallbackOK := valueOr.Fallback.(*IdentExpr)
	if !valueOr.Span.IsValid() || !valueOK || value.Name != "value" || !value.Span.IsValid() || !fallbackOK || fallback.Name != "fallback" || !fallback.Span.IsValid() || value.Span.Start >= fallback.Span.Start {
		t.Fatalf("value_or AST = %#v", program.Classes[0].Methods[0].Body)
	}
}

func TestV140PrimitiveOptionalDefaultingAdmitsAllPrimitivePayloads(t *testing.T) {
	for _, primitive := range []string{"string", "int", "float", "bool"} {
		t.Run(primitive, func(t *testing.T) {
			source := fmt.Sprintf(`public Class Root { public %[1]s ValueOr(Optional<%[1]s> value, %[1]s fallback) => value_or(value, fallback); }`, primitive)
			analysis := analyzeOptionalSource(t, source, PipeLangLanguageContractV140)
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
			payloadType := core.Functions[0].ReturnType
			optionalType := core.Functions[0].Parameters[0].Type
			presentPayload := coreeval.Value{Type: payloadType}
			fallback := coreeval.Value{Type: payloadType}
			switch primitive {
			case "string":
				presentPayload.String = "present"
				fallback.String = "fallback"
			case "int":
				presentPayload.Int = 42
				fallback.Int = -7
			case "float":
				presentPayload.Float = math.NaN()
				fallback.Float = math.Copysign(0, -1)
			case "bool":
				presentPayload.Bool = true
			}
			present := coreeval.Value{Type: optionalType, Optional: &coreeval.OptionalValue{Present: true, Value: &presentPayload}}
			absent := coreeval.Value{Type: optionalType, Optional: &coreeval.OptionalValue{}}
			presentOutcome, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{present, fallback})
			if err != nil || !presentOutcome.OK {
				t.Fatalf("present %s value_or = %#v (%v)", primitive, presentOutcome, err)
			}
			absentOutcome, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{absent, fallback})
			if err != nil || !absentOutcome.OK {
				t.Fatalf("absent %s value_or = %#v (%v)", primitive, absentOutcome, err)
			}
			if primitive == "float" && (!math.IsNaN(presentOutcome.Value.Float) || !math.Signbit(absentOutcome.Value.Float)) {
				t.Fatalf("float value_or lost NaN or signed-zero behavior: present=%#v absent=%#v", presentOutcome, absentOutcome)
			}
			generated, err := gobackend.Generate(core)
			if err != nil {
				t.Fatal(err)
			}
			compileAndRunGeneratedGoFiles(t, generated, []byte(optionalDefaultingPrimitiveGoTest(primitive)))
		})
	}
}

func TestV140PrimitiveOptionalDefaultingRejectsExcludedForms(t *testing.T) {
	tests := []struct {
		name string
		src  string
		code DiagnosticCode
	}{
		{name: "literal fallback", src: `public Class Root { public string ValueOr(Optional<string> value, string fallback) => value_or(value, "fixed"); }`, code: CodeExpressionType},
		{name: "computed fallback", src: `public Class Root { public int ValueOr(Optional<int> value, int fallback) => value_or(value, fallback + 1); }`, code: CodeExpressionType},
		{name: "nested optional", src: `public Class Root { public string ValueOr(string value, string fallback) => value_or(some(value), fallback); }`, code: CodeExpressionType},
		{name: "reordered parameters", src: `public Class Root { public string ValueOr(string fallback, Optional<string> value) => value_or(value, fallback); }`, code: CodeInvalidType},
		{name: "mismatched fallback", src: `public Class Root { public string ValueOr(Optional<string> value, int fallback) => value_or(value, fallback); }`, code: CodeInvalidType},
		{name: "extra parameter", src: `public Class Root { public string ValueOr(Optional<string> value, string fallback, bool extra) => value_or(value, fallback); }`, code: CodeInvalidType},
		{name: "wrong return", src: `public Class Root { public bool ValueOr(Optional<string> value, string fallback) => value_or(value, fallback); }`, code: CodeInvalidType},
		{name: "optional equality remains excluded", src: `public Class Root { public bool Same(Optional<string> left, Optional<string> right) => left == right; }`, code: CodeInvalidType},
		{name: "optional projection remains excluded", src: `public Class Root { public string Value(Optional<string> value) => value.Value; }`, code: CodeInvalidType},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analysis := analyzeOptionalSource(t, test.src, PipeLangLanguageContractV140)
			if !analysis.Diagnostics.HasErrors() {
				t.Fatal("excluded v0.14.0 Optional defaulting form was accepted")
			}
			assertDiagnosticCode(t, analysis, test.code)
		})
	}
}

func TestPrimitiveOptionalDefaultingRequiresExplicitV140Migration(t *testing.T) {
	source := `public Class Root { public string ValueOr(Optional<string> value, string fallback) => value_or(value, fallback); }`
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040, PipeLangLanguageContractV050, PipeLangLanguageContractV060, PipeLangLanguageContractV070, PipeLangLanguageContractV080, PipeLangLanguageContractV090, PipeLangLanguageContractV100, PipeLangLanguageContractV110, PipeLangLanguageContractV120, PipeLangLanguageContractV130} {
		analysis := analyzeOptionalSource(t, source, contract)
		if !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted primitive Optional defaulting", contract)
		}
	}
}

func TestV140PreservesFrozenOptionalResultTextAndRecordContracts(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
	}{
		{name: "Optional construction", source: `public Class Root { public Optional<string> Present(string value) => some(value); }`},
		{name: "Optional absence", source: `public Class Root { public Optional<string> Absent() => none<string>(); }`},
		{name: "Optional transport", source: `public Class Root { public Optional<string> Forward(Optional<string> value) => value; }`},
		{name: "Optional inspection", source: `public Class Root { public bool HasValue(Optional<string> value) => has_value(value); }`},
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
			analysis := analyzeOptionalSource(t, test.source, PipeLangLanguageContractV140)
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
			if core.LanguageContract != coreir.LanguageContractV140 {
				t.Fatalf("Core contract = %q", core.LanguageContract)
			}
			if _, err := gobackend.Generate(core); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func compileAndRunOptionalDefaultingGo(t *testing.T, generated []byte) {
	t.Helper()
	testSource := fmt.Sprintf(`package %s

import "testing"

func expectPanic(t *testing.T, invoke func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	invoke()
}

func TestGeneratedOptionalDefaulting(t *testing.T) {
	if got := PipeLangValueOr(pipelangSomeValue("chosen"), "fallback"); got != "chosen" {
		t.Fatalf("present value_or = %%q", got)
	}
	if got := PipeLangValueOr(pipelangNoneValue[string](), "fallback"); got != "fallback" {
		t.Fatalf("absent value_or = %%q", got)
	}
	var invalid PipeLangOptional[string]
	expectPanic(t, func() { PipeLangValueOr(invalid, "fallback") })
	expectPanic(t, func() { PipeLangValueOr(pipelangSomeValue("chosen"), string([]byte{0xff})) })
}
`, gobackend.PackageName)
	compileAndRunGeneratedGoFiles(t, generated, []byte(testSource))
}

func optionalDefaultingPrimitiveGoTest(primitive string) string {
	if primitive == "float" {
		return fmt.Sprintf(`package %s

import (
	"math"
	"testing"
)

func TestPrimitiveOptionalDefaulting(t *testing.T) {
	if present := PipeLangValueOr(pipelangSomeValue(math.NaN()), math.Copysign(0, -1)); !math.IsNaN(present) {
		t.Fatalf("present float value_or = %%v", present)
	}
	if absent := PipeLangValueOr(pipelangNoneValue[float64](), math.Copysign(0, -1)); !math.Signbit(absent) {
		t.Fatalf("absent float value_or = %%v", absent)
	}
}
`, gobackend.PackageName)
	}
	present, fallback, wantPresent, wantAbsent := `"chosen"`, `"fallback"`, `"chosen"`, `"fallback"`
	switch primitive {
	case "int":
		present, fallback, wantPresent, wantAbsent = "int64(42)", "int64(-7)", "int64(42)", "int64(-7)"
	case "bool":
		present, fallback, wantPresent, wantAbsent = "true", "false", "true", "false"
	}
	return fmt.Sprintf(`package %s

import "testing"

func TestPrimitiveOptionalDefaulting(t *testing.T) {
	if present := PipeLangValueOr(pipelangSomeValue(%s), %s); present != %s {
		t.Fatalf("present value_or = %%v", present)
	}
	if absent := PipeLangValueOr(pipelangNoneValue[%s](), %s); absent != %s {
		t.Fatalf("absent value_or = %%v", absent)
	}
}
`, gobackend.PackageName, present, fallback, wantPresent, generatedGoPrimitiveType(primitive), fallback, wantAbsent)
}

func generatedGoPrimitiveType(primitive string) string {
	switch primitive {
	case "int":
		return "int64"
	case "bool":
		return "bool"
	default:
		return "string"
	}
}
