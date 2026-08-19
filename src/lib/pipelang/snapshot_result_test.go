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

func TestV190SnapshotResultHIRCoreEvaluatorAndGo(t *testing.T) {
	source, err := os.ReadFile("testdata/snapshot-result.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "snapshot-result.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV190
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}

	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	rowsOK := projectedMemberNamed(t, projectedTypeNamed(t, projection, "Root"), "RowsOk")
	if projection.LanguageContract != PipeLangLanguageContractV190 || projection.Schema != PipeLangSemanticProjectionVersion || rowsOK.Type.Identity == nil || rowsOK.Type.Identity.PackageID != PipeLangBuiltinPackageID || rowsOK.Type.Identity.Path != PipeLangResultSemanticPath || len(rowsOK.Type.Arguments) != 2 || rowsOK.Type.Arguments[0].Identity == nil || rowsOK.Type.Arguments[0].Identity.Path != PipeLangListSemanticPath || rowsOK.Type.Arguments[0].Arguments[0].Identity == nil || rowsOK.Type.Arguments[0].Arguments[0].Identity.Path != "app.root.containerrow" || rowsOK.Type.Arguments[1].Primitive != TypeString {
		t.Fatalf("snapshot Result semantic projection = %#v / %#v", projection, rowsOK)
	}

	names := []string{"RowsOk", "RowsFailed", "ForwardRows", "RowsSucceeded", "RowsOr", "ErrorOr"}
	var typed hir.Program
	for index, name := range names {
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
	wantHIRKinds := []hir.ExprKind{hir.ExprResultOK, hir.ExprResultErr, hir.ExprReference, hir.ExprResultIsOK, hir.ExprResultSuccessOr, hir.ExprResultFailureOr}
	if typed.LanguageContract != coreir.LanguageContractV190 || len(typed.Functions) != len(wantHIRKinds) {
		t.Fatalf("snapshot Result HIR = %#v", typed)
	}
	for index, kind := range wantHIRKinds {
		if typed.Functions[index].Body.Kind != kind {
			t.Fatalf("snapshot Result HIR function %d = %#v", index, typed.Functions[index])
		}
	}
	priorHIR := typed
	priorHIR.LanguageContract = coreir.LanguageContractV180
	if _, err := LowerHIRToCore(priorHIR); err == nil {
		t.Fatal("v0.18.0 HIR implicitly accepted snapshot Result")
	}
	malformedHIR := typed
	malformedHIR.Functions = append([]hir.Function(nil), typed.Functions...)
	malformedHIR.Functions[0].Body = typed.Functions[0].Body
	malformedOK := *typed.Functions[0].Body.ResultOK
	malformedOK.Value = nil
	malformedHIR.Functions[0].Body.ResultOK = &malformedOK
	if _, err := LowerHIRToCore(malformedHIR); err == nil {
		t.Fatal("Core lowering accepted malformed snapshot Result HIR")
	}

	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	wantCoreKinds := []coreir.ExprKind{coreir.ExprResultOK, coreir.ExprResultErr, coreir.ExprReference, coreir.ExprResultIsOK, coreir.ExprResultSuccessOr, coreir.ExprResultFailureOr}
	for index, kind := range wantCoreKinds {
		if core.Functions[index].Body.Kind != kind {
			t.Fatalf("snapshot Result Core function %d = %#v", index, core.Functions[index])
		}
	}
	malformedCore := core.Functions[0]
	malformedCore.Body = core.Functions[0].Body
	malformedCoreOK := *core.Functions[0].Body.ResultOK
	malformedCoreOK.Value = nil
	malformedCore.Body.ResultOK = &malformedCoreOK
	if err := coreir.ValidateFunction(malformedCore); err == nil {
		t.Fatal("Core accepted malformed snapshot Result construction")
	}
	priorCore := core
	priorCore.LanguageContract = coreir.LanguageContractV180
	if _, err := gobackend.Generate(priorCore); err == nil {
		t.Fatal("v0.18.0 Core implicitly accepted snapshot Result")
	}

	listType := core.Functions[0].Parameters[0].Type
	rowType := listType.List.Element
	row := snapshotResultRow(rowType, "container-1", "api", true)
	rows := coreeval.Value{Type: listType, List: []coreeval.Value{row}}
	ok, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{rows})
	if err != nil || !ok.OK || len(ok.Value.List) != 1 {
		t.Fatalf("snapshot Result ok = %#v (%v)", ok, err)
	}
	rows.List[0].Record[0].String = "mutated"
	if ok.Value.List[0].Record[0].String != "container-1" {
		t.Fatal("snapshot Result ok aliases caller-owned list storage")
	}
	failureText := coreeval.Value{Type: core.Functions[1].Parameters[0].Type, String: "daemon unavailable"}
	failed, err := coreeval.Evaluate(core.Functions[1], []coreeval.Value{failureText})
	if err != nil || failed.OK || failed.Failure == nil || failed.Failure.String != "daemon unavailable" {
		t.Fatalf("snapshot Result err = %#v (%v)", failed, err)
	}
	resultType := core.Functions[0].ReturnType
	okValue := coreeval.Value{Type: resultType, Result: &ok}
	failedValue := coreeval.Value{Type: resultType, Result: &failed}
	forwarded, err := coreeval.Evaluate(core.Functions[2], []coreeval.Value{okValue})
	if err != nil || !forwarded.OK || len(forwarded.Value.List) != 1 {
		t.Fatalf("snapshot Result identity = %#v (%v)", forwarded, err)
	}
	ok.Value.List[0].Record[0].String = "mutated"
	if forwarded.Value.List[0].Record[0].String != "container-1" {
		t.Fatal("snapshot Result identity aliases caller-owned Result storage")
	}
	forwardedValue := coreeval.Value{Type: resultType, Result: &forwarded}
	for _, test := range []struct {
		value coreeval.Value
		want  bool
	}{{forwardedValue, true}, {failedValue, false}} {
		outcome, err := coreeval.Evaluate(core.Functions[3], []coreeval.Value{test.value})
		if err != nil || outcome.Value.Bool != test.want {
			t.Fatalf("snapshot Result is_ok = %#v (%v)", outcome, err)
		}
	}
	fallbackRow := snapshotResultRow(rowType, "fallback", "cached", false)
	fallback := coreeval.Value{Type: listType, List: []coreeval.Value{fallbackRow}}
	selected, err := coreeval.Evaluate(core.Functions[4], []coreeval.Value{forwardedValue, fallback})
	if err != nil || selected.Value.List[0].Record[0].String != "container-1" {
		t.Fatalf("snapshot Result success_or success = %#v (%v)", selected, err)
	}
	defaulted, err := coreeval.Evaluate(core.Functions[4], []coreeval.Value{failedValue, fallback})
	if err != nil || defaulted.Value.List[0].Record[0].String != "fallback" {
		t.Fatalf("snapshot Result success_or failure = %#v (%v)", defaulted, err)
	}
	fallback.List[0].Record[0].String = "mutated"
	if defaulted.Value.List[0].Record[0].String != "fallback" {
		t.Fatal("snapshot Result success_or aliases fallback storage")
	}
	textFallback := coreeval.Value{Type: core.Functions[5].Parameters[1].Type, String: "none"}
	errorOutcome, err := coreeval.Evaluate(core.Functions[5], []coreeval.Value{failedValue, textFallback})
	if err != nil || errorOutcome.Value.String != "daemon unavailable" {
		t.Fatalf("snapshot Result failure_or failure = %#v (%v)", errorOutcome, err)
	}
	noError, err := coreeval.Evaluate(core.Functions[5], []coreeval.Value{forwardedValue, textFallback})
	if err != nil || noError.Value.String != "none" {
		t.Fatalf("snapshot Result failure_or success = %#v (%v)", noError, err)
	}
	invalidFallback := coreeval.Value{Type: listType, List: []coreeval.Value{snapshotResultRow(rowType, string([]byte{0xff}), "bad", false)}}
	if _, err := coreeval.Evaluate(core.Functions[4], []coreeval.Value{forwardedValue, invalidFallback}); err == nil {
		t.Fatal("snapshot Result success_or accepted invalid unselected fallback")
	}
	invalidFailure := coreeval.Value{Type: core.Functions[1].Parameters[0].Type, String: string([]byte{0xff})}
	if _, err := coreeval.Evaluate(core.Functions[1], []coreeval.Value{invalidFailure}); err == nil {
		t.Fatal("snapshot Result err accepted invalid UTF-8")
	}

	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	second, err := gobackend.Generate(core)
	if err != nil || !bytes.Equal(generated, second) {
		t.Fatalf("snapshot Result generated Go is nondeterministic: %v", err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(snapshotResultGeneratedGoTest()))
	assertCompilerGolden(t, "snapshot-result.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "snapshot-result.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "snapshot-result.go", generated)
}

func TestV190SnapshotResultParserPreservesTypeArgumentsOperandsAndSpans(t *testing.T) {
	const source = `public Record Row { public string Id; } public Class Root { public Result<List<Row>, string> Value(List<Row> value) => ok<List<Row>, string>(value); }`
	sources, diagnostics := NewSourceSet([]SourceInput{{Path: "root.pipe", Data: []byte(source)}})
	if diagnostics.HasErrors() {
		t.Fatal(diagnosticError(sources, diagnostics))
	}
	program, err := parseSourceFileWithLanguageContract(sources, sources.Files()[0], PipeLangLanguageContractV190)
	if err != nil {
		t.Fatal(err)
	}
	result, ok := program.Classes[0].Methods[0].Body.(*ResultOKExpr)
	if !ok {
		t.Fatalf("snapshot Result AST = %#v", program.Classes[0].Methods[0].Body)
	}
	operand, operandOK := result.Value.(*IdentExpr)
	if !result.Span.IsValid() || !result.SuccessType.Span.IsValid() || !result.FailureType.Span.IsValid() || !operandOK || operand.Name != "value" || !operand.Span.IsValid() {
		t.Fatalf("snapshot Result AST = %#v", program.Classes[0].Methods[0].Body)
	}
}

func TestV190SnapshotResultRejectsExcludedForms(t *testing.T) {
	tests := []string{
		`public Class Root { public Result<List<string>, string> Value(List<string> value) => ok<List<string>, string>(value); }`,
		`public Record Row { public string Id; } public Class Root { public Result<List<Row>, bool> Value(List<Row> value) => ok<List<Row>, bool>(value); }`,
		`public Record Row { public string Id; } public Class Root { public Result<List<Row>, string> Value(List<Row> value) => ok<List<Row>, string>(empty_list<Row>()); }`,
		`public Record Row { public string Id; } public Class Root { public List<Row> Value(Result<List<Row>, string> value) => success_or(value, empty_list<Row>()); }`,
		`public Record Row { public string Id; } public Record Holder { public Result<List<Row>, string> Value; }`,
		`public Record Row { public string Id; } public Interface Root { public Result<List<Row>, string> Value(List<Row> value); }`,
	}
	for index, source := range tests {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
		input.LanguageContract = PipeLangLanguageContractV190
		if analysis := AnalyzeSemanticModuleSet(input); !analysis.Diagnostics.HasErrors() {
			t.Fatalf("excluded snapshot Result form %d was accepted", index)
		}
	}
}

func TestSnapshotResultRequiresExplicitV190Migration(t *testing.T) {
	source := `public Record Row { public string Id; } public Class Root { public Result<List<Row>, string> Value(List<Row> value) => ok<List<Row>, string>(value); }`
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040, PipeLangLanguageContractV050, PipeLangLanguageContractV060, PipeLangLanguageContractV070, PipeLangLanguageContractV080, PipeLangLanguageContractV090, PipeLangLanguageContractV100, PipeLangLanguageContractV110, PipeLangLanguageContractV120, PipeLangLanguageContractV130, PipeLangLanguageContractV140, PipeLangLanguageContractV150, PipeLangLanguageContractV160, PipeLangLanguageContractV170, PipeLangLanguageContractV180} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
		input.LanguageContract = contract
		if analysis := AnalyzeSemanticModuleSet(input); !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted snapshot Result", contract)
		}
	}
}

