package applicationir

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang"
	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/tests/containedexec"
)

func TestV810TerminalTreeApplicationConsumer(t *testing.T) {
	source, err := os.ReadFile("testdata/docker-observability.pipe")
	if err != nil {
		t.Fatal(err)
	}
	original := `public DockerSnapshot Project(DockerSnapshot snapshot) => snapshot;`
	replacement := `public DockerSnapshot Project(DockerSnapshot snapshot) {
  string key = snapshot.Identity;
  if (key == "") { return snapshot; } else {
   string normalized = trim(key);
   if (normalized == "") { return snapshot; } else {
    string value = normalized;
    if (value == "ready") { return snapshot; } else { return snapshot; }
   }
  }
 }`
	if strings.Count(string(source), original) != 1 {
		t.Fatal("fixture mutation target missing")
	}
	changed := []byte(strings.Replace(string(source), original, replacement, 1))
	fixture := loadReviewApplicationSource(t, changed, pipelang.PipeLangLanguageContractV810)
	app, err := Project(fixture.semantic, &fixture.core, fixture.spec)
	if err != nil {
		t.Fatal(err)
	}
	if app.Metadata.LanguageContract != "v0.81.0" {
		t.Fatal("consumer language metadata drift")
	}
	got, err := CanonicalJSON(app)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Project(fixture.semantic, &fixture.core, fixture.spec)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := CanonicalJSON(again)
	if err != nil || !bytes.Equal(got, repeated) {
		t.Fatal("consumer output nondeterministic")
	}
	old := loadReviewApplicationFixture(t)
	oldApp, err := Project(old.semantic, &old.core, old.spec)
	if err != nil {
		t.Fatal(err)
	}
	if app.Schema != oldApp.Schema || app.Identity != oldApp.Identity || len(app.Sections) != len(oldApp.Sections) {
		t.Fatal("consumer identity/shape drift")
	}
	var function coreir.Function
	for _, f := range fixture.core.Functions {
		if f.Name == "Project" {
			function = f
		}
	}
	if function.Body.ImmutableLocal == nil {
		t.Fatal("root local absent")
	}
	outer := function.Body.ImmutableLocal.Return.Conditional
	if outer == nil || outer.WhenTrue.Conditional != nil || outer.WhenFalse.ImmutableLocal == nil || outer.WhenFalse.ImmutableLocal.Return.Conditional.WhenFalse.ImmutableLocal.Return.Conditional == nil {
		t.Fatal("asymmetric depth-three Core missing")
	}
	for _, input := range []string{"", "   ", "ready", "other"} {
		record := coreeval.Value{Type: function.Parameters[0].Type, Record: []coreeval.Value{{Type: function.Parameters[0].Type.Record.Fields[0].Type, String: input}}}
		result, err := coreeval.EvaluateProgram(fixture.core, function.Identity, []coreeval.Value{record})
		if err != nil || !result.OK || result.Value.Record[0].String != input {
			t.Fatalf("consumer input %q: %#v %v", input, result, err)
		}
	}
	generated, err := gobackend.Generate(fixture.core)
	if err != nil {
		t.Fatal(err)
	}
	signature := regexp.MustCompile(`func PipeLangProject\(p0 ([A-Za-z0-9_]+)\)`).FindSubmatch(generated)
	if len(signature) != 2 {
		t.Fatal("generated Project signature absent")
	}
	recordName := string(signature[1])
	dir := t.TempDir()
	for name, data := range map[string][]byte{
		"go.mod":       []byte("module application-consumer-check\n\ngo 1.25\n"),
		"generated.go": generated,
		"generated_test.go": []byte("package " + gobackend.PackageName + "\n" + `import "testing"
func TestProject(t *testing.T) {
 for _,value:=range []string{"","   ","ready","other"} {
  input:=` + recordName + `{Identity:value}
  if got:=PipeLangProject(input);got!=input {t.Fatal(got)}
 }
}
`),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "test", "-count=1", "-p=1", "-timeout=25s", ".")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off")
	if output, err := containedexec.CombinedOutput(command); err != nil {
		t.Fatalf("generated consumer Go: %v\n%s", err, output)
	}
}
