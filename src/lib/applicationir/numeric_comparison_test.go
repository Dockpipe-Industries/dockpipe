package applicationir

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang"
	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/tests/containedexec"
)

func TestNumericComparisonApplicationConsumer(t *testing.T) {
	source, err := os.ReadFile("testdata/docker-observability.pipe")
	if err != nil {
		t.Fatal(err)
	}
	original := `public DockerSnapshot Project(DockerSnapshot snapshot) => snapshot;`
	replacement := `public DockerSnapshot Project(DockerSnapshot snapshot) {
 string key = snapshot.Identity;
 int size = key == "" ? 0 : 1;
 if (size > 0) { return snapshot; } else { return snapshot; }
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
		t.Fatal("numeric body changed Application IR identity or output")
	}
	var function coreir.Function
	for _, candidate := range fixture.core.Functions {
		if candidate.Name == "Project" {
			function = candidate
		}
	}
	if function.Body.ImmutableLocal == nil || function.Body.ImmutableLocal.Return.ImmutableLocal == nil || function.Body.ImmutableLocal.Return.ImmutableLocal.Return.Conditional == nil {
		t.Fatal("consumer conditional local/numeric branch structure absent")
	}
	for _, identity := range []string{"", "ready"} {
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
	dir := t.TempDir()
	for name, data := range map[string][]byte{
		"go.mod":       []byte("module numeric-consumer-check\n\ngo 1.25\n"),
		"generated.go": generated,
		"generated_test.go": []byte(`package pipelanggenerated
import ("reflect"; "testing")
func TestProject(t *testing.T) {
 run := reflect.ValueOf(PipeLangProject)
 for _, identity := range []string{"", "ready"} {
  record := reflect.New(run.Type().In(0)).Elem()
  record.FieldByName("Identity").SetString(identity)
  got := run.Call([]reflect.Value{record})[0]
  if !reflect.DeepEqual(got.Interface(), record.Interface()) { t.Fatal(got) }
 }
}`),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command(filepath.Join(containedexec.GoRoot(t), "bin", "go"), "test", "-count=1", "-p=1", "-timeout=25s", ".")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off")
	if output, err := containedexec.CombinedOutput(command); err != nil {
		t.Fatalf("generated consumer Go: %v\n%s", err, output)
	}
}
