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

func blockAnalysis(source string, version LanguageContract) *Analysis {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "blocks.pipe", source)}, nil)
	input.LanguageContract = version
	return AnalyzeSemanticModuleSet(input)
}

func TestV1140Blocks(t *testing.T) {
	cases := []struct {
		name, body string
		want       [2]int64
	}{
		{"direct", `return 7;`, [2]int64{7, 7}},
		{"join", `if(flag){ int x=1; } else { int x=2; } int x=9; return x;`, [2]int64{9, 9}},
		{"early", `if(flag){return 1;} return 2;`, [2]int64{2, 1}},
		{"nested", `{if(flag){{return 3;}}} return 4;`, [2]int64{4, 3}},
		{"sequential", `if(flag){int x=7;} if(!flag){return 5;} return 6;`, [2]int64{5, 6}},
		{"empty", `{} if(flag){} else{} return 8;`, [2]int64{8, 8}},
		{"scope_reuse", `{int x=1;} int x=10; {int y=x;} return x;`, [2]int64{10, 10}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1140, `public Class Choices{public int Select(bool flag){`+c.body+`}}`, []string{"Select"})
			f := coreFunctionNamed(t, p, "Select")
			if f.Body.Kind != coreir.ExprBlock {
				t.Fatal("lost structured block")
			}
			prepared, err := coreeval.PrepareProgram(p)
			if err != nil {
				t.Fatal(err)
			}
			for i, flag := range []bool{false, true} {
				got, err := prepared.Evaluate(f.Identity, []coreeval.Value{{Type: f.Parameters[0].Type, Bool: flag}})
				if err != nil || !got.OK || got.Value.Int != c.want[i] {
					t.Fatal(got, err, c.want[i])
				}
			}
			generated, err := gobackend.Generate(p)
			if err != nil {
				t.Fatal(err)
			}
			again, err := gobackend.Generate(p)
			if err != nil || !bytes.Equal(generated, again) {
				t.Fatal("nondeterministic emission", err)
			}
			compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package pipelanggenerated
 import "testing"
 func TestValues(t *testing.T){if PipeLangSelect(false)!=%d || PipeLangSelect(true)!=%d{t.Fatal("wrong joined return")}}`, c.want[0], c.want[1])))
		})
	}
}

func TestV1140BlockRefusals(t *testing.T) {
	cases := []struct{ body, diagnostic string }{
		{`int x=1;`, "without returning"},
		{`if(flag){return 1;}`, "without returning"},
		{`return 1; return 2;`, "unreachable"},
		{`if(flag){return 1;}else{return 2;} int x=3;`, "unreachable"},
		{`{return 1;} return 2;`, "unreachable"},
		{`if(flag){int x=1;} return x;`, "x"},
		{`{int x=1;} return x;`, "x"},
		{`int x=x; return x;`, "x"},
		{`int x=1; {int x=2;} return x;`, "shadows"},
		{`bool flag=true; return 1;`, "shadows"},
		{`int x; return 1;`, "="},
		{`int x=1; x=2; return x;`, ""},
		{`if(1){return 1;}return 2;`, "bool"},
		{`if(flag){return true;}return 2;`, "type"},
		{`if(flag){int x=false;}return 2;`, "type"},
		{`while(flag){return 1;}return 2;`, ""},
		{`return;`, ""},
	}
	for _, c := range cases {
		t.Run(c.body, func(t *testing.T) {
			a := blockAnalysis(`public Class Choices{public int Select(bool flag){`+c.body+`}}`, PipeLangLanguageContractV1140)
			if a.Error() == nil || !strings.Contains(a.Error().Error(), c.diagnostic) {
				t.Fatalf("wanted %q refusal, got %v", c.diagnostic, a.Error())
			}
			if _, ok := AsDiagnostics(a.Error()); !ok {
				t.Fatal("unstructured error")
			}
		})
	}
	for _, v := range []LanguageContract{LegacyLanguageContract, PipeLangLanguageContractV390, PipeLangLanguageContractV1120, PipeLangLanguageContractV1130} {
		a := blockAnalysis(`public Class Choices{public int Select(bool flag){if(flag){return 1;}return 2;}}`, v)
		if a.Error() == nil {
			t.Fatal("old version accepted blocks", v)
		}
	}
}

func TestV1140BlockCallsAndOrder(t *testing.T) {
	source := `public Class Choices{
 public bool Condition(bool value)=>value;
 public int Observe(int value)=>value;
 public int Select(bool flag){int first=Observe(1); if(Condition(flag)){int branch=Observe(2); return Observe(3);} {int joined=Observe(4);} return Observe(5);}
 }`
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1140, source, []string{"Select"})
	generated, err := gobackend.Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	observed := string(generated)
	for name, probe := range map[string]string{"Condition": `trace=append(trace,9)`, "Observe": `trace=append(trace,p0)`} {
		marker := regexp.MustCompile(`func PipeLang` + name + `\([^\n]*\) [^\n{]+ \{`).FindString(observed)
		if marker == "" {
			t.Fatal("missing trace marker", name)
		}
		observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
	}
	compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(`package pipelanggenerated
 import("testing";"reflect")
 var trace []int64
 func TestTrace(t *testing.T){for _,flag:=range []bool{false,true}{trace=nil;got:=PipeLangSelect(flag);want:=int64(5);order:=[]int64{1,9,4,5};if flag{want=3;order=[]int64{1,9,2,3}};if got!=want||!reflect.DeepEqual(trace,order){t.Fatal(got,trace)}}}`))
	f := coreFunctionNamed(t, p, "Select")
	for _, flag := range []bool{false, true} {
		want := int64(5)
		if flag {
			want = 3
		}
		o, e := coreeval.EvaluateProgram(p, f.Identity, []coreeval.Value{{Type: f.Parameters[0].Type, Bool: flag}})
		if e != nil || !o.OK || o.Value.Int != want {
			t.Fatal(o, e)
		}
	}
}

func TestV1140BlockCoreRefusals(t *testing.T) {
	_, base := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1140, `public Class Choices{public int Select(bool flag){if(flag){int inner=1;}int x=2;return x;}}`, []string{"Select"})
	raw, _ := json.Marshal(base)
	for _, name := range []string{"missing_return", "unreachable", "bad_condition", "escaping", "self_read", "wrong_local_type", "wrong_return_type", "bad_position", "shadow", "unknown", "missing_body", "nested_value", "old_version", "cyclic_block", "hidden_call"} {
		t.Run(name, func(t *testing.T) {
			var p coreir.Program
			if err := json.Unmarshal(raw, &p); err != nil {
				t.Fatal(err)
			}
			f := &p.Functions[len(p.Functions)-1]
			b := f.Body.Block
			switch name {
			case "missing_return":
				b.Statements = b.Statements[:2]
			case "unreachable":
				b.Statements = append(b.Statements, b.Statements[2])
			case "bad_condition":
				b.Statements[0].Value = b.Statements[1].Value
			case "escaping":
				b.Statements = b.Statements[:1]
				pos := 1
				b.Statements = append(b.Statements, coreir.Statement{Kind: "return", Value: &coreir.Expr{Kind: coreir.ExprReference, Type: f.ReturnType, Parameter: &pos}})
			case "self_read":
				b.Statements[1].Value = b.Statements[2].Value
			case "wrong_local_type":
				b.Statements[1].Local.Type = f.Parameters[0].Type
			case "wrong_return_type":
				b.Statements[2].Value = b.Statements[0].Value
			case "bad_position":
				b.Statements[1].Local.Position = 9
			case "shadow":
				b.Statements[1].Local.Name = "flag"
			case "unknown":
				b.Statements[1].Kind = "jump"
			case "missing_body":
				b.Statements[0].Then = nil
			case "nested_value":
				b.Statements[2].Value = &coreir.Expr{Kind: coreir.ExprBlock, Type: f.ReturnType, Block: &coreir.Block{Statements: []coreir.Statement{{Kind: "return", Value: b.Statements[1].Value}}}}
			case "old_version":
				p.LanguageContract = coreir.LanguageContractV1130
			case "cyclic_block":
				b.Statements[0].Then = b
			case "hidden_call":
				b.Statements[0].Then.Statements[0].Value = &coreir.Expr{Kind: coreir.ExprCall, Type: f.ReturnType, Call: &coreir.Call{Target: coreir.SemanticIdentity{PackageID: "forged", Path: "missing"}}}
			}
			if err := coreir.ValidateProgram(p); err == nil {
				t.Fatal("forged Core accepted")
			}
		})
	}
	// Prepared evaluation owns a full snapshot, including the new block graph.
	prepared, err := coreeval.PrepareProgram(base)
	if err != nil {
		t.Fatal(err)
	}
	f := base.Functions[len(base.Functions)-1]
	f.Body.Block.Statements[1].Value.Literal.Int = 99
	out, err := prepared.Evaluate(f.Identity, []coreeval.Value{{Type: f.Parameters[0].Type, Bool: false}})
	if err != nil || out.Value.Int != 2 {
		t.Fatal("snapshot alias", out, err)
	}
	if reflect.DeepEqual(base, coreir.Program{}) {
		t.Fatal("empty fixture")
	}
}

func TestV1140BlockInheritedValues(t *testing.T) {
	for i := 0; i < 3; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			arms := i + 1
			testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV1140, true, arms&1 != 0, arms&2 != 0)
			testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV1140, true, arms&1 != 0, arms&2 != 0)
		})
	}
}

func TestV1140BlockEnumAndHIR(t *testing.T) {
	source := `public Enum Mode{Idle="idle";Busy="busy";} public Class Choices{public Mode Select(bool flag){if(flag){{return Mode.Busy;}}Mode next=Mode.Idle;return next;}}`
	a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1140, source, []string{"Select"})
	f := coreFunctionNamed(t, p, "Select")
	for _, flag := range []bool{false, true} {
		want := "idle"
		if flag {
			want = "busy"
		}
		o, err := coreeval.EvaluateProgram(p, f.Identity, []coreeval.Value{{Type: f.Parameters[0].Type, Bool: flag}})
		if err != nil || !o.OK || o.Value.String != want {
			t.Fatal(o, err)
		}
	}
	generated, err := gobackend.Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(`package pipelanggenerated
 import "testing"
 func TestValues(t *testing.T){if string(PipeLangSelect(false))!="idle"||string(PipeLangSelect(true))!="busy"{t.Fatal("wrong enum block")}}`))
	h, err := LowerSemanticMethodToHIR(a, semanticMethodNamed(t, a, "Select").Identity)
	if err != nil {
		t.Fatal(err)
	}
	b := h.Functions[len(h.Functions)-1].Body.Block
	if len(b.Statements) != 3 || b.Statements[1].Local.Span.File == "" || b.Statements[1].Span.File == "" || b.Statements[1].Local.Binding.Name != "next" {
		t.Fatal("lost HIR local identity/span")
	}
	b.Statements[1].Local.Binding.Position = 90
	if _, err := LowerHIRToCore(h); err == nil {
		t.Fatal("forged HIR local accepted")
	}
}
