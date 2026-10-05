package pipelang

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

func TestComputedResultArgumentRecovery(t *testing.T) {
	for _, contract := range []LanguageContract{PipeLangLanguageContractV360, PipeLangLanguageContractV800} {
		t.Run(string(contract), func(t *testing.T) {
			program := reviewCore(t, contract, `public Class Root {
 public Result<int,ArithmeticError> Make(int value) => value + 1;
 public string Recover(Result<int,ArithmeticError> value) => match(value){ ok(item) => "ok", err(problem) => "recovered" };
 public string Run(int value) => Recover(Make(value));
}`, "Run")
			function := coreFunctionNamed(t, program, "Run")
			for _, tc := range []struct {
				input int64
				want  string
			}{{1, "ok"}, {math.MaxInt64, "recovered"}} {
				outcome, err := coreeval.EvaluateProgram(program, function.Identity, []coreeval.Value{{Type: function.Parameters[0].Type, Int: tc.input}})
				if err != nil || !outcome.OK || !coreir.TypeEqual(outcome.Value.Type, function.ReturnType) || outcome.Value.String != tc.want {
					t.Fatalf("Run(%d) = %#v, %v; want string %q", tc.input, outcome, err, tc.want)
				}
			}
			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			compileAndRunGeneratedGoFiles(t, generated, []byte(`package pipelanggenerated
import "testing"
func TestRecovery(t *testing.T) {
 if got := PipeLangRun(1); got != "ok" { t.Fatal(got) }
 if got := PipeLangRun(9223372036854775807); got != "recovered" { t.Fatal(got) }
}`))
		})
	}
}

func TestComputedResultArgumentCarrierMatrix(t *testing.T) {
	for _, payload := range []string{"int", "float", "string", "List<Row>"} {
		t.Run(payload, func(t *testing.T) {
			failure := "string"
			if payload == "int" || payload == "float" {
				failure = "ArithmeticError"
			}
			carrier := fmt.Sprintf("Result<%s,%s>", payload, failure)
			program := reviewCore(t, PipeLangLanguageContractV800, fmt.Sprintf(`public Record Row { public string Id; }
public Class Root {
 public %s Forward(%s value) => value;
 public %s Run(%s value) => Forward(Forward(value));
}`, carrier, carrier, carrier, carrier), "Run")
			function := coreFunctionNamed(t, program, "Run")
			parameter := function.Parameters[0].Type
			value := coreeval.Value{Type: parameter.Result.Success}
			switch payload {
			case "int":
				value.Int = 7
			case "float":
				value.Float = 2.5
			case "string":
				value.String = "value"
			case "List<Row>":
				row := value.Type.List.Element
				value.List = []coreeval.Value{{Type: row, Record: []coreeval.Value{{Type: row.Record.Fields[0].Type, String: "row"}}}}
			}
			for _, success := range []bool{true, false} {
				result := coreeval.Outcome{OK: success, Value: coreeval.Value{Type: parameter.Result.Success}}
				if success {
					result.Value = value
				} else if failure == "ArithmeticError" {
					result.Error = coreir.ArithmeticOverflow
				} else {
					result.Failure = &coreeval.Value{Type: parameter.Result.Failure, String: "failure"}
				}
				outcome, err := coreeval.EvaluateProgram(program, function.Identity, []coreeval.Value{{Type: parameter, Result: &result}})
				if err != nil || !reflect.DeepEqual(outcome, result) {
					t.Fatalf("success=%t: %#v, %v", success, outcome, err)
				}
			}
			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			// Reflection constructs the generated carrier without depending on private
			// backend record-name allocation. Both states cross the actual Go entrypoint.
			compileAndRunGeneratedGoFiles(t, generated, []byte(`package pipelanggenerated
import ("reflect"; "testing")
func TestCarriers(t *testing.T) {
 run := reflect.ValueOf(PipeLangRun)
 for _, success := range []bool{true, false} {
  value := reflect.New(run.Type().In(0)).Elem()
  value.FieldByName("OK").SetBool(success)
  payload := value.FieldByName("Value")
  if success {
   if payload.Kind() == reflect.Slice { payload.Set(reflect.MakeSlice(payload.Type(), 0, 0)) }
  } else { value.FieldByName("Error").SetString("overflow") }
  got := run.Call([]reflect.Value{value})[0]
  if !reflect.DeepEqual(got.Interface(), value.Interface()) { t.Fatalf("success=%t got=%v want=%v", success, got, value) }
 }
}`))
		})
	}
}

func TestComputedResultArgumentCopiesSnapshot(t *testing.T) {
	program := reviewCore(t, PipeLangLanguageContractV800, `public Record Row { public string Id; }
public Class Root {
 public Result<List<Row>,string> Forward(Result<List<Row>,string> value) => value;
 public Result<List<Row>,string> Run(Result<List<Row>,string> value) => Forward(Forward(value));
}`, "Run")
	function := coreFunctionNamed(t, program, "Run")
	carrier := function.Parameters[0].Type
	row := carrier.Result.Success.List.Element
	input := coreeval.Value{Type: carrier, Result: &coreeval.Outcome{OK: true, Value: coreeval.Value{
		Type: carrier.Result.Success,
		List: []coreeval.Value{{Type: row, Record: []coreeval.Value{{Type: row.Record.Fields[0].Type, String: "original"}}}},
	}}}
	outcome, err := coreeval.EvaluateProgram(program, function.Identity, []coreeval.Value{input})
	if err != nil || !outcome.OK || len(outcome.Value.List) != 1 {
		t.Fatalf("snapshot = %#v, %v", outcome, err)
	}
	outcome.Value.List[0].Record[0].String = "changed"
	if input.Result.Value.List[0].Record[0].String != "original" {
		t.Fatal("computed carrier aliases its input snapshot")
	}
}
