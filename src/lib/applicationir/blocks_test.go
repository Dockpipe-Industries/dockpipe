package applicationir

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang"
	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/tests/containedexec"
)

// General blocks preserve the application snapshot boundary and canonical projection.
func TestV1140BlockApplicationConsumer(t *testing.T) {
	source, err := os.ReadFile("testdata/docker-observability.pipe")
	if err != nil {
		t.Fatal(err)
	}
	original := `public DockerSnapshot Project(DockerSnapshot snapshot) => snapshot;`
	helper := `public Mode ChooseMode(string key)=>key=="ready" ? Mode.Ready : Mode.Hidden;
 public string ChooseKey(string key)=>match(ChooseMode(key)){Mode.Ready=>"ready",Mode.Hidden=>"hidden"};
 public DockerSnapshot Project(DockerSnapshot snapshot){if(snapshot.Identity=="ready"){{return snapshot;}} {string key=ChooseKey(snapshot.Identity);} return snapshot;}`
	if strings.Count(string(source), original) != 1 {
		t.Fatal("fixture target absent")
	}
	changed := []byte(`public Enum Mode{Ready="ready";Hidden="hidden";} ` + strings.Replace(string(source), original, helper, 1))
	fixture := loadReviewApplicationSource(t, changed, pipelang.PipeLangLanguageContractV1140)
	baseline := loadReviewApplicationSource(t, source, pipelang.PipeLangLanguageContractV1140)
	app, err := Project(fixture.semantic, &fixture.core, fixture.spec)
	if err != nil {
		t.Fatal(err)
	}
	old, err := Project(baseline.semantic, &baseline.core, baseline.spec)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := CanonicalJSON(app)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := CanonicalJSON(old)
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatal("canonical Application IR changed", err)
	}
	var project, choose coreir.Function
	for _, f := range fixture.core.Functions {
		if f.Name == "Project" {
			project = f
		}
		if f.Name == "ChooseKey" {
			choose = f
		}
	}
	if choose.Body.Match == nil || choose.Body.Match.Value.Type.Kind != coreir.TypeEnum {
		t.Fatal("executable enum dependency missing")
	}
	generated, err := gobackend.Generate(fixture.core)
	if err != nil {
		t.Fatal(err)
	}
	signature := regexp.MustCompile(`func PipeLangProject\(p0 ([A-Za-z0-9_]+)\)`).FindSubmatch(generated)
	if len(signature) != 2 {
		t.Fatal("missing Project signature")
	}
	var checks strings.Builder
	for _, raw := range []string{"", "ready", "hidden", " ready ", "unknown"} {
		record := coreeval.Value{Type: project.Parameters[0].Type, Record: []coreeval.Value{{Type: project.Parameters[0].Type.Record.Fields[0].Type, String: raw}}}
		got, err := coreeval.EvaluateProgram(fixture.core, project.Identity, []coreeval.Value{record})
		if err != nil || !got.OK || !reflect.DeepEqual(got.Value, record) {
			t.Fatal(got, err)
		}
		want := "hidden"
		if raw == "ready" {
			want = "ready"
		}
		result, err := coreeval.EvaluateProgram(fixture.core, choose.Identity, []coreeval.Value{{Type: choose.Parameters[0].Type, String: raw}})
		if err != nil || !result.OK || result.Value.String != want {
			t.Fatal(result, err)
		}
		fmt.Fprintf(&checks, "{input:=%s{Identity:%q};if got:=PipeLangProject(input);got!=input{t.Fatal(got)}}\nif PipeLangChooseKey(%q)!=%q{t.Fatal(\"enum helper\")}\n", signature[1], raw, raw, want)
	}
	dir := t.TempDir()
	for name, data := range map[string][]byte{"go.mod": []byte("module application-enum-check\n\ngo 1.25\n"), "generated.go": generated, "generated_test.go": []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestConsumer(t *testing.T){%s}", gobackend.PackageName, checks.String()))} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command(filepath.Join(containedexec.GoRoot(t), "bin", "go"), "test", "-count=1", "-p=1", "-timeout=25s", ".")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off")
	if output, err := containedexec.CombinedOutput(command); err != nil {
		t.Fatalf("consumer generated Go: %v\n%s", err, output)
	}
}
