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

func TestV170RecordListAppendHIRCoreAndGo(t *testing.T) {
	source, err := os.ReadFile("testdata/record-list-append.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-list-append.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV170
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}

	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	method := projectedMemberNamed(t, projectedTypeNamed(t, projection, "Root"), "AppendRow")
	if projection.LanguageContract != PipeLangLanguageContractV170 || projection.Schema != PipeLangSemanticProjectionVersion || method.Type.Identity == nil || method.Type.Identity.PackageID != PipeLangBuiltinPackageID || method.Type.Identity.Path != PipeLangListSemanticPath || len(method.Type.Arguments) != 1 || method.Type.Arguments[0].Identity == nil || method.Type.Arguments[0].Identity.Path != "app.root.containerrow" || len(method.Parameters) != 2 || method.Parameters[0].Type.Identity == nil || method.Parameters[0].Type.Identity.PackageID != PipeLangBuiltinPackageID || method.Parameters[0].Type.Identity.Path != PipeLangListSemanticPath || len(method.Parameters[0].Type.Arguments) != 1 || method.Parameters[0].Type.Arguments[0].Identity == nil || method.Parameters[0].Type.Arguments[0].Identity.Path != "app.root.containerrow" || method.Parameters[1].Type.Identity == nil || method.Parameters[1].Type.Identity.Path != "app.root.containerrow" {
		t.Fatalf("record-list append projection = %#v / %#v", projection, method)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "AppendRow").Identity)
	if err != nil {
		t.Fatal(err)
	}
	function := typed.Functions[0]
	if typed.LanguageContract != coreir.LanguageContractV170 || function.Body.Kind != hir.ExprListAppend || function.Body.ListAppend == nil || function.Body.ListAppend.Values == nil || function.Body.ListAppend.Value == nil || function.Body.ListAppend.Values.Reference == nil || function.Body.ListAppend.Values.Reference.Position != 0 || function.Body.ListAppend.Value.Reference == nil || function.Body.ListAppend.Value.Reference.Position != 1 || function.ReturnType.Kind != hir.TypeList || len(function.Parameters) != 2 || function.Parameters[0].Type.Kind != hir.TypeList || function.Parameters[1].Type.Kind != hir.TypeRecord {
		t.Fatalf("record-list append HIR = %#v", typed)
	}

	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	coreFunction := core.Functions[0]
	if core.LanguageContract != coreir.LanguageContractV170 || coreFunction.Body.Kind != coreir.ExprListAppend || coreFunction.Body.ListAppend == nil || coreFunction.Body.ListAppend.Values == nil || coreFunction.Body.ListAppend.Value == nil || coreFunction.Body.ListAppend.Values.Parameter == nil || *coreFunction.Body.ListAppend.Values.Parameter != 0 || coreFunction.Body.ListAppend.Value.Parameter == nil || *coreFunction.Body.ListAppend.Value.Parameter != 1 || !coreir.TypeEqual(coreFunction.ReturnType, coreFunction.Parameters[0].Type) || !coreir.TypeEqual(coreFunction.ReturnType.List.Element, coreFunction.Parameters[1].Type) {
		t.Fatalf("record-list append Core = %#v", core)
	}

	listType := coreFunction.Parameters[0].Type
	rowType := coreFunction.Parameters[1].Type
	row := recordListAppendValue(rowType, "container-1", "api", true)
	next := recordListAppendValue(rowType, "container-2", "worker", false)
	inputRows := coreeval.Value{Type: listType, List: []coreeval.Value{row}}
	outcome, err := coreeval.Evaluate(coreFunction, []coreeval.Value{inputRows, next})
	if err != nil || !outcome.OK || len(outcome.Value.List) != 2 || outcome.Value.List[0].Record[0].String != "container-1" || outcome.Value.List[1].Record[0].String != "container-2" {
		t.Fatalf("record-list append outcome = %#v (%v)", outcome, err)
	}
	inputRows.List[0].Record[0].String = "mutated"
	next.Record[0].String = "mutated"
	if outcome.Value.List[0].Record[0].String != "container-1" || outcome.Value.List[1].Record[0].String != "container-2" {
		t.Fatal("record-list append result aliases caller-owned storage")
	}
	empty, err := coreeval.Evaluate(coreFunction, []coreeval.Value{{Type: listType, List: make([]coreeval.Value, 0)}, row})
	if err != nil || !empty.OK || len(empty.Value.List) != 1 {
		t.Fatalf("empty record-list append outcome = %#v (%v)", empty, err)
	}
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{{Type: listType}, row}); err == nil {
		t.Fatal("Core evaluator accepted a nil append list")
	}
	invalidExisting := recordListAppendValue(rowType, string([]byte{0xff}), "api", true)
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{{Type: listType, List: []coreeval.Value{invalidExisting}}, row}); err == nil {
		t.Fatal("Core evaluator accepted invalid UTF-8 in an existing append element")
	}
	invalidAppended := recordListAppendValue(rowType, "container-2", string([]byte{0xff}), false)
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{{Type: listType, List: []coreeval.Value{row}}, invalidAppended}); err == nil {
		t.Fatal("Core evaluator accepted invalid UTF-8 in the appended element")
	}

	malformedHIR := typed
	malformedHIR.Functions = append([]hir.Function(nil), typed.Functions...)
	malformedHIR.Functions[0].Body = typed.Functions[0].Body
	malformedAppend := *typed.Functions[0].Body.ListAppend
	malformedAppend.Value = nil
	malformedHIR.Functions[0].Body.ListAppend = &malformedAppend
	if _, err := LowerHIRToCore(malformedHIR); err == nil {
		t.Fatal("Core lowering accepted malformed record-list append HIR")
	} else if diagnostics, ok := AsDiagnostics(err); !ok || len(diagnostics) != 1 || diagnostics[0].Code != CodeCoreLowering {
		t.Fatalf("malformed append HIR diagnostic = %#v (%v)", diagnostics, err)
	}
	priorHIR := typed
	priorHIR.LanguageContract = coreir.LanguageContractV160
	if _, err := LowerHIRToCore(priorHIR); err == nil {
		t.Fatal("v0.16.0 HIR implicitly accepted record-list append")
	}

	malformedCore := coreFunction
	malformedCore.Body = coreFunction.Body
	malformedCoreAppend := *coreFunction.Body.ListAppend
	malformedCoreAppend.Values = nil
	malformedCore.Body.ListAppend = &malformedCoreAppend
	if err := coreir.ValidateFunction(malformedCore); err == nil {
		t.Fatal("Core accepted a malformed record-list append")
	}
	priorCore := core
	priorCore.LanguageContract = coreir.LanguageContractV160
	if _, err := gobackend.Generate(priorCore); err == nil {
		t.Fatal("v0.16.0 Core implicitly accepted record-list append")
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
		t.Fatal("record-list append generated Go is nondeterministic")
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(recordListAppendGeneratedGoTest()))

	assertCompilerGolden(t, "record-list-append.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "record-list-append.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "record-list-append.go", generated)
}

