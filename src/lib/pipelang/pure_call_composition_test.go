package pipelang

import (
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
)

func TestV370GeneralPureCallCompositionPipeline(t *testing.T) {
	source := `public Record Row { public string Id; }
	public Class Root {
		public string Normalize(string value) => trim(value);
		public string Selected(Optional<string> value) => match(value){ some(item) => Normalize(item), none => "" };
		public bool Search(string value, string query) => contains_casefolded(Normalize(value), Normalize(query));
		public string Join(string left, string right) => Normalize(left) + Normalize(right);
		public bool Before(string left, string right) => Normalize(left) < Normalize(right);
		public Row Copy(Row row) => row;
		public Optional<Row> SomeCopy(Row row) => some(Copy(row));
		public Row Build(string id) => new Row { Id = Normalize(id) };
	}`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "composition.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV370
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}

	selectedIdentity := semanticMethodNamed(t, analysis, "Selected").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, selectedIdentity)
	if err != nil {
		t.Fatal(err)
	}
	selectedHIR := hirFunctionNamed(t, typed, "Selected")
	if typed.LanguageContract != coreir.LanguageContractV370 || selectedHIR.Body.Kind != hir.ExprMatch || selectedHIR.Body.Match.Arms[0].Body.Kind != hir.ExprCall {
		t.Fatalf("Selected HIR = %#v", typed)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	selectedCore := coreFunctionNamed(t, core, "Selected")
	optionalType := selectedCore.Parameters[0].Type
	payload := coreeval.Value{Type: optionalType.Optional.Value, String: "  api  "}
	present := coreeval.Value{Type: optionalType, Optional: &coreeval.OptionalValue{Present: true, Value: &payload}}
	outcome, err := coreeval.EvaluateProgram(core, coreir.SemanticIdentity{PackageID: string(selectedIdentity.PackageID), Path: string(selectedIdentity.Path)}, []coreeval.Value{present})
	if err != nil || !outcome.OK || outcome.Value.String != "api" {
		t.Fatalf("Selected evaluation = %#v, %v", outcome, err)
	}
	absent := coreeval.Value{Type: optionalType, Optional: &coreeval.OptionalValue{}}
	outcome, err = coreeval.EvaluateProgram(core, coreir.SemanticIdentity{PackageID: string(selectedIdentity.PackageID), Path: string(selectedIdentity.Path)}, []coreeval.Value{absent})
	if err != nil || !outcome.OK || outcome.Value.String != "" {
		t.Fatalf("Selected absence = %#v, %v", outcome, err)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "return PipeLangNormalize(") {
		t.Fatalf("generated Go lacks call inside match arm:\n%s", generated)
	}

	searchIdentity := semanticMethodNamed(t, analysis, "Search").Identity
	searchHIR, err := LowerSemanticMethodToHIR(analysis, searchIdentity)
	if err != nil {
		t.Fatal(err)
	}
	search := hirFunctionNamed(t, searchHIR, "Search")
	if search.Body.Kind != hir.ExprTextContainsCaseFolded || search.Body.TextContains.Value.Kind != hir.ExprCall || search.Body.TextContains.Query.Kind != hir.ExprCall {
		t.Fatalf("Search HIR = %#v", search.Body)
	}
	searchCore, err := LowerHIRToCore(searchHIR)
	if err != nil {
		t.Fatal(err)
	}
	searchOutcome, err := coreeval.EvaluateProgram(searchCore, coreir.SemanticIdentity{PackageID: string(searchIdentity.PackageID), Path: string(searchIdentity.Path)}, []coreeval.Value{
		{Type: coreFunctionNamed(t, searchCore, "Search").Parameters[0].Type, String: "  DockPipe  "},
		{Type: coreFunctionNamed(t, searchCore, "Search").Parameters[1].Type, String: "dockpipe"},
	})
	if err != nil || !searchOutcome.OK || !searchOutcome.Value.Bool {
		t.Fatalf("Search evaluation = %#v, %v", searchOutcome, err)
	}

	beforeIdentity := semanticMethodNamed(t, analysis, "Before").Identity
	beforeHIR, err := LowerSemanticMethodToHIR(analysis, beforeIdentity)
	if err != nil {
		t.Fatal(err)
	}
	beforeCore, err := LowerHIRToCore(beforeHIR)
	if err != nil {
		t.Fatal(err)
	}
	beforeFunction := coreFunctionNamed(t, beforeCore, "Before")
	beforeOutcome, err := coreeval.EvaluateProgram(beforeCore, coreir.SemanticIdentity{PackageID: string(beforeIdentity.PackageID), Path: string(beforeIdentity.Path)}, []coreeval.Value{
		{Type: beforeFunction.Parameters[0].Type, String: " beta "},
		{Type: beforeFunction.Parameters[1].Type, String: "gamma"},
	})
	if err != nil || !beforeOutcome.OK || !beforeOutcome.Value.Bool {
		t.Fatalf("Before evaluation = %#v, %v", beforeOutcome, err)
	}

	someCopyIdentity := semanticMethodNamed(t, analysis, "SomeCopy").Identity
	someCopyHIR, err := LowerSemanticMethodToHIR(analysis, someCopyIdentity)
	if err != nil {
		t.Fatal(err)
	}
	someCopy := hirFunctionNamed(t, someCopyHIR, "SomeCopy")
	if someCopy.Body.Kind != hir.ExprOptionalSome || someCopy.Body.Some.Value.Kind != hir.ExprCall {
		t.Fatalf("SomeCopy HIR = %#v", someCopy.Body)
	}
	someCopyCore, err := LowerHIRToCore(someCopyHIR)
	if err != nil {
		t.Fatal(err)
	}
	someCopyFunction := coreFunctionNamed(t, someCopyCore, "SomeCopy")
	rowType := someCopyFunction.Parameters[0].Type
	row := coreeval.Value{Type: rowType, Record: []coreeval.Value{{Type: rowType.Record.Fields[0].Type, String: "row-1"}}}
	copyOutcome, err := coreeval.EvaluateProgram(someCopyCore, coreir.SemanticIdentity{PackageID: string(someCopyIdentity.PackageID), Path: string(someCopyIdentity.Path)}, []coreeval.Value{row})
	if err != nil || !copyOutcome.OK || copyOutcome.Value.Optional == nil || copyOutcome.Value.Optional.Value == nil || copyOutcome.Value.Optional.Value.Record[0].String != "row-1" {
		t.Fatalf("SomeCopy evaluation = %#v, %v", copyOutcome, err)
	}
	row.Record[0].String = "mutated"
	if copyOutcome.Value.Optional.Value.Record[0].String != "row-1" {
		t.Fatal("composed call result aliases caller-owned record storage")
	}

	buildIdentity := semanticMethodNamed(t, analysis, "Build").Identity
	buildHIR, err := LowerSemanticMethodToHIR(analysis, buildIdentity)
	if err != nil {
		t.Fatal(err)
	}
	build := hirFunctionNamed(t, buildHIR, "Build")
	if build.Body.Kind != hir.ExprRecordConstruct || build.Body.Record.Fields[0].Value.Kind != hir.ExprCall {
		t.Fatalf("Build HIR = %#v", build.Body)
	}
}

