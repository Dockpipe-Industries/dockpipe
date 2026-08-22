package pipelang

import (
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
)

const safeIndexSource = `public Record Row { public string Id; } public Class Root { public Optional<Row> RowAt(List<Row> values, int index) => values[index]; }`

func TestLaterContractsPreserveDirectionalSortAndSafeIndexing(t *testing.T) {
	for _, contract := range []LanguageContract{PipeLangLanguageContractV350, PipeLangLanguageContractV360, PipeLangLanguageContractV370, PipeLangLanguageContractV380} {
		for name, source := range map[string]string{
			"directional sort": `public Record Row { public string Name; } public Class Root { public List<Row> Sort(List<Row> values) => sort_by_ordinal(values, Row.Name, descending); }`,
			"safe indexing":    safeIndexSource,
		} {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
			input.LanguageContract = contract
			if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
				t.Fatalf("%s rejected %s accepted by an earlier contract: %v", contract, name, err)
			}
		}
	}
}

func TestV330SafeGeneralIndexingReusesListAtPipeline(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", safeIndexSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV330
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	method := projectedMemberNamed(t, projectedTypeNamed(t, projection, "Root"), "RowAt")
	if projection.LanguageContract != PipeLangLanguageContractV330 || len(method.Parameters) != 2 || method.Type.Identity == nil || method.Type.Identity.Path != PipeLangOptionalSemanticPath {
		t.Fatalf("projection = %#v", method)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "RowAt").Identity)
	if err != nil {
		t.Fatal(err)
	}
	if typed.LanguageContract != coreir.LanguageContractV330 || typed.Functions[0].Body.Kind != hir.ExprListAt {
		t.Fatalf("HIR = %#v", typed)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if core.LanguageContract != coreir.LanguageContractV330 || core.Functions[0].Body.Kind != coreir.ExprListAt {
		t.Fatalf("Core = %#v", core)
	}
	listType := core.Functions[0].Parameters[0].Type
	rowType := listType.List.Element
	row := coreeval.Value{Type: rowType, Record: []coreeval.Value{{Type: rowType.Record.Fields[0].Type, String: "one"}}}
	rows := coreeval.Value{Type: listType, List: []coreeval.Value{row}}
	for _, tc := range []struct {
		index   int64
		present bool
	}{{0, true}, {-1, false}, {1, false}} {
		got, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{rows, {Type: coreir.SignedInteger(64), Int: tc.index}})
		if err != nil || !got.OK || got.Value.Optional == nil || got.Value.Optional.Present != tc.present {
			t.Fatalf("index %d = %#v, %v", tc.index, got, err)
		}
	}
	if _, err := gobackend.Generate(core); err != nil {
		t.Fatal(err)
	}
}

func TestV330SafeGeneralIndexingSyntaxAndBounds(t *testing.T) {
	sources, diagnostics := NewSourceSet([]SourceInput{{Path: "root.pipe", Data: []byte(safeIndexSource)}})
	if diagnostics.HasErrors() {
		t.Fatal(diagnosticError(sources, diagnostics))
	}
	program, err := parseSourceFileWithLanguageContract(sources, sources.Files()[0], PipeLangLanguageContractV330)
	if err != nil {
		t.Fatal(err)
	}
	at, ok := program.Classes[0].Methods[0].Body.(*ListAtExpr)
	if !ok || !at.Postfix || !at.Span.IsValid() {
		t.Fatalf("AST = %#v", program.Classes[0].Methods[0].Body)
	}
	for _, tc := range []struct{ name, source string }{
		{"primitive receiver", `public Record Row { public string Id; } public Class Root { public Optional<Row> RowAt(List<string> values, int index) => values[index]; }`},
		{"float index", `public Record Row { public string Id; } public Class Root { public Optional<Row> RowAt(List<Row> values, float index) => values[index]; }`},
		{"computed index", `public Record Row { public string Id; } public Class Root { public Optional<Row> RowAt(List<Row> values, int index) => values[index + 1]; }`},
		{"nested use", `public Record Row { public string Id; } public Class Root { public bool Has(List<Row> values, int index) => has_value(values[index]); }`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", tc.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV330
			if a := AnalyzeSemanticModuleSet(input); !a.Diagnostics.HasErrors() {
				t.Fatal("excluded indexing form accepted")
			}
		})
	}
	prior := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", safeIndexSource)}, nil)
	prior.LanguageContract = PipeLangLanguageContractV320
	if a := AnalyzeSemanticModuleSet(prior); !a.Diagnostics.HasErrors() {
		t.Fatal("v0.32.0 accepted postfix indexing")
	}
	legacy := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", `public Record Row { public string Id; } public Class Root { public Optional<Row> RowAt(List<Row> values, int index) => at(values, index); }`)}, nil)
	legacy.LanguageContract = PipeLangLanguageContractV330
	if err := AnalyzeSemanticModuleSet(legacy).Error(); err != nil {
		t.Fatalf("v0.33.0 broke at compatibility: %v", err)
	}
}