func TestV170ParserPreservesRecordListAppendOperandsAndSpan(t *testing.T) {
	const source = `public Record Row { public string Id; } public Class Root { public List<Row> AppendRow(List<Row> values, Row value) => append(values, value); }`
	sources, diagnostics := NewSourceSet([]SourceInput{{Path: "root.pipe", Data: []byte(source)}})
	if diagnostics.HasErrors() {
		t.Fatal(diagnosticError(sources, diagnostics))
	}
	program, err := parseSourceFileWithLanguageContract(sources, sources.Files()[0], PipeLangLanguageContractV170)
	if err != nil {
		t.Fatal(err)
	}
	appendExpr, ok := program.Classes[0].Methods[0].Body.(*ListAppendExpr)
	if !ok {
		t.Fatalf("record-list append AST = %#v", program.Classes[0].Methods[0].Body)
	}
	values, valuesOK := appendExpr.Values.(*IdentExpr)
	value, valueOK := appendExpr.Value.(*IdentExpr)
	if !appendExpr.Span.IsValid() || !valuesOK || values.Name != "values" || !values.Span.IsValid() || !valueOK || value.Name != "value" || !value.Span.IsValid() {
		t.Fatalf("record-list append AST = %#v", program.Classes[0].Methods[0].Body)
	}
}

