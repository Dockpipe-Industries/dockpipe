package pipelang

import (
	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
	"testing"
)

func TestV320RecordListSortByDirectionsPipeline(t *testing.T) {
	source := `public Record Row { public string Id; public string State; public string Name; } public Class Root { public List<Row> Sort(List<Row> values) => sort_by_ordinal(values, Row.State, descending, Row.Name, ascending); }`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV320
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	identity := semanticMethodNamed(t, analysis, "Sort").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	if typed.Functions[0].Body.Kind != hir.ExprListSortByOrdinalDirections {
		t.Fatalf("HIR = %#v", typed.Functions[0].Body)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if core.Functions[0].Body.Kind != coreir.ExprListSortByOrdinalDirections {
		t.Fatalf("Core = %#v", core.Functions[0].Body)
	}
	listType := core.Functions[0].Parameters[0].Type
	row := listType.List.Element
	rows := coreeval.Value{Type: listType, List: []coreeval.Value{coreeval.Value{Type: row, Record: []coreeval.Value{{Type: row.Record.Fields[0].Type, String: "one"}, {Type: row.Record.Fields[1].Type, String: "running"}, {Type: row.Record.Fields[2].Type, String: "beta"}}}, coreeval.Value{Type: row, Record: []coreeval.Value{{Type: row.Record.Fields[0].Type, String: "two"}, {Type: row.Record.Fields[1].Type, String: "exited"}, {Type: row.Record.Fields[2].Type, String: "zeta"}}}, coreeval.Value{Type: row, Record: []coreeval.Value{{Type: row.Record.Fields[0].Type, String: "three"}, {Type: row.Record.Fields[1].Type, String: "running"}, {Type: row.Record.Fields[2].Type, String: "Alpha"}}}}}
	outcome, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{rows})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"three", "one", "two"}
	for i, id := range want {
		if outcome.Value.List[i].Record[0].String != id {
			t.Fatalf("row %d = %#v", i, outcome.Value.List[i])
		}
	}
	if _, err := gobackend.Generate(core); err != nil {
		t.Fatal(err)
	}
}

func TestV320SortDirectionsAreContextualAndRequired(t *testing.T) {
	for _, source := range []string{
		`public Record Row { public string Name; } public Class Root { public List<Row> Sort(List<Row> values) => sort_by_ordinal(values, Row.Name, sideways); }`,
	} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
		input.LanguageContract = PipeLangLanguageContractV320
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatalf("accepted %s", source)
		}
	}
	prior := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", `public Record Row { public string Name; } public Class Root { public List<Row> Sort(List<Row> values) => sort_by_ordinal(values, Row.Name, descending); }`)}, nil)
	prior.LanguageContract = PipeLangLanguageContractV310
	if AnalyzeSemanticModuleSet(prior).Error() == nil {
		t.Fatal("v0.31 accepted directions")
	}
}

func TestLaterContractsPreserveLegacyAndDirectionalOrdinalSortSpellings(t *testing.T) {
	contracts := []LanguageContract{PipeLangLanguageContractV330, PipeLangLanguageContractV340, PipeLangLanguageContractV350, PipeLangLanguageContractV360}
	legacySources := []struct {
		source string
		kind   hir.ExprKind
	}{
		{`public Record Row { public string Name; } public Class Root { public List<Row> Sort(List<Row> values) => sort_by_ordinal(values, Row.Name); }`, hir.ExprListSortByOrdinalText},
		{`public Record Row { public string State; public string Name; } public Class Root { public List<Row> Sort(List<Row> values) => sort_by_ordinal(values, Row.State, Row.Name); }`, hir.ExprListSortByOrdinalTexts},
	}
	for _, contract := range contracts {
		for _, fixture := range legacySources {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", fixture.source)}, nil)
			input.LanguageContract = contract
			analysis := AnalyzeSemanticModuleSet(input)
			if err := analysis.Error(); err != nil {
				t.Fatalf("%s legacy: %v", contract, err)
			}
			typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Sort").Identity)
			if err != nil {
				t.Fatal(err)
			}
			if typed.Functions[0].Body.Kind != fixture.kind {
				t.Fatalf("%s legacy kind = %s", contract, typed.Functions[0].Body.Kind)
			}
		}
		source := `public Record Row { public string State; public string Name; } public Class Root { public List<Row> Sort(List<Row> values) => sort_by_ordinal(values, Row.State, descending, Row.Name, ascending); }`
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
		input.LanguageContract = contract
		analysis := AnalyzeSemanticModuleSet(input)
		if err := analysis.Error(); err != nil {
			t.Fatalf("%s directional: %v", contract, err)
		}
		typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Sort").Identity)
		if err != nil {
			t.Fatal(err)
		}
		if typed.Functions[0].Body.Kind != hir.ExprListSortByOrdinalDirections {
			t.Fatalf("%s directional kind = %s", contract, typed.Functions[0].Body.Kind)
		}
	}
}

func TestLaterContractsRejectMixedOrdinalSortForms(t *testing.T) {
	for _, source := range []string{
		`public Record Row { public string State; public string Name; } public Class Root { public List<Row> Sort(List<Row> values) => sort_by_ordinal(values, Row.State, descending, Row.Name); }`,
		`public Record Row { public string State; public string Name; } public Class Root { public List<Row> Sort(List<Row> values) => sort_by_ordinal(values, Row.State, Row.Name, ascending); }`,
	} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
		input.LanguageContract = PipeLangLanguageContractV350
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatalf("accepted mixed form: %s", source)
		}
	}
}
