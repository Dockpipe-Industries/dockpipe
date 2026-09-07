package pipelang

import (
	"fmt"
	"reflect"
	"sync"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
)

func TestPreparedEvaluationOwnershipAndParity(t *testing.T) {
	for _, sourceType := range []string{"string", "int", "Row", "Optional<Row>", "List<Row>", "Result<List<Row>,string>", "Result<int,ArithmeticError>"} {
		t.Run(sourceType, func(t *testing.T) {
			source := fmt.Sprintf(`public Record Row { public string Text; } public Class Root { public %[1]s Echo(%[1]s value)=>value; public %[1]s Run(%[1]s value)=>Echo(value); }`, sourceType)
			program := reviewCore(t, PipeLangLanguageContractV800, source, "Run")
			function := coreFunctionNamed(t, program, "Run")
			prepared, err := coreeval.PrepareProgram(program)
			if err != nil {
				t.Fatal(err)
			}
			// Build independent arguments and expected values so mutating every
			// input Core node cannot also mutate the oracle's type metadata.
			independent := reviewCore(t, PipeLangLanguageContractV800, source, "Run")
			independentFunction := coreFunctionNamed(t, independent, "Run")
			type observation struct {
				identity coreir.SemanticIdentity
				args     []coreeval.Value
				outcome  coreeval.Outcome
				err      string
			}
			var observations []observation
			for _, tc := range canonicalHostArguments(independentFunction.Parameters[0].Type) {
				args := []coreeval.Value{tc.value}
				got, err := coreeval.EvaluateProgram(independent, independentFunction.Identity, args)
				observations = append(observations, observation{independentFunction.Identity, args, got, fmt.Sprint(err)})
			}
			erasePreparedTestGraph(reflect.ValueOf(&program).Elem())
			if _, err := coreeval.EvaluateProgram(program, function.Identity, nil); err == nil {
				t.Fatal("ordinary evaluation reused stale validation")
			}
			for i, observation := range observations {
				got, err := prepared.Evaluate(observation.identity, observation.args)
				if fmt.Sprint(err) != observation.err || !reflect.DeepEqual(got, observation.outcome) {
					t.Fatalf("case %d parity: %#v %v want %#v %s", i, got, err, observation.outcome, observation.err)
				}
				// Includes type metadata, carrier payloads, fields and backing slices.
				erasePreparedTestGraph(reflect.ValueOf(&got).Elem())
				again, err := prepared.Evaluate(observation.identity, observation.args)
				if fmt.Sprint(err) != observation.err || !reflect.DeepEqual(again, observation.outcome) {
					t.Fatalf("case %d returned value aliases preparation", i)
				}
			}
			if _, err := prepared.Evaluate(coreir.SemanticIdentity{}, nil); err == nil {
				t.Fatal("unknown identity accepted")
			}
			if _, err := prepared.Evaluate(independentFunction.Identity, nil); err == nil {
				t.Fatal("missing argument accepted")
			}
			var wg sync.WaitGroup
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for _, observation := range observations {
						got, err := prepared.Evaluate(observation.identity, observation.args)
						if fmt.Sprint(err) != observation.err || !reflect.DeepEqual(got, observation.outcome) {
							t.Error("concurrent evaluation drift")
						}
					}
				}()
			}
			wg.Wait()
		})
	}
	var empty coreeval.PreparedProgram
	for _, program := range []*coreeval.PreparedProgram{nil, &empty} {
		if _, err := program.Evaluate(coreir.SemanticIdentity{}, nil); err == nil {
			t.Fatal("unprepared evaluator accepted")
		}
	}
}

// Mutate every reachable field, not only root slice headers or a chosen node kind.
func erasePreparedTestGraph(value reflect.Value) {
	switch value.Kind() {
	case reflect.Pointer:
		if !value.IsNil() {
			erasePreparedTestGraph(value.Elem())
		}
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			erasePreparedTestGraph(value.Field(i))
		}
	case reflect.Slice:
		for i := 0; i < value.Len(); i++ {
			erasePreparedTestGraph(value.Index(i))
		}
	}
	if value.CanSet() {
		value.Set(reflect.Zero(value.Type()))
	}
}

func TestPreparedLiteralMetadataOwnership(t *testing.T) {
	program := reviewCore(t, PipeLangLanguageContractV800, `public Class Root { public int Constant()=>7; }`, "Constant")
	function := coreFunctionNamed(t, program, "Constant")
	identity := coreir.SemanticIdentity{PackageID: function.Identity.PackageID, Path: function.Identity.Path}
	prepared, err := coreeval.PrepareProgram(program)
	if err != nil {
		t.Fatal(err)
	}
	erasePreparedTestGraph(reflect.ValueOf(&program).Elem())
	first, err := prepared.Evaluate(identity, nil)
	if err != nil || !first.OK || first.Value.Int != 7 {
		t.Fatalf("literal snapshot changed: %#v %v", first, err)
	}
	erasePreparedTestGraph(reflect.ValueOf(&first).Elem())
	second, err := prepared.Evaluate(identity, nil)
	if err != nil || !second.OK || second.Value.Int != 7 || second.Value.Type.Numeric == nil || second.Value.Type.Numeric.Bits != 64 {
		t.Fatalf("returned metadata changed snapshot: %#v %v", second, err)
	}
}

func TestPreparedExactIdentityPair(t *testing.T) {
	program := reviewCore(t, PipeLangLanguageContractV800, `public Class Root { public int Constant()=>7; }`, "Constant")
	program.Functions[0].Identity.PackageID = "pkg\x00segment"
	program.Functions[0].Identity.Path = "method"
	actual := program.Functions[0].Identity
	collision := coreir.SemanticIdentity{PackageID: "pkg", Path: "segment\x00method"}
	if err := coreir.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
	prepared, err := coreeval.PrepareProgram(program)
	if err != nil {
		t.Fatal(err)
	}
	for _, evaluate := range []func(coreir.SemanticIdentity) (coreeval.Outcome, error){
		func(id coreir.SemanticIdentity) (coreeval.Outcome, error) {
			return coreeval.EvaluateProgram(program, id, nil)
		},
		func(id coreir.SemanticIdentity) (coreeval.Outcome, error) { return prepared.Evaluate(id, nil) },
	} {
		if got, err := evaluate(actual); err != nil || got.Value.Int != 7 {
			t.Fatalf("exact identity rejected: %#v %v", got, err)
		}
		if _, err := evaluate(collision); err == nil || err.Error() != "selected function semantic identity was not found" {
			t.Fatalf("different identity pair aliased entrypoint: %v", err)
		}
	}
}