func TestV170RecordListAppendRejectsExcludedForms(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{name: "primitive list", src: `public Class Root { public List<string> AppendValue(List<string> values, string value) => append(values, value); }`},
		{name: "wrong return", src: `public Record Row { public string Id; } public Class Root { public Row AppendRow(List<Row> values, Row value) => append(values, value); }`},
		{name: "mismatched record", src: `public Record Row { public string Id; } public Record Other { public string Id; } public Class Root { public List<Row> AppendRow(List<Row> values, Other value) => append(values, value); }`},
		{name: "reversed parameters", src: `public Record Row { public string Id; } public Class Root { public List<Row> AppendRow(Row value, List<Row> values) => append(values, value); }`},
		{name: "computed list", src: `public Record Row { public string Id; } public Class Root { public List<Row> AppendRow(Row value) => append(empty_list<Row>(), value); }`},
		{name: "computed record", src: `public Record Row { public string Id; } public Class Root { public List<Row> AppendRow(List<Row> values, string id) => append(values, new Row { Id = id }); }`},
		{name: "reordered operands", src: `public Record Row { public string Id; } public Class Root { public List<Row> AppendRow(List<Row> values, Row value) => append(value, values); }`},
		{name: "extra parameter", src: `public Record Row { public string Id; } public Class Root { public List<Row> AppendRow(List<Row> values, Row value, bool extra) => append(values, value); }`},
		{name: "list field", src: `public Record Row { public string Id; } public Class Root { public List<Row> Values; public List<Row> AppendRow(List<Row> values, Row value) => append(values, value); }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.src)}, nil)
			input.LanguageContract = PipeLangLanguageContractV170
			analysis := AnalyzeSemanticModuleSet(input)
			if !analysis.Diagnostics.HasErrors() {
				t.Fatal("excluded v0.17.0 record-list append form was accepted")
			}
		})
	}
}

func TestRecordListAppendRequiresExplicitV170Migration(t *testing.T) {
	source := `public Record Row { public string Id; } public Class Root { public List<Row> AppendRow(List<Row> values, Row value) => append(values, value); }`
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040, PipeLangLanguageContractV050, PipeLangLanguageContractV060, PipeLangLanguageContractV070, PipeLangLanguageContractV080, PipeLangLanguageContractV090, PipeLangLanguageContractV100, PipeLangLanguageContractV110, PipeLangLanguageContractV120, PipeLangLanguageContractV130, PipeLangLanguageContractV140, PipeLangLanguageContractV150, PipeLangLanguageContractV160} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
		input.LanguageContract = contract
		analysis := AnalyzeSemanticModuleSet(input)
		if !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted record-list append", contract)
		}
	}
}

func TestV170PreservesV160RecordListCountContract(t *testing.T) {
	source, err := os.ReadFile("testdata/record-list-count.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-list-count.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV170
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "CountRows").Identity)
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

func recordListAppendValue(rowType coreir.Type, id, name string, running bool) coreeval.Value {
	return coreeval.Value{Type: rowType, Record: []coreeval.Value{
		{Type: rowType.Record.Fields[0].Type, String: id},
		{Type: rowType.Record.Fields[1].Type, String: name},
		{Type: rowType.Record.Fields[2].Type, Bool: running},
	}}
}

func recordListAppendGeneratedGoTest() string {
	return fmt.Sprintf(`package %s

import "testing"

func expectAppendPanic(t *testing.T, invoke func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	invoke()
}

func TestGeneratedRecordListAppend(t *testing.T) {
	rows := []PipeLangRecordTestPackageAppRootContainerrow{
		{Id: "container-1", Name: "api", Running: true},
	}
	value := PipeLangRecordTestPackageAppRootContainerrow{Id: "container-2", Name: "worker"}
	result := PipeLangAppendRow(rows, value)
	if len(result) != 2 || result[0].Id != "container-1" || result[1].Id != "container-2" {
		t.Fatalf("append result = %%#v", result)
	}
	rows[0].Id = "mutated"
	value.Id = "mutated"
	if result[0].Id != "container-1" || result[1].Id != "container-2" {
		t.Fatal("append result aliases caller-owned storage")
	}
	if empty := PipeLangAppendRow(make([]PipeLangRecordTestPackageAppRootContainerrow, 0), PipeLangRecordTestPackageAppRootContainerrow{Id: "only", Name: "one"}); len(empty) != 1 || empty[0].Id != "only" {
		t.Fatalf("empty append result = %%#v", empty)
	}
	invalidExisting := []PipeLangRecordTestPackageAppRootContainerrow{{Id: string([]byte{0xff}), Name: "bad"}}
	expectAppendPanic(t, func() { PipeLangAppendRow(invalidExisting, PipeLangRecordTestPackageAppRootContainerrow{Id: "ok", Name: "ok"}) })
	invalidAppended := PipeLangRecordTestPackageAppRootContainerrow{Id: "bad", Name: string([]byte{0xff})}
	expectAppendPanic(t, func() { PipeLangAppendRow(make([]PipeLangRecordTestPackageAppRootContainerrow, 0), invalidAppended) })
	var nilRows []PipeLangRecordTestPackageAppRootContainerrow
	expectAppendPanic(t, func() { PipeLangAppendRow(nilRows, PipeLangRecordTestPackageAppRootContainerrow{Id: "ok", Name: "ok"}) })
}
`, gobackend.PackageName)
}
