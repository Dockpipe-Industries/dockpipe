package pipelang

import (
	"bytes"
	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestV1130Enums(t *testing.T) {
	for _, body := range []string{
		`public Mode Select(Mode value)=>value;`,
		`public Mode Select()=>Mode.Idle;`,
		`public bool Select(Mode value)=>value == Mode.Idle;`,
		`public string Select(Mode value)=>match(value){Mode.Idle=>"idle",Mode.Busy=>"busy"};`,
	} {
		t.Run(body, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1130, `public Enum Mode { Idle="idle"; Busy="busy"; } public Class Choices {`+body+`}`, []string{"Select"})
			if err := coreir.ValidateProgram(p); err != nil {
				t.Fatal(err)
			}
			if _, err := gobackend.Generate(p); err != nil {
				t.Fatal(err)
			}
			f := p.Functions[len(p.Functions)-1]
			var args []coreeval.Value
			for _, a := range f.Parameters {
				args = append(args, coreeval.Value{Type: a.Type, String: "idle"})
			}
			result, err := coreeval.EvaluateProgram(p, f.Identity, args)
			if err != nil || !result.OK {
				t.Fatalf("%+v %v", result, err)
			}
		})
	}
}

func enumAnalysis(source string) *Analysis {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "enum.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV1130
	return AnalyzeSemanticModuleSet(input)
}

func TestV1130EnumRejections(t *testing.T) {
	head := `public Enum Mode { Idle="idle"; Busy="busy"; } public Enum Other { Idle="idle"; } `
	for _, source := range []string{
		`public Enum Mode {} public Class Choices{ public bool Select()=>true; }`,
		`private Enum Mode { Idle="idle"; } public Class Choices{public bool Select()=>true;}`,
		`public Enum Mode { Idle=""; } public Class Choices{public bool Select()=>true;}`,
		`public Enum Mode { Idle="idle"; Busy="idle"; } public Class Choices{public bool Select()=>true;}`,
		`public Enum Mode { Idle="idle"; Idle="busy"; } public Class Choices{public bool Select()=>true;}`,
		`public Enum Mode { Idle; } public Class Choices{public bool Select()=>true;}`,
		`public Enum Mode { Idle=1; } public Class Choices{public bool Select()=>true;}`,
		head + `public Class Choices { public bool Select(Mode x)=>x == Other.Idle; }`,
		head + `public Class Choices { public bool Select(Mode x)=>x < Mode.Idle; }`,
		head + `public Class Choices { public Mode Select()=>Mode.Missing; }`,
		head + `public Class Choices { public Mode Select()=>"idle"; }`,
		head + `public Class Choices { public string Select(Mode x)=>match(x){Mode.Idle=>"a"}; }`,
		head + `public Class Choices { public string Select(Mode x)=>match(x){Mode.Idle=>"a",Mode.Idle=>"b",Mode.Busy=>"c"}; }`,
		head + `public Class Choices { public string Select(Mode x)=>match(x){Mode.Idle=>"a",Other.Idle=>"b"}; }`,
		head + `public Class Choices { public string Select(Mode x)=>match(x){Mode.Idle=>"a",_=>"b"}; }`,
		head + `public Class Choices { public string Select(Mode x)=>match(x){Mode.Idle(v)=>"a",Mode.Busy=>"b"}; }`,
		head + `public Class Choices { public string Select(Mode x)=>match(x){Mode.Idle=>"a",Mode.Busy=>false}; }`,
		head + `public Class Choices { public Mode value; public bool Select()=>true; }`,
		head + `public Class Choices { private Mode Select(Mode x)=>x; }`,
	} {
		t.Run(source, func(t *testing.T) {
			a := enumAnalysis(source)
			if a.Error() == nil {
				t.Fatal("invalid enum source accepted")
			}
			if _, ok := AsDiagnostics(a.Error()); !ok {
				t.Fatal("unstructured error", a.Error())
			}
		})
	}
	for _, version := range []LanguageContract{LegacyLanguageContract, PipeLangLanguageContractV010, PipeLangLanguageContractV1120} {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "enum.pipe", head+`public Class Choices{public bool Select()=>true;}`)}, nil)
		input.LanguageContract = version
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("old source version accepted enums", version)
		}
	}
}

