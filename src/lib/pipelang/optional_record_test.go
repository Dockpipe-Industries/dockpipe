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

func TestV180PrimitiveRecordOptionalHIRCoreAndGo(t *testing.T) {
	source, err := os.ReadFile("testdata/optional-record.pipe")
	if err != nil {
		t.Fatal(err)
	}
	analysis := analyzeOptionalSource(t, string(source), PipeLangLanguageContractV180)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}

	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	projectedRoot := projectedTypeNamed(t, projection, "Root")
	presentProjection := projectedMemberNamed(t, projectedRoot, "PresentRow")
	if projection.LanguageContract != PipeLangLanguageContractV180 || projection.Schema != PipeLangSemanticProjectionVersion || presentProjection.Type.Identity == nil || presentProjection.Type.Identity.PackageID != PipeLangBuiltinPackageID || presentProjection.Type.Identity.Path != PipeLangOptionalSemanticPath || len(presentProjection.Type.Arguments) != 1 || presentProjection.Type.Arguments[0].Identity == nil || presentProjection.Type.Arguments[0].Identity.Path != "app.root.containerrow" || len(presentProjection.Parameters) != 1 || presentProjection.Parameters[0].Type.Identity == nil || presentProjection.Parameters[0].Type.Identity.Path != "app.root.containerrow" {
		t.Fatalf("Optional<R> semantic projection = %#v / %#v", projection, presentProjection)
	}
	rowOrProjection := projectedMemberNamed(t, projectedRoot, "RowOr")
	if rowOrProjection.Type.Identity == nil || rowOrProjection.Type.Identity.Path != "app.root.containerrow" || len(rowOrProjection.Parameters) != 2 || rowOrProjection.Parameters[0].Type.Identity == nil || rowOrProjection.Parameters[0].Type.Identity.Path != PipeLangOptionalSemanticPath || len(rowOrProjection.Parameters[0].Type.Arguments) != 1 || rowOrProjection.Parameters[0].Type.Arguments[0].Identity == nil || rowOrProjection.Parameters[0].Type.Arguments[0].Identity.Path != "app.root.containerrow" || rowOrProjection.Parameters[1].Type.Identity == nil || rowOrProjection.Parameters[1].Type.Identity.Path != "app.root.containerrow" {
		t.Fatalf("Optional<R> value_or projection = %#v", rowOrProjection)
	}

	var typed hir.Program
	for index, name := range []string{"PresentRow", "AbsentRow", "ForwardRow", "HasRow", "RowOr"} {
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
	if typed.LanguageContract != coreir.LanguageContractV180 || len(typed.Functions) != 5 || typed.Functions[0].Body.Kind != hir.ExprOptionalSome || typed.Functions[1].Body.Kind != hir.ExprOptionalNone || typed.Functions[2].Body.Kind != hir.ExprReference || typed.Functions[3].Body.Kind != hir.ExprOptionalHasValue || typed.Functions[4].Body.Kind != hir.ExprOptionalValueOr || typed.Functions[0].ReturnType.Kind != hir.TypeOptional || typed.Functions[0].ReturnType.Optional == nil || typed.Functions[0].ReturnType.Optional.Value.Kind != hir.TypeRecord {
		t.Fatalf("Optional<R> HIR = %#v", typed)
	}
	priorHIR := typed
	priorHIR.LanguageContract = coreir.LanguageContractV170
	if _, err := LowerHIRToCore(priorHIR); err == nil {
		t.Fatal("v0.17.0 HIR implicitly accepted Optional<R>")
	}

	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if core.LanguageContract != coreir.LanguageContractV180 || len(core.Functions) != 5 || core.Functions[0].Body.Kind != coreir.ExprOptionalSome || core.Functions[1].Body.Kind != coreir.ExprOptionalNone || core.Functions[2].Body.Kind != coreir.ExprReference || core.Functions[3].Body.Kind != coreir.ExprOptionalHasValue || core.Functions[4].Body.Kind != coreir.ExprOptionalValueOr || core.Functions[0].ReturnType.Kind != coreir.TypeOptional || core.Functions[0].ReturnType.Optional == nil || core.Functions[0].ReturnType.Optional.Value.Kind != coreir.TypeRecord {
		t.Fatalf("Optional<R> Core = %#v", core)
	}
	priorCore := core
	priorCore.LanguageContract = coreir.LanguageContractV170
	if _, err := gobackend.Generate(priorCore); err == nil {
		t.Fatal("v0.17.0 Core implicitly accepted Optional<R>")
	}

	rowType := core.Functions[0].Parameters[0].Type
	row := optionalRecordValue(rowType, "container-1", "api", true)
	present, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{row})
	if err != nil || !present.OK || present.Value.Optional == nil || !present.Value.Optional.Present || present.Value.Optional.Value == nil || present.Value.Optional.Value.Record[0].String != "container-1" {
		t.Fatalf("Optional<R> present = %#v (%v)", present, err)
	}
	row.Record[0].String = "mutated"
	if present.Value.Optional.Value.Record[0].String != "container-1" {
		t.Fatal("Optional<R> some result aliases caller-owned record storage")
	}
	absent, err := coreeval.Evaluate(core.Functions[1], nil)
	if err != nil || !absent.OK || absent.Value.Optional == nil || absent.Value.Optional.Present || absent.Value.Optional.Value != nil {
		t.Fatalf("Optional<R> absent = %#v (%v)", absent, err)
	}
	forwarded, err := coreeval.Evaluate(core.Functions[2], []coreeval.Value{present.Value})
	if err != nil || forwarded.Value.Optional == nil || forwarded.Value.Optional.Value == nil || forwarded.Value.Optional.Value.Record[0].String != "container-1" {
		t.Fatalf("Optional<R> forward = %#v (%v)", forwarded, err)
	}
	present.Value.Optional.Value.Record[0].String = "mutated"
	if forwarded.Value.Optional.Value.Record[0].String != "container-1" {
		t.Fatal("Optional<R> transport result aliases its input payload")
	}
	for _, test := range []struct {
		name  string
		value coreeval.Value
		want  bool
	}{{name: "present", value: forwarded.Value, want: true}, {name: "absent", value: absent.Value}} {
		t.Run("has row "+test.name, func(t *testing.T) {
			outcome, err := coreeval.Evaluate(core.Functions[3], []coreeval.Value{test.value})
			if err != nil || !outcome.OK || outcome.Value.Bool != test.want {
				t.Fatalf("Optional<R> has_value = %#v (%v)", outcome, err)
			}
		})
	}
	fallback := optionalRecordValue(rowType, "fallback", "worker", false)
	selected, err := coreeval.Evaluate(core.Functions[4], []coreeval.Value{forwarded.Value, fallback})
	if err != nil || selected.Value.Record[0].String != "container-1" {
		t.Fatalf("Optional<R> present value_or = %#v (%v)", selected, err)
	}
	defaulted, err := coreeval.Evaluate(core.Functions[4], []coreeval.Value{absent.Value, fallback})
	if err != nil || defaulted.Value.Record[0].String != "fallback" {
		t.Fatalf("Optional<R> absent value_or = %#v (%v)", defaulted, err)
	}
	fallback.Record[0].String = "mutated"
	if defaulted.Value.Record[0].String != "fallback" {
		t.Fatal("Optional<R> value_or result aliases caller-owned fallback storage")
	}
	invalidFallback := optionalRecordValue(rowType, string([]byte{0xff}), "bad", false)
	if _, err := coreeval.Evaluate(core.Functions[4], []coreeval.Value{forwarded.Value, invalidFallback}); err == nil {
		t.Fatal("Optional<R> value_or accepted invalid UTF-8 in an unselected fallback")
	}
	invalidPresent := optionalRecordValue(rowType, "ok", string([]byte{0xff}), false)
	if _, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{invalidPresent}); err == nil {
		t.Fatal("Optional<R> some accepted invalid UTF-8 record payload")
	}
	malformed := forwarded.Value
	malformed.Optional = nil
	if _, err := coreeval.Evaluate(core.Functions[2], []coreeval.Value{malformed}); err == nil {
		t.Fatal("Optional<R> transport accepted a non-canonical Optional value")
	}

	malformedCore := core.Functions[0]
	malformedCore.Body = core.Functions[0].Body
	malformedSome := *malformedCore.Body.Some
	malformedSome.Value = nil
	malformedCore.Body.Some = &malformedSome
	if err := coreir.ValidateFunction(malformedCore); err == nil {
		t.Fatal("Core accepted malformed Optional<R> construction")
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
		t.Fatal("Optional<R> generated Go is nondeterministic")
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(optionalRecordGeneratedGoTest()))

	assertCompilerGolden(t, "optional-record.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "optional-record.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "optional-record.go", generated)
}

