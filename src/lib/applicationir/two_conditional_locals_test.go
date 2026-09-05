package applicationir

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang"
	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

func TestV830TwoConditionalLocalsApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, false)
}

func TestV840FiniteConditionalLocalsApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, true)
}

func testConditionalLocalsApplicationConsumer(t *testing.T, finite bool) {
	contract, prior, boundary := pipelang.PipeLangLanguageContractV830, pipelang.PipeLangLanguageContractV820, "one ternary"
	if finite {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV840, pipelang.PipeLangLanguageContractV830, "at most two"
	}
	for _, descendant := range []bool{false, true} {
		t.Run(fmt.Sprintf("descendant=%t", descendant), func(t *testing.T) {
			source, err := os.ReadFile("testdata/docker-observability.pipe")
			if err != nil {
				t.Fatal(err)
			}
			original := `public DockerSnapshot Project(DockerSnapshot snapshot) => snapshot;`
			choice := `string selected = suffix && normalized != "" ? normalized + "!" : normalized;`
			helper := `public string ChooseKey(string raw,bool clean,bool suffix,bool enabled) {
    string normalized = clean ? trim(raw) : raw;
   `
			if descendant {
				helper += `if(enabled){` + choice + `return selected;}else{return normalized;}`
			} else {
				helper += choice + `if(enabled){return selected;}else{return normalized;}`
			}
			if finite {
				helper = strings.Replace(helper, choice, choice+`string third=enabled && selected != "" ? selected+"?" : selected;string fourth=clean && third != "" ? third+"#" : third;`, 1)
				helper = strings.Replace(helper, "return selected;", "return fourth;", 1)
			}
			helper += `}
   public DockerSnapshot Project(DockerSnapshot snapshot) {
    string key = snapshot.Identity;
    bool clean = key != "raw";
    bool suffix = key != "plain";
    bool enabled = key != "disabled";
    string selected = ChooseKey(key,clean,suffix,enabled);
    if(selected == "ready!"){return snapshot;}else{return snapshot;}
   }`
			if strings.Count(string(source), original) != 1 {
				t.Fatal("fixture target absent")
			}
			changed := []byte(strings.Replace(string(source), original, helper, 1))
			// The prior version must reject this consumer at its retained choice bound.
			module := pipelang.ModuleInput{ID: "app.root", Namespace: "app.root", DeclarationSpan: pipelang.Span{File: "docker-observability.pipe"}, Sources: []pipelang.SourceInput{{Path: "docker-observability.pipe", Data: changed}}}
			input := pipelang.ModuleSetInput{LanguageContract: prior, PackageID: "docker.observability", Root: "app.root", Modules: []pipelang.ModuleInput{module}}
			input.Lock.Modules = []pipelang.LockedModule{{ID: module.ID, SourceSHA256: pipelang.ModuleSourceSHA256(module.Sources), SemanticSHA256: pipelang.ModuleSemanticSHA256(input.PackageID, module.Namespace, nil)}}
			if err := pipelang.AnalyzeSemanticModuleSet(input).Error(); err == nil || !strings.Contains(err.Error(), boundary) {
				t.Fatalf("%s consumer boundary: %v", prior, err)
			}
			fixture := loadReviewApplicationSource(t, changed, contract)
			again := loadReviewApplicationSource(t, changed, contract)
			if !reflect.DeepEqual(fixture.semantic, again.semantic) || !reflect.DeepEqual(fixture.core, again.core) {
				t.Fatal("nondeterministic semantic/Core")
			}
			if fixture.semantic.Schema != "pipelang.semantic.v1" || fixture.semantic.CompilerContract != "pipelang.compiler.v1" || fixture.core.CompilerContract != "pipelang.compiler.v1" || fixture.core.LanguageContract != string(contract) {
				t.Fatal("identity drift")
			}
			app, err := Project(fixture.semantic, &fixture.core, fixture.spec)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := CanonicalJSON(app)
			if err != nil {
				t.Fatal(err)
			}
			baseline := loadReviewApplicationSource(t, source, contract)
			old, err := Project(baseline.semantic, &baseline.core, baseline.spec)
			if err != nil {
				t.Fatal(err)
			}
			expected, err := CanonicalJSON(old)
			if err != nil || !bytes.Equal(actual, expected) || app.Schema != "dockpipe.application.v1" {
				t.Fatal("canonical Application IR changed")
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
			if project.Body.ImmutableLocal == nil || choose.Body.ImmutableLocal == nil {
				t.Fatal("executable dependency missing")
			}
			generated, err := gobackend.Generate(fixture.core)
			if err != nil {
				t.Fatal(err)
			}
			repeated, err := gobackend.Generate(again.core)
			if err != nil || !bytes.Equal(generated, repeated) {
				t.Fatal("nondeterministic Go")
			}
			signature := regexp.MustCompile(`func PipeLangProject\(p0 ([A-Za-z0-9_]+)\)`).FindSubmatch(generated)
			if len(signature) != 2 {
				t.Fatal("generated signature absent")
			}
			var checks strings.Builder
			for _, raw := range []string{"", "   ", " ready ", "raw", "plain", "disabled"} {
				record := coreeval.Value{Type: project.Parameters[0].Type, Record: []coreeval.Value{{Type: project.Parameters[0].Type.Record.Fields[0].Type, String: raw}}}
				got, err := coreeval.EvaluateProgram(fixture.core, project.Identity, []coreeval.Value{record})
				if err != nil || !got.OK || !reflect.DeepEqual(got.Value, record) {
					t.Fatalf("snapshot %q: %#v %v", raw, got, err)
				}
				fmt.Fprintf(&checks, "{input:=%s{Identity:%q};if got:=PipeLangProject(input);got!=input{t.Fatal(got)}}\n", signature[1], raw)
				// Assert helper results independently; identical snapshot branches cannot hide wrong choices.
				for mask := 0; mask < 8; mask++ {
					normalized := raw
					if mask&1 != 0 {
						normalized = strings.TrimSpace(raw)
					}
					want := normalized
					if mask&2 != 0 && mask&4 != 0 && normalized != "" {
						want += "!"
					}
					if finite && mask&4 != 0 && want != "" {
						want += "?"
						if mask&1 != 0 {
							want += "#"
						}
					}
					args := []coreeval.Value{{Type: choose.Parameters[0].Type, String: raw}}
					for bit := 0; bit < 3; bit++ {
						args = append(args, coreeval.Value{Type: choose.Parameters[bit+1].Type, Bool: mask&(1<<bit) != 0})
					}
					got, err := coreeval.EvaluateProgram(fixture.core, choose.Identity, args)
					if err != nil || !got.OK || got.Value.String != want {
						t.Fatalf("helper %q/%d: %#v %v", raw, mask, got, err)
					}
					standalone, err := coreeval.Evaluate(choose, args)
					if err != nil || !reflect.DeepEqual(standalone, got) {
						t.Fatal("standalone helper disagrees")
					}
					fmt.Fprintf(&checks, "if got:=PipeLangChooseKey(%q,%t,%t,%t);got!=%q{t.Fatal(got)}\n", raw, mask&1 != 0, mask&2 != 0, mask&4 != 0, want)
				}
			}
			dir := t.TempDir()
			for name, data := range map[string][]byte{"go.mod": []byte("module application-choice-check\n\ngo 1.25\n"), "generated.go": generated, "generated_test.go": []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestConsumer(t *testing.T){%s}", gobackend.PackageName, checks.String()))} {
				if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "test", ".")
			command.Dir = dir
			command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off")
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("consumer generated Go: %v\n%s", err, output)
			}
		})
	}
}
