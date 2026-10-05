package pipelang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

func namespaceCore(t *testing.T, source string, entries ...string) coreir.Program {
	t.Helper()
	var program coreir.Program
	seen := map[string]bool{}
	for _, entry := range entries {
		part := reviewCore(t, PipeLangLanguageContractV800, source, entry)
		if program.CompilerContract == "" {
			program = part
			program.Functions = nil
		}
		for _, function := range part.Functions {
			key := function.Identity.PackageID + ":" + function.Identity.Path
			if !seen[key] {
				program.Functions = append(program.Functions, function)
				seen[key] = true
			}
		}
	}
	return program
}

func namespaceGenerate(t *testing.T, program coreir.Program) gobackend.Generated {
	t.Helper()
	before, _ := json.Marshal(program)
	generated, err := gobackend.GenerateWithNames(program)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(program)
	if !bytes.Equal(before, after) {
		t.Fatal("generation mutated Core input")
	}
	plain, err := gobackend.Generate(program)
	if err != nil || !bytes.Equal(plain, generated.Source) {
		t.Fatalf("Generate disagrees: %v", err)
	}
	program.Functions = append([]coreir.Function(nil), program.Functions...)
	for left, right := 0, len(program.Functions)-1; left < right; left, right = left+1, right-1 {
		program.Functions[left], program.Functions[right] = program.Functions[right], program.Functions[left]
	}
	reordered, err := gobackend.GenerateWithNames(program)
	if err != nil || !reflect.DeepEqual(generated, reordered) {
		t.Fatalf("generation depends on function order: %v", err)
	}
	return generated
}

func namespaceBinding(t *testing.T, generated gobackend.Generated, program coreir.Program, name string) string {
	t.Helper()
	identity := coreFunctionNamed(t, program, name).Identity
	for _, binding := range generated.Functions {
		if binding.Identity.PackageID == identity.PackageID && binding.Identity.Path == identity.Path {
			return binding.Name
		}
	}
	t.Fatalf("missing binding for %s", name)
	return ""
}

func TestGoNamespaceRuntimeCollisions(t *testing.T) {
	for _, name := range []string{"ArithmeticResult", "ArithmeticError", "ArithmeticOverflow", "ArithmeticDivisionByZero"} {
		t.Run(name, func(t *testing.T) {
			program := reviewCore(t, PipeLangLanguageContractV800, fmt.Sprintf(`public Class Root {
 public Result<int,ArithmeticError> %s(int x) => x + 1;
 public Result<int,ArithmeticError> Run(int x) => %s(x);
}`, name, name), "Run")
			generated := namespaceGenerate(t, program)
			allocated := namespaceBinding(t, generated, program, name)
			if allocated == "PipeLang"+name {
				t.Fatal("runtime collision was not allocated")
			}
			for _, input := range []int64{41, 9223372036854775807} {
				got, err := coreeval.EvaluateProgram(program, coreFunctionNamed(t, program, "Run").Identity, []coreeval.Value{{Type: coreFunctionNamed(t, program, "Run").Parameters[0].Type, Int: input}})
				if err != nil {
					t.Fatal(err)
				}
				if input == 41 && (!got.OK || got.Value.Int != 42) {
					t.Fatalf("evaluation: %#v", got)
				}
				if input != 41 && got.OK {
					t.Fatalf("overflow was lost: %#v", got)
				}
			}
			compileAndRunGeneratedGoFiles(t, generated.Source, []byte(fmt.Sprintf(`package pipelanggenerated
import "testing"
func TestCalls(t *testing.T) {
 for _, call := range []func(int64) PipeLangArithmeticResult[int64]{PipeLangRun, %s} {
  if got := call(41); !got.OK || got.Value != 42 { t.Fatal(got) }
  if got := call(9223372036854775807); got.OK || got.Error != PipeLangArithmeticOverflow { t.Fatal(got) }
 }
}`, allocated)))
		})
	}
	t.Run("text Result", func(t *testing.T) {
		program := reviewCore(t, PipeLangLanguageContractV800, `public Class Root {
 public Result<string,string> Result(string value) => ok<string,string>(value);
 public Result<string,string> Run(string value) => Result(value);
}`, "Run")
		generated := namespaceGenerate(t, program)
		compileAndRunGeneratedGoFiles(t, generated.Source, []byte(`package pipelanggenerated
import "testing"
func TestResult(t *testing.T) { if got := PipeLangRun("hello"); !got.OK || got.Value != "hello" { t.Fatal(got) } }
`))
	})
}