func TestV180PrimitiveRecordOptionalRejectsExcludedForms(t *testing.T) {
	tests := []struct {
		name string
		src  string
		code DiagnosticCode
	}{
		{name: "list payload", src: `public Record Row { public string Id; } public Class Root { public Optional<List<Row>> Forward(Optional<List<Row>> value) => value; }`, code: CodeInvalidType},
		{name: "nested payload", src: `public Record Row { public string Id; } public Class Root { public Optional<Optional<Row>> Forward(Optional<Optional<Row>> value) => value; }`, code: CodeInvalidType},
		{name: "record field", src: `public Record Row { public string Id; } public Record Holder { public Optional<Row> Value; }`, code: CodeInvalidType},
		{name: "class field", src: `public Record Row { public string Id; } public Class Root { public Optional<Row> Value; }`, code: CodeInvalidType},
		{name: "interface signature", src: `public Record Row { public string Id; } public Interface IRoot { public Optional<Row> Present(Row value); }`, code: CodeInvalidType},
		{name: "constructed some", src: `public Record Row { public string Id; } public Class Root { public Optional<Row> Present(Row value) => some(new Row { Id = value.Id }); }`, code: CodeExpressionType},
		{name: "extra some parameter", src: `public Record Row { public string Id; } public Class Root { public Optional<Row> Present(Row value, bool extra) => some(value); }`, code: CodeInvalidType},
		{name: "none mismatch", src: `public Record Row { public string Id; } public Record Other { public string Id; } public Class Root { public Optional<Row> Absent() => none<Other>(); }`, code: CodeExpressionType},
		{name: "value_or mismatch", src: `public Record Row { public string Id; } public Record Other { public string Id; } public Class Root { public Row ValueOr(Optional<Row> value, Other fallback) => value_or(value, fallback); }`, code: CodeInvalidType},
		{name: "optional equality", src: `public Record Row { public string Id; } public Class Root { public bool Same(Optional<Row> left, Optional<Row> right) => left == right; }`, code: CodeInvalidType},
		{name: "optional projection", src: `public Record Row { public string Id; } public Class Root { public Row Value(Optional<Row> value) => value.Value; }`, code: CodeInvalidType},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analysis := analyzeOptionalSource(t, test.src, PipeLangLanguageContractV180)
			assertDiagnosticCode(t, analysis, test.code)
		})
	}
}