func snapshotResultRow(rowType coreir.Type, id, name string, running bool) coreeval.Value {
	return coreeval.Value{Type: rowType, Record: []coreeval.Value{
		{Type: rowType.Record.Fields[0].Type, String: id},
		{Type: rowType.Record.Fields[1].Type, String: name},
		{Type: rowType.Record.Fields[2].Type, Bool: running},
	}}
}

func snapshotResultGeneratedGoTest() string {
	return fmt.Sprintf(`package %s

import "testing"

func expectSnapshotResultPanic(t *testing.T, invoke func()) {
	t.Helper()
	defer func() { if recover() == nil { t.Fatal("expected panic") } }()
	invoke()
}

func TestGeneratedSnapshotResult(t *testing.T) {
	row := PipeLangRecordTestPackageAppRootContainerrow{Id: "container-1", Name: "api", Running: true}
	rows := []PipeLangRecordTestPackageAppRootContainerrow{row}
	ok := PipeLangRowsOk(rows)
	rows[0].Id = "mutated"
	if !PipeLangRowsSucceeded(ok) || PipeLangRowsOr(ok, []PipeLangRecordTestPackageAppRootContainerrow{{Id: "fallback"}})[0].Id != "container-1" { t.Fatal("ok Result mismatch") }
	forwarded := PipeLangForwardRows(ok)
	ok.Value[0].Id = "mutated"
	if forwarded.Value[0].Id != "container-1" { t.Fatal("identity Result aliases input") }
	failed := PipeLangRowsFailed("daemon unavailable")
	fallback := []PipeLangRecordTestPackageAppRootContainerrow{{Id: "fallback", Name: "cached"}}
	selected := PipeLangRowsOr(failed, fallback)
	fallback[0].Id = "mutated"
	if selected[0].Id != "fallback" || PipeLangErrorOr(failed, "none") != "daemon unavailable" || PipeLangErrorOr(forwarded, "none") != "none" { t.Fatal("Result default mismatch") }
	expectSnapshotResultPanic(t, func() { PipeLangRowsFailed(string([]byte{0xff})) })
	expectSnapshotResultPanic(t, func() { PipeLangRowsOr(forwarded, []PipeLangRecordTestPackageAppRootContainerrow{{Id: string([]byte{0xff})}}) })
	expectSnapshotResultPanic(t, func() { PipeLangForwardRows(PipeLangResult[[]PipeLangRecordTestPackageAppRootContainerrow, string]{OK: true, Error: "bad"}) })
}
`, gobackend.PackageName)
}
