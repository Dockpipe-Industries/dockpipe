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
	"dockpipe/tests/containedexec"
)

func TestV830TwoConditionalLocalsApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, false)
}

func TestV840FiniteConditionalLocalsApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, true)
}

func TestV850StraightLineConditionalLocalsApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, true, true)
}

func TestV860ConditionalReturnCompositionApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, true, true, true)
}

func TestV870TerminalLeafConditionalReturnsApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, true, true, true, true)
}

func TestV880NestedStraightLineReturnsApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, true, true, true, false, true)
}

func TestV890NestedTerminalLeafReturnsApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, true, true, true, false, true, true)
}

func TestV900NestedStraightLineInitializersApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, true, true, true, false, true, false, true)
}

func TestV910NestedTerminalInitializersApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, true, true, true, false, true, false, true, true)
}

func TestV920NestedArrowMethodsApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, false, true, false, false, false, false, false, false, true)
}

func TestV930DepthThreeReturnsApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, true, true, true, false, true, false, true, false, false, true)
}

func TestV940DepthThreeTerminalLeafReturnsApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, true, true, true, false, true, false, true, false, false, true, true)
}

func TestV950DepthThreeArrowMethodsApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, false, true, false, false, false, false, false, false, true, false, false, true)
}

func TestV960DepthThreeStraightLineInitializersApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, true, true, true, false, true, false, false, false, false, true, false, false, true)
}

func TestV970DepthThreeTerminalInitializersApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, true, true, true, false, true, false, false, false, false, true, false, false, true, true)
}

func TestV980ConditionalBooleanSelectorsApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, false, true, false, false, false, false, false, false, false, false, false, false, false, false, true)
}

func TestV990TerminalLeafBooleanSelectorsApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, false, true, false, false, false, false, false, false, false, false, false, false, false, false, true, true)
}

func TestV1000ArrowBooleanSelectorsApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, false, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true)
}

func TestV1010StraightLineBooleanSelectorInitializersApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, false, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true)
}

func TestV1020TerminalBooleanSelectorInitializersApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, false, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, true)
}

func TestV1030TerminalConditionalTestsApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, false, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, false, true)
}

func TestV1040NestedTerminalConditionalTestsApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, false, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, false, true, true)
}
func TestV1050DepthThreeTerminalConditionalTestsApplicationConsumer(t *testing.T) {
	testConditionalLocalsApplicationConsumer(t, false, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, false, true, true, true)
}