func TestPrimitiveRecordOptionalRequiresExplicitV180Migration(t *testing.T) {
	source := `public Record Row { public string Id; } public Class Root { public Optional<Row> Present(Row value) => some(value); }`
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040, PipeLangLanguageContractV050, PipeLangLanguageContractV060, PipeLangLanguageContractV070, PipeLangLanguageContractV080, PipeLangLanguageContractV090, PipeLangLanguageContractV100, PipeLangLanguageContractV110, PipeLangLanguageContractV120, PipeLangLanguageContractV130, PipeLangLanguageContractV140, PipeLangLanguageContractV150, PipeLangLanguageContractV160, PipeLangLanguageContractV170} {
		analysis := analyzeOptionalSource(t, source, contract)
		if !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted Optional<R>", contract)
		}
	}
}

func TestV180PreservesPrimitiveOptionalAndRecordListAppend(t *testing.T) {
	for _, source := range []string{
		`public Class Root { public string ValueOr(Optional<string> value, string fallback) => value_or(value, fallback); }`,
		`public Record Row { public string Id; } public Class Root { public List<Row> AppendRow(List<Row> values, Row value) => append(values, value); }`,
	} {
		analysis := analyzeOptionalSource(t, source, PipeLangLanguageContractV180)
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
		if _, err := gobackend.Generate(core); err != nil {
			t.Fatal(err)
		}
	}
}

func optionalRecordValue(rowType coreir.Type, id, name string, running bool) coreeval.Value {
	return coreeval.Value{Type: rowType, Record: []coreeval.Value{
		{Type: rowType.Record.Fields[0].Type, String: id},
		{Type: rowType.Record.Fields[1].Type, String: name},
		{Type: rowType.Record.Fields[2].Type, Bool: running},
	}}
}

func optionalRecordGeneratedGoTest() string {
	return fmt.Sprintf(`package %s

import "testing"

func expectOptionalRecordPanic(t *testing.T, invoke func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	invoke()
}

func TestGeneratedOptionalRecord(t *testing.T) {
	row := PipeLangRecordTestPackageAppRootContainerrow{Id: "container-1", Name: "api", Running: true}
	present := PipeLangPresentRow(row)
	row.Id = "mutated"
	if !PipeLangHasRow(present) || PipeLangRowOr(present, PipeLangRecordTestPackageAppRootContainerrow{Id: "fallback"}).Id != "container-1" {
		t.Fatal("present Optional record mismatch")
	}
	if !PipeLangHasRow(PipeLangForwardRow(present)) {
		t.Fatal("forwarded Optional record mismatch")
	}
	fallback := PipeLangRecordTestPackageAppRootContainerrow{Id: "fallback", Name: "worker"}
	if got := PipeLangRowOr(PipeLangAbsentRow(), fallback); got.Id != "fallback" {
		t.Fatalf("absent Optional record = %%#v", got)
	}
	expectOptionalRecordPanic(t, func() {
		PipeLangRowOr(present, PipeLangRecordTestPackageAppRootContainerrow{Id: string([]byte{0xff})})
	})
	var invalid PipeLangOptional[PipeLangRecordTestPackageAppRootContainerrow]
	expectOptionalRecordPanic(t, func() { PipeLangForwardRow(invalid) })
}
`, gobackend.PackageName)
}
