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

func TestV200RecordListAtHIRCoreEvaluatorAndGo(t *testing.T) {
	source, err := os.ReadFile("testdata/record-list-at.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-list-at.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV200
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}

	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	method := projectedMemberNamed(t, projectedTypeNamed(t, projection, "Root"), "RowAt")
	if projection.LanguageContract != PipeLangLanguageContractV200 || projection.Schema != PipeLangSemanticProjectionVersion || method.Type.Identity == nil || method.Type.Identity.PackageID != PipeLangBuiltinPackageID || method.Type.Identity.Path != PipeLangOptionalSemanticPath || len(method.Type.Arguments) != 1 || method.Type.Arguments[0].Identity == nil || method.Type.Arguments[0].Identity.Path != "app.root.containerrow" || len(method.Parameters) != 2 || method.Parameters[0].Type.Identity == nil || method.Parameters[0].Type.Identity.Path != PipeLangListSemanticPath || len(method.Parameters[0].Type.Arguments) != 1 || method.Parameters[0].Type.Arguments[0].Identity == nil || method.Parameters[0].Type.Arguments[0].Identity.Path != "app.root.containerrow" || method.Parameters[1].Type.Primitive != TypeInt {
		t.Fatalf("record-list at projection = %#v / %#v", projection, method)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "RowAt").Identity)
	if err != nil {
		t.Fatal(err)
	}
	function := typed.Functions[0]
	if typed.LanguageContract != coreir.LanguageContractV200 || function.Body.Kind != hir.ExprListAt || function.Body.ListAt == nil || function.Body.ListAt.Values == nil || function.Body.ListAt.Values.Reference == nil || function.Body.ListAt.Values.Reference.Position != 0 || function.Body.ListAt.Index == nil || function.Body.ListAt.Index.Reference == nil || function.Body.ListAt.Index.Reference.Position != 1 || function.ReturnType.Kind != hir.TypeOptional || function.ReturnType.Optional == nil || function.ReturnType.Optional.Value.Kind != hir.TypeRecord {
		t.Fatalf("record-list at HIR = %#v", typed)
	}

	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	coreFunction := core.Functions[0]
	if core.LanguageContract != coreir.LanguageContractV200 || coreFunction.Body.Kind != coreir.ExprListAt || coreFunction.Body.ListAt == nil || coreFunction.Body.ListAt.Values == nil || coreFunction.Body.ListAt.Values.Parameter == nil || *coreFunction.Body.ListAt.Values.Parameter != 0 || coreFunction.Body.ListAt.Index == nil || coreFunction.Body.ListAt.Index.Parameter == nil || *coreFunction.Body.ListAt.Index.Parameter != 1 || coreFunction.ReturnType.Kind != coreir.TypeOptional || coreFunction.ReturnType.Optional == nil || coreFunction.ReturnType.Optional.Value.Kind != coreir.TypeRecord {
		t.Fatalf("record-list at Core = %#v", core)
	}

	listType := coreFunction.Parameters[0].Type
	rowType := listType.List.Element
	first := optionalRecordValue(rowType, "container-1", "api", true)
	second := optionalRecordValue(rowType, "container-2", "worker", false)
	rows := coreeval.Value{Type: listType, List: []coreeval.Value{first, second}}
	for _, test := range []struct {
		name    string
		index   int64
		present bool
		id      string
	}{
		{name: "first", index: 0, present: true, id: "container-1"},
		{name: "second", index: 1, present: true, id: "container-2"},
		{name: "negative", index: -1},
		{name: "past-end", index: 2},
		{name: "max-int", index: int64(^uint64(0) >> 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			outcome, err := coreeval.Evaluate(coreFunction, []coreeval.Value{rows, {Type: coreir.SignedInteger(64), Int: test.index}})
			if err != nil || !outcome.OK || outcome.Value.Optional == nil || outcome.Value.Optional.Present != test.present {
				t.Fatalf("at outcome = %#v (%v)", outcome, err)
			}
			if test.present && (outcome.Value.Optional.Value == nil || outcome.Value.Optional.Value.Record[0].String != test.id) {
				t.Fatalf("at payload = %#v", outcome.Value.Optional)
			}
			if !test.present && outcome.Value.Optional.Value != nil {
				t.Fatalf("absent at payload = %#v", outcome.Value.Optional)
			}
		})
	}
	selected, err := coreeval.Evaluate(coreFunction, []coreeval.Value{rows, {Type: coreir.SignedInteger(64)}})
	if err != nil {
		t.Fatal(err)
	}
	rows.List[0].Record[0].String = "mutated"
	if selected.Value.Optional.Value.Record[0].String != "container-1" {
		t.Fatal("record-list at result aliases caller-owned record storage")
	}
	invalid := rows
	invalid.List = append([]coreeval.Value(nil), rows.List...)
	invalid.List[1] = optionalRecordValue(rowType, string([]byte{0xff}), "bad", false)
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{invalid, {Type: coreir.SignedInteger(64), Int: -1}}); err == nil {
		t.Fatal("record-list at accepted invalid UTF-8 in an unselected list element")
	}
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{{Type: listType}, {Type: coreir.SignedInteger(64)}}); err == nil {
		t.Fatal("record-list at accepted a nil list")
	}

	malformedHIR := typed
	malformedHIR.Functions = append([]hir.Function(nil), typed.Functions...)
	malformedHIR.Functions[0].Body = typed.Functions[0].Body
	malformedAt := *typed.Functions[0].Body.ListAt
	malformedAt.Index = nil
	malformedHIR.Functions[0].Body.ListAt = &malformedAt
	if _, err := LowerHIRToCore(malformedHIR); err == nil {
		t.Fatal("Core lowering accepted malformed record-list at HIR")
	} else if diagnostics, ok := AsDiagnostics(err); !ok || len(diagnostics) != 1 || diagnostics[0].Code != CodeCoreLowering {
		t.Fatalf("malformed at HIR diagnostic = %#v (%v)", diagnostics, err)
	}
	priorHIR := typed
	priorHIR.LanguageContract = coreir.LanguageContractV190
	if _, err := LowerHIRToCore(priorHIR); err == nil {
		t.Fatal("v0.19.0 HIR implicitly accepted record-list at")
	}

	malformedCore := coreFunction
	malformedCore.Body = coreFunction.Body
	malformedCoreAt := *coreFunction.Body.ListAt
	malformedCoreAt.Index = nil
	malformedCore.Body.ListAt = &malformedCoreAt
	if err := coreir.ValidateFunction(malformedCore); err == nil {
		t.Fatal("Core accepted malformed record-list at")
	}
	priorCore := core
	priorCore.LanguageContract = coreir.LanguageContractV190
	if _, err := gobackend.Generate(priorCore); err == nil {
		t.Fatal("v0.19.0 Core implicitly accepted record-list at")
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
		t.Fatal("record-list at generated Go is nondeterministic")
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(recordListAtGeneratedGoTest()))

	assertCompilerGolden(t, "record-list-at.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "record-list-at.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "record-list-at.go", generated)
}

