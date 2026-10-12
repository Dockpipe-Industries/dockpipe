package pipelang

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
)

func sharedHIRGraphSource(count int) string {
	var source strings.Builder
	source.WriteString("public Class Root { public bool F0(bool x) => x; public bool F1(bool x) => x;")
	for i := 2; i < count; i++ {
		fmt.Fprintf(&source, "public bool F%d(bool x) => F%d(x) && F%d(x);", i, i-1, i-2)
	}
	source.WriteString("public bool Unreachable(bool x) => x; }")
	return source.String()
}

func analyzeHIRGraph(t testing.TB, source string) *Analysis {
	t.Helper()
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "graph.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV800
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	return analysis
}

func lowerHIRGraphCounting(t *testing.T, analysis *Analysis, identity SemanticIdentity, want []string) hir.Program {
	t.Helper()
	counts := make(map[string]int)
	typed, err := lowerSemanticMethodGraphToHIR(analysis, identity, func(a *Analysis, id SemanticIdentity) (hir.Function, error) {
		key := semanticIdentityKey(id)
		counts[key]++
		if counts[key] != 1 {
			t.Fatalf("method %s lowered %d times", id.String(), counts[key])
		}
		return lowerSemanticFunctionToHIR(a, id)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != len(want) {
		t.Fatalf("lowered %d methods, want %d", len(counts), len(want))
	}
	var names []string
	for _, function := range typed.Functions {
		names = append(names, function.Name)
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("dependency order = %v, want %v", names, want)
	}
	public, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(typed, public) {
		t.Fatal("instrumented and public lowering differ")
	}
	return typed
}

func TestHIRSharedDependencyGraphLowersEachMethodOnce(t *testing.T) {
	for _, count := range []int{21, 25, 29, 128} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			analysis := analyzeHIRGraph(t, sharedHIRGraphSource(count))
			identity := semanticMethodNamed(t, analysis, fmt.Sprintf("F%d", count-1)).Identity
			// The first branch discovers F1 before F0, then completes F2 upwards.
			want := []string{"F1", "F0"}
			for i := 2; i < count; i++ {
				want = append(want, fmt.Sprintf("F%d", i))
			}
			typed := lowerHIRGraphCounting(t, analysis, identity, want)
			core, err := LowerHIRToCore(typed)
			if err != nil {
				t.Fatal(err)
			}
			generated, err := gobackend.Generate(core)
			if err != nil {
				t.Fatal(err)
			}
			again, err := LowerSemanticMethodToHIR(analysis, identity)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(typed, again) {
				t.Fatal("HIR changed across lowering requests")
			}
			againCore, err := LowerHIRToCore(again)
			if err != nil {
				t.Fatal(err)
			}
			againGo, err := gobackend.Generate(againCore)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(core, againCore) || !bytes.Equal(generated, againGo) {
				t.Fatal("Core or Go output is not deterministic")
			}
			// Runtime true inputs deliberately expand this graph; use the smallest
			// case for value agreement, independently of the compiler scaling proof.
			if count == 21 {
				root := coreFunctionNamed(t, core, "F20")
				for _, value := range []bool{false, true} {
					outcome, err := coreeval.EvaluateProgram(core, root.Identity, []coreeval.Value{{Type: root.Parameters[0].Type, Bool: value}})
					if err != nil || !outcome.OK || outcome.Value.Bool != value {
						t.Fatalf("evaluation(%v) = %#v, %v", value, outcome, err)
					}
				}
				compileAndRunGeneratedGoFiles(t, generated, []byte(`package pipelanggenerated
import "testing"
func TestSharedGraph(t *testing.T) {
 for _, x := range []bool{false, true} { if F := PipeLangF20(x); F != x { t.Fatalf("got %v, want %v", F, x) } }
}`))
			}
		})
	}
}

func TestHIRSharedNamedPredicateDependencyClosure(t *testing.T) {
	analysis := analyzeHIRGraph(t, `public Record Row { public string Name; public string State; }
public Class Root {
 public bool Matches(Row row, string query) => contains_casefolded(row.Name, trim(query)) || contains_casefolded(row.State, trim(query));
 public List<Row> First(List<Row> rows, string query) => filter(rows, Matches, query);
 public List<Row> Second(List<Row> rows, string query) => filter(rows, Matches, query);
 public List<Row> Search(List<Row> rows, string query) => First(Second(rows, query), query);
 public bool Unreachable(bool x) => x;
}`)
	identity := semanticMethodNamed(t, analysis, "Search").Identity
	typed := lowerHIRGraphCounting(t, analysis, identity, []string{"Matches", "First", "Second", "Search"})
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gobackend.Generate(core); err != nil {
		t.Fatal(err)
	}
	// A new root gets its own closed graph, with no cross-request cache leakage.
	lowerHIRGraphCounting(t, analysis, semanticMethodNamed(t, analysis, "Second").Identity, []string{"Matches", "Second"})
}

func TestHIRDependencyGraphRejectsCycles(t *testing.T) {
	for _, body := range []string{
		`public bool A(bool x) => A(x);`,
		`public bool A(bool x) => B(x); public bool B(bool x) => A(x);`,
	} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "cycle.pipe", "public Class Root {"+body+"}")}, nil)
		input.LanguageContract = PipeLangLanguageContractV800
		analysis := AnalyzeSemanticModuleSet(input)
		if len(analysis.Diagnostics) != 1 || analysis.Diagnostics[0].Code != CodePureCallCycle {
			t.Fatalf("cycle diagnostics = %#v", analysis.Diagnostics)
		}
		if _, err := LowerSemanticMethodToHIR(analysis, SemanticIdentity{}); err == nil {
			t.Fatal("HIR accepted failed cyclic analysis")
		}
	}
	// Defend the traversal itself if checked syntax is later made inconsistent.
	analysis := analyzeHIRGraph(t, `public Class Root { public bool A(bool x) => x; public bool B(bool x) => A(x); }`)
	a := semanticMethodNamed(t, analysis, "A").Identity
	b := semanticMethodNamed(t, analysis, "B").Identity
	methodByIdentity(analysis, a).Body = methodByIdentity(analysis, b).Body
	if _, err := LowerSemanticMethodToHIR(analysis, b); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("HIR cycle error = %v", err)
	}
}

func BenchmarkHIRSharedDependencyGraph(b *testing.B) {
	for _, count := range []int{21, 25, 29, 128} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			analysis := analyzeHIRGraph(b, sharedHIRGraphSource(count))
			method := analysis.Program.Classes[0].Methods[count-1]
			identity, ok := analysis.SemanticIDs.IdentityForSpan(method.Span)
			if !ok {
				b.Fatal("missing root identity")
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := LowerSemanticMethodToHIR(analysis, identity); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
