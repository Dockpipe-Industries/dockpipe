package pipelang

import (
	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/gobackend"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func TestV1150AssignmentValueComposition(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"record", `mutable Row x=Make("old"); Row old=x; x=Make("new"); return old.Name;`, "old"},
		{"enum", `mutable Mode x=Mode.Idle; Mode old=x; x=Mode.Busy; return match(old){Mode.Idle=>"idle",Mode.Busy=>"busy"};`, "idle"},
		{"optional", `mutable Optional<string> x=some("old"); Optional<string> old=x; x=none<string>(); return value_or(old,"missing");`, "old"},
		{"result", `mutable Result<string,string> x=ok<string,string>("old"); Result<string,string> old=x; x=err<string,string>("bad"); return success_or(old,"missing");`, "old"},
		{"list", `mutable List<Row> x=list(Make("old")); List<Row> old=x; Row next=Make("new"); x=append(x,next); return count(old)==1 ? "old" : "changed";`, "old"},
		{"delayed_record", `Row x; {x=Make("late");} return x.Name;`, "late"},
		{"unused_types", `Row row; Mode mode; Optional<string> opt; List<Row> rows; Result<string,string> result; Result<List<Row>,string> snapshot; return "unused";`, "unused"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			source := `public Record Row{public string Name;}public Enum Mode{Idle="idle";Busy="busy";}public Class Choices{public Row Make(string name)=>new Row { Name = name };public string Select(){` + c.body + `}}`
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1150, source, []string{"Select"})
			f := coreFunctionNamed(t, p, "Select")
			got, err := coreeval.EvaluateProgram(p, f.Identity, nil)
			if err != nil || !got.OK || got.Value.String != c.want {
				t.Fatal(got, err)
			}
			g, err := gobackend.Generate(p)
			if err != nil {
				t.Fatal(err)
			}
			compileAndRunGeneratedGoFiles(t, g, []byte(fmt.Sprintf("package pipelanggenerated\nimport \"testing\"\nfunc TestValue(t *testing.T){if PipeLangSelect()!=%q{t.Fatal(\"copy or delayed value\")}}", c.want)))
		})
	}
}

func TestV1150AssignmentOrder(t *testing.T) {
	source := `public Class Choices{
 public string Observe(string value)=>value;
 public string Combine(string left,string right)=>left+right;
 public string Select(bool flag){mutable string x=Observe("A");string old=x;x=Combine(Observe(x+"B"),Observe(x+"C"));if(flag){x=Observe(x+"T");}else{x=Observe(x+"F");}return old+x;}}`
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1150, source, []string{"Select"})
	f := coreFunctionNamed(t, p, "Select")
	for _, flag := range []bool{false, true} {
		want := "AABACF"
		if flag {
			want = "AABACT"
		}
		got, err := coreeval.EvaluateProgram(p, f.Identity, []coreeval.Value{{Type: f.Parameters[0].Type, Bool: flag}})
		if err != nil || !got.OK || got.Value.String != want {
			t.Fatal(got, err)
		}
	}
	g, err := gobackend.Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	text := string(g)
	marker := regexp.MustCompile(`func PipeLangObserve\([^\n]*\) [^\n{]+ \{`).FindString(text)
	if marker == "" {
		t.Fatal("missing observer")
	}
	text = strings.Replace(text, marker, marker+"\ntrace=append(trace,p0)", 1)
	compileAndRunGeneratedGoFiles(t, []byte(text), []byte(`package pipelanggenerated
 import("testing";"reflect")
 var trace []string
 func TestOrder(t *testing.T){for _,flag:=range []bool{false,true}{trace=nil;want:="AABACF";last:="ABACF";if flag{want="AABACT";last="ABACT"};if got:=PipeLangSelect(flag);got!=want{t.Fatal(got)};if !reflect.DeepEqual(trace,[]string{"A","AB","AC",last}){t.Fatal(trace)}}}`))
}

func TestV1150AssignmentHIR(t *testing.T) {
	source := `public Class Choices{public int Select(){mutable int x; x=1; return x;}}`
	a, _ := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1150, source, []string{"Select"})
	h, err := LowerSemanticMethodToHIR(a, semanticMethodNamed(t, a, "Select").Identity)
	if err != nil {
		t.Fatal(err)
	}
	b := h.Functions[0].Body.Block
	if !b.Statements[0].Mutable || b.Statements[0].Value != nil || b.Statements[1].Target == nil || b.Statements[1].Target.Name != "x" || b.Statements[1].Span.File == "" {
		t.Fatal("missing HIR assignment identity")
	}
	encoded, _ := json.Marshal(h)
	if err := json.Unmarshal(encoded, &h); err != nil {
		t.Fatal(err)
	}
	if _, err := LowerHIRToCore(h); err != nil {
		t.Fatal("valid serialized HIR refused", err)
	}
	for _, kind := range []string{"name", "owner", "position", "binding", "extra"} {
		t.Run(kind, func(t *testing.T) {
			if err := json.Unmarshal(encoded, &h); err != nil {
				t.Fatal(err)
			}
			b := h.Functions[0].Body.Block
			switch kind {
			case "name":
				b.Statements[1].Target.Name = "other"
			case "owner":
				b.Statements[1].Target.Function.Path = "other"
			case "position":
				b.Statements[1].Target.Position = 9
			case "binding":
				b.Statements[1].Target.Kind = "parameter"
			case "extra":
				b.Statements[2].Mutable = true
			}
			if _, err := LowerHIRToCore(h); err == nil {
				t.Fatal("forged HIR assignment accepted")
			}
		})
	}
	projection, err := BuildSemanticProjection(a)
	if err != nil || projection.LanguageContract != PipeLangLanguageContractV1150 {
		t.Fatal("semantic projection", err)
	}
}

func TestV1150AssignmentAfterPropagation(t *testing.T) {
	source := `public Class Choices{
 public Optional<string> Normalize(Optional<string> value){string text=propagate(value);return some(text);}
 public Optional<string> Read(Optional<string> input){mutable Optional<string> value=Normalize(input);value=Normalize(value);return value;}}`
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1150, source, []string{"Read"})
	f := coreFunctionNamed(t, p, "Read")
	typ := f.Parameters[0].Type
	for _, present := range []bool{false, true} {
		input := coreeval.Value{Type: typ, Optional: &coreeval.OptionalValue{Present: present}}
		if present {
			input.Optional.Value = &coreeval.Value{Type: typ.Optional.Value, String: "kept"}
		}
		got, err := coreeval.EvaluateProgram(p, f.Identity, []coreeval.Value{input})
		if err != nil || !got.OK || got.Value.Optional == nil || got.Value.Optional.Present != present {
			t.Fatal(got, err)
		}
		if present && got.Value.Optional.Value.String != "kept" {
			t.Fatal("lost propagated value")
		}
	}
	g, err := gobackend.Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, g, []byte(`package pipelanggenerated
 import "testing"
 func TestRead(t *testing.T){if got,ok:=PipeLangRead(pipelangSomeValue("kept")).(pipelangOptionalSome[string]);!ok||got.value!="kept"{t.Fatal("present")};if _,ok:=PipeLangRead(pipelangNoneValue[string]()).(pipelangOptionalNone[string]);!ok{t.Fatal("absent")}}`))
}