func TestV1130EnumComposition(t *testing.T) {
	for _, body := range []string{
		`public Mode Select(Mode value,bool flag)=>flag ? value : Mode.Busy;`,
		`public Mode Select(Mode value,bool flag){ Mode selected=flag ? Mode.Busy : value; return selected; }`,
		`public Mode Echo(Mode value)=>value; public Mode Select(Mode value,bool flag)=>Echo(flag ? Mode.Busy : value);`,
		`public Mode Echo(Mode value)=>value; public string Select(Mode value,bool flag)=>match(Echo(value)){Mode.Idle=>flag ? "a" : "b",Mode.Busy=>"c"};`,
		`public string Select(Mode value,bool flag){string result=match(value){Mode.Idle=>"a",Mode.Busy=>"b"};return flag ? result : "c";}`,
		`public Mode Select(Mode value,bool flag)=>match(value){Mode.Idle=>Mode.Busy,Mode.Busy=>Mode.Idle};`,
	} {
		t.Run(body, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1130, `public Enum Mode{Idle="idle";Busy="busy";} public Class Choices{`+body+`}`, []string{"Select"})
			if _, err := gobackend.Generate(p); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestV1130EnumNative(t *testing.T) {
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1130, `public Enum Mode{Idle="_";Busy="忙";} public Class Choices{
 public Mode Echo(Mode value)=>value;
 public string Select(Mode value,bool flag){Mode chosen=flag ? value : Mode.Busy;return match(Echo(chosen)){Mode.Idle=>"idle",Mode.Busy=>"busy"};}
 public bool Equal(Mode left,Mode right)=>left==right;
 public Mode Construct()=>Mode.Idle;
 }`, []string{"Select", "Equal", "Construct"})
	generated, err := gobackend.Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(`package pipelanggenerated
 import "testing"
 func TestEnum(t *testing.T){
 if PipeLangSelect("_",true)!="idle" || PipeLangSelect("_",false)!="busy" || PipeLangSelect("忙",true)!="busy" {t.Fatal("selection")}
 if !PipeLangEqual("_","_") || PipeLangEqual("_","忙") {t.Fatal("nominal equality")}
 if PipeLangConstruct()!="_"{t.Fatal("construction")}
 func(){defer func(){if recover()==nil{t.Fatal("invalid enum argument accepted")}}();PipeLangSelect("invalid",false)}()
 }`))
	f := coreFunctionNamed(t, p, "Select")
	for _, test := range []struct {
		tag  string
		flag bool
		want string
	}{{"_", true, "idle"}, {"_", false, "busy"}, {"忙", true, "busy"}} {
		got, err := coreeval.EvaluateProgram(p, f.Identity, []coreeval.Value{{Type: f.Parameters[0].Type, String: test.tag}, {Type: f.Parameters[1].Type, Bool: test.flag}})
		if err != nil || !got.OK || got.Value.String != test.want {
			t.Fatalf("%+v %v", got, err)
		}
	}
	if _, err := coreeval.EvaluateProgram(p, f.Identity, []coreeval.Value{{Type: f.Parameters[0].Type, String: "invalid"}, {Type: f.Parameters[1].Type}}); err == nil {
		t.Fatal("evaluator accepted invalid unused enum")
	}
}