func TestV200ParserPreservesRecordListAtOperandsAndSpan(t *testing.T) {
	const source = `public Record Row { public string Id; } public Class Root { public Optional<Row> RowAt(List<Row> values, int index) => at(values, index); }`
	sources, diagnostics := NewSourceSet([]SourceInput{{Path: "root.pipe", Data: []byte(source)}})
	if diagnostics.HasErrors() {
		t.Fatal(diagnosticError(sources, diagnostics))
	}
	program, err := parseSourceFileWithLanguageContract(sources, sources.Files()[0], PipeLangLanguageContractV200)
	if err != nil {
		t.Fatal(err)
	}
	at, ok := program.Classes[0].Methods[0].Body.(*ListAtExpr)
	if !ok {
		t.Fatalf("record-list at AST = %#v", program.Classes[0].Methods[0].Body)
	}
	values, valuesOK := at.Values.(*IdentExpr)
	index, indexOK := at.Index.(*IdentExpr)
	if !at.Span.IsValid() || !valuesOK || values.Name != "values" || !values.Span.IsValid() || !indexOK || index.Name != "index" || !index.Span.IsValid() {
		t.Fatalf("record-list at AST = %#v", program.Classes[0].Methods[0].Body)
	}
}

func TestV200RecordListAtRejectsExcludedForms(t *testing.T) {
	tests := []struct {
		name string
		src  string
		code DiagnosticCode
	}{
		{name: "primitive list", src: `public Record Row { public string Id; } public Class Root { public Optional<Row> RowAt(List<string> values, int index) => at(values, index); }`, code: CodeInvalidType},
		{name: "wrong return", src: `public Record Row { public string Id; } public Class Root { public Row RowAt(List<Row> values, int index) => at(values, index); }`, code: CodeInvalidType},
		{name: "float index", src: `public Record Row { public string Id; } public Class Root { public Optional<Row> RowAt(List<Row> values, float index) => at(values, index); }`, code: CodeInvalidType},
		{name: "reordered parameters", src: `public Record Row { public string Id; } public Class Root { public Optional<Row> RowAt(int index, List<Row> values) => at(values, index); }`, code: CodeInvalidType},
		{name: "computed list", src: `public Record Row { public string Id; } public Class Root { public Optional<Row> RowAt(List<Row> values, Row value, int index) => at(append(values, value), index); }`, code: CodeInvalidType},
		{name: "computed index", src: `public Record Row { public string Id; } public Class Root { public Optional<Row> RowAt(List<Row> values, int index) => at(values, index + 1); }`, code: CodeExpressionType},
		{name: "nested at", src: `public Record Row { public string Id; } public Class Root { public bool HasRow(List<Row> values, int index) => has_value(at(values, index)); }`, code: CodeInvalidType},
		{name: "list field", src: `public Record Row { public string Id; } public Class Root { public List<Row> Values; public Optional<Row> RowAt(List<Row> values, int index) => at(values, index); }`, code: CodeInvalidType},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.src)}, nil)
			input.LanguageContract = PipeLangLanguageContractV200
			analysis := AnalyzeSemanticModuleSet(input)
			if !analysis.Diagnostics.HasErrors() {
				t.Fatal("excluded v0.20.0 record-list at form was accepted")
			}
			if analysis.Diagnostics[0].Code != test.code {
				t.Fatalf("record-list at diagnostic = %s, want %s", analysis.Diagnostics[0].Code, test.code)
			}
		})
	}
}

