package pipelang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

func admissionFixture(t *testing.T, name string) coreir.Program {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name+".core.json"))
	if err != nil {
		t.Fatal(err)
	}
	var program coreir.Program
	if err := json.Unmarshal(data, &program); err != nil {
		t.Fatal(err)
	}
	return program
}

func assertAdmissionRejected(t *testing.T, program coreir.Program, want string) {
	t.Helper()
	validation := coreir.ValidateProgram(program)
	if validation == nil || !strings.Contains(validation.Error(), want) {
		t.Fatalf("Core admission = %v; want %q", validation, want)
	}
	// Admission must precede function lookup and argument checking, including empty programs.
	_, evaluation := coreeval.EvaluateProgram(program, coreir.SemanticIdentity{}, nil)
	if evaluation == nil || evaluation.Error() != validation.Error() {
		t.Fatalf("evaluation = %v; Core = %v", evaluation, validation)
	}
	_, preparation := coreeval.PrepareProgram(program)
	if preparation == nil || preparation.Error() != validation.Error() {
		t.Fatalf("preparation = %v; Core = %v", preparation, validation)
	}
	generated, generation := gobackend.Generate(program)
	backend, ok := generation.(*gobackend.Error)
	if !ok || backend.Code != "PLGO0001" || backend.Message != validation.Error() || generated != nil {
		t.Fatalf("generation = %v; Core = %v", generation, validation)
	}
}

func TestCoreAdmissionIdentities(t *testing.T) {
	t.Run("original-trim-downgrade", func(t *testing.T) {
		program := admissionFixture(t, "text-trim")
		program.LanguageContract = coreir.LanguageContractV010
		assertAdmissionRejected(t, program, "text_trim requires language contract")
	})
	for _, name := range []string{"tiny-pure-function", "text-trim"} {
		for _, empty := range []bool{false, true} {
			for _, identity := range []string{"compiler", "language"} {
				for _, invalid := range []string{"", "unknown", "v0.999.0", "v0.01.0", "v0.1.1", "v0.0.0.1"} {
					t.Run(fmt.Sprintf("%s/empty=%v/%s/%s", name, empty, identity, invalid), func(t *testing.T) {
						program := admissionFixture(t, name)
						if empty {
							program.Functions = nil
						}
						if identity == "compiler" {
							program.CompilerContract = invalid
						} else {
							program.LanguageContract = invalid
						}
						assertAdmissionRejected(t, program, "unsupported Core IR "+identity+" contract")
					})
				}
			}
		}
	}
}

func admissionValue(typ coreir.Type) coreeval.Value {
	value := coreeval.Value{Type: typ}
	switch typ.Kind {
	case coreir.TypeRecord:
		for _, field := range typ.Record.Fields {
			value.Record = append(value.Record, admissionValue(field.Type))
		}
	case coreir.TypeList:
		value.List = []coreeval.Value{}
	case coreir.TypeOptional:
		value.Optional = &coreeval.OptionalValue{}
	case coreir.TypeResult:
		value.Result = &coreeval.Outcome{OK: true, Value: admissionValue(typ.Result.Success)}
	}
	return value
}

func admissionOutcomes(t *testing.T, program coreir.Program) []coreeval.Outcome {
	t.Helper()
	var outcomes []coreeval.Outcome
	for _, function := range program.Functions {
		var arguments []coreeval.Value
		for _, parameter := range function.Parameters {
			arguments = append(arguments, admissionValue(parameter.Type))
		}
		outcome, err := coreeval.EvaluateProgram(program, function.Identity, arguments)
		if err != nil {
			t.Fatalf("%s: %v", function.Name, err)
		}
		outcomes = append(outcomes, outcome)
	}
	return outcomes
}