func testConditionalLocalsApplicationConsumer(t *testing.T, finite bool, straightOption ...bool) {
	straight := len(straightOption) > 0 && straightOption[0]
	returnChoice := len(straightOption) > 1 && straightOption[1]
	contract, prior, boundary := pipelang.PipeLangLanguageContractV830, pipelang.PipeLangLanguageContractV820, "one ternary"
	if finite {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV840, pipelang.PipeLangLanguageContractV830, "at most two"
	}
	if straight {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV850, pipelang.PipeLangLanguageContractV840, "exactly one conditional"
	}
	if returnChoice {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV860, pipelang.PipeLangLanguageContractV850, "ordinary return"
	}
	leafReturns := len(straightOption) > 2 && straightOption[2]
	if leafReturns {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV870, pipelang.PipeLangLanguageContractV860, "new return/condition/argument placements"
	}
	nestedReturns := len(straightOption) > 3 && straightOption[3]
	if nestedReturns {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV880, pipelang.PipeLangLanguageContractV870, "nested ternaries"
	}
	nestedLeaves := len(straightOption) > 4 && straightOption[4]
	if nestedLeaves {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV890, pipelang.PipeLangLanguageContractV880, "nested ternaries"
	}
	nestedInitializers := len(straightOption) > 5 && straightOption[5]
	if nestedInitializers {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV900, pipelang.PipeLangLanguageContractV890, "nested initializers"
	}
	nestedTreeInitializers := len(straightOption) > 6 && straightOption[6]
	if nestedTreeInitializers {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV910, pipelang.PipeLangLanguageContractV900, "nested ternaries"
	}
	arrow := len(straightOption) > 7 && straightOption[7]
	if arrow {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV920, pipelang.PipeLangLanguageContractV910, "block-bodied method"
	}
	depthThree := len(straightOption) > 8 && straightOption[8]
	if depthThree {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV930, pipelang.PipeLangLanguageContractV920, "depth-two"
	}
	depthThreeLeaves := len(straightOption) > 9 && straightOption[9]
	if depthThreeLeaves {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV940, pipelang.PipeLangLanguageContractV930, "nested ternaries"
	}
	depthThreeArrow := len(straightOption) > 10 && straightOption[10]
	if depthThreeArrow {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV950, pipelang.PipeLangLanguageContractV940, "block-bodied method"
	}
	depthThreeInitializers := len(straightOption) > 11 && straightOption[11]
	if depthThreeInitializers {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV960, pipelang.PipeLangLanguageContractV950, "initializers retain depth two"
	}

	depthThreeTreeInitializers := len(straightOption) > 12 && straightOption[12]
	if depthThreeTreeInitializers {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV970, pipelang.PipeLangLanguageContractV960, "nested ternaries"
	}

	booleanSelector := len(straightOption) > 13 && straightOption[13]
	if booleanSelector {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV980, pipelang.PipeLangLanguageContractV970, "new condition/argument placements"
	}
	leafBooleanSelector := len(straightOption) > 14 && straightOption[14]
	if leafBooleanSelector {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV990, pipelang.PipeLangLanguageContractV980, "new argument/condition placements"
	}
	arrowBooleanSelector := len(straightOption) > 15 && straightOption[15]
	if arrowBooleanSelector {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV1000, pipelang.PipeLangLanguageContractV990, "require a block-bodied method"
	}
	selectorInitializer := len(straightOption) > 16 && straightOption[16]
	if selectorInitializer {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV1010, pipelang.PipeLangLanguageContractV1000, "new initializer"
	}
	terminalSelectorInitializer := len(straightOption) > 17 && straightOption[17]
	if terminalSelectorInitializer {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV1020, pipelang.PipeLangLanguageContractV1010, "new argument/condition placements"
	}
	conditionalTests := len(straightOption) > 18 && straightOption[18]
	if conditionalTests {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV1030, pipelang.PipeLangLanguageContractV1020, "new argument/condition placements"
	}
	nestedTests := len(straightOption) > 19 && straightOption[19]
	if nestedTests {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV1040, pipelang.PipeLangLanguageContractV1030, "new argument/condition placements"
	}
	depthThreeTests := len(straightOption) > 20 && straightOption[20]
	if depthThreeTests {
		contract, prior, boundary = pipelang.PipeLangLanguageContractV1050, pipelang.PipeLangLanguageContractV1040, "new argument/condition placements"
	}
	layouts := []bool{false, true}
	if straight {
		layouts = []bool{false}
	}
	for _, descendant := range layouts {
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
			if straight {
				helper = `public string ChooseKey(string raw,bool clean,bool suffix,bool enabled){
 string normalized=clean ? trim(raw) : raw;
 string selected=suffix && normalized != "" ? normalized+"!" : normalized;
 string third=enabled && selected != "" ? selected+"?" : selected;
 string fourth=clean && third != "" ? third+"#" : third;
 string result=enabled ? fourth : normalized;
 return result;`
			}
			if returnChoice {
				helper = strings.Replace(helper, "string result=enabled ? fourth : normalized;\n return result;", "return enabled ? fourth : normalized;", 1)
			}
			if leafReturns {
				helper = strings.Replace(helper, "return enabled ? fourth : normalized;", "if(enabled){return fourth != \"\" ? fourth : normalized;}else{return clean ? normalized : raw;}", 1)
			}
			if nestedReturns {
				before := helper
				helper = strings.Replace(helper, "return enabled ? fourth : normalized;", `return enabled ? (suffix ? fourth : third) : (clean ? normalized : raw);`, 1)
				if helper == before {
					t.Fatal("nested consumer replacement missed")
				}
			}
			if nestedLeaves {
				helper = strings.Replace(helper, "return enabled ? (suffix ? fourth : third) : (clean ? normalized : raw);", "if(enabled){return suffix ? (clean ? fourth : fourth) : third;}else{return clean ? normalized : (suffix ? raw : raw);}", 1)
			}
			if nestedInitializers {
				helper = strings.Replace(helper, "normalized=clean ? trim(raw) : raw;", "normalized = clean ? (suffix ? trim(raw) : trim(raw)) : (enabled ? raw : raw);", 1)
				helper = strings.Replace(helper, "selected=suffix && normalized != \"\" ? normalized+\"!\" : normalized;", "selected = suffix && normalized != \"\" ? (enabled ? normalized + \"!\" : normalized + \"!\") : normalized;", 1)
			}
			if depthThreeInitializers {
				helper = strings.Replace(helper, "normalized=clean ? trim(raw) : raw;", `normalized=clean ? (suffix ? (enabled ? trim(raw) : trim(raw)) : trim(raw)) : (enabled ? (suffix ? raw : raw) : raw);`, 1)
				helper = strings.Replace(helper, `selected=suffix && normalized != "" ? normalized+"!" : normalized;`, `selected=suffix && normalized != "" ? (enabled ? (clean ? normalized+"!" : normalized+"!") : normalized+"!") : normalized;`, 1)
			}
			if nestedTreeInitializers {
				open := strings.Index(helper, "{")
				body := helper[open+1:]
				helper = helper[:open+1] + "if(enabled){" + body + "}else{" + body + "}"
			}
			if arrow {
				helper = `public string ChooseKey(string raw,bool clean,bool suffix,bool enabled)=>
 enabled ? (clean ? trim(raw) : raw) : (suffix ? raw+"!" : raw);`
			}
			if depthThree {
				before := helper
				helper = strings.Replace(helper, "suffix ? fourth : third", "suffix ? (clean ? fourth : third) : third", 1)
				if helper == before {
					t.Fatal("depth-three consumer replacement missed")
				}
			}
			if depthThreeLeaves || depthThreeTreeInitializers {
				open := strings.Index(helper, "{")
				body := helper[open+1:]
				helper = helper[:open+1] + "if(enabled){" + body + "}else{" + body + "}"
			}
			if depthThreeArrow {
				helper = `public string ChooseKey(string raw,bool clean,bool suffix,bool enabled)=>enabled ? (clean ? (suffix ? trim(raw) : trim(raw)) : raw) : (suffix ? raw+"!" : raw);`
			}
			if booleanSelector {
				helper = `public string ChooseKey(string raw,bool clean,bool suffix,bool enabled){
 string normalized=clean ? (suffix ? (enabled ? trim(raw) : trim(raw)) : trim(raw)) : raw;
 return (clean ? suffix : enabled) ? normalized+"!" : normalized;`
			}
			if leafBooleanSelector {
				helper = strings.Replace(helper, "return (clean ? suffix : enabled) ? normalized+\"!\" : normalized;", "if(clean){return (clean ? suffix : enabled) ? normalized+\"!\" : normalized;}else{return (clean ? suffix : enabled) ? normalized+\"!\" : normalized;}", 1)
			}
			if arrowBooleanSelector {
				helper = `public string ChooseKey(string raw,bool clean,bool suffix,bool enabled)=>(clean ? suffix : enabled) ? raw+"!" : raw;`
			}
			if selectorInitializer {
				helper = `public string ChooseKey(string raw,bool clean,bool suffix,bool enabled){string selected=(clean ? suffix : enabled) ? raw+"!" : raw;return selected;`
			}
			if terminalSelectorInitializer {
				helper = `public string ChooseKey(string raw,bool clean,bool suffix,bool enabled){
                 string root=(clean ? suffix : enabled) ? raw+"!" : raw;
                 if(clean){string mid=(clean ? suffix : enabled) ? raw+"!" : raw;
                  if(suffix){if(enabled){string leaf=(clean ? suffix : enabled) ? raw+"!" : raw;return leaf;}else{return mid;}}else{return root;}
                 }else{string other=(clean ? suffix : enabled) ? raw+"!" : raw;return other;}`
			}
			if conditionalTests {
				helper = `public string ChooseKey(string raw,bool clean,bool suffix,bool enabled){string root=raw;
 if(clean ? suffix : enabled){bool mid=suffix;
 if(mid ? clean : enabled){if(enabled ? clean : suffix){return root+"!";}else{return root+"!";}}else{return root+"!";}
 }else{return root;}`
			}
			if nestedTests {
				helper = strings.Replace(helper, "clean ? suffix : enabled", "clean ? (suffix ? true : false) : (enabled ? true : false)", 1)
				helper = strings.Replace(helper, "mid ? clean : enabled", "mid ? (clean ? true : false) : enabled", 1)
				helper = strings.Replace(helper, "enabled ? clean : suffix", "enabled ? clean : (suffix ? true : false)", 1)
			}
			if depthThreeTests {
				helper = strings.ReplaceAll(helper, "suffix ? true : false", "suffix ? (clean ? true : true) : (enabled ? false : false)")
				helper = strings.ReplaceAll(helper, "enabled ? true : false", "enabled ? (suffix ? true : true) : (clean ? false : false)")
				helper = strings.ReplaceAll(helper, "clean ? true : false", "clean ? (enabled ? true : true) : (suffix ? false : false)")
			}
			closing := "}"
			if arrow || arrowBooleanSelector {
				closing = ""
			}
			helper += closing + `
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
			choicePresent := choose.Body.ImmutableLocal != nil
			if nestedTreeInitializers || depthThreeLeaves || depthThreeTreeInitializers {
				choicePresent = choose.Body.Conditional != nil && choose.Body.Conditional.TerminalStatement && choose.Body.Conditional.WhenTrue != nil && choose.Body.Conditional.WhenTrue.ImmutableLocal != nil && choose.Body.Conditional.WhenFalse != nil && choose.Body.Conditional.WhenFalse.ImmutableLocal != nil
			}
			if arrow {
				choicePresent = choose.Body.Conditional != nil && !choose.Body.Conditional.TerminalStatement && choose.Body.Conditional.WhenTrue.Conditional != nil && choose.Body.Conditional.WhenFalse.Conditional != nil
			}

			if arrowBooleanSelector {
				choicePresent = choose.Body.Conditional != nil && !choose.Body.Conditional.TerminalStatement && choose.Body.Conditional.Condition.Conditional != nil
			}
			if selectorInitializer {
				choicePresent = choose.Body.ImmutableLocal != nil && choose.Body.ImmutableLocal.Initializer != nil && choose.Body.ImmutableLocal.Initializer.Conditional != nil && choose.Body.ImmutableLocal.Initializer.Conditional.Condition.Conditional != nil
			}
			if terminalSelectorInitializer {
				choicePresent = choicePresent && choose.Body.ImmutableLocal.Return != nil && choose.Body.ImmutableLocal.Return.Conditional != nil && choose.Body.ImmutableLocal.Return.Conditional.TerminalStatement && choose.Body.ImmutableLocal.Return.Conditional.WhenTrue.ImmutableLocal != nil
			}
			if conditionalTests {
				choicePresent = choose.Body.ImmutableLocal != nil && choose.Body.ImmutableLocal.Return.Conditional != nil && choose.Body.ImmutableLocal.Return.Conditional.Condition.Conditional != nil
			}
			if nestedTests && choicePresent {
				test := choose.Body.ImmutableLocal.Return.Conditional.Condition.Conditional
				choicePresent = test.WhenTrue.Conditional != nil && test.WhenFalse.Conditional != nil
			}
			if depthThreeTests && choicePresent {
				test := choose.Body.ImmutableLocal.Return.Conditional.Condition.Conditional
				choicePresent = test.WhenTrue.Conditional.WhenTrue.Conditional != nil && test.WhenFalse.Conditional.WhenFalse.Conditional != nil
			}
			if depthThreeArrow && choicePresent {
				choicePresent = choose.Body.Conditional.WhenTrue.Conditional.WhenTrue != nil && choose.Body.Conditional.WhenTrue.Conditional.WhenTrue.Conditional != nil
			}
			if depthThreeLeaves && choicePresent {
				for _, branch := range []*coreir.Expr{choose.Body.Conditional.WhenTrue, choose.Body.Conditional.WhenFalse} {
					tail := branch
					for tail != nil && tail.ImmutableLocal != nil {
						tail = tail.ImmutableLocal.Return
					}
					if tail == nil || tail.Conditional == nil || tail.Conditional.WhenTrue == nil || tail.Conditional.WhenTrue.Conditional == nil || tail.Conditional.WhenTrue.Conditional.WhenTrue == nil || tail.Conditional.WhenTrue.Conditional.WhenTrue.Conditional == nil {
						t.Fatal("depth-three leaf absent")
					}
				}
			}
			if depthThreeInitializers {
				body := choose.Body
				if depthThreeTreeInitializers {
					body = *body.Conditional.WhenTrue
				}
				init := body.ImmutableLocal.Initializer
				if init.Conditional == nil || init.Conditional.WhenTrue.Conditional == nil || init.Conditional.WhenTrue.Conditional.WhenTrue.Conditional == nil {
					t.Fatal("depth-three initializer absent")
				}
			}
			if project.Body.ImmutableLocal == nil || !choicePresent {
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
						if mask&1 != 0 && (!nestedReturns || mask&2 != 0) {
							want += "#"
						}
					}
					if arrow {
						want = raw
						if mask&4 != 0 {
							if mask&1 != 0 {
								want = strings.TrimSpace(raw)
							}
						} else if mask&2 != 0 {
							want = raw + "!"
						}
					}
					if booleanSelector {
						want = normalized
						selected := mask&4 != 0
						if mask&1 != 0 {
							selected = mask&2 != 0
						}
						if selected {
							want += "!"
						}
					}
					if arrowBooleanSelector || selectorInitializer {
						picked := mask&4 != 0
						if mask&1 != 0 {
							picked = mask&2 != 0
						}
						want = raw
						if picked {
							want = raw + "!"
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
			command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "test", "-count=1", "-p=1", "-timeout=25s", ".")
			command.Dir = dir
			command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off")
			if output, err := containedexec.CombinedOutput(command); err != nil {
				t.Fatalf("consumer generated Go: %v\n%s", err, output)
			}
		})
	}
}