func TestGoNamespaceNormalizedCalls(t *testing.T) {
	program := namespaceCore(t, `public Class Left {
 public string Echo(string value) => value + "A";
 public string First(string value) => Echo(value);
}
public Class Right {
 public string Echo(string value) => value + "B";
 public string Second(string value) => Echo(value);
}`, "First", "Second")
	generated := namespaceGenerate(t, program)
	names := map[string]bool{}
	for _, binding := range generated.Functions {
		if names[binding.Name] {
			t.Fatal("functions share a name")
		}
		names[binding.Name] = true
	}
	compileAndRunGeneratedGoFiles(t, generated.Source, []byte(`package pipelanggenerated
import "testing"
func TestCalls(t *testing.T) {
 if got := PipeLangFirst("x"); got != "xA" { t.Fatal(got) }
 if got := PipeLangSecond("x"); got != "xB" { t.Fatal(got) }
}`))
}

func TestGoNamespacePredicateAndRecordDeclaration(t *testing.T) {
	program := namespaceCore(t, `public Record Row { public string Name; }
public Class Root {
 public bool RecordTestPackageAppRootRow(Row row, string q) => row.Name >= q;
 public List<Row> Search(List<Row> rows, string q) => filter(rows, RecordTestPackageAppRootRow, q);
}`, "Search")
	generated := namespaceGenerate(t, program)
	// The source method normalizes to the same name as the emitted Row type.
	if !strings.Contains(string(generated.Source), "func PipeLangRecordTestPackageAppRootRow__2(") {
		t.Fatalf("record/function collision was not allocated:\n%s", generated.Source)
	}
	compileAndRunGeneratedGoFiles(t, generated.Source, []byte(`package pipelanggenerated
import "testing"
func TestPredicate(t *testing.T) {
 rows := []PipeLangRecordTestPackageAppRootRow{{Name:"Alpha"}, {Name:"Beta"}}
 if got := PipeLangSearch(rows, "B"); len(got) != 1 || got[0].Name != "Beta" { t.Fatal(got) }
}`))
}

func TestGoNamespaceRecordIdentitiesAndHelpers(t *testing.T) {
	program := namespaceCore(t, `public Record Lower { public string Name; }
public Record Upper { public string Name; }
public Class Root {
 public Lower MakeLower(string name) => new Lower { Name = name };
 public Upper MakeUpper(string name) => new Upper { Name = name };
 public Optional<Lower> LowerOptional(Lower value) => some(value);
 public Optional<Upper> UpperOptional(Upper value) => some(value);
 public List<Lower> LowerList(Lower value) => list(value);
 public List<Upper> UpperList(Upper value) => list(value);
 public Result<List<Lower>,string> LowerResult(List<Lower> values) => ok<List<Lower>,string>(values);
 public Result<List<Upper>,string> UpperResult(List<Upper> values) => ok<List<Upper>,string>(values);
}`, "MakeLower", "MakeUpper", "LowerOptional", "UpperOptional", "LowerList", "UpperList", "LowerResult", "UpperResult")
	// Core identities can differ while their target identifiers normalize alike.
	// Preserve schemas and all references, including nested carrier metadata.
	encoded, err := json.Marshal(program)
	if err != nil {
		t.Fatal(err)
	}
	encoded = bytes.ReplaceAll(encoded, []byte("app.root.lower"), []byte("app.root.shared-a"))
	encoded = bytes.ReplaceAll(encoded, []byte("app.root.upper"), []byte("app.root.shared.a"))
	if err := json.Unmarshal(encoded, &program); err != nil {
		t.Fatal(err)
	}
	generated := namespaceGenerate(t, program)
	compileAndRunGeneratedGoFiles(t, generated.Source, []byte(`package pipelanggenerated
import "testing"
func TestRecords(t *testing.T) {
 lower, upper := PipeLangMakeLower("lower"), PipeLangMakeUpper("upper")
 if !pipelangHasValue(PipeLangLowerOptional(lower)) || !pipelangHasValue(PipeLangUpperOptional(upper)) { t.Fatal("missing Optional") }
 if got := PipeLangLowerResult(PipeLangLowerList(lower)); !got.OK || len(got.Value) != 1 || got.Value[0].Name != "lower" { t.Fatal(got) }
 if got := PipeLangUpperResult(PipeLangUpperList(upper)); !got.OK || len(got.Value) != 1 || got.Value[0].Name != "upper" { t.Fatal(got) }
}`))
}
