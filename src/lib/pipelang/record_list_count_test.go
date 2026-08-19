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

func TestV160RecordListCountHIRCoreAndGo(t *testing.T) {
	source, err := os.ReadFile("testdata/record-list-count.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-list-count.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV160
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}

	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	method := projectedMemberNamed(t, projectedTypeNamed(t, projection, "Root"), "CountRows")
	if projection.LanguageContract != PipeLangLanguageContractV160 || projection.Schema != PipeLangSemanticProjectionVersion || method.Type.Kind != TypeRefPrimitive || method.Type.Primitive != TypeInt || len(method.Parameters) != 1 || method.Parameters[0].Type.Identity == nil || method.Parameters[0].Type.Identity.PackageID != PipeLangBuiltinPackageID || method.Parameters[0].Type.Identity.Path != PipeLangListSemanticPath || len(method.Parameters[0].Type.Arguments) != 1 || method.Parameters[0].Type.Arguments[0].Identity == nil || method.Parameters[0].Type.Arguments[0].Identity.Path != "app.root.containerrow" {
		t.Fatalf("record-list count projection = %#v / %#v", projection, method)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "CountRows").Identity)
	if err != nil {
		t.Fatal(err)
	}
	function := typed.Functions[0]
	if typed.LanguageContract != coreir.LanguageContractV160 || function.Body.Kind != hir.ExprListCount || function.Body.ListCount == nil || function.Body.ListCount.Value == nil || function.Body.ListCount.Value.Reference == nil || function.Body.ListCount.Value.Reference.Position != 0 || function.ReturnType.Kind != hir.TypeNumeric || function.Parameters[0].Type.Kind != hir.TypeList {
		t.Fatalf("record-list count HIR = %#v", typed)
	}

	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	coreFunction := core.Functions[0]
	if core.LanguageContract != coreir.LanguageContractV160 || coreFunction.Body.Kind != coreir.ExprListCount || coreFunction.Body.ListCount == nil || coreFunction.Body.ListCount.Value == nil || coreFunction.Body.ListCount.Value.Parameter == nil || *coreFunction.Body.ListCount.Value.Parameter != 0 || !coreir.TypeEqual(coreFunction.ReturnType, coreir.SignedInteger(64)) {
		t.Fatalf("record-list count Core = %#v", core)
	}

	listType := coreFunction.Parameters[0].Type
	rowType := listType.List.Element
	row := coreeval.Value{Type: rowType, Record: []coreeval.Value{
		{Type: rowType.Record.Fields[0].Type, String: "container-1"},
		{Type: rowType.Record.Fields[1].Type, String: "api"},
		{Type: rowType.Record.Fields[2].Type, Bool: true},
	}}
	for name, value := range map[string]coreeval.Value{
		"empty": {Type: listType, List: make([]coreeval.Value, 0)},
		"two":   {Type: listType, List: []coreeval.Value{row, row}},
	} {
		outcome, err := coreeval.Evaluate(coreFunction, []coreeval.Value{value})
		if err != nil || !outcome.OK || outcome.Value.Int != int64(len(value.List)) {
			t.Fatalf("%s count outcome = %#v (%v)", name, outcome, err)
		}
	}
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{{Type: listType}}); err == nil {
		t.Fatal("Core evaluator accepted a nil record-list count operand")
	}
	invalidRow := row
	invalidRow.Record = append([]coreeval.Value(nil), row.Record...)
	invalidRow.Record[0].String = string([]byte{0xff})
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{{Type: listType, List: []coreeval.Value{invalidRow}}}); err == nil {
		t.Fatal("Core evaluator accepted invalid UTF-8 in a counted list element")
	}

	malformedHIR := typed
	malformedHIR.Functions = append([]hir.Function(nil), typed.Functions...)
	malformedHIR.Functions[0].Body = typed.Functions[0].Body
	malformedCount := *typed.Functions[0].Body.ListCount
	malformedCount.Value = nil
	malformedHIR.Functions[0].Body.ListCount = &malformedCount
	if _, err := LowerHIRToCore(malformedHIR); err == nil {
		t.Fatal("Core lowering accepted malformed record-list count HIR")
	} else if diagnostics, ok := AsDiagnostics(err); !ok || len(diagnostics) != 1 || diagnostics[0].Code != CodeCoreLowering {
		t.Fatalf("malformed count HIR diagnostic = %#v (%v)", diagnostics, err)
	}
	priorHIR := typed
	priorHIR.LanguageContract = coreir.LanguageContractV150
	if _, err := LowerHIRToCore(priorHIR); err == nil {
		t.Fatal("v0.15.0 HIR implicitly accepted record-list count")
	}

	malformedCore := coreFunction
	malformedCore.Body = coreFunction.Body
	malformedCoreCount := *coreFunction.Body.ListCount
	malformedCoreCount.Value = nil
	malformedCore.Body.ListCount = &malformedCoreCount
	if err := coreir.ValidateFunction(malformedCore); err == nil {
		t.Fatal("Core accepted a malformed record-list count")
	}
	priorCore := core
	priorCore.LanguageContract = coreir.LanguageContractV150
	if _, err := gobackend.Generate(priorCore); err == nil {
		t.Fatal("v0.15.0 Core implicitly accepted record-list count")
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
		t.Fatal("record-list count generated Go is nondeterministic")
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(recordListCountGeneratedGoTest()))

	assertCompilerGolden(t, "record-list-count.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "record-list-count.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "record-list-count.go", generated)
}

