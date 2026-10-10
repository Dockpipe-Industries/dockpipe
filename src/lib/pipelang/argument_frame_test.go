package pipelang

import (
	"reflect"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
)

func TestEvaluationFramesPreserveCallerStorage(t *testing.T) {
	source := `public Class Root {
 public string Helper(string raw,bool pick) { string clean=trim(raw); if(pick){string suffix=clean+"H";return suffix;}else{return clean;} }
 public string Run(string raw,bool pick) { string first=Helper(raw,pick);string second=Helper(first,!pick);if(pick){string branch=second+"T";return branch;}else{string branch=second+"F";return branch;} }
 }`
	program := reviewCore(t, PipeLangLanguageContractV960, source, "Run")
	function := coreFunctionNamed(t, program, "Run")
	prepared, err := coreeval.PrepareProgram(program)
	if err != nil {
		t.Fatal(err)
	}
	for _, pick := range []bool{false, true} {
		want := "abcHF"
		if pick {
			want = "abcHT"
		}
		for _, evaluate := range []func([]coreeval.Value) (coreeval.Outcome, error){
			func(args []coreeval.Value) (coreeval.Outcome, error) {
				return coreeval.EvaluateProgram(program, function.Identity, args)
			},
			func(args []coreeval.Value) (coreeval.Outcome, error) {
				return prepared.Evaluate(function.Identity, args)
			},
		} {
			backing := make([]coreeval.Value, 16)
			backing[0] = coreeval.Value{Type: function.Parameters[0].Type, String: " abc "}
			backing[1] = coreeval.Value{Type: function.Parameters[1].Type, Bool: pick}
			for i := 2; i < len(backing); i++ {
				backing[i] = coreeval.Value{String: "caller spare capacity", Int: int64(i)}
			}
			before := append([]coreeval.Value(nil), backing...)
			for repeat := 0; repeat < 3; repeat++ {
				got, err := evaluate(backing[:2])
				if err != nil || !got.OK || got.Value.String != want {
					t.Fatalf("got %#v %v want %q", got, err, want)
				}
				if !reflect.DeepEqual(backing, before) {
					t.Fatal("evaluation changed caller backing storage")
				}
			}
		}
	}
}
