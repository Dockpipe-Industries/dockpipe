package pipelang

import (
	"fmt"
	"os"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

func reviewCore(t *testing.T, contract LanguageContract, source, method string) coreir.Program {
	t.Helper()
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "review.pipe", source)}, nil)
	input.LanguageContract = contract
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, method).Identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	return program
}

func reviewCompileGenerated(t *testing.T, program coreir.Program) {
	t.Helper()
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte("package "+gobackend.PackageName+"\n"))
}

func TestReviewGeneratedDirectionalSortCompiles(t *testing.T) {
	program := reviewCore(t, PipeLangLanguageContractV320, `public Record Row { public string Id; public string State; public string Name; } public Class Root { public List<Row> Sort(List<Row> values) => sort_by_ordinal(values, Row.State, descending, Row.Name, ascending); }`, "Sort")
	reviewCompileGenerated(t, program)
}

func TestReviewGeneratedOptionalPropagationCompiles(t *testing.T) {
	for _, contract := range []LanguageContract{PipeLangLanguageContractV340, PipeLangLanguageContractV350, PipeLangLanguageContractV360, PipeLangLanguageContractV370} {
		t.Run(string(contract), func(t *testing.T) {
			program := reviewCore(t, contract, `public Class Root { public Optional<string> Forward(Optional<string> value) => some(propagate(value)); }`, "Forward")
			reviewCompileGenerated(t, program)
		})
	}
}

func TestReviewGeneratedSafeIndexCompilesAcrossContracts(t *testing.T) {
	for _, contract := range []LanguageContract{PipeLangLanguageContractV330, PipeLangLanguageContractV340, PipeLangLanguageContractV350, PipeLangLanguageContractV360, PipeLangLanguageContractV370} {
		t.Run(string(contract), func(t *testing.T) {
			program := reviewCore(t, contract, `public Record Row { public string Id; } public Class Root { public Optional<Row> RowAt(List<Row> values, int index) => values[index]; }`, "RowAt")
			reviewCompileGenerated(t, program)
		})
	}
}

func TestReviewLegacyAtCompilesAcrossLaterContracts(t *testing.T) {
	for _, contract := range []LanguageContract{PipeLangLanguageContractV330, PipeLangLanguageContractV340, PipeLangLanguageContractV350, PipeLangLanguageContractV360, PipeLangLanguageContractV370} {
		t.Run(string(contract), func(t *testing.T) {
			program := reviewCore(t, contract, `public Record Row { public string Id; } public Class Root { public Optional<Row> RowAt(List<Row> values, int index) => at(values, index); }`, "RowAt")
			reviewCompileGenerated(t, program)
		})
	}
}

func TestReviewLaterContractsPreserveNamedPredicate(t *testing.T) {
	source, err := os.ReadFile("testdata/record-list-filter-predicate.pipe")
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []LanguageContract{PipeLangLanguageContractV320, PipeLangLanguageContractV330, PipeLangLanguageContractV340, PipeLangLanguageContractV350, PipeLangLanguageContractV360, PipeLangLanguageContractV370} {
		t.Run(string(contract), func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-list-filter-predicate.pipe", string(source))}, nil)
			input.LanguageContract = contract
			analysis := AnalyzeSemanticModuleSet(input)
			if err := analysis.Error(); err != nil {
				t.Fatalf("later contract rejected v0.31 source: %v", err)
			}
			if _, err := BuildSemanticProjection(analysis); err != nil {
				t.Fatal(err)
			}
			typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Search").Identity)
			if err != nil {
				t.Fatal(err)
			}
			program, err := LowerHIRToCore(typed)
			if err != nil {
				t.Fatal(err)
			}
			if err := coreir.ValidateProgram(program); err != nil {
				t.Fatal(err)
			}
			listType := program.Functions[1].Parameters[0].Type
			rowType := listType.List.Element
			rows := coreeval.Value{Type: listType, List: []coreeval.Value{v310PredicateRow(rowType, "one", "nginx", "running")}}
			query := coreeval.Value{Type: program.Functions[1].Parameters[1].Type, String: " running "}
			outcome, err := coreeval.EvaluateProgram(program, program.Functions[1].Identity, []coreeval.Value{rows, query})
			if err != nil || len(outcome.Value.List) != 1 || outcome.Value.List[0].Record[0].String != "one" {
				t.Fatalf("compatibility evaluator outcome = %#v (%v)", outcome, err)
			}
			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			compileAndRunGeneratedGoFiles(t, generated, []byte(v310PredicateGeneratedGoTest()))
		})
	}
}

