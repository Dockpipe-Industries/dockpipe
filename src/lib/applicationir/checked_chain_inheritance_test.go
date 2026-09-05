package applicationir

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang"
	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

func TestCheckedChainInheritanceApplicationConsumer(t *testing.T) {
	for _, stages := range []int{2, 3} {
		t.Run(fmt.Sprint(stages), func(t *testing.T) { checkedChainInheritanceConsumer(t, stages) })
	}
}

func checkedChainInheritanceConsumer(t *testing.T, stages int) {
	t.Helper()
	source, err := os.ReadFile("testdata/docker-observability.pipe")
	if err != nil {
		t.Fatal(err)
	}
	original := `public DockerSnapshot Project(DockerSnapshot snapshot) => snapshot;`
	replacement := `public Result<int, ArithmeticError> Start(int value) => value + 1;
 public int Initial(bool fail) => fail ? 9223372036854775807 : 1;
 public Result<int, ArithmeticError> Advance(Result<int, ArithmeticError> carrier, int first, int second`
	if stages == 3 {
		replacement += ", int third"
	}
	replacement += `) {
  int cursor = propagate(carrier);
  Result<int, ArithmeticError> firstCarrier = cursor + first;
  int afterFirst = propagate(firstCarrier);
 `
	if stages == 3 {
		replacement += `Result<int, ArithmeticError> secondCarrier = afterFirst + second;
 int afterSecond = propagate(secondCarrier);
 return afterSecond + third;`
	} else {
		replacement += "return afterFirst + second;"
	}
	replacement += `}
 public string Describe(Result<int, ArithmeticError> cursor) => match(cursor) { ok(value) => "ready", err(error) => "failed" };
 public DockerSnapshot Project(DockerSnapshot snapshot) {
  string key = snapshot.Identity;
  bool incoming = key == "incoming";
  int initial = Initial(incoming);
  bool failFirst = key == "stage1";
  int first = Initial(failFirst);
  bool failSecond = key == "stage2";
  int second = Initial(failSecond);
 `
	if stages == 3 {
		replacement += `bool failThird = key == "stage3"; int third = Initial(failThird);`
	}
	replacement += `string status = Describe(Advance(Start(initial), first, second`
	if stages == 3 {
		replacement += ", third"
	}
	replacement += `));
  if (status == "ready") { return snapshot; } else { return snapshot; }
 }`
	if strings.Count(string(source), original) != 1 {
		t.Fatal("fixture mutation target missing")
	}
	changed := []byte(strings.Replace(string(source), original, replacement, 1))
	fixture := loadReviewApplicationSource(t, changed, pipelang.PipeLangLanguageContractV820)
	again := loadReviewApplicationSource(t, changed, pipelang.PipeLangLanguageContractV820)
	if !reflect.DeepEqual(fixture.semantic, again.semantic) || !reflect.DeepEqual(fixture.core, again.core) {
		t.Fatal("semantic/Core output is nondeterministic")
	}
	if fixture.semantic.Schema != "pipelang.semantic.v1" || fixture.semantic.CompilerContract != "pipelang.compiler.v1" ||
		fixture.core.CompilerContract != "pipelang.compiler.v1" || fixture.core.LanguageContract != "v0.82.0" {
		t.Fatal("compiler/semantic identity drift")
	}
	app, err := Project(fixture.semantic, &fixture.core, fixture.spec)
	if err != nil {
		t.Fatal(err)
	}
	got, err := CanonicalJSON(app)
	if err != nil {
		t.Fatal(err)
	}
	baseline := loadReviewApplicationSource(t, source, pipelang.PipeLangLanguageContractV820)
	old, err := Project(baseline.semantic, &baseline.core, baseline.spec)
	if err != nil {
		t.Fatal(err)
	}
	want, err := CanonicalJSON(old)
	if err != nil || !bytes.Equal(got, want) || app.Schema != "dockpipe.application.v1" {
		t.Fatal("checked-propagation body changed Application IR identity or output")
	}
	var function coreir.Function
	for _, candidate := range fixture.core.Functions {
		if candidate.Name == "Project" {
			function = candidate
		}
	}
	if function.Body.ImmutableLocal == nil {
		t.Fatal("consumer dependency call absent")
	}
	found := false
	for _, candidate := range fixture.core.Functions {
		if candidate.Name == "Advance" {
			found = candidate.Body.ImmutableLocal != nil && candidate.Body.ImmutableLocal.Initializer.Propagate != nil && candidate.Body.ImmutableLocal.Return.ImmutableLocal != nil
		}
	}
	if !found {
		t.Fatal("consumer checked-propagation dependency absent")
	}
	for _, identity := range []string{"ready", "incoming", "stage1", "stage2", "stage3"} {
		record := coreeval.Value{Type: function.Parameters[0].Type, Record: []coreeval.Value{{Type: function.Parameters[0].Type.Record.Fields[0].Type, String: identity}}}
		outcome, err := coreeval.EvaluateProgram(fixture.core, function.Identity, []coreeval.Value{record})
		if err != nil || !outcome.OK || !reflect.DeepEqual(outcome.Value, record) {
			t.Fatalf("identity %q: %#v, %v", identity, outcome, err)
		}
	}
	generated, err := gobackend.Generate(fixture.core)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := gobackend.Generate(again.core)
	if err != nil || !bytes.Equal(generated, repeated) {
		t.Fatal("generated consumer Go is nondeterministic")
	}
	// Assert chain outcomes as well as unchanged snapshot output, so the two equal
	// snapshot return branches cannot hide a failed or skipped helper computation.
	var advance coreir.Function
	for _, candidate := range fixture.core.Functions {
		if candidate.Name == "Advance" {
			advance = candidate
		}
	}
	var generatedChecks strings.Builder
	for failAt := -2; failAt < stages; failAt++ {
		carrier := coreeval.Outcome{OK: failAt != -1, Value: coreeval.Value{Type: advance.Parameters[1].Type, Int: 2}}
		if failAt == -1 {
			carrier.Value.Int = 0
			carrier.Error = coreir.ArithmeticOverflow
		}
		args := []coreeval.Value{{Type: advance.Parameters[0].Type, Result: &carrier}}
		var operands []string
		for i := 0; i < stages; i++ {
			value := int64(1)
			if i == failAt {
				value = math.MaxInt64
			}
			args = append(args, coreeval.Value{Type: advance.Parameters[i+1].Type, Int: value})
			operands = append(operands, fmt.Sprint(value))
		}
		want := int64(2 + stages)
		failure := coreir.ArithmeticError("")
		if failAt != -2 {
			want = 0
			failure = coreir.ArithmeticOverflow
		}
		got, err := coreeval.EvaluateProgram(fixture.core, advance.Identity, args)
		if err != nil || got.OK != (failure == "") || got.Error != failure || got.Value.Int != want {
			t.Fatalf("chain failure stage %d: %#v, %v", failAt, got, err)
		}
		fmt.Fprintf(&generatedChecks, "{got:=PipeLangAdvance(PipeLangArithmeticResult[int64]{OK:%t,Value:%d,Error:%q},%s);if got.OK!=%t || got.Error!=%q || got.Value!=%d {t.Fatalf(\"chain stage %d: %%#v\",got)}}\n", carrier.OK, carrier.Value.Int, carrier.Error, strings.Join(operands, ","), failure == "", failure, want, failAt)
	}
	dir := t.TempDir()
	for name, data := range map[string][]byte{
		"go.mod":       []byte("module checked-chain-consumer-check\n\ngo 1.25\n"),
		"generated.go": generated,
		"generated_test.go": []byte(`package pipelanggenerated
import ("reflect"; "testing")
func TestProject(t *testing.T) {
 run := reflect.ValueOf(PipeLangProject)
 for _, identity := range []string{"ready", "incoming", "stage1", "stage2", "stage3"} {
  record := reflect.New(run.Type().In(0)).Elem()
  record.FieldByName("Identity").SetString(identity)
  got := run.Call([]reflect.Value{record})[0]
  if !reflect.DeepEqual(got.Interface(), record.Interface()) { t.Fatal(got) }
 }
` + generatedChecks.String() + "}\n"),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "test", ".")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated consumer Go: %v\n%s", err, output)
	}
}