func TestV1130EnumCoreBoundary(t *testing.T) {
	source := `public Enum Mode{Idle="idle";Busy="busy";} public Class Choices{public Mode Construct()=>Mode.Idle;public string Select(Mode value)=>match(value){Mode.Idle=>"idle",Mode.Busy=>"busy"};}`
	_, base := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1130, source, []string{"Construct", "Select"})
	encoded, _ := json.Marshal(base)
	cases := map[string]func(*coreir.Program){
		"old contract":           func(p *coreir.Program) { p.LanguageContract = coreir.LanguageContractV1120 },
		"nil enum":               func(p *coreir.Program) { p.Functions[0].ReturnType.Enum = nil },
		"foreign representation": func(p *coreir.Program) { p.Functions[0].ReturnType.Primitive = coreir.PrimitiveString },
		"no identity":            func(p *coreir.Program) { p.Functions[0].ReturnType.Identity = nil },
		"duplicate tag":          func(p *coreir.Program) { m := p.Functions[0].ReturnType.Enum.Members; m[1].Tag = m[0].Tag },
		"duplicate name":         func(p *coreir.Program) { m := p.Functions[0].ReturnType.Enum.Members; m[1].Name = m[0].Name },
		"duplicate identity":     func(p *coreir.Program) { m := p.Functions[0].ReturnType.Enum.Members; m[1].Identity = m[0].Identity },
		"wrong member owner":     func(p *coreir.Program) { p.Functions[0].ReturnType.Enum.Members[0].Identity.PackageID = "foreign" },
		"noncanonical order":     func(p *coreir.Program) { m := p.Functions[0].ReturnType.Enum.Members; m[0], m[1] = m[1], m[0] },
		"invalid member name":    func(p *coreir.Program) { p.Functions[0].ReturnType.Enum.Members[0].Name = "not a name" },
		"empty member path": func(p *coreir.Program) {
			typ := &p.Functions[0].ReturnType
			typ.Enum.Members[0].Identity.Path = typ.Identity.Path + "."
		},
		"empty tag":          func(p *coreir.Program) { p.Functions[0].ReturnType.Enum.Members[0].Tag = "" },
		"invalid utf8":       func(p *coreir.Program) { p.Functions[0].ReturnType.Enum.Members[0].Tag = string([]byte{255}) },
		"unknown literal":    func(p *coreir.Program) { p.Functions[0].Body.Literal.String = "missing" },
		"ordinal payload":    func(p *coreir.Program) { p.Functions[0].Body.Literal.Int = 1 },
		"conflicting schema": func(p *coreir.Program) { p.Functions[0].Body.Type.Enum.Members[0].Name = "Renamed" },
		"missing arm":        func(p *coreir.Program) { p.Functions[1].Body.Match.Arms = p.Functions[1].Body.Match.Arms[:1] },
		"duplicate arm":      func(p *coreir.Program) { p.Functions[1].Body.Match.Arms[1].Tag = p.Functions[1].Body.Match.Arms[0].Tag },
		"unknown arm":        func(p *coreir.Program) { p.Functions[1].Body.Match.Arms[0].Tag = "missing" },
		"wildcard arm":       func(p *coreir.Program) { p.Functions[1].Body.Match.Arms[0].Tag = "_" },
		"payload binding":    func(p *coreir.Program) { zero := 0; p.Functions[1].Body.Match.Arms[0].Binding = &zero },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			var p coreir.Program
			if err := json.Unmarshal(encoded, &p); err != nil {
				t.Fatal(err)
			}
			mutate(&p)
			if err := coreir.ValidateProgram(p); err == nil {
				t.Fatal("forged Core accepted")
			}
			if _, err := gobackend.Generate(p); err == nil {
				t.Fatal("Go accepted forged Core")
			}
			if _, err := coreeval.PrepareProgram(p); err == nil {
				t.Fatal("evaluator prepared forged Core")
			}
		})
	}
}

func TestV1130EnumIdentityAndOrder(t *testing.T) {
	const tail = `public Class Choices{public Mode Select(Mode value)=>match(value){Mode.Idle=>Mode.Busy,Mode.Busy=>Mode.Idle};}`
	var previous []byte
	var previousType coreir.Type
	for _, members := range []string{`Idle="idle";Busy="busy";`, `Busy="busy";Idle="idle";`} {
		a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1130, `public Enum Mode{`+members+`}`+tail, []string{"Select"})
		projection, err := BuildSemanticProjection(a)
		if err != nil {
			t.Fatal(err)
		}
		found := 0
		for _, module := range projection.Modules {
			for _, typ := range module.Types {
				if typ.Kind == SemanticEnum {
					if typ.Identity == nil || typ.Name != "Mode" {
						t.Fatal(typ)
					}
					for _, m := range typ.Members {
						if m.Kind != SemanticEnumMember || m.Identity == nil || m.EnumTag == "" || m.Declaration.File == "" {
							t.Fatal(m)
						}
						found++
					}
				}
			}
		}
		if found != 2 {
			t.Fatal("enum projection lost members")
		}
		f := coreFunctionNamed(t, p, "Select")
		generated, err := gobackend.Generate(p)
		if err != nil {
			t.Fatal(err)
		}
		if previous != nil && (!bytes.Equal(previous, generated) || !coreir.TypeEqual(previousType, f.ReturnType)) {
			t.Fatal("declaration reordering changed nominal tags or generated Go")
		}
		previous = generated
		previousType = f.ReturnType
		prepared, err := coreeval.PrepareProgram(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, pair := range [][2]string{{"idle", "busy"}, {"busy", "idle"}} {
			args := []coreeval.Value{{Type: f.Parameters[0].Type, String: pair[0]}}
			got, err := prepared.Evaluate(f.Identity, args)
			if err != nil || !got.OK || got.Value.String != pair[1] {
				t.Fatal(got, err)
			}
			got.Value.Type.Enum.Members[0].Tag = "corrupt"
			again, err := prepared.Evaluate(f.Identity, args)
			if err != nil || again.Value.String != pair[1] {
				t.Fatal("prepared snapshot aliased outcome", err)
			}
		}
	}
}