func TestV370CompositionRetainsDirectControlFlowCarriers(t *testing.T) {
	cases := []string{
		`public Class Root { public Optional<string> Forward(Optional<string> value) => value; public string Read(Optional<string> value) => match(Forward(value)){ some(item) => item, none => "" }; }`,
		`public Class Root { public Optional<string> Forward(Optional<string> value) => value; public Optional<string> Read(Optional<string> value) => some(propagate(Forward(value))); }`,
	}
	for _, source := range cases {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "bad-composition.pipe", source)}, nil)
		input.LanguageContract = PipeLangLanguageContractV370
		analysis := AnalyzeSemanticModuleSet(input)
		if len(analysis.Diagnostics) != 1 || analysis.Diagnostics[0].Code != CodeExpressionType || !strings.Contains(analysis.Diagnostics[0].Message, "direct match and propagate carriers") {
			t.Fatalf("diagnostics = %#v", analysis.Diagnostics)
		}
	}
}

func TestV370ComposedRecordConstructionRetainsExistingShape(t *testing.T) {
	source := `public Record Row { public string Id; public string Name; }
	public Class Root {
		public string Normalize(string value) => trim(value);
		public Row Build(string id, string name) => new Row { Name = Normalize(name), Id = id };
	}`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "bad-record-composition.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV370
	analysis := AnalyzeSemanticModuleSet(input)
	if len(analysis.Diagnostics) != 1 || analysis.Diagnostics[0].Code != CodeInvalidType || !strings.Contains(analysis.Diagnostics[0].Message, "declaration order") {
		t.Fatalf("diagnostics = %#v", analysis.Diagnostics)
	}
}

func TestV370CompositionDoesNotMigrateV360(t *testing.T) {
	source := `public Class Root { public string Normalize(string value) => trim(value); public string Read(string value) => trim(Normalize(value)); }`
	for _, contract := range []LanguageContract{PipeLangLanguageContractV360, PipeLangLanguageContractV370} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "versioned-composition.pipe", source)}, nil)
		input.LanguageContract = contract
		analysis := AnalyzeSemanticModuleSet(input)
		if contract == PipeLangLanguageContractV360 {
			if len(analysis.Diagnostics) != 1 || analysis.Diagnostics[0].Code != CodeExpressionType {
				t.Fatalf("v0.36.0 diagnostics = %#v", analysis.Diagnostics)
			}
			continue
		}
		if err := analysis.Error(); err != nil {
			t.Fatal(err)
		}
		program, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Read").Identity)
		if err != nil {
			t.Fatal(err)
		}
		core, err := LowerHIRToCore(program)
		if err != nil {
			t.Fatal(err)
		}
		core.LanguageContract = coreir.LanguageContractV360
		if err := coreir.ValidateProgram(core); err == nil {
			t.Fatal("v0.36.0 Core accepted v0.37.0 call placement")
		}
	}
}

func TestV370CoreFindsCycleBelowExistingExpression(t *testing.T) {
	source := `public Class Root { public string Identity(string value) => value; public string Read(string value) => trim(Identity(value)); }`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "nested-cycle.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV370
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Read").Identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	identity := coreFunctionNamed(t, program, "Identity")
	read := coreFunctionNamed(t, program, "Read")
	position := 0
	nested := coreir.Expr{Kind: coreir.ExprCall, Type: identity.ReturnType, Call: &coreir.Call{
		Target: read.Identity, TargetName: read.Name,
		Arguments: []*coreir.Expr{{Kind: coreir.ExprReference, Type: identity.Parameters[0].Type, Parameter: &position}},
	}}
	identity.Body = coreir.Expr{Kind: coreir.ExprTextTrim, Type: identity.ReturnType, TextTrim: &coreir.TextTrim{Value: &nested}}
	for index := range program.Functions {
		if program.Functions[index].Name == identity.Name {
			program.Functions[index] = identity
		}
	}
	if err := coreir.ValidateProgram(program); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("nested Core cycle error = %v", err)
	}
}