func TestReviewV350AdmitsFloatArithmeticResultMatch(t *testing.T) {
	program := reviewCore(t, PipeLangLanguageContractV350, `public Class Root { public string Read(Result<float, ArithmeticError> value) => match(value){ ok(item) => "ok", err(problem) => "error" }; }`, "Read")
	reviewCompileGenerated(t, program)
}

func TestReviewGeneratedArithmeticMatchValidatesCompleteCarrier(t *testing.T) {
	program := reviewCore(t, PipeLangLanguageContractV350, `public Class Root { public string Read(Result<int, ArithmeticError> value) => match(value){ ok(item) => "ok", err(problem) => "error" }; }`, "Read")
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	testSource := fmt.Sprintf(`package %s

import "testing"

func TestGenerated(t *testing.T) {
	defer func() {
		if recover() == nil { t.Fatal("generated arithmetic match accepted malformed carrier") }
	}()
	%s(PipeLangArithmeticResult[int64]{OK: true, Error: PipeLangArithmeticOverflow})
}
`, gobackend.PackageName, gobackend.FunctionName(program.Functions[0]))
	compileAndRunGeneratedGoFiles(t, generated, []byte(testSource))
}

func TestReviewCoreRejectsMalformedMatches(t *testing.T) {
	valid := reviewCore(t, PipeLangLanguageContractV350, `public Class Root { public string Read(Optional<string> value) => match(value){ some(item) => item, none => "missing" }; }`, "Read")
	base := valid.Functions[0]
	cases := map[string]func(*coreir.Function){
		"missing arm": func(f *coreir.Function) {
			f.Body.Match.Arms = append([]coreir.MatchArm(nil), f.Body.Match.Arms[:1]...)
		},
		"duplicate arm": func(f *coreir.Function) {
			f.Body.Match.Arms = append(f.Body.Match.Arms, f.Body.Match.Arms[1])
		},
		"arm after wildcard": func(f *coreir.Function) {
			f.Body.Match.Arms[1].Tag = "_"
			f.Body.Match.Arms = append(f.Body.Match.Arms, coreir.MatchArm{Tag: "none", Body: f.Body.Match.Arms[1].Body})
		},
		"invalid tag": func(f *coreir.Function) {
			f.Body.Match.Arms[1].Tag = "bogus"
		},
		"missing payload binding": func(f *coreir.Function) {
			f.Body.Match.Arms[0].Binding = nil
		},
		"binding on absence": func(f *coreir.Function) {
			position := len(f.Parameters)
			f.Body.Match.Arms[1].Binding = &position
		},
		"noncanonical binding": func(f *coreir.Function) {
			position := 0
			f.Body.Match.Arms[0].Binding = &position
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			function := base
			arms := append([]coreir.MatchArm(nil), base.Body.Match.Arms...)
			match := *base.Body.Match
			match.Arms = arms
			function.Body.Match = &match
			mutate(&function)
			if err := coreir.ValidateFunction(function); err == nil {
				t.Error("ValidateFunction accepted malformed match")
			}
			program := valid
			program.Functions = []coreir.Function{function}
			if err := coreir.ValidateProgram(program); err == nil {
				t.Error("ValidateProgram accepted malformed match")
			}
			if _, err := gobackend.Generate(program); err == nil {
				t.Errorf("Go backend accepted malformed match: %s", fmt.Sprint(name))
			}
		})
	}
}

func TestReviewAcceptedCoreMatchHasNoReachableGeneratedPanic(t *testing.T) {
	program := reviewCore(t, PipeLangLanguageContractV350, `public Class Root { public string Read(Optional<string> value) => match(value){ some(item) => item, none => "missing" }; }`, "Read")
	program.Functions[0].Body.Match.Arms = append([]coreir.MatchArm(nil), program.Functions[0].Body.Match.Arms[:1]...)
	if _, err := gobackend.Generate(program); err == nil {
		t.Fatal("backend accepted a Core match with a reachable non-exhaustive panic")
	}
}