func TestV1130EnumLazyOrder(t *testing.T) {
	source := `public Enum Mode{Idle="idle";Busy="busy";} public Class Choices{
 public Mode Observe(Mode value)=>value;
 public string Arm(string value)=>value;
 public Mode Choose(bool flag)=>flag ? Mode.Idle : Mode.Busy;
 public string Select(bool flag)=>match(Observe(Choose(flag))){Mode.Idle=>Arm("idle"),Mode.Busy=>Arm("busy")};
 }`
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1130, source, []string{"Select"})
	generated, err := gobackend.Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(`package pipelanggenerated
 import "testing"
 func TestValues(t *testing.T){if PipeLangSelect(true)!="idle"||PipeLangSelect(false)!="busy"{t.Fatal("wrong enum arm")}}`))
	observed := string(generated)
	for name, probe := range map[string]string{"Choose": `enumTrace=append(enumTrace,"choose")`, "Observe": `enumTrace=append(enumTrace,"observe:"+string(p0))`, "Arm": `enumTrace=append(enumTrace,"arm:"+p0)`} {
		marker := regexp.MustCompile(`func PipeLang` + name + `\([^\n]*\) [^\n{]+ \{`).FindString(observed)
		if marker == "" || strings.Count(observed, marker) != 1 {
			t.Fatal("missing trace marker", name)
		}
		observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
	}
	compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(`package pipelanggenerated
 import("testing";"reflect")
 var enumTrace []string
 func TestTrace(t *testing.T){for _,flag:=range []bool{false,true}{enumTrace=nil;want:="busy";if flag{want="idle"};if PipeLangSelect(flag)!=want||!reflect.DeepEqual(enumTrace,[]string{"choose","observe:"+want,"arm:"+want}){t.Fatal(enumTrace)}}}`))
}