func TestRecordListAtRequiresExplicitV200Migration(t *testing.T) {
	source := `public Record Row { public string Id; } public Class Root { public Optional<Row> RowAt(List<Row> values, int index) => at(values, index); }`
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040, PipeLangLanguageContractV050, PipeLangLanguageContractV060, PipeLangLanguageContractV070, PipeLangLanguageContractV080, PipeLangLanguageContractV090, PipeLangLanguageContractV100, PipeLangLanguageContractV110, PipeLangLanguageContractV120, PipeLangLanguageContractV130, PipeLangLanguageContractV140, PipeLangLanguageContractV150, PipeLangLanguageContractV160, PipeLangLanguageContractV170, PipeLangLanguageContractV180, PipeLangLanguageContractV190} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
		input.LanguageContract = contract
		analysis := AnalyzeSemanticModuleSet(input)
		if !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted record-list at", contract)
		}
	}
}

func TestV200PreservesV190SnapshotResult(t *testing.T) {
	source, err := os.ReadFile("testdata/snapshot-result.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "snapshot-result.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV200
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"RowsOk", "RowsFailed", "ForwardRows", "RowsSucceeded", "RowsOr", "ErrorOr"} {
		typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, name).Identity)
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

func recordListAtGeneratedGoTest() string {
	return fmt.Sprintf(`package %s

import "testing"

func expectRecordListAtPanic(t *testing.T, invoke func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	invoke()
}

func TestGeneratedRecordListAt(t *testing.T) {
	rows := []PipeLangRecordTestPackageAppRootContainerrow{
		{Id: "container-1", Name: "api", Running: true},
		{Id: "container-2", Name: "worker"},
	}
	selected := PipeLangRowAt(rows, 1)
	selectedRow, present := selected.(pipelangOptionalSome[PipeLangRecordTestPackageAppRootContainerrow])
	if !present || selectedRow.value.Id != "container-2" {
		t.Fatal("selected record mismatch")
	}
	for _, index := range []int64{-1, 2, int64(^uint64(0) >> 1)} {
		if _, present := PipeLangRowAt(rows, index).(pipelangOptionalSome[PipeLangRecordTestPackageAppRootContainerrow]); present {
			t.Fatalf("index %%d unexpectedly selected a row", index)
		}
	}
	invalid := append([]PipeLangRecordTestPackageAppRootContainerrow(nil), rows...)
	invalid[1].Id = string([]byte{0xff})
	expectRecordListAtPanic(t, func() { PipeLangRowAt(invalid, -1) })
	var nilRows []PipeLangRecordTestPackageAppRootContainerrow
	expectRecordListAtPanic(t, func() { PipeLangRowAt(nilRows, 0) })
}
`, gobackend.PackageName)
}