func TestV160ParserPreservesRecordListCountOperandAndSpan(t *testing.T) {
	const source = `public Record Row { public string Id; } public Class Root { public int CountRows(List<Row> values) => count(values); }`
	sources, diagnostics := NewSourceSet([]SourceInput{{Path: "root.pipe", Data: []byte(source)}})
	if diagnostics.HasErrors() {
		t.Fatal(diagnosticError(sources, diagnostics))
	}
	program, err := parseSourceFileWithLanguageContract(sources, sources.Files()[0], PipeLangLanguageContractV160)
	if err != nil {
		t.Fatal(err)
	}
	count, ok := program.Classes[0].Methods[0].Body.(*ListCountExpr)
	if !ok {
		t.Fatalf("record-list count AST = %#v", program.Classes[0].Methods[0].Body)
	}
	value, valueOK := count.Value.(*IdentExpr)
	if !count.Span.IsValid() || !valueOK || value.Name != "values" || !value.Span.IsValid() {
		t.Fatalf("record-list count AST = %#v", program.Classes[0].Methods[0].Body)
	}
}

func TestV160RecordListCountRejectsExcludedForms(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{name: "primitive list", src: `public Class Root { public int CountRows(List<string> values) => count(values); }`},
		{name: "wrong return", src: `public Record Row { public string Id; } public Class Root { public bool CountRows(List<Row> values) => count(values); }`},
		{name: "computed operand", src: `public Record Row { public string Id; } public Class Root { public int CountRows() => count(empty_list<Row>()); }`},
		{name: "non-complete body", src: `public Record Row { public string Id; } public Class Root { public int CountRows(List<Row> values) => count(values) + 1; }`},
		{name: "list field", src: `public Record Row { public string Id; } public Class Root { public List<Row> Values; public int CountRows(List<Row> values) => count(values); }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.src)}, nil)
			input.LanguageContract = PipeLangLanguageContractV160
			analysis := AnalyzeSemanticModuleSet(input)
			if !analysis.Diagnostics.HasErrors() {
				t.Fatal("excluded v0.16.0 record-list count form was accepted")
			}
		})
	}
}

func TestRecordListCountRequiresExplicitV160Migration(t *testing.T) {
	source := `public Record Row { public string Id; } public Class Root { public int CountRows(List<Row> values) => count(values); }`
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040, PipeLangLanguageContractV050, PipeLangLanguageContractV060, PipeLangLanguageContractV070, PipeLangLanguageContractV080, PipeLangLanguageContractV090, PipeLangLanguageContractV100, PipeLangLanguageContractV110, PipeLangLanguageContractV120, PipeLangLanguageContractV130, PipeLangLanguageContractV140, PipeLangLanguageContractV150} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
		input.LanguageContract = contract
		analysis := AnalyzeSemanticModuleSet(input)
		if !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted record-list count", contract)
		}
	}
}

func TestV160PreservesV150RecordListContract(t *testing.T) {
	source, err := os.ReadFile("testdata/record-list.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-list.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV160
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"MakeRow", "EmptyRows", "OneRow", "ForwardRows"} {
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

func recordListCountGeneratedGoTest() string {
	return fmt.Sprintf(`package %s

import "testing"

func expectCountPanic(t *testing.T, invoke func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	invoke()
}

func TestGeneratedRecordListCount(t *testing.T) {
	rows := []PipeLangRecordTestPackageAppRootContainerrow{
		{Id: "container-1", Name: "api", Running: true},
		{Id: "container-2", Name: "worker", Running: false},
	}
	if count := PipeLangCountRows(rows); count != 2 {
		t.Fatalf("count = %%d", count)
	}
	if count := PipeLangCountRows(make([]PipeLangRecordTestPackageAppRootContainerrow, 0)); count != 0 {
		t.Fatalf("empty count = %%d", count)
	}
	invalid := append([]PipeLangRecordTestPackageAppRootContainerrow(nil), rows...)
	invalid[0].Id = string([]byte{0xff})
	expectCountPanic(t, func() { PipeLangCountRows(invalid) })
	var nilRows []PipeLangRecordTestPackageAppRootContainerrow
	expectCountPanic(t, func() { PipeLangCountRows(nilRows) })
}
`, gobackend.PackageName)
}