func TestV1130EnumCompositionBounds(t *testing.T) {
	head := `public Enum Mode{Idle="idle";Busy="busy";} public Class Choices{public Mode Select(Mode value,bool flag)=>`
	for _, kind := range []string{"conditional", "match"} {
		for depth := 1; depth <= 4; depth++ {
			expr := "value"
			for i := 0; i < depth; i++ {
				if kind == "conditional" {
					expr = "flag ? (" + expr + ") : Mode.Busy"
				} else {
					expr = "match(value){Mode.Idle=>" + expr + ",Mode.Busy=>Mode.Busy}"
				}
			}
			t.Run(fmt.Sprintf("%s/%d", kind, depth), func(t *testing.T) {
				source := head + expr + ";}"
				a := enumAnalysis(source)
				if depth == 4 {
					if a.Error() == nil {
						t.Fatal("unbounded enum composition")
					}
					return
				}
				_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1130, source, []string{"Select"})
				if err := coreir.ValidateProgram(p); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestV1130EnumInheritance(t *testing.T) {
	for _, source := range []string{arrowSelectorValueArmsSource, v990Wrap(t, terminalLeafSelectorValueArmsSeed), straightLineSelectorValueArmsSource, terminalCombinedSelectorArmsSource, terminalInnerSelectorArmsSource, terminalSelectorValueArmsSource, terminalBooleanSelectorTestsSource, conditionalBooleanSelectorsSource, straightLineBooleanSelectorInitializersSource, terminalBooleanSelectorInitializersSource, arrowBooleanSelectorsSource, v990Wrap(t, conditionalBooleanSelectorsSource), v940WrapBody(t, depthThreeStraightLineInitializersSource, terminalTrees(3)[25], "a"), depthThreeStraightLineInitializersSource, depthThreeArrowMethodsSource, depthThreeTerminalLeafReturnsSource, depthThreeStraightLineReturnsSource, nestedArrowMethodsSource, nestedTerminalInitializersSource, nestedStraightLineInitializersSource, nestedTerminalLeafReturnsSource, nestedStraightLineReturnsSource, terminalLeafConditionalReturnsSource, conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource, `public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV1120, PipeLangLanguageContractV1130} {
			a, p := conditionalLocalTreeProgramVersion(t, contract, source, []string{"Select"})
			h, err := LowerSemanticMethodToHIR(a, semanticMethodNamed(t, a, "Select").Identity)
			if err != nil {
				t.Fatal(err)
			}
			projection, err := BuildSemanticProjection(a)
			if err != nil {
				t.Fatal(err)
			}
			g, err := gobackend.Generate(p)
			if err != nil {
				t.Fatal(err)
			}
			h.LanguageContract = coreir.LanguageContractV1120
			p.LanguageContract = coreir.LanguageContractV1120
			projection.LanguageContract = PipeLangLanguageContractV1120
			artifacts := [][]byte{g}
			for _, v := range []any{h, p, projection} {
				b, err := json.Marshal(v)
				if err != nil {
					t.Fatal(err)
				}
				artifacts = append(artifacts, b)
			}
			if baseline == nil {
				baseline = artifacts
			} else if !reflect.DeepEqual(baseline, artifacts) {
				t.Fatal("inherited HIR/Core/semantic/Go changed")
			}
		}
	}
}

func TestV1130EnumModules(t *testing.T) {
	root := testModule("app.root", "root.pipe", `public Class Choices{public Mode Select(Mode value)=>match(value){Mode.Idle=>Mode.Busy,Mode.Busy=>Mode.Idle};}`, ImportDecl{Kind: ImportSymbol, Module: "lib.shared", Symbol: "Mode", Span: Span{File: "root.pipe"}})
	shared := testModule("lib.shared", "shared.pipe", `public Enum Mode{Idle="idle";Busy="busy";}`)
	var previous []byte
	for _, modules := range [][]ModuleInput{{root, shared}, {shared, root}} {
		input := semanticTestModuleSet("app.root", modules, map[ModuleID][]ModuleID{"app.root": {"lib.shared"}})
		input.LanguageContract = PipeLangLanguageContractV1130
		a := AnalyzeSemanticModuleSet(input)
		if err := a.Error(); err != nil {
			t.Fatal(err)
		}
		h, err := LowerSemanticMethodToHIR(a, semanticMethodNamed(t, a, "Select").Identity)
		if err != nil {
			t.Fatal(err)
		}
		p, err := LowerHIRToCore(h)
		if err != nil {
			t.Fatal(err)
		}
		f := coreFunctionNamed(t, p, "Select")
		if f.ReturnType.Identity == nil || !strings.Contains(f.ReturnType.Identity.Path, "lib.shared") {
			t.Fatal("enum lost defining module identity")
		}
		generated, err := gobackend.Generate(p)
		if err != nil {
			t.Fatal(err)
		}
		if previous != nil && !bytes.Equal(previous, generated) {
			t.Fatal("module enumeration changed Go")
		}
		previous = generated
	}
}

func TestV1130EnumAdditionalRefusals(t *testing.T) {
	head := `public Enum Mode{Idle="idle";Busy="busy";}`
	for _, tail := range []string{
		`public Interface I{public Mode Value;}`,
		`public Class Choices{public List<Mode> Values;public bool Select()=>true;}`,
		`public Record Row{public Mode Value;}`,
		`public Class Choices{public Mode Select(Mode value,List<string> other)=>value;}`,
		`public Class Choices{public List<string> Select(Mode value,List<string> other)=>other;}`,
		`public Class Choices{public Mode Select(Mode value){Mode selected=value;selected=Mode.Idle;return selected;}}`,
		`public Class Choices{public Mode Select(Mode value){Mode selected=value;Mode selected=Mode.Idle;return selected;}}`,
		`public Class Choices{public Mode Select(Mode Mode)=>Mode.Idle;}`,
		`public Class Choices{public Mode Select(Mode value)=>true ? value : "idle";}`,
	} {
		t.Run(tail, func(t *testing.T) {
			if err := enumAnalysis(head + tail).Error(); err == nil {
				t.Fatal("unsupported enum shape accepted")
			}
		})
	}
	// Independent Core depth refusal cannot rely on the source checker.
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1130, head+`public Class Choices{public Mode Select(Mode value,bool flag)=>flag ? (flag ? (flag ? value : Mode.Busy) : Mode.Busy) : Mode.Busy;}`, []string{"Select"})
	f := &p.Functions[0]
	body := f.Body
	f.Body = coreir.Expr{Kind: coreir.ExprConditional, Type: f.ReturnType, Conditional: &coreir.Conditional{Condition: body.Conditional.Condition, WhenTrue: &body, WhenFalse: body.Conditional.WhenFalse}}
	if err := coreir.ValidateProgram(p); err == nil {
		t.Fatal("forged depth-four Core accepted")
	}
}

func TestV1130EnumTerminalScopes(t *testing.T) {
	source := `public Enum Mode{Idle="idle";Busy="busy";} public Class Choices{
 public Mode Echo(Mode value)=>value;
 public string Select(Mode value,bool flag,string raw){if(flag){Mode chosen=Echo(value);string text=trim(raw);return match(chosen){Mode.Idle=>text,Mode.Busy=>"busy"};}else{Mode chosen=Mode.Busy;return match(chosen){Mode.Idle=>"unreachable",Mode.Busy=>raw};}}
 }`
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1130, source, []string{"Select"})
	f := coreFunctionNamed(t, p, "Select")
	prepared, err := coreeval.PrepareProgram(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{"idle", "busy"} {
		for _, flag := range []bool{true, false} {
			want := " raw "
			if flag {
				want = "raw"
				if tag == "busy" {
					want = "busy"
				}
			}
			got, err := prepared.Evaluate(f.Identity, []coreeval.Value{{Type: f.Parameters[0].Type, String: tag}, {Type: f.Parameters[1].Type, Bool: flag}, {Type: f.Parameters[2].Type, String: " raw "}})
			if err != nil || !got.OK || got.Value.String != want {
				t.Fatal(got, err)
			}
		}
	}
	generated, err := gobackend.Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(`package pipelanggenerated
 import "testing"
 func TestBranches(t *testing.T){if PipeLangSelect("idle",true," raw ")!="raw"||PipeLangSelect("busy",true," raw ")!="busy"||PipeLangSelect("idle",false," raw ")!=" raw "{t.Fatal("branch scope")}}`))
}

func TestV1130EnumInheritedTypes(t *testing.T) {
	for i := 0; i < 3; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			arms := i + 1
			testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV1130, true, arms&1 != 0, arms&2 != 0)
		})
	}
}
func TestV1130EnumInheritedCarriers(t *testing.T) {
	for i := 0; i < 3; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			arms := i + 1
			testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV1130, true, arms&1 != 0, arms&2 != 0)
		})
	}
}

func TestV1130EnumForgedTerminalPlacement(t *testing.T) {
	source := `public Enum Mode{Idle="idle";Busy="busy";} public Class Choices{public Mode Select(Mode value,bool flag)=>flag ? (flag ? value : Mode.Busy) : Mode.Idle;}`
	_, base := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1130, source, []string{"Select"})
	for _, mutation := range []string{"hidden terminal", "nil condition", "nil true", "nil false"} {
		t.Run(mutation, func(t *testing.T) {
			raw, _ := json.Marshal(base)
			var p coreir.Program
			if err := json.Unmarshal(raw, &p); err != nil {
				t.Fatal(err)
			}
			n := p.Functions[0].Body.Conditional.WhenTrue.Conditional
			switch mutation {
			case "hidden terminal":
				n.TerminalStatement = true
			case "nil condition":
				n.Condition = nil
			case "nil true":
				n.WhenTrue = nil
			case "nil false":
				n.WhenFalse = nil
			}
			if err := coreir.ValidateProgram(p); err == nil {
				t.Fatal("malformed conditional accepted")
			}
			if _, err := gobackend.Generate(p); err == nil {
				t.Fatal("malformed conditional generated")
			}
			if _, err := coreeval.PrepareProgram(p); err == nil {
				t.Fatal("malformed conditional prepared")
			}
		})
	}
}