func TestCoreAdmissionFeatureFixtures(t *testing.T) {
	paths, err := filepath.Glob("testdata/*.core.json")
	if err != nil || len(paths) != 29 {
		t.Fatalf("fixture inventory = %d, %v", len(paths), err)
	}
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".core.json")
		t.Run(name, func(t *testing.T) {
			program := admissionFixture(t, name)
			if err := coreir.ValidateProgram(program); err != nil {
				t.Fatal(err)
			}
			original, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			outcomes := admissionOutcomes(t, program)
			var minor int
			if _, err := fmt.Sscanf(program.LanguageContract, "v0.%d.0", &minor); err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(name, "checked-") || name == "result-transport" {
				// These Core capabilities predate their public source spellings.
				internal := program
				internal.LanguageContract = coreir.LanguageContractV010
				generated, err := gobackend.Generate(internal)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(original, generated) || !reflect.DeepEqual(outcomes, admissionOutcomes(t, internal)) {
					t.Fatal("internal v0.1 Core compatibility changed")
				}
			} else if minor > 1 {
				downgraded := program
				downgraded.LanguageContract = fmt.Sprintf("v0.%d.0", minor-1)
				assertAdmissionRejected(t, downgraded, "requires language contract")
			}
			for _, contract := range []string{coreir.LanguageContractV830, coreir.LanguageContractV840, coreir.LanguageContractV850, coreir.LanguageContractV860} {
				program.LanguageContract = contract
				latest, err := gobackend.Generate(program)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(original, latest) || !reflect.DeepEqual(outcomes, admissionOutcomes(t, program)) {
					t.Fatal("accepted metadata changed generated bytes or evaluator outcomes")
				}
			}

		})
	}
}

func TestCoreAdmissionSupportedVersions(t *testing.T) {
	program := reviewCore(t, PipeLangLanguageContractV010, `public Class Root { public bool Run(bool value) => value; }`, "Run")
	original, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	outcomes := admissionOutcomes(t, program)
	for minor := 1; minor <= 85; minor++ {
		t.Run(fmt.Sprint(minor), func(t *testing.T) {
			program.LanguageContract = fmt.Sprintf("v0.%d.0", minor)
			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(original, generated) || !reflect.DeepEqual(outcomes, admissionOutcomes(t, program)) {
				t.Fatal("accepted version changed behavior")
			}
		})
	}
}

func TestCoreAdmissionLaterAndNestedFeatures(t *testing.T) {
	for _, tc := range []struct {
		name, source, previous string
		contract               LanguageContract
	}{
		{"text-result", `public Class Root { public Result<string,string> Run(Result<string,string> value) => value; }`, "v0.24.0", PipeLangLanguageContractV250},
		{"directional-sort", `public Record Row { public string Id; public string Name; } public Class Root { public List<Row> Run(List<Row> values) => sort_by_ordinal(values, Row.Id, descending, Row.Name, ascending); }`, "v0.31.0", PipeLangLanguageContractV320},
		{"propagation", `public Class Root { public Optional<string> Run(Optional<string> value) => some(propagate(value)); }`, "v0.33.0", PipeLangLanguageContractV340},
		{"match", `public Class Root { public string Run(Optional<string> value) => match(value){ some(item) => item, none => "empty" }; }`, "v0.34.0", PipeLangLanguageContractV350},
		{"nested-trim", `public Class Root { public string Echo(string value) => value; public string Run(string value) => Echo(trim(value)); }`, "v0.25.0", PipeLangLanguageContractV360},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program := reviewCore(t, tc.contract, tc.source, "Run")
			admissionOutcomes(t, program)
			reviewCompileGenerated(t, program)
			program.LanguageContract = tc.previous
			assertAdmissionRejected(t, program, "requires language contract")
		})
	}
}

func TestCoreAdmissionChecksWholeProgram(t *testing.T) {
	t.Run("uncalled-function", func(t *testing.T) {
		program := reviewCore(t, PipeLangLanguageContractV260, `public Class Root { public bool Run(bool value) => value; }`, "Run")
		trim := admissionFixture(t, "text-trim")
		program.Functions = append(program.Functions, trim.Functions...)
		if err := coreir.ValidateProgram(program); err != nil {
			t.Fatal(err)
		}
		program.LanguageContract = coreir.LanguageContractV250
		assertAdmissionRejected(t, program, "text_trim requires language contract")
		selected := program.Functions[0]
		_, err := coreeval.EvaluateProgram(program, selected.Identity, []coreeval.Value{{Type: selected.Parameters[0].Type, Bool: true}})
		if err == nil || !strings.Contains(err.Error(), "text_trim requires language contract") {
			t.Fatalf("uncalled function escaped admission: %v", err)
		}
	})
	t.Run("unused-parameter", func(t *testing.T) {
		program := reviewCore(t, PipeLangLanguageContractV800, `public Class Root { public bool Run(Optional<string> unused, bool value) => value ? true : false; }`, "Run")
		admissionOutcomes(t, program)
		program.LanguageContract = coreir.LanguageContractV120
		assertAdmissionRejected(t, program, "optional type requires language contract")
	})
}
